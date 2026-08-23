"""Conservative external-evidence identity resolution for Thought Forest claims.

This module deliberately resolves only deterministic source identity. It does not
fetch remote metadata, infer study quality, or promote a source's authority.
"""

from __future__ import annotations

import hashlib
import json
import re
from collections.abc import Mapping, Sequence
from pathlib import Path
from typing import Any, Literal
from urllib.parse import parse_qsl, unquote, urlencode, urlsplit, urlunsplit

from pydantic import BaseModel, Field, model_validator

_TRACKING_QUERY_KEYS = {"fbclid", "gclid", "mc_cid", "mc_eid"}
_PROFESSIONAL_ORGANIZATION_DOMAINS = {
    "aafp.org",
    "aaos.org",
    "acsm.org",
    "apta.org",
    "iasp-pain.org",
}
_PUBLISHER_DOMAINS = {
    "bmj.com",
    "tandfonline.com",
}

EXTERNAL_EVIDENCE_REVIEW_SCHEMA_VERSION = "bodysense.external-evidence-review.v1"


class ReviewedExternalSource(BaseModel):
    canonical_key: str = Field(min_length=1)
    canonical_url: str = Field(min_length=1)
    authority_tier: Literal["A", "B"]
    evidence_level: str = Field(min_length=1)
    license_status: Literal["citation_only", "verified_reuse"]
    review_status: Literal["reviewed"]
    reviewed_by: str = Field(min_length=1)
    review_basis: str = Field(min_length=1)


class ReviewedClaimSupportRelation(BaseModel):
    claim_id: str = Field(min_length=1)
    claim_content_hash: str = Field(min_length=16)
    canonical_key: str = Field(min_length=1)
    support_status: Literal["reviewed_support"]
    support_scope: Literal["direct"]
    review_status: Literal["reviewed"]


class ExternalEvidenceReviewManifest(BaseModel):
    schema_version: str
    review_id: str = Field(min_length=1)
    snapshot_git_commit: str = Field(min_length=1)
    reviewed_at: str = Field(min_length=1)
    sources: list[ReviewedExternalSource]
    relations: list[ReviewedClaimSupportRelation]

    @model_validator(mode="after")
    def validate_schema(self) -> "ExternalEvidenceReviewManifest":
        if self.schema_version != EXTERNAL_EVIDENCE_REVIEW_SCHEMA_VERSION:
            raise ValueError(f"unsupported external evidence review schema: {self.schema_version}")
        return self


def _payload(reference: Mapping[str, Any] | Any) -> dict[str, Any]:
    if isinstance(reference, Mapping):
        return {str(key): value for key, value in reference.items()}
    if isinstance(reference, BaseModel):
        return {str(key): value for key, value in reference.model_dump().items()}
    raise TypeError("external reference must be a mapping or Pydantic model")


def _strip_www(host: str) -> str:
    return host[4:] if host.startswith("www.") else host


def _domain_matches(host: str, domains: set[str]) -> bool:
    return any(host == domain or host.endswith(f".{domain}") for domain in domains)


def _normalized_web_url(url: str) -> tuple[str, str]:
    parsed = urlsplit(url.strip())
    scheme = parsed.scheme.lower()
    host = (parsed.hostname or "").lower()
    if not scheme or not host:
        raise ValueError(f"external evidence URL must be absolute: {url}")
    if scheme not in {"http", "https"}:
        raise ValueError(f"unsupported external evidence URL scheme: {scheme}")

    netloc = host
    if parsed.port is not None:
        netloc = f"{host}:{parsed.port}"
    path = parsed.path or "/"
    query_items = [
        (key, value)
        for key, value in parse_qsl(parsed.query, keep_blank_values=True)
        if not key.lower().startswith("utm_") and key.lower() not in _TRACKING_QUERY_KEYS
    ]
    query = urlencode(sorted(query_items))
    canonical_url = urlunsplit((scheme, netloc, path, query, ""))
    return canonical_url, _strip_www(host)


def _blocked_governance() -> tuple[list[str], dict[str, str]]:
    reasons = [
        "support_unreviewed",
        "authority_unresolved",
        "evidence_level_unresolved",
        "license_unknown",
    ]
    governance = {
        "authority_tier": "unresolved",
        "evidence_level": "unresolved",
        "license_status": "unknown",
        "support_status": "unreviewed",
        "admissibility_status": "blocked",
    }
    return reasons, governance


def resolve_external_reference(reference: Mapping[str, Any] | Any) -> dict[str, Any]:
    """Resolve a URL to a stable identity without asserting evidence quality."""
    raw = _payload(reference)
    original_url = str(raw.get("url") or "").strip()
    canonical_url, bare_host = _normalized_web_url(original_url)
    parsed = urlsplit(canonical_url)

    provider = "web"
    source_type = "web_page"
    canonical_key: str

    pubmed_match = re.fullmatch(r"/(\d+)/?", parsed.path)
    if bare_host == "pubmed.ncbi.nlm.nih.gov" and pubmed_match:
        pmid = pubmed_match.group(1)
        provider = "pubmed"
        source_type = "bibliographic_index"
        canonical_key = f"pmid:{pmid}"
        canonical_url = f"https://pubmed.ncbi.nlm.nih.gov/{pmid}/"
    elif bare_host == "doi.org" and parsed.path.strip("/"):
        doi = unquote(parsed.path.strip("/")).lower()
        provider = "doi"
        source_type = "doi_resolver"
        canonical_key = f"doi:{doi}"
        canonical_url = f"https://doi.org/{doi}"
    else:
        if _domain_matches(bare_host, _PROFESSIONAL_ORGANIZATION_DOMAINS):
            source_type = "professional_organization_page"
        elif _domain_matches(bare_host, _PUBLISHER_DOMAINS):
            source_type = "publisher_page"
        digest = hashlib.sha256(canonical_url.encode("utf-8")).hexdigest()[:32]
        canonical_key = f"url:{digest}"

    blocking_reasons, governance = _blocked_governance()
    return {
        "reference_id": str(raw.get("reference_id") or ""),
        "label": str(raw.get("label") or original_url),
        "original_url": original_url,
        "canonical_url": canonical_url,
        "canonical_key": canonical_key,
        "provider": provider,
        "source_type": source_type,
        "relation_scope": str(raw.get("scope") or "section_direct"),
        "reference_line": int(raw.get("line") or 0),
        "resolution_status": "identity_resolved",
        **governance,
        "blocking_reasons": blocking_reasons,
    }


def load_external_evidence_review(
    path: str | Path,
) -> ExternalEvidenceReviewManifest:
    resolved = Path(path).resolve()
    payload = json.loads(resolved.read_text(encoding="utf-8"))
    return ExternalEvidenceReviewManifest.model_validate(payload)


def apply_external_evidence_review(
    *,
    claim_id: str,
    claim_content_hash: str,
    resolved_references: Sequence[Mapping[str, Any]],
    review_manifest: ExternalEvidenceReviewManifest,
) -> list[dict[str, Any]]:
    """Apply only exact source + claim-version reviews to resolved references."""
    sources = {source.canonical_key: source for source in review_manifest.sources}
    relations = {
        (relation.claim_id, relation.claim_content_hash, relation.canonical_key): relation
        for relation in review_manifest.relations
    }
    reviewed: list[dict[str, Any]] = []
    for reference in resolved_references:
        item = dict(reference)
        canonical_key = str(item.get("canonical_key") or "")
        source_review = sources.get(canonical_key)
        relation_review = relations.get((claim_id, claim_content_hash, canonical_key))
        if (
            source_review is not None
            and relation_review is not None
            and source_review.canonical_url == item.get("canonical_url")
        ):
            item.update(
                {
                    "authority_tier": source_review.authority_tier,
                    "evidence_level": source_review.evidence_level,
                    "license_status": source_review.license_status,
                    "support_status": relation_review.support_status,
                    "admissibility_status": "admissible_for_claim_review",
                    "blocking_reasons": [],
                    "external_review_id": review_manifest.review_id,
                    "external_reviewed_at": review_manifest.reviewed_at,
                    "external_reviewed_by": source_review.reviewed_by,
                    "support_scope": relation_review.support_scope,
                }
            )
        reviewed.append(item)
    return reviewed


def build_claim_admissibility(
    direct_references: Sequence[Mapping[str, Any]],
) -> dict[str, Any]:
    """Return conservative evidence readiness; publication remains a later gate."""
    if not direct_references:
        return {
            "status": "blocked",
            "evidence_ready_for_claim_review": False,
            "publication_eligible": False,
            "blocking_reasons": ["no_direct_external_reference"],
            "direct_reference_count": 0,
            "admissible_reference_count": 0,
        }

    admissible_count = sum(
        1
        for reference in direct_references
        if reference.get("admissibility_status") == "admissible_for_claim_review"
    )
    if admissible_count > 0:
        return {
            "status": "evidence_ready_for_claim_review",
            "evidence_ready_for_claim_review": True,
            "publication_eligible": False,
            "blocking_reasons": ["claim_review_unreviewed"],
            "direct_reference_count": len(direct_references),
            "admissible_reference_count": admissible_count,
        }

    reasons: list[str] = []
    for reference in direct_references:
        for reason in reference.get("blocking_reasons", []):
            text = str(reason)
            if text and text not in reasons:
                reasons.append(text)

    return {
        "status": "blocked",
        "evidence_ready_for_claim_review": False,
        "publication_eligible": False,
        "blocking_reasons": reasons,
        "direct_reference_count": len(direct_references),
        "admissible_reference_count": 0,
    }
