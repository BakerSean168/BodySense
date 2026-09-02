"""Parent-side bounded orchestration for health-document extraction.

Historical configurations run one short-lived extraction worker. Verified
configurations run two sequential subprocesses: the primary RapidOCR/native-PDF
worker exits first, then a Tesseract verifier runs only for OCR-derived pages.
This keeps ONNX and Tesseract native memory from overlapping in the ai-service
cgroup while preserving an explicit independent-consensus evidence boundary.
"""

from __future__ import annotations

import asyncio
import hashlib
import sys
from pathlib import Path

from ..configuration.health_document_config import get_health_document_configuration
from ..models.ocr import HealthDocumentVerifierResponse, OCRResponse
from .verification import finalize_verified_response

WORKER_TIMEOUT_SECONDS = 180.0
MAX_WORKER_OUTPUT_BYTES = 20 * 1024 * 1024
MAX_VERIFIER_OUTPUT_BYTES = 2 * 1024 * 1024
LEGACY_TESSERACT_CONFIGURATION_ID = "hdex-config-14af808ef184bf8b"


class HealthDocumentWorkerError(RuntimeError):
    pass


def orchestrator_source_sha256() -> str:
    return hashlib.sha256(Path(__file__).read_bytes()).hexdigest()


async def _run_module(
    module: str,
    args: list[str],
    file_bytes: bytes,
    *,
    timeout_seconds: float,
    max_output_bytes: int,
    label: str,
) -> bytes:
    process = await asyncio.create_subprocess_exec(
        sys.executable,
        "-m",
        module,
        *args,
        stdin=asyncio.subprocess.PIPE,
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.PIPE,
    )
    try:
        stdout, _stderr = await asyncio.wait_for(
            process.communicate(input=file_bytes),
            timeout=timeout_seconds,
        )
    except TimeoutError as exc:
        process.kill()
        await process.wait()
        raise HealthDocumentWorkerError(f"{label} timed out") from exc
    if process.returncode != 0:
        # stderr is intentionally not surfaced: third-party OCR libraries can
        # include source-derived text in diagnostics.
        raise HealthDocumentWorkerError(f"{label} failed with exit code {process.returncode}")
    if len(stdout) > max_output_bytes:
        raise HealthDocumentWorkerError(f"{label} output exceeds bounded size")
    return stdout


async def run_health_document_worker(
    file_bytes: bytes,
    mime_type: str,
    configuration_id: str,
    *,
    timeout_seconds: float = WORKER_TIMEOUT_SECONDS,
) -> OCRResponse:
    primary_stdout = await _run_module(
        "src.document_pipeline.worker",
        ["--configuration-id", configuration_id, "--mime-type", mime_type],
        file_bytes,
        timeout_seconds=timeout_seconds,
        max_output_bytes=MAX_WORKER_OUTPUT_BYTES,
        label="health-document primary worker",
    )
    try:
        primary = OCRResponse.model_validate_json(primary_stdout)
    except Exception as exc:
        raise HealthDocumentWorkerError(
            "health-document primary worker returned invalid JSON"
        ) from exc

    if configuration_id == LEGACY_TESSERACT_CONFIGURATION_ID:
        return primary

    try:
        config = get_health_document_configuration(configuration_id)
    except ValueError as exc:
        raise HealthDocumentWorkerError("unknown health-document configuration") from exc
    if config.verification_revision is None:
        return primary
    if config.orchestrator_sha256 != orchestrator_source_sha256():
        raise HealthDocumentWorkerError("health-document orchestrator source identity mismatch")

    ocr_pages = sorted({page.page for page in primary.result.pages if page.method == "rapidocr"})
    if ocr_pages:
        verifier_stdout = await _run_module(
            "src.document_pipeline.verifier_worker",
            [
                "--configuration-id",
                configuration_id,
                "--mime-type",
                mime_type,
                "--pages",
                ",".join(str(page) for page in ocr_pages),
            ],
            file_bytes,
            timeout_seconds=timeout_seconds,
            max_output_bytes=MAX_VERIFIER_OUTPUT_BYTES,
            label="health-document verifier worker",
        )
        try:
            verifier = HealthDocumentVerifierResponse.model_validate_json(verifier_stdout)
        except Exception as exc:
            raise HealthDocumentWorkerError(
                "health-document verifier worker returned invalid JSON"
            ) from exc
        if verifier.configuration_id != configuration_id:
            raise HealthDocumentWorkerError("health-document verifier configuration mismatch")
        if verifier.verification_revision != config.verification_revision:
            raise HealthDocumentWorkerError("health-document verifier revision mismatch")
    else:
        verifier = HealthDocumentVerifierResponse(
            configuration_id=configuration_id,
            verification_revision=config.verification_revision,
            indicators=[],
        )

    return finalize_verified_response(
        primary,
        verifier,
        policy_revision=config.admissibility_policy_revision,
    )
