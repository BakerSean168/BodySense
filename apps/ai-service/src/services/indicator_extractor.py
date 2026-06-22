"""Health indicator extraction from OCR text."""

import re

from ..models.ocr import HealthIndicator

# Common health indicator patterns
# Pattern: indicator name + value + optional unit + optional reference range
INDICATOR_PATTERNS = [
    # Pattern: Name: Value Unit (Reference: Range)
    # Example: 维生素D: 25.3 ng/mL (参考范围: 30-100)
    re.compile(
        r"(?P<name>维生素[A-Za-z0-9]+|维[生素]+[A-Za-z0-9]+)"
        r"[\s:：]+"
        r"(?P<value>[\d.]+)"
        r"\s*"
        r"(?P<unit>[a-zA-Z/μ]+[a-zA-Z0-9/μ]*)?"
        r"(?:.*?(?:参考|正常|标准|范围)[^：:\d]*[:：]?\s*(?P<range>[\d.]+\s*[-~～]\s*[\d.]+))?",
        re.IGNORECASE,
    ),
    # Pattern: 铁蛋白: 50 ng/mL
    re.compile(
        r"(?P<name>铁蛋白|血红蛋白|白细胞|红细胞|血小板|血糖|胆固醇|甘油三酯|尿酸|肌酐|尿素氮)"
        r"[\s:：]+"
        r"(?P<value>[\d.]+)"
        r"\s*"
        r"(?P<unit>[a-zA-Z/μ]+[a-zA-Z0-9/μ]*)?"
        r"(?:.*?(?:参考|正常|标准|范围)[^：:\d]*[:：]?\s*(?P<range>[\d.]+\s*[-~～]\s*[\d.]+))?",
        re.IGNORECASE,
    ),
    # Pattern for table-like format: Name  Value  Unit  Range
    re.compile(
        r"(?P<name>[^\d\n]{2,20})"
        r"\s+"
        r"(?P<value>[\d.]+)"
        r"\s+"
        r"(?P<unit>[a-zA-Z/μ%]+[a-zA-Z0-9/μ]*)?"
        r"\s*"
        r"(?P<range>[\d.]+\s*[-~～]\s*[\d.]+)?",
    ),
]

# Keywords that indicate health indicators
HEALTH_KEYWORDS = [
    "维生素",
    "铁蛋白",
    "血红蛋白",
    "白细胞",
    "红细胞",
    "血小板",
    "血糖",
    "胆固醇",
    "甘油三酯",
    "尿酸",
    "肌酐",
    "尿素氮",
    "钙",
    "镁",
    "锌",
    "铁",
    "叶酸",
    "B12",
    "D",
    "A",
    "E",
    "K",
    "C",
]


def extract_indicators(text: str) -> list[HealthIndicator]:
    """
    Extract health indicators from OCR text.

    Args:
        text: OCR-extracted text from a health report

    Returns:
        List of extracted health indicators
    """
    if not text or not text.strip():
        return []

    indicators: list[HealthIndicator] = []
    seen_names: set[str] = set()

    # Try each pattern
    for pattern in INDICATOR_PATTERNS:
        for match in pattern.finditer(text):
            name = match.group("name").strip()
            value = match.group("value")
            unit = match.group("unit") if match.group("unit") else None
            ref_range = match.group("range") if match.group("range") else None

            # Skip if name is too short or already seen
            if len(name) < 2 or name.lower() in seen_names:
                continue

            # Determine confidence based on context
            confidence = _determine_confidence(name, value, unit, text)

            indicators.append(
                HealthIndicator(
                    name=name,
                    value=value,
                    unit=unit,
                    reference_range=ref_range,
                    confidence=confidence,
                )
            )
            seen_names.add(name.lower())

    return indicators


def _determine_confidence(
    name: str,
    value: str,
    unit: str | None,
    full_text: str,
) -> str:
    """
    Determine confidence level for an extracted indicator.

    Returns:
        "high", "medium", or "low"
    """
    # High confidence: has unit AND reference range context
    if unit and _has_reference_context(full_text, name):
        return "high"

    # Medium confidence: has unit but no reference range
    if unit:
        return "medium"

    # Low confidence: no unit, possibly misrecognized
    return "low"


def _has_reference_context(text: str, indicator_name: str) -> bool:
    """Check if the text has reference range context near the indicator."""
    # Look for reference range keywords near the indicator
    ref_keywords = ["参考", "正常", "标准", "范围", "reference", "normal"]
    idx = text.find(indicator_name)
    if idx == -1:
        return False

    # Check 200 characters after the indicator
    context = text[idx : idx + 200]
    return any(kw in context for kw in ref_keywords)


def get_overall_confidence(indicators: list[HealthIndicator]) -> str:
    """
    Determine overall confidence based on individual indicators.

    Returns:
        "high", "medium", or "low"
    """
    if not indicators:
        return "low"

    confidences = [ind.confidence for ind in indicators]
    high_count = confidences.count("high")
    total = len(confidences)

    if high_count / total >= 0.7:
        return "high"
    elif high_count / total >= 0.3:
        return "medium"
    return "low"
