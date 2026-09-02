"""Health-critical benchmark metrics for document extraction candidates."""

from __future__ import annotations

import re
import unicodedata
from collections.abc import Iterable

from .contracts import (
    AccuracySummary,
    CorpusDocument,
    EvidenceAuthoritySummary,
    FixtureBenchmarkResult,
    PredictedIndicator,
    SourceAccuracyCounts,
    SourceAccuracySummary,
)

_ALIAS_TO_ID = {
    "维生素d": "vitamin_d",
    "25-羟维生素d": "vitamin_d",
    "vitamind": "vitamin_d",
    "铁蛋白": "ferritin",
    "血红蛋白": "hemoglobin",
    "白细胞": "wbc",
    "红细胞": "rbc",
    "血小板": "platelet",
    "血糖": "glucose",
    "胆固醇": "cholesterol",
    "甘油三酯": "triglyceride",
    "尿酸": "uric_acid",
    "肌酐": "creatinine",
    "尿素氮": "bun",
}

_REFERENCE_MARKERS = ("参考范围", "参考", "范围", "reference", "normal")
_NUMBER_RE = re.compile(r"[-+]?\d+(?:\.\d+)?")

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


def _canonical_unicode(value: str) -> str:
    return unicodedata.normalize("NFKC", value).translate(_DASH_TRANSLATION)


def _normalize_token(value: str | None) -> str:
    if value is None:
        return ""
    value = _canonical_unicode(value)
    return (
        re.sub(r"\s+", "", value)
        .replace("～", "-")
        .replace("~", "-")
        .replace("—", "-")
        .replace("–", "-")
        .replace("µ", "u")
        .replace("μ", "u")
        .replace("：", ":")
        .lower()
    )


def _normalize_line(value: str) -> str:
    value = _canonical_unicode(value)
    return (
        re.sub(r"\s+", " ", value)
        .replace("～", "-")
        .replace("~", "-")
        .replace("—", "-")
        .replace("–", "-")
        .replace("µ", "u")
        .replace("μ", "u")
        .replace("：", ":")
        .strip()
        .lower()
    )


def canonical_indicator_id(name: str) -> str | None:
    normalized = _normalize_token(name)
    return _ALIAS_TO_ID.get(normalized)


def _aliases_for_indicator(indicator_id: str, display_name: str) -> tuple[str, ...]:
    aliases = {_normalize_line(display_name)}
    aliases.update(
        _normalize_line(alias) for alias, target in _ALIAS_TO_ID.items() if target == indicator_id
    )
    return tuple(sorted(alias for alias in aliases if alias))


def _measurement_prefix(line: str) -> str:
    positions = [line.find(marker) for marker in _REFERENCE_MARKERS if marker in line]
    if not positions:
        return line
    return line[: min(positions)]


def _first_measurement_number(line: str, aliases: tuple[str, ...]) -> str | None:
    alias_positions = [(line.find(alias), alias) for alias in aliases if alias in line]
    if not alias_positions:
        return None
    start, alias = min(alias_positions, key=lambda item: item[0])
    tail = _measurement_prefix(line[start + len(alias) :])
    match = _NUMBER_RE.search(tail)
    return match.group(0) if match else None


def evaluate_source_text(document: CorpusDocument, raw_text: str) -> SourceAccuracyCounts:
    """Evaluate OCR/source text before the production indicator parser.

    This metric intentionally does not call ``indicator_extractor``. It asks
    whether the source text itself preserves the annotated row's indicator name,
    measurement value, unit and reference range. That prevents parser limitations
    (for example a regex that does not understand ``10^9/L``) from being
    misattributed to the OCR engine during technology selection.
    """

    lines = [_normalize_line(line) for line in raw_text.splitlines() if line.strip()]
    counts = {
        "truth_indicators": 0,
        "name_present": 0,
        "numeric_exact": 0,
        "unit_exact": 0,
        "reference_range_exact": 0,
        "row_bundle_exact": 0,
        "critical_indicators": 0,
        "critical_numeric_errors": 0,
    }

    for truth in document.indicators:
        counts["truth_indicators"] += 1
        if truth.critical_numeric:
            counts["critical_indicators"] += 1

        aliases = _aliases_for_indicator(truth.indicator_id, truth.display_name)
        matching_lines = [line for line in lines if any(alias in line for alias in aliases)]
        if not matching_lines:
            continue
        counts["name_present"] += 1

        expected_value = _normalize_token(truth.value)
        expected_unit = _normalize_token(truth.unit)
        expected_range = _normalize_token(truth.reference_range)

        best_value = False
        best_unit = False
        best_range = False
        best_bundle = False
        observed_measurement: str | None = None

        for line in matching_lines:
            first_number = _first_measurement_number(line, aliases)
            value_ok = first_number is not None and _normalize_token(first_number) == expected_value
            compact_line = _normalize_token(line)
            unit_ok = not expected_unit or expected_unit in compact_line
            range_ok = not expected_range or expected_range in compact_line
            bundle_ok = value_ok and unit_ok and range_ok
            best_value = best_value or value_ok
            best_unit = best_unit or unit_ok
            best_range = best_range or range_ok
            best_bundle = best_bundle or bundle_ok
            if observed_measurement is None and first_number is not None:
                observed_measurement = _normalize_token(first_number)

        counts["numeric_exact"] += int(best_value)
        counts["unit_exact"] += int(best_unit)
        counts["reference_range_exact"] += int(best_range)
        counts["row_bundle_exact"] += int(best_bundle)
        if (
            truth.critical_numeric
            and not best_value
            and observed_measurement is not None
            and observed_measurement != expected_value
        ):
            counts["critical_numeric_errors"] += 1

    return SourceAccuracyCounts(**counts)


def summarize_source_accuracy(results: Iterable[FixtureBenchmarkResult]) -> SourceAccuracySummary:
    total = 0
    names = 0
    numeric = 0
    units = 0
    ranges = 0
    bundles = 0
    critical_total = 0
    critical_errors = 0
    for result in results:
        counts = result.source_counts
        if counts is None:
            continue
        total += counts.truth_indicators
        names += counts.name_present
        numeric += counts.numeric_exact
        units += counts.unit_exact
        ranges += counts.reference_range_exact
        bundles += counts.row_bundle_exact
        critical_total += counts.critical_indicators
        critical_errors += counts.critical_numeric_errors

    denominator = total or 1
    return SourceAccuracySummary(
        truth_indicators=total,
        name_recall=names / denominator,
        numeric_exact_match=numeric / denominator,
        unit_exact_match=units / denominator,
        reference_range_exact_match=ranges / denominator,
        row_bundle_exact_match=bundles / denominator,
        critical_numeric_error_rate=critical_errors / (critical_total or 1),
    )


def summarize_accuracy(
    documents: Iterable[CorpusDocument],
    predictions: dict[str, list[PredictedIndicator]],
) -> AccuracySummary:
    truth_total = 0
    matched = 0
    numeric_exact = 0
    unit_exact = 0
    range_exact = 0
    indicator_exact = 0
    row_association_exact = 0
    critical_total = 0
    critical_numeric_errors = 0

    for document in documents:
        predicted_by_id: dict[str, PredictedIndicator] = {}
        for prediction in predictions.get(document.fixture_id, []):
            candidate_id = canonical_indicator_id(prediction.name)
            if candidate_id is not None and candidate_id not in predicted_by_id:
                predicted_by_id[candidate_id] = prediction

        for truth in document.indicators:
            truth_total += 1
            if truth.critical_numeric:
                critical_total += 1
            prediction = predicted_by_id.get(truth.indicator_id)
            if prediction is None:
                continue
            matched += 1
            value_ok = _normalize_token(prediction.value) == _normalize_token(truth.value)
            unit_ok = _normalize_token(prediction.unit) == _normalize_token(truth.unit)
            range_ok = _normalize_token(prediction.reference_range) == _normalize_token(
                truth.reference_range
            )
            numeric_exact += int(value_ok)
            row_association_exact += int(value_ok)
            unit_exact += int(unit_ok)
            range_exact += int(range_ok)
            all_ok = value_ok and unit_ok and range_ok
            indicator_exact += int(all_ok)
            if truth.critical_numeric and not value_ok:
                critical_numeric_errors += 1

    denominator = truth_total or 1
    return AccuracySummary(
        truth_indicators=truth_total,
        matched_indicators=matched,
        numeric_exact_match=numeric_exact / denominator,
        unit_exact_match=unit_exact / denominator,
        reference_range_exact_match=range_exact / denominator,
        indicator_exact_match=indicator_exact / denominator,
        row_association_accuracy=row_association_exact / denominator,
        critical_numeric_error_rate=critical_numeric_errors / (critical_total or 1),
    )


def summarize_evidence_authority(
    documents: Iterable[CorpusDocument],
    predictions: dict[str, list[PredictedIndicator]],
) -> EvidenceAuthoritySummary:
    truth_total = 0
    critical_total = 0
    auto_admitted = 0
    exact_auto_admitted = 0
    needs_review = 0
    verification_disagreements = 0
    critical_false_admissions = 0

    for document in documents:
        predicted_by_id: dict[str, PredictedIndicator] = {}
        for prediction in predictions.get(document.fixture_id, []):
            candidate_id = canonical_indicator_id(prediction.name)
            if candidate_id is not None and candidate_id not in predicted_by_id:
                predicted_by_id[candidate_id] = prediction

        for truth in document.indicators:
            truth_total += 1
            if truth.critical_numeric:
                critical_total += 1
            prediction = predicted_by_id.get(truth.indicator_id)
            if prediction is None:
                continue
            if prediction.admissibility_status == "needs_review":
                needs_review += 1
            if prediction.verification_status == "disagreement":
                verification_disagreements += 1
            if prediction.admissibility_status != "admissible":
                continue

            auto_admitted += 1
            value_ok = _normalize_token(prediction.value) == _normalize_token(truth.value)
            unit_ok = _normalize_token(prediction.unit) == _normalize_token(truth.unit)
            range_ok = _normalize_token(prediction.reference_range) == _normalize_token(
                truth.reference_range
            )
            if value_ok and unit_ok and range_ok:
                exact_auto_admitted += 1
            if truth.critical_numeric and not value_ok:
                critical_false_admissions += 1

    return EvidenceAuthoritySummary(
        truth_indicators=truth_total,
        auto_admitted=auto_admitted,
        exact_auto_admitted=exact_auto_admitted,
        needs_review=needs_review,
        verification_disagreements=verification_disagreements,
        critical_false_admissions=critical_false_admissions,
        auto_admission_coverage=auto_admitted / (truth_total or 1),
        auto_admission_exact_rate=exact_auto_admitted / (auto_admitted or 1),
        critical_false_admission_rate=critical_false_admissions / (critical_total or 1),
    )
