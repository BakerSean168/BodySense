"""Benchmark engine adapters for health-document extraction candidates."""

from .rapidocr_ppocrv6 import run_rapidocr_fixture
from .rapidocr_ppocrv6_bounded import run_rapidocr_bounded_fixture
from .tesseract_baseline import run_tesseract_fixture

__all__ = [
    "run_rapidocr_bounded_fixture",
    "run_rapidocr_fixture",
    "run_tesseract_fixture",
]
