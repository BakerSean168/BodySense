"""Evaluate the default published Thought Forest retrieval/citation/grounding path."""

from __future__ import annotations

import argparse
import asyncio
import hashlib
import json
import sys
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Literal

PROJECT_ROOT = Path(__file__).resolve().parents[1]
# Host layout is <repo>/apps/ai-service; the production image uses /app.
# `.parent.parent` is total (unlike parents[1]) and keeps explicit --cases
# usable in both layouts even when the image intentionally does not contain docs/.
REPO_ROOT = PROJECT_ROOT.parent.parent
if str(PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(PROJECT_ROOT))

from src.rag.knowledge_library import KnowledgeLibrary, SearchResult  # noqa: E402
from src.services.agent.reply_fallback import build_citation_payload  # noqa: E402

SCHEMA_VERSION = "bodysense.published-knowledge-eval.v1"
EVALUATOR_REVISION = "published-knowledge-eval-v2"
THRESHOLD_SCHEMA_VERSION = "bodysense.published-knowledge-qualification-thresholds.v1"
DEFAULT_CASES = REPO_ROOT / "docs/knowledges/eval/published-knowledge-pilot.jsonl"


@dataclass(frozen=True)
class PublishedEvalCase:
    case_id: str
    query: str
    expect: Literal["hit", "no_result"]
    expected_unit_key: str = ""
    expected_claim_id: str = ""
    expected_evidence_terms: tuple[str, ...] = ()


@dataclass(frozen=True)
class PublishedEvalObservation:
    case_id: str
    query: str
    retrieval_status: str
    citation_status: str
    grounding_status: str
    identity_status: str
    provenance_status: str
    passed: bool
    top_similarity: float | None = None
    returned_unit_key: str = ""
    reasons: tuple[str, ...] = ()


@dataclass(frozen=True)
class PublishedQualificationThresholds:
    dataset_sha256: str
    expected_cases: int
    min_pass_rate: float
    min_positive_hits: int
    min_negative_rejections: int
    min_citation_valid: int
    min_grounding_supported: int
    require_publication_identity: bool


def load_thresholds(path: Path, cases_path: Path) -> PublishedQualificationThresholds:
    payload = json.loads(path.read_text(encoding="utf-8"))
    if payload.get("schema_version") != THRESHOLD_SCHEMA_VERSION:
        raise ValueError(f"unsupported qualification thresholds schema at {path}")
    expected_hash = str(payload.get("dataset_sha256") or "").strip().lower()
    actual_hash = hashlib.sha256(cases_path.read_bytes()).hexdigest()
    if expected_hash != actual_hash:
        raise ValueError(
            f"qualification dataset hash mismatch: expected={expected_hash} actual={actual_hash}"
        )
    thresholds = PublishedQualificationThresholds(
        dataset_sha256=expected_hash,
        expected_cases=int(payload.get("expected_cases") or 0),
        min_pass_rate=float(payload.get("min_pass_rate") or 0.0),
        min_positive_hits=int(payload.get("min_positive_hits") or 0),
        min_negative_rejections=int(payload.get("min_negative_rejections") or 0),
        min_citation_valid=int(payload.get("min_citation_valid") or 0),
        min_grounding_supported=int(payload.get("min_grounding_supported") or 0),
        require_publication_identity=bool(payload.get("require_publication_identity", True)),
    )
    if thresholds.expected_cases <= 0 or not (0.0 <= thresholds.min_pass_rate <= 1.0):
        raise ValueError(f"invalid qualification thresholds at {path}")
    return thresholds


def evaluate_thresholds(
    thresholds: PublishedQualificationThresholds,
    report: dict[str, object],
) -> tuple[bool, tuple[str, ...]]:
    summary = report.get("summary")
    publication = report.get("publication")
    if not isinstance(summary, dict) or not isinstance(publication, dict):
        return False, ("qualification_report_shape_invalid",)
    reasons: list[str] = []
    checks = (
        ("cases", thresholds.expected_cases, "exact"),
        ("pass_rate", thresholds.min_pass_rate, "min"),
        ("positive_hits", thresholds.min_positive_hits, "min"),
        ("negative_rejections", thresholds.min_negative_rejections, "min"),
        ("citation_valid", thresholds.min_citation_valid, "min"),
        ("grounding_supported", thresholds.min_grounding_supported, "min"),
    )
    for key, threshold, mode in checks:
        value = summary.get(key)
        if not isinstance(value, (int, float)):
            reasons.append(f"{key}_missing")
        elif mode == "exact" and value != threshold:
            reasons.append(f"{key}_expected_{threshold}_got_{value}")
        elif mode == "min" and value < threshold:
            reasons.append(f"{key}_below_{threshold}_got_{value}")
    if thresholds.require_publication_identity:
        for key in ("publication_id", "publication_key", "publication_batch_key"):
            if not str(publication.get(key) or "").strip():
                reasons.append(f"{key}_missing")
        if publication.get("published_version") is None:
            reasons.append("published_version_missing")
    return not reasons, tuple(reasons)


def load_cases(path: Path) -> list[PublishedEvalCase]:
    cases: list[PublishedEvalCase] = []
    for line_number, raw in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        line = raw.strip()
        if not line:
            continue
        payload = json.loads(line)
        case_id = str(payload.get("case_id") or "").strip()
        query = str(payload.get("query") or "").strip()
        expect = str(payload.get("expect") or "").strip()
        if not case_id or not query or expect not in {"hit", "no_result"}:
            raise ValueError(f"invalid published eval case at {path}:{line_number}")
        expected_unit_key = str(payload.get("expected_unit_key") or "").strip()
        expected_claim_id = str(payload.get("expected_claim_id") or "").strip()
        terms = tuple(str(item).strip() for item in payload.get("expected_evidence_terms") or [])
        if expect == "hit" and (not expected_unit_key or not expected_claim_id or not terms):
            raise ValueError(f"hit case missing expected fields at {path}:{line_number}")
        cases.append(
            PublishedEvalCase(
                case_id=case_id,
                query=query,
                expect=expect,  # type: ignore[arg-type]
                expected_unit_key=expected_unit_key,
                expected_claim_id=expected_claim_id,
                expected_evidence_terms=terms,
            )
        )
    if not cases:
        raise ValueError(f"no published eval cases found in {path}")
    return cases


def _normalized_text(value: str) -> str:
    return "".join(value.lower().split())


def evaluate_case(
    case: PublishedEvalCase,
    results: list[SearchResult],
    *,
    expected_publication_key: str,
) -> PublishedEvalObservation:
    if case.expect == "no_result":
        if not results:
            return PublishedEvalObservation(
                case_id=case.case_id,
                query=case.query,
                retrieval_status="expected_empty",
                citation_status="not_applicable",
                grounding_status="not_applicable",
                identity_status="not_applicable",
                provenance_status="not_applicable",
                passed=True,
            )
        top = results[0]
        return PublishedEvalObservation(
            case_id=case.case_id,
            query=case.query,
            retrieval_status="unexpected_result",
            citation_status="invalid",
            grounding_status="not_applicable",
            identity_status=(
                "match" if top.publication_key == expected_publication_key else "mismatch"
            ),
            provenance_status="not_applicable",
            passed=False,
            top_similarity=top.similarity,
            returned_unit_key=top.unit_key,
            reasons=("negative_case_returned_published_knowledge",),
        )

    if not results:
        return PublishedEvalObservation(
            case_id=case.case_id,
            query=case.query,
            retrieval_status="miss",
            citation_status="not_applicable",
            grounding_status="not_applicable",
            identity_status="not_applicable",
            provenance_status="not_applicable",
            passed=False,
            reasons=("expected_published_unit_not_retrieved",),
        )

    top = results[0]
    citation = build_citation_payload(top)
    reasons: list[str] = []
    retrieval_status = "hit" if top.unit_key == case.expected_unit_key else "miss"
    if retrieval_status != "hit":
        reasons.append("wrong_top1_unit")

    identity_ok = bool(
        top.publication_id
        and top.published_version is not None
        and top.publication_key == expected_publication_key
        and top.lifecycle_status == "published"
        and citation.get("publication_id") == top.publication_id
        and citation.get("published_version") == top.published_version
        and citation.get("publication_key") == expected_publication_key
        and citation.get("unit_key") == case.expected_unit_key
        and citation.get("claim_id") == case.expected_claim_id
        and citation.get("claim_review_id")
    )
    if not identity_ok:
        reasons.append("publication_or_claim_identity_mismatch")

    locator = citation.get("source_locator")
    provenance_ok = bool(
        isinstance(locator, dict)
        and locator.get("locator_type") == "markdown_lines"
        and locator.get("git_commit")
        and locator.get("path")
        and int(locator.get("line_start") or 0) > 0
        and int(locator.get("line_end") or 0) >= int(locator.get("line_start") or 0)
    )
    if not provenance_ok:
        reasons.append("citation_provenance_invalid")

    evidence_text = _normalized_text("\n".join([top.title, top.summary, top.body_markdown]))
    missing_terms = [
        term for term in case.expected_evidence_terms if _normalized_text(term) not in evidence_text
    ]
    grounding_ok = not missing_terms
    if missing_terms:
        reasons.append("expected_claim_terms_not_grounded:" + ",".join(missing_terms))

    citation_ok = identity_ok and provenance_ok
    passed = retrieval_status == "hit" and citation_ok and grounding_ok
    return PublishedEvalObservation(
        case_id=case.case_id,
        query=case.query,
        retrieval_status=retrieval_status,
        citation_status="valid" if citation_ok else "invalid",
        grounding_status="supported" if grounding_ok else "rejected",
        identity_status="match" if identity_ok else "mismatch",
        provenance_status="valid" if provenance_ok else "invalid",
        passed=passed,
        top_similarity=top.similarity,
        returned_unit_key=top.unit_key,
        reasons=tuple(reasons),
    )


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cases", type=Path, default=DEFAULT_CASES)
    parser.add_argument("--publication-key", required=True)
    parser.add_argument("--top-k", type=int, default=3)
    parser.add_argument("--json-report", type=Path)
    parser.add_argument("--thresholds", type=Path)
    return parser.parse_args()


async def main() -> int:
    args = parse_args()
    cases_path = args.cases.resolve()
    cases = load_cases(cases_path)
    thresholds = load_thresholds(args.thresholds.resolve(), cases_path) if args.thresholds else None
    library = KnowledgeLibrary()
    await library.initialize()
    observations: list[PublishedEvalObservation] = []
    publication_id = ""
    publication_batch_key = ""
    published_version: int | None = None
    try:
        for case in cases:
            results = await library.search(
                case.query,
                top_k=max(1, args.top_k),
                source_type="thought_forest_note",
                min_quality_score=0.90,
            )
            observation = evaluate_case(
                case,
                results,
                expected_publication_key=args.publication_key,
            )
            observations.append(observation)
            if results and results[0].publication_key == args.publication_key:
                publication_id = publication_id or results[0].publication_id
                publication_batch_key = publication_batch_key or results[0].publication_batch_key
                published_version = published_version or results[0].published_version
            print(json.dumps(asdict(observation), ensure_ascii=False))
    finally:
        await library.close()

    passed = sum(1 for item in observations if item.passed)
    report = {
        "schema_version": SCHEMA_VERSION,
        "evaluator_revision": EVALUATOR_REVISION,
        "publication": {
            "publication_id": publication_id,
            "publication_key": args.publication_key,
            "publication_batch_key": publication_batch_key,
            "published_version": published_version,
        },
        "summary": {
            "cases": len(observations),
            "passed": passed,
            "pass_rate": round(passed / len(observations), 4),
            "positive_hits": sum(1 for item in observations if item.retrieval_status == "hit"),
            "negative_rejections": sum(
                1 for item in observations if item.retrieval_status == "expected_empty"
            ),
            "citation_valid": sum(1 for item in observations if item.citation_status == "valid"),
            "grounding_supported": sum(
                1 for item in observations if item.grounding_status == "supported"
            ),
        },
        "observations": [asdict(item) for item in observations],
    }
    qualification_passed = passed == len(observations)
    qualification_reasons: tuple[str, ...] = ()
    if thresholds is not None:
        qualification_passed, qualification_reasons = evaluate_thresholds(thresholds, report)
        report["qualification"] = {
            "threshold_schema_version": THRESHOLD_SCHEMA_VERSION,
            "dataset_sha256": thresholds.dataset_sha256,
            "passed": qualification_passed,
            "reasons": list(qualification_reasons),
        }
    if args.json_report:
        args.json_report.parent.mkdir(parents=True, exist_ok=True)
        args.json_report.write_text(
            json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8"
        )
    print(json.dumps(report["summary"], ensure_ascii=False))
    if thresholds is not None:
        print(json.dumps(report["qualification"], ensure_ascii=False))
    return 0 if qualification_passed else 1


if __name__ == "__main__":
    raise SystemExit(asyncio.run(main()))
