"""Run real-provider Diagnosis v10 acceptance against the staging LiteLLM route.

This script intentionally exercises only benign, provider-executed structured-safety
cases. Deterministic safety blocking/abstention is covered separately by qualification
and policy tests. No provider credential is read or emitted; the script uses the normal
internal LiteLLM configuration already available to ai-service.
"""

from __future__ import annotations

import argparse
import asyncio
import json
import sys
import time
from collections import Counter
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[1]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from src.models.safety import SafetyEnvelopeV2  # noqa: E402
from src.services.diagnosis_service import DiagnosisService  # noqa: E402

CONFIGURATION_ID = "diag-config-3f64de162dc937ee"
DEFAULT_CASES = ROOT / "data/evals/diagnosis_v10_provider_acceptance_cases.json"
DEFAULT_OUTPUT = ROOT / "data/evals/reports/diagnosis_v10_provider_acceptance.json"


@dataclass
class StartRateLimiter:
    min_interval_seconds: float
    _lock: asyncio.Lock = field(default_factory=asyncio.Lock)
    _last_start: float | None = None

    async def wait(self) -> None:
        async with self._lock:
            now = time.monotonic()
            if self._last_start is not None:
                delay = self.min_interval_seconds - (now - self._last_start)
                if delay > 0:
                    await asyncio.sleep(delay)
            self._last_start = time.monotonic()


def _validate_payload(payload: dict[str, Any], configuration_id: str) -> list[str]:
    failures: list[str] = []
    if payload.get("status") != "completed":
        failures.append("status_not_completed")

    governance = payload.get("governance")
    if not isinstance(governance, dict):
        failures.append("governance_missing")
    elif governance.get("verdict") == "rejected":
        failures.append("governance_rejected")
    elif governance.get("verdict") not in {"accepted", "degraded"}:
        failures.append("governance_unknown")

    candidates = payload.get("candidates")
    if not isinstance(candidates, list) or not candidates:
        failures.append("candidate_contract_failed")

    config = payload.get("agent_configuration")
    if not isinstance(config, dict) or config.get("id") != configuration_id:
        failures.append("configuration_mismatch")

    provenance = payload.get("execution_provenance")
    if not isinstance(provenance, dict) or provenance.get("status") != "executed":
        failures.append("agent_not_executed")

    if "treatment" in payload or "training_plan" in payload:
        failures.append("forbidden_side_effect")

    return failures


async def _run_one(
    service: DiagnosisService,
    *,
    ordinal: int,
    case: dict[str, Any],
    semaphore: asyncio.Semaphore,
    rate_limiter: StartRateLimiter,
) -> dict[str, Any]:
    started = time.perf_counter()
    async with semaphore:
        await rate_limiter.wait()
        try:
            inputs = case["inputs"]
            envelope = SafetyEnvelopeV2.model_validate(inputs["safety_envelope"])
            payload = await service.generate_diagnosis(
                user_id=f"provider-acceptance-{ordinal:02d}",
                body_state_revision=int(inputs["body_state_revision"]),
                configuration_id=CONFIGURATION_ID,
                body_state=inputs["body_state"],
                relevant_history=inputs.get("relevant_history") or [],
                profile=inputs.get("profile") or {},
                safety_envelope=envelope,
            )
            failures = _validate_payload(payload, CONFIGURATION_ID)
            provenance = payload.get("execution_provenance") or {}
            governance = payload.get("governance") or {}
            return {
                "ordinal": ordinal,
                "case": case["name"],
                "success": not failures,
                "duration_ms": round((time.perf_counter() - started) * 1000),
                "status": payload.get("status"),
                "governance_verdict": governance.get("verdict"),
                "candidate_count": len(payload.get("candidates") or []),
                "configuration_id": (
                    payload.get("agent_configuration") or {}
                ).get("id"),
                "provider_adapter": provenance.get("provider_adapter"),
                "gateway_reported_model": provenance.get("gateway_reported_model"),
                "input_tokens": (provenance.get("usage") or {}).get("input_tokens"),
                "output_tokens": (provenance.get("usage") or {}).get("output_tokens"),
                "failures": failures,
            }
        except Exception as exc:  # real-provider qualification must retain transport failures
            return {
                "ordinal": ordinal,
                "case": case["name"],
                "success": False,
                "duration_ms": round((time.perf_counter() - started) * 1000),
                "error_type": type(exc).__name__,
                "error": str(exc)[:1000],
                "failures": ["provider_error"],
            }


async def _run(
    samples: int,
    concurrency: int,
    min_start_interval_seconds: float,
    cases_path: Path,
) -> dict[str, Any]:
    document = json.loads(cases_path.read_text(encoding="utf-8"))
    source_cases = document.get("cases") or []
    if document.get("scope") != "structured_capture_only":
        raise ValueError("provider acceptance corpus must declare structured_capture_only scope")
    if not source_cases:
        raise ValueError("provider acceptance case corpus is empty")
    if any(case.get("expected_status") != "completed" for case in source_cases):
        raise ValueError("provider acceptance case corpus must contain completed benign cases only")
    for case in source_cases:
        envelope = (case.get("inputs") or {}).get("safety_envelope") or {}
        if envelope.get("legacy_state_present") is not False:
            raise ValueError("provider acceptance hard gate excludes legacy-state migration cases")
        if envelope.get("requires_review") is not False or not (envelope.get("coverage") or {}).get(
            "complete"
        ):
            raise ValueError(
                "provider acceptance cases require complete nonblocking safety coverage"
            )

    cases = [
        source_cases[(ordinal - 1) % len(source_cases)]
        for ordinal in range(1, samples + 1)
    ]
    semaphore = asyncio.Semaphore(concurrency)
    rate_limiter = StartRateLimiter(min_start_interval_seconds)
    service = DiagnosisService()
    results = await asyncio.gather(
        *(
            _run_one(
                service,
                ordinal=ordinal,
                case=case,
                semaphore=semaphore,
                rate_limiter=rate_limiter,
            )
            for ordinal, case in enumerate(cases, 1)
        )
    )

    failure_counts = Counter(
        failure
        for result in results
        for failure in result.get("failures", [])
    )
    successes = sum(1 for result in results if result["success"])
    durations = [int(result["duration_ms"]) for result in results]

    return {
        "name": "diagnosis-v10-provider-acceptance-v1",
        "configuration_id": CONFIGURATION_ID,
        "logical_model": "bodysense-diagnosis",
        "dataset": {
            "path": str(cases_path.relative_to(ROOT)),
            "scope": document.get("scope"),
            "case_names": [case["name"] for case in source_cases],
        },
        "execution": {
            "samples": samples,
            "concurrency": concurrency,
            "min_start_interval_seconds": min_start_interval_seconds,
        },
        "summary": {
            "total": len(results),
            "successes": successes,
            "success_rate": successes / len(results) if results else 0.0,
            "errors": failure_counts["provider_error"],
            "contract_failures": sum(
                count
                for name, count in failure_counts.items()
                if name
                in {
                    "status_not_completed",
                    "governance_missing",
                    "governance_unknown",
                    "candidate_contract_failed",
                    "agent_not_executed",
                    "forbidden_side_effect",
                }
            ),
            "governance_rejections": failure_counts["governance_rejected"],
            "configuration_mismatches": failure_counts["configuration_mismatch"],
            "duration_ms": {
                "min": min(durations) if durations else None,
                "max": max(durations) if durations else None,
                "mean": round(sum(durations) / len(durations)) if durations else None,
            },
        },
        "samples": results,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--samples", type=int, default=20)
    parser.add_argument("--concurrency", type=int, default=3)
    parser.add_argument("--min-start-interval-seconds", type=float, default=5.0)
    parser.add_argument("--cases", type=Path, default=DEFAULT_CASES)
    parser.add_argument("--json-output", type=Path, default=DEFAULT_OUTPUT)
    args = parser.parse_args()
    if args.samples < 1:
        raise SystemExit("--samples must be positive")
    if args.concurrency < 1:
        raise SystemExit("--concurrency must be positive")
    if args.min_start_interval_seconds < 4.0:
        raise SystemExit("--min-start-interval-seconds must be >= 4 for the staging provider")

    report = asyncio.run(
        _run(
            args.samples,
            args.concurrency,
            args.min_start_interval_seconds,
            args.cases,
        )
    )
    args.json_output.parent.mkdir(parents=True, exist_ok=True)
    args.json_output.write_text(
        json.dumps(report, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    summary = report["summary"]
    print(
        "provider acceptance: "
        f"{summary['successes']}/{summary['total']} success, "
        f"errors={summary['errors']}, "
        f"contract_failures={summary['contract_failures']}, "
        f"governance_rejections={summary['governance_rejections']}, "
        f"configuration_mismatches={summary['configuration_mismatches']}"
    )
    return 0 if summary["successes"] == summary["total"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
