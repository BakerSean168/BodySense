"""OCR service using Tesseract for text extraction."""

import io
import logging
from pathlib import Path

import pytesseract
from PIL import Image

logger = logging.getLogger(__name__)


def extract_text_from_image(image_bytes: bytes) -> tuple[str, float]:
    """
    Extract text from an image using Tesseract OCR.

    Args:
        image_bytes: Raw image bytes

    Returns:
        Tuple of (extracted text, average confidence)
    """
    # Convert bytes to image
    image = Image.open(io.BytesIO(image_bytes))

    # Run OCR with Chinese and English support
    data = pytesseract.image_to_data(image, lang="chi_sim+eng", output_type=pytesseract.Output.DICT)

    # Extract text and confidence scores
    lines = []
    confidences = []
    current_line = []

    for i, text in enumerate(data["text"]):
        if text.strip():
            current_line.append(text)
            conf = int(data["conf"][i])
            if conf > 0:  # Tesseract uses -1 for low confidence
                confidences.append(conf / 100.0)

        # New line when we hit a line break
        if data["line_num"][i] != data["line_num"][min(i + 1, len(data["text"]) - 1)]:
            if current_line:
                lines.append(" ".join(current_line))
                current_line = []

    # Add last line
    if current_line:
        lines.append(" ".join(current_line))

    full_text = "\n".join(lines)
    avg_confidence = sum(confidences) / len(confidences) if confidences else 0.0

    return full_text, avg_confidence


def extract_text_from_pdf(pdf_bytes: bytes) -> tuple[str, float]:
    """
    Extract text from a PDF by converting pages to images and running OCR.

    Args:
        pdf_bytes: Raw PDF bytes

    Returns:
        Tuple of (extracted text, average confidence)
    """
    try:
        import fitz  # PyMuPDF
    except ImportError:
        logger.warning("PyMuPDF not installed, falling back to basic PDF extraction")
        return _extract_text_from_pdf_basic(pdf_bytes)

    doc = fitz.open(stream=pdf_bytes, filetype="pdf")
    all_text = []
    all_confidences = []

    for page_num in range(len(doc)):
        page = doc.load_page(page_num)
        # Render page to image (300 DPI for better OCR quality)
        pix = page.get_pixmap(dpi=300)
        img_bytes = pix.tobytes("png")

        text, confidence = extract_text_from_image(img_bytes)
        if text:
            all_text.append(f"--- Page {page_num + 1} ---\n{text}")
            all_confidences.append(confidence)

    doc.close()

    full_text = "\n\n".join(all_text)
    avg_confidence = (
        sum(all_confidences) / len(all_confidences) if all_confidences else 0.0
    )

    return full_text, avg_confidence


def _extract_text_from_pdf_basic(pdf_bytes: bytes) -> tuple[str, float]:
    """Basic PDF text extraction without OCR (for text-based PDFs)."""
    try:
        import fitz

        doc = fitz.open(stream=pdf_bytes, filetype="pdf")
        text_parts = []
        for page in doc:
            text_parts.append(page.get_text())
        doc.close()

        full_text = "\n".join(text_parts)
        return full_text, 0.8 if full_text.strip() else 0.0
    except ImportError:
        return "PDF processing requires PyMuPDF. Please install it.", 0.0


def extract_text(file_bytes: bytes, mime_type: str) -> tuple[str, float]:
    """
    Extract text from a file (image or PDF).

    Args:
        file_bytes: Raw file bytes
        mime_type: MIME type of the file

    Returns:
        Tuple of (extracted text, average confidence)
    """
    if mime_type == "application/pdf":
        return extract_text_from_pdf(file_bytes)
    elif mime_type.startswith("image/"):
        return extract_text_from_image(file_bytes)
    else:
        raise ValueError(f"Unsupported file type: {mime_type}")
