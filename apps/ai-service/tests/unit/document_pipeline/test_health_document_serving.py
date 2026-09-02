from __future__ import annotations

import fitz

from src.document_pipeline import serving_engine
from src.document_pipeline.serving_engine import ExtractedDocument
from src.document_pipeline.structured_indicator_parser import SourceTextBlock
from src.models.ocr import DocumentPageEvidence, DocumentSourceBlock


def _pdf_bytes(*, second_blank: bool = False) -> bytes:
    doc = fitz.open()
    page = doc.new_page()
    page.insert_text((72, 72), "Health report Vitamin D 17.8 ng/mL reference 30-100")
    if second_blank:
        doc.new_page()
    payload = doc.tobytes(no_new_id=True)
    doc.close()
    return payload


def test_born_digital_pdf_uses_native_text_without_ocr(monkeypatch):
    monkeypatch.setattr(serving_engine, "verify_health_document_runtime", lambda _config: {})

    def unexpected(*_args, **_kwargs):
        raise AssertionError("born-digital page must not invoke OCR")

    monkeypatch.setattr(serving_engine, "_rapidocr_blocks", unexpected)
    result = serving_engine.extract_document(_pdf_bytes(), "application/pdf")
    assert len(result.pages) == 1
    assert result.pages[0].method == "native_pdf_text"
    assert result.pages[0].confidence == 1.0
    assert result.source_blocks[0].coordinate_space == "pdf_points"


def test_mixed_pdf_uses_native_then_ocr_fallback(monkeypatch):
    monkeypatch.setattr(serving_engine, "verify_health_document_runtime", lambda _config: {})

    seen_configuration_ids: list[str] = []

    def fake_ocr(_bytes: bytes, *, page: int, configuration_id: str):
        seen_configuration_ids.append(configuration_id)
        block = SourceTextBlock(
            source_ref=f"page:{page}:ocr-block:1",
            page=page,
            text="血红蛋白 142 g/L 参考范围 130-175",
        )
        source = DocumentSourceBlock(
            source_ref=block.source_ref,
            page=page,
            method="rapidocr",
            bbox=[0.0] * 8,
            coordinate_space="ocr_pixels",
            confidence=0.97,
            text_sha256="1" * 64,
        )
        return [block], [source], 0.97

    monkeypatch.setattr(serving_engine, "_rapidocr_blocks", fake_ocr)
    result = serving_engine.extract_document(_pdf_bytes(second_blank=True), "application/pdf")
    assert [page.method for page in result.pages] == ["native_pdf_text", "rapidocr"]
    assert result.pages[1].native_text_quality_reason_codes == ["empty_native_text"]
    from src.configuration.health_document_config import get_default_health_document_configuration

    assert seen_configuration_ids == [get_default_health_document_configuration().configuration_id]


def test_worker_response_attaches_provenance_sources_and_admissibility(monkeypatch):
    from src.configuration.health_document_config import get_default_health_document_configuration
    from src.document_pipeline import worker

    config = get_default_health_document_configuration()
    monkeypatch.setattr(
        worker,
        "verify_health_document_runtime",
        lambda _config: {"configuration_id": config.configuration_id},
    )
    monkeypatch.setattr(worker, "worker_source_sha256", lambda: config.worker_sha256)
    monkeypatch.setattr(
        worker,
        "extract_document",
        lambda _bytes, _mime, _config: ExtractedDocument(
            raw_text="维生素D 17.8 ng/mL 参考范围 30-100",
            confidence=0.98,
            parser_blocks=[
                SourceTextBlock(
                    source_ref="page:1:ocr-block:1",
                    page=1,
                    text="维生素D 17.8 ng/mL 参考范围 30-100",
                )
            ],
            pages=[
                DocumentPageEvidence(
                    page=1,
                    method="rapidocr",
                    source_refs=["page:1:ocr-block:1"],
                    confidence=0.98,
                )
            ],
            source_blocks=[
                DocumentSourceBlock(
                    source_ref="page:1:ocr-block:1",
                    page=1,
                    method="rapidocr",
                    bbox=[0.0] * 8,
                    coordinate_space="ocr_pixels",
                    confidence=0.98,
                    text_sha256="1" * 64,
                )
            ],
        ),
    )
    response = worker.build_response(b"input", "image/png", config.configuration_id)
    indicator = response.result.indicators[0]
    assert indicator.indicator_id == "vitamin_d"
    assert indicator.evidence_admissibility.status == "needs_review"
    assert indicator.evidence_verification is not None
    assert indicator.evidence_verification.status == "pending"
    assert indicator.evidence_verification.revision == config.verification_revision
    assert indicator.source_refs == ["page:1:ocr-block:1"]
    assert response.result.mechanism_provenance is not None
    assert response.result.mechanism_provenance.configuration_id == config.configuration_id
    assert (
        response.result.mechanism_provenance.execution_topology_revision
        == config.execution_topology_revision
    )


def test_ocr_cells_are_reconstructed_into_visual_rows_with_original_source_refs() -> None:
    cells = [
        serving_engine._OCRCell(
            source_ref="page:1:ocr-block:1",
            text="维生素D",
            bbox=(10.0, 100.0, 100.0, 100.0, 100.0, 140.0, 10.0, 140.0),
        ),
        serving_engine._OCRCell(
            source_ref="page:1:ocr-block:2",
            text="17.8",
            bbox=(200.0, 102.0, 250.0, 102.0, 250.0, 140.0, 200.0, 140.0),
        ),
        serving_engine._OCRCell(
            source_ref="page:1:ocr-block:3",
            text="ng/mL",
            bbox=(300.0, 101.0, 360.0, 101.0, 360.0, 141.0, 300.0, 141.0),
        ),
        serving_engine._OCRCell(
            source_ref="page:1:ocr-block:4",
            text="30-100",
            bbox=(420.0, 103.0, 500.0, 103.0, 500.0, 142.0, 420.0, 142.0),
        ),
        serving_engine._OCRCell(
            source_ref="page:1:ocr-block:5",
            text="铁蛋白",
            bbox=(10.0, 180.0, 100.0, 180.0, 100.0, 220.0, 10.0, 220.0),
        ),
    ]
    rows = serving_engine._reconstruct_ocr_rows(cells, page=1)
    assert [row.text for row in rows] == ["维生素D 17.8 ng/mL 30-100", "铁蛋白"]
    assert rows[0].source_refs == (
        "page:1:ocr-block:1",
        "page:1:ocr-block:2",
        "page:1:ocr-block:3",
        "page:1:ocr-block:4",
    )
