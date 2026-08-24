"""Runtime answer-claim attribution for published knowledge.

This is intentionally a narrow deterministic policy. It verifies that a model
only attributes claims to published Thought Forest evidence retrieved in the
current turn, and assigns a conservative lexical grounding status. It does not
pretend to be a semantic medical judge.
"""

from __future__ import annotations

import re
from typing import Any, Mapping, Sequence

ANSWER_ATTRIBUTION_POLICY_REVISION = "consultation-answer-attribution-v1"
MAX_ATTRIBUTION_CLAIMS = 6
MAX_EVIDENCE_REFS_PER_CLAIM = 3


def build_published_evidence_binding(result: Any) -> dict[str, Any] | None:
    """Return the immutable runtime binding for one published Thought Forest result."""
    if str(getattr(result, "source_type", "") or "") != "thought_forest_note":
        return None
    if str(getattr(result, "lifecycle_status", "") or "") != "published":
        return None

    publication_id = str(getattr(result, "publication_id", "") or "").strip()
    publication_key = str(getattr(result, "publication_key", "") or "").strip()
    publication_batch_key = str(getattr(result, "publication_batch_key", "") or "").strip()
    unit_key = str(getattr(result, "unit_key", "") or "").strip()
    published_version = getattr(result, "published_version", None)
    if not (
        publication_id
        and publication_key
        and publication_batch_key
        and unit_key
        and isinstance(published_version, int)
        and published_version > 0
    ):
        return None

    metadata = dict(getattr(result, "unit_metadata", {}) or {})
    locator = dict(metadata.get("source_locator") or {})
    claim_candidate = dict(metadata.get("claim_candidate") or {})
    claim_review = dict(metadata.get("claim_review") or {})
    claim_id = str(claim_candidate.get("claim_id") or "").strip()
    claim_review_id = str(claim_review.get("review_id") or "").strip()
    if not claim_id or not claim_review_id:
        return None
    if (
        locator.get("locator_type") != "markdown_lines"
        or not locator.get("git_commit")
        or not locator.get("path")
        or int(locator.get("line_start") or 0) <= 0
        or int(locator.get("line_end") or 0) < int(locator.get("line_start") or 0)
    ):
        return None

    evidence_ref = f"published:{publication_id}:v{published_version}:{unit_key}"
    title = str(getattr(result, "title", "") or "")
    summary = str(getattr(result, "summary", "") or "")
    body = str(getattr(result, "body_markdown", "") or "")
    return {
        "evidence_ref": evidence_ref,
        "publication_id": publication_id,
        "publication_key": publication_key,
        "publication_batch_key": publication_batch_key,
        "published_version": published_version,
        "unit_key": unit_key,
        "claim_id": claim_id,
        "claim_review_id": claim_review_id,
        "claim_kind": str(claim_candidate.get("claim_kind") or ""),
        "source_locator": locator,
        "_evidence_text": "\n".join(part for part in (title, summary, body) if part),
    }


def _normalize_text(value: str) -> str:
    value = re.sub(r"https?://\S+", "", value.lower())
    return re.sub(r"[^a-z0-9\u4e00-\u9fff]+", "", value)


def _lexical_terms(value: str) -> set[str]:
    normalized = value.lower()
    terms = {
        token
        for token in re.findall(r"[a-z][a-z0-9._+-]{2,}", normalized)
        if token not in {"the", "and", "with", "for"}
    }
    for run in re.findall(r"[\u4e00-\u9fff]+", normalized):
        for n in (2, 3):
            if len(run) < n:
                continue
            for index in range(len(run) - n + 1):
                terms.add(run[index : index + n])
    return terms


def _grounding_status(claim_text: str, evidence_text: str) -> tuple[str, list[str]]:
    claim_normalized = _normalize_text(claim_text)
    evidence_normalized = _normalize_text(evidence_text)
    if not claim_normalized:
        return "rejected", ["empty_claim"]
    if claim_normalized in evidence_normalized:
        return "supported", ["claim_text_contained_in_retrieved_evidence"]

    claim_terms = _lexical_terms(claim_text)
    evidence_terms = _lexical_terms(evidence_text)
    if not claim_terms:
        return "rejected", ["no_meaningful_claim_terms"]
    matched = claim_terms & evidence_terms
    overlap = len(matched) / len(claim_terms)
    if len(matched) >= 2 and overlap >= 0.45:
        return "supported", ["lexical_support_sufficient"]
    if len(matched) >= 2:
        return "degraded", ["lexical_support_partial"]
    return "rejected", ["claim_not_supported_by_attributed_evidence"]


def _public_binding(binding: Mapping[str, Any]) -> dict[str, Any]:
    return {key: value for key, value in binding.items() if not key.startswith("_")}


def validate_and_evaluate_attribution(
    claims: Sequence[Mapping[str, Any]],
    retrieved_by_ref: Mapping[str, Mapping[str, Any]],
) -> list[dict[str, Any]]:
    """Validate model attribution against this turn's retrieved publication bindings."""
    if not isinstance(claims, Sequence) or isinstance(claims, (str, bytes)):
        raise ValueError("claims must be an array")
    if not claims:
        raise ValueError("attribution requires at least one claim")
    if len(claims) > MAX_ATTRIBUTION_CLAIMS:
        raise ValueError(f"attribution supports at most {MAX_ATTRIBUTION_CLAIMS} claims")

    evaluated: list[dict[str, Any]] = []
    for index, raw in enumerate(claims):
        if not isinstance(raw, Mapping):
            raise ValueError(f"claim {index} must be an object")
        claim_text = str(raw.get("claim_text") or "").strip()
        if not claim_text:
            raise ValueError(f"claim {index} claim_text is required")
        if len(claim_text) > 500:
            raise ValueError(f"claim {index} claim_text exceeds 500 characters")
        refs_raw = raw.get("evidence_refs")
        if not isinstance(refs_raw, list) or not refs_raw:
            raise ValueError(f"claim {index} evidence_refs must be a non-empty array")
        refs = [str(item).strip() for item in refs_raw if str(item).strip()]
        if not refs:
            raise ValueError(f"claim {index} evidence_refs must not be empty")
        if len(refs) > MAX_EVIDENCE_REFS_PER_CLAIM:
            raise ValueError(
                f"claim {index} supports at most {MAX_EVIDENCE_REFS_PER_CLAIM} evidence refs"
            )
        if len(set(refs)) != len(refs):
            raise ValueError(f"claim {index} evidence_refs must be unique")

        bindings: list[Mapping[str, Any]] = []
        for evidence_ref in refs:
            binding = retrieved_by_ref.get(evidence_ref)
            if binding is None:
                raise ValueError(f"evidence_ref {evidence_ref!r} was not retrieved in this turn")
            bindings.append(binding)

        statuses = [
            _grounding_status(claim_text, str(binding.get("_evidence_text") or ""))
            for binding in bindings
        ]
        if any(status == "supported" for status, _ in statuses):
            grounding_status = "supported"
        elif any(status == "degraded" for status, _ in statuses):
            grounding_status = "degraded"
        else:
            grounding_status = "rejected"
        reasons = sorted({reason for _, reason_list in statuses for reason in reason_list})
        evaluated.append(
            {
                "policy_revision": ANSWER_ATTRIBUTION_POLICY_REVISION,
                "claim_text": claim_text,
                "evidence_refs": refs,
                "grounding_status": grounding_status,
                "reason_codes": reasons,
                "bindings": [_public_binding(binding) for binding in bindings],
            }
        )
    return evaluated
