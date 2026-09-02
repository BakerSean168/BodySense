"""Deterministic page routing for health-document extraction.

Born-digital PDF text is preferred when it is structurally usable. OCR is a
fallback for scanned or corrupted pages, not a mandatory lossy conversion.
"""

from __future__ import annotations

import hashlib
import re
from dataclasses import dataclass

NATIVE_TEXT_QUALITY_POLICY_REVISION = "health-document-native-text-quality-v1"
MIN_NON_WHITESPACE_CHARS = 20
MIN_SEMANTIC_CHAR_RATIO = 0.60
MAX_REPLACEMENT_CHAR_RATIO = 0.0

_CJK_RE = re.compile(r"[\u3400-\u9fff]")


@dataclass(frozen=True)
class NativeTextQualityDecision:
    usable: bool
    policy_revision: str
    non_whitespace_chars: int
    semantic_char_ratio: float
    replacement_char_ratio: float
    reason_codes: tuple[str, ...]


def native_text_quality_policy_fingerprint() -> str:
    payload = (
        f"{NATIVE_TEXT_QUALITY_POLICY_REVISION}|"
        f"min_non_whitespace={MIN_NON_WHITESPACE_CHARS}|"
        f"min_semantic_ratio={MIN_SEMANTIC_CHAR_RATIO:.4f}|"
        f"max_replacement_ratio={MAX_REPLACEMENT_CHAR_RATIO:.4f}"
    )
    return hashlib.sha256(payload.encode()).hexdigest()


def evaluate_native_text_quality(text: str) -> NativeTextQualityDecision:
    compact = [char for char in text if not char.isspace()]
    total = len(compact)
    if total == 0:
        return NativeTextQualityDecision(
            usable=False,
            policy_revision=NATIVE_TEXT_QUALITY_POLICY_REVISION,
            non_whitespace_chars=0,
            semantic_char_ratio=0.0,
            replacement_char_ratio=0.0,
            reason_codes=("empty_native_text",),
        )

    semantic = sum(char.isalnum() or _CJK_RE.fullmatch(char) is not None for char in compact)
    replacement = sum(char == "\ufffd" for char in compact)
    semantic_ratio = semantic / total
    replacement_ratio = replacement / total
    reasons: list[str] = []
    if total < MIN_NON_WHITESPACE_CHARS:
        reasons.append("native_text_too_short")
    if semantic_ratio < MIN_SEMANTIC_CHAR_RATIO:
        reasons.append("native_text_low_semantic_ratio")
    if replacement_ratio > MAX_REPLACEMENT_CHAR_RATIO:
        reasons.append("native_text_contains_replacement_chars")
    return NativeTextQualityDecision(
        usable=not reasons,
        policy_revision=NATIVE_TEXT_QUALITY_POLICY_REVISION,
        non_whitespace_chars=total,
        semantic_char_ratio=semantic_ratio,
        replacement_char_ratio=replacement_ratio,
        reason_codes=tuple(reasons) or ("native_text_usable",),
    )
