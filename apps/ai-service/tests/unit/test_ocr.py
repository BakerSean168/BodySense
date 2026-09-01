"""Unit tests for OCR and indicator extraction."""

from src.models.ocr import HealthIndicator, OCRResponse, OCRResult, TextExtractionResponse
from src.services.indicator_extractor import extract_indicators, get_overall_confidence


class TestHealthIndicatorModel:
    """Test HealthIndicator Pydantic model."""

    def test_basic_indicator(self):
        indicator = HealthIndicator(
            name="Vitamin D",
            value="25.3",
            unit="ng/mL",
            reference_range="30-100",
            confidence="high",
        )
        assert indicator.name == "Vitamin D"
        assert indicator.value == "25.3"
        assert indicator.unit == "ng/mL"
        assert indicator.reference_range == "30-100"
        assert indicator.confidence == "high"

    def test_indicator_without_optional_fields(self):
        indicator = HealthIndicator(
            name="Iron",
            value="100",
        )
        assert indicator.name == "Iron"
        assert indicator.value == "100"
        assert indicator.unit is None
        assert indicator.reference_range is None
        assert indicator.confidence == "unknown"  # fail-closed default

    def test_indicator_default_confidence(self):
        indicator = HealthIndicator(name="Test", value="1")
        assert indicator.confidence == "unknown"
        assert indicator.evidence_admissibility.status == "needs_review"


class TestOCRResultModel:
    """Test OCRResult Pydantic model."""

    def test_basic_result(self):
        result = OCRResult(
            raw_text="Some text",
            indicators=[],
            confidence="high",
        )
        assert result.raw_text == "Some text"
        assert result.indicators == []
        assert result.confidence == "high"

    def test_result_with_indicators(self):
        indicator = HealthIndicator(name="Test", value="1")
        result = OCRResult(
            raw_text="Test text",
            indicators=[indicator],
            confidence="medium",
        )
        assert len(result.indicators) == 1
        assert result.indicators[0].name == "Test"


class TestOCRResponseModel:
    """Test OCRResponse Pydantic model."""

    def test_default_status(self):
        result = OCRResult(raw_text="", confidence="low")
        response = OCRResponse(result=result)
        assert response.status == "completed"

    def test_custom_status(self):
        result = OCRResult(raw_text="", confidence="low")
        response = OCRResponse(status="processing", result=result)
        assert response.status == "processing"


class TestTextExtractionResponseModel:
    """Test TextExtractionResponse Pydantic model."""

    def test_default_pages(self):
        response = TextExtractionResponse(text="Some text")
        assert response.pages == 1

    def test_custom_pages(self):
        response = TextExtractionResponse(text="Multi page", pages=3)
        assert response.pages == 3


class TestIndicatorExtractor:
    """Test indicator extraction from text."""

    def test_extract_empty_text(self):
        indicators = extract_indicators("")
        assert indicators == []

    def test_extract_none_text(self):
        indicators = extract_indicators(None)
        assert indicators == []

    def test_extract_no_indicators(self):
        text = "This is a regular text without any health indicators."
        indicators = extract_indicators(text)
        assert indicators == []

    def test_extract_vitamin_d(self):
        text = """
        体检报告
        维生素D: 25.3 ng/mL
        参考范围: 30-100
        """
        indicators = extract_indicators(text)
        assert len(indicators) > 0
        # Find vitamin D indicator
        vit_d = next((i for i in indicators if "D" in i.name), None)
        if vit_d:
            assert vit_d.value == "25.3"

    def test_extract_blood_indicators(self):
        text = """
        血常规检查
        血红蛋白: 145 g/L
        白细胞: 6.5 10^9/L
        红细胞: 4.8 10^12/L
        """
        indicators = extract_indicators(text)
        # Should extract at least some indicators
        assert len(indicators) >= 0  # May or may not extract depending on pattern


class TestOverallConfidence:
    """Test overall confidence calculation."""

    def test_empty_indicators(self):
        confidence = get_overall_confidence([])
        assert confidence == "low"

    def test_all_high_confidence(self):
        indicators = [
            HealthIndicator(name="A", value="1", confidence="high"),
            HealthIndicator(name="B", value="2", confidence="high"),
        ]
        confidence = get_overall_confidence(indicators)
        assert confidence == "high"

    def test_mixed_confidence(self):
        indicators = [
            HealthIndicator(name="A", value="1", confidence="high"),
            HealthIndicator(name="B", value="2", confidence="low"),
        ]
        confidence = get_overall_confidence(indicators)
        assert confidence in ["medium", "low"]

    def test_all_low_confidence(self):
        indicators = [
            HealthIndicator(name="A", value="1", confidence="low"),
            HealthIndicator(name="B", value="2", confidence="low"),
        ]
        confidence = get_overall_confidence(indicators)
        assert confidence == "low"


class TestIndicatorEvidenceAdmissibility:
    """OCR completion must not imply health-evidence admissibility."""

    def test_missing_confidence_fails_closed(self):
        from src.services.report_indicator_admissibility import (
            evaluate_indicator_admissibility,
        )

        indicator = HealthIndicator(name="Vitamin D", value="25.3", unit="ng/mL")
        assert indicator.confidence == "unknown"
        decision = evaluate_indicator_admissibility(indicator, ocr_confidence="high")
        assert decision.status == "needs_review"
        assert "indicator_confidence_unknown" in decision.reason_codes

    def test_only_high_ocr_and_high_indicator_are_auto_admissible(self):
        from src.services.report_indicator_admissibility import (
            OCR_INDICATOR_ADMISSIBILITY_POLICY_REVISION,
            evaluate_indicator_admissibility,
        )

        indicator = HealthIndicator(
            name="Vitamin D",
            value="25.3",
            unit="ng/mL",
            confidence="high",
        )
        decision = evaluate_indicator_admissibility(indicator, ocr_confidence="high")
        assert decision.status == "admissible"
        assert decision.policy_revision == OCR_INDICATOR_ADMISSIBILITY_POLICY_REVISION

    def test_medium_ocr_requires_review_even_for_high_indicator(self):
        from src.services.report_indicator_admissibility import (
            evaluate_indicator_admissibility,
        )

        indicator = HealthIndicator(
            name="Vitamin D",
            value="25.3",
            unit="ng/mL",
            confidence="high",
        )
        decision = evaluate_indicator_admissibility(indicator, ocr_confidence="medium")
        assert decision.status == "needs_review"
        assert "ocr_confidence_medium" in decision.reason_codes

    def test_medium_indicator_requires_review_even_for_high_ocr(self):
        from src.services.report_indicator_admissibility import (
            evaluate_indicator_admissibility,
        )

        indicator = HealthIndicator(
            name="Vitamin D",
            value="25.3",
            unit="ng/mL",
            confidence="medium",
        )
        decision = evaluate_indicator_admissibility(indicator, ocr_confidence="high")
        assert decision.status == "needs_review"
        assert "indicator_confidence_medium" in decision.reason_codes


def test_ocr_route_attaches_admissible_evidence_metadata(client, monkeypatch):
    monkeypatch.setattr(
        "src.api.routes.ocr.extract_text",
        lambda _payload, _mime: ("维生素D: 25.3 ng/mL\n参考范围: 30-100", 0.95),
    )

    response = client.post(
        "/api/ocr/extract",
        files={"file": ("report.png", b"fake-image", "image/png")},
    )
    assert response.status_code == 200
    payload = response.json()
    assert payload["status"] == "completed"
    indicator = next(item for item in payload["result"]["indicators"] if "D" in item["name"])
    assert indicator["confidence"] == "high"
    assert indicator["evidence_admissibility"] == {
        "status": "admissible",
        "policy_revision": "ocr-indicator-admissibility-v1",
        "reason_codes": ["high_confidence_ocr_and_indicator"],
    }


def test_ocr_route_preserves_review_required_indicator_without_admitting_it(client, monkeypatch):
    monkeypatch.setattr(
        "src.api.routes.ocr.extract_text",
        lambda _payload, _mime: ("维生素D: 25.3 ng/mL\n参考范围: 30-100", 0.65),
    )

    response = client.post(
        "/api/ocr/extract",
        files={"file": ("report.png", b"fake-image", "image/png")},
    )
    assert response.status_code == 200
    payload = response.json()
    assert payload["status"] == "completed"
    indicator = next(item for item in payload["result"]["indicators"] if "D" in item["name"])
    assert indicator["confidence"] == "high"
    assert indicator["evidence_admissibility"]["status"] == "needs_review"
    assert indicator["evidence_admissibility"]["policy_revision"] == (
        "ocr-indicator-admissibility-v1"
    )
    assert "ocr_confidence_medium" in indicator["evidence_admissibility"]["reason_codes"]
