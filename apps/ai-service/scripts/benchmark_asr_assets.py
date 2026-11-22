"""Asset preparation helpers for the ASR candidate benchmark.

Downloads and caches the FunASR runtime, Hugging Face model files and
CTranslate2 model bundles used by benchmark_asr_candidates.py.
"""

from __future__ import annotations

import urllib.request
import zipfile
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parents[1]
DATA_ROOT = PROJECT_ROOT / "data" / "benchmarks" / "asr_field_test"
RUNTIME_ROOT = DATA_ROOT / "funasr_runtime"
RUNTIME_VERSION = "runtime-llamacpp-v0.1.1"
RUNTIME_ZIP_URL = (
    "https://github.com/modelscope/FunASR/releases/download/"
    f"{RUNTIME_VERSION}/funasr-llamacpp-windows-x64.zip"
)


def ensure_funasr_runtime() -> Path:
    runtime_dir = RUNTIME_ROOT / "windows-x64"
    if runtime_dir.exists():
        return runtime_dir

    RUNTIME_ROOT.mkdir(parents=True, exist_ok=True)
    zip_path = RUNTIME_ROOT / "funasr-llamacpp-windows-x64.zip"
    if not zip_path.exists():
        download_file(RUNTIME_ZIP_URL, zip_path)
    with zipfile.ZipFile(zip_path, "r") as archive:
        archive.extractall(runtime_dir)
    return runtime_dir


def ensure_hf_file(repo_id: str, file_name: str, output_dir: Path) -> Path:
    output_dir.mkdir(parents=True, exist_ok=True)
    target = output_dir / file_name
    if target.exists():
        return target

    url = f"https://huggingface.co/{repo_id}/resolve/main/{file_name}"
    download_file(url, target)
    return target


def ensure_ct2_model(repo_id: str, output_dir: Path) -> Path:
    required_files = [
        "config.json",
        "preprocessor_config.json",
        "tokenizer.json",
        "vocabulary.json",
        "model.bin",
    ]
    for file_name in required_files:
        ensure_hf_file(repo_id, file_name, output_dir)
    return output_dir


def download_file(url: str, target: Path) -> None:
    target.parent.mkdir(parents=True, exist_ok=True)
    with urllib.request.urlopen(url) as response, target.open("wb") as output:
        while True:
            chunk = response.read(1024 * 1024)
            if not chunk:
                break
            output.write(chunk)


def directory_size(path: Path) -> int:
    if path.is_file():
        return path.stat().st_size
    return sum(item.stat().st_size for item in path.rglob("*") if item.is_file())


def format_seconds(value: float) -> str:
    total = max(0, int(round(value)))
    minutes, seconds = divmod(total, 60)
    hours, minutes = divmod(minutes, 60)
    if hours:
        return f"{hours:02d}:{minutes:02d}:{seconds:02d}"
    return f"{minutes:02d}:{seconds:02d}"
