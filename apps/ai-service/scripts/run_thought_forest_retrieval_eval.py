"""Evaluate unpublished Thought Forest retrieval without changing publication state."""

from __future__ import annotations

import argparse
import asyncio
import json
import sys
from dataclasses import dataclass
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parents[1]
REPO_ROOT = PROJECT_ROOT.parents[1]
if str(PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(PROJECT_ROOT))

try:
    from dotenv import load_dotenv
except ModuleNotFoundError:
    load_dotenv = None

if load_dotenv:
    for env_path in [PROJECT_ROOT / ".env", REPO_ROOT / ".env"]:
        if env_path.exists():
            load_dotenv(env_path, override=False)
            break

from src.rag.knowledge_library import KnowledgeLibrary, SearchResult  # noqa: E402

DEFAULT_CASES = REPO_ROOT / "docs/knowledges/eval/thought-forest-unpublished.jsonl"


@dataclass(frozen=True)
class EvalCase:
    query: str
    expected_problem_slug: str
    expected_claim_kinds: tuple[str, ...]


def load_cases(path: Path) -> list[EvalCase]:
    cases: list[EvalCase] = []
    for line_number, raw in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        line = raw.strip()
        if not line:
            continue
        payload = json.loads(line)
        query = str(payload.get("query") or "").strip()
        problem = str(payload.get("expected_problem_slug") or "").strip()
        claim_kinds = tuple(str(item) for item in payload.get("expected_claim_kinds") or [])
        if not query or not problem or not claim_kinds:
            raise ValueError(f"invalid eval case at {path}:{line_number}")
        cases.append(EvalCase(query, problem, claim_kinds))
    if not cases:
        raise ValueError(f"no eval cases found in {path}")
    return cases


def result_claim_kind(result: SearchResult) -> str | None:
    candidate = dict(result.unit_metadata.get("claim_candidate") or {})
    value = candidate.get("claim_kind")
    return str(value) if value else None


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cases", type=Path, default=DEFAULT_CASES)
    parser.add_argument("--top-k", type=int, default=5)
    parser.add_argument("--min-topk-recall", type=float, default=0.0)
    parser.add_argument("--min-claim-hit-rate", type=float, default=0.0)
    return parser.parse_args()


async def main() -> int:
    args = parse_args()
    if args.top_k < 1:
        raise ValueError("--top-k must be >= 1")
    cases = load_cases(args.cases.resolve())
    library = KnowledgeLibrary()
    await library.initialize()
    try:
        top1_hits = 0
        topk_hits = 0
        claim_hits = 0
        for case in cases:
            results = await library.search(
                query=case.query,
                top_k=args.top_k,
                source_type="thought_forest_note",
                include_unpublished=True,
            )
            expected_results = [
                result for result in results if result.problem_slug == case.expected_problem_slug
            ]
            top1_hit = bool(results and results[0].problem_slug == case.expected_problem_slug)
            topk_hit = bool(expected_results)
            claim_hit = any(
                result_claim_kind(result) in case.expected_claim_kinds
                for result in expected_results
            )
            top1_hits += int(top1_hit)
            topk_hits += int(topk_hit)
            claim_hits += int(claim_hit)
            rendered = [
                f"{result.problem_slug}/{result_claim_kind(result) or '-'}:{result.title}"
                for result in results[:3]
            ]
            print(
                json.dumps(
                    {
                        "query": case.query,
                        "top1_hit": top1_hit,
                        "topk_hit": topk_hit,
                        "claim_hit": claim_hit,
                        "top3": rendered,
                    },
                    ensure_ascii=False,
                )
            )
    finally:
        await library.close()

    total = len(cases)
    top1_accuracy = top1_hits / total
    topk_recall = topk_hits / total
    claim_hit_rate = claim_hits / total
    summary = {
        "cases": total,
        "top1_accuracy": round(top1_accuracy, 4),
        "topk_problem_recall": round(topk_recall, 4),
        "claim_kind_hit_rate": round(claim_hit_rate, 4),
        "publication_state": "unpublished-only-eval",
    }
    print(json.dumps(summary, ensure_ascii=False))
    if topk_recall < args.min_topk_recall or claim_hit_rate < args.min_claim_hit_rate:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(asyncio.run(main()))
