"""Source-grounded deterministic health-indicator parser v1."""

from __future__ import annotations

import hashlib
import re
import unicodedata
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable

from ..models.ocr import HealthIndicator

HEALTH_INDICATOR_PARSER_REVISION = "health-indicator-parser-v3-table-rows"

_INDICATORS: tuple[tuple[str, tuple[str, ...]], ...] = (
    ("vitamin_d", ("25-羟维生素D", "25(OH)D", "维生素D", "Vitamin D")),
    ("ferritin", ("铁蛋白", "Ferritin")),
    ("hemoglobin", ("血红蛋白", "Hemoglobin", "HGB")),
    ("wbc", ("白细胞", "WBC")),
    ("rbc", ("红细胞", "RBC")),
    ("platelet", ("血小板", "Platelet", "PLT")),
    ("glucose", ("血糖", "Glucose", "GLU")),
    ("cholesterol", ("胆固醇", "Cholesterol", "TC")),
    ("triglyceride", ("甘油三酯", "Triglyceride", "TG")),
    ("uric_acid", ("尿酸", "Uric Acid", "UA")),
    ("creatinine", ("肌酐", "Creatinine", "CREA")),
    ("bun", ("尿素氮", "BUN")),
)

_NAME_TO_ID = {
    alias.lower().replace(" ", ""): (indicator_id, aliases[0])
    for indicator_id, aliases in _INDICATORS
    for alias in aliases
}
_ALIAS_PATTERN = "|".join(
    re.escape(alias)
    for _, aliases in _INDICATORS
    for alias in sorted(aliases, key=len, reverse=True)
)
_VALUE_RE = r"[-+]?\d+(?:\.\d+)?"
_UNIT_RE = (
    r"(?:10\s*\^\s*\d+\s*/\s*[A-Za-z]+|[A-Za-zµμ%]+(?:\s*\^\s*\d+)?(?:\s*/\s*[A-Za-z0-9µμ^]+)?)"
)
_DASH_TRANSLATION = str.maketrans(
    {
        "‐": "-",
        "‑": "-",
        "‒": "-",
        "–": "-",
        "—": "-",
        "−": "-",
        "～": "-",
        "~": "-",
    }
)


def _normalize_source_text(value: str) -> str:
    return unicodedata.normalize("NFKC", value).translate(_DASH_TRANSLATION)


_RANGE_RE = rf"{_VALUE_RE}\s*[-~～—–]\s*{_VALUE_RE}"
_ROW_RE = re.compile(
    rf"(?P<name>{_ALIAS_PATTERN})\s*[:：]?\s*"
    rf"(?P<value>{_VALUE_RE})\s*"
    rf"(?P<unit>{_UNIT_RE})?"
    rf"(?:\s*(?:(?:参考范围|参考|范围|reference(?:\s*range)?|normal)\s*[:：]?\s*)?"
    rf"(?P<range>{_RANGE_RE}))?",
    re.IGNORECASE,
)


@dataclass(frozen=True)
class SourceTextBlock:
    source_ref: str
    page: int
    text: str
    source_refs: tuple[str, ...] = ()

    def evidence_refs(self) -> list[str]:
        return list(self.source_refs or (self.source_ref,))


def parser_source_sha256() -> str:
    return hashlib.sha256(Path(__file__).read_bytes()).hexdigest()


def _canonicalize_name(name: str) -> tuple[str, str]:
    key = _normalize_source_text(name).lower().replace(" ", "")
    return _NAME_TO_ID[key]


def _normalize_unit(unit: str | None) -> str | None:
    if not unit:
        return None
    normalized = _normalize_source_text(unit)
    return re.sub(r"\s+", "", normalized).replace("µ", "u").replace("μ", "u")


def _normalize_range(value: str | None) -> str | None:
    if not value:
        return None
    return re.sub(r"\s+", "", _normalize_source_text(value))


def extract_structured_indicators(blocks: Iterable[SourceTextBlock]) -> list[HealthIndicator]:
    indicators: list[HealthIndicator] = []
    seen: set[str] = set()
    for block in blocks:
        for raw_line in block.text.splitlines():
            line = _normalize_source_text(raw_line)
            if not line.strip():
                continue
            for match in _ROW_RE.finditer(line):
                indicator_id, canonical_name = _canonicalize_name(match.group("name"))
                if indicator_id in seen:
                    continue
                unit = _normalize_unit(match.group("unit"))
                reference_range = _normalize_range(match.group("range"))
                confidence = "high" if unit and reference_range else "medium" if unit else "low"
                indicators.append(
                    HealthIndicator(
                        indicator_id=indicator_id,
                        name=canonical_name,
                        value=match.group("value"),
                        unit=unit,
                        reference_range=reference_range,
                        confidence=confidence,
                        source_refs=block.evidence_refs(),
                        source_page=block.page,
                        parser_revision=HEALTH_INDICATOR_PARSER_REVISION,
                    )
                )
                seen.add(indicator_id)
    return indicators
