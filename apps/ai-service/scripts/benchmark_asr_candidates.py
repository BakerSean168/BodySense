"""Benchmark ASR candidates on local BodySense videos."""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import time
import urllib.request
import zipfile
from dataclasses import asdict, dataclass
from pathlib import Path

import psutil

PROJECT_ROOT = Path(__file__).resolve().parents[1]
DATA_ROOT = PROJECT_ROOT / "data" / "benchmarks" / "asr_field_test"
RUNTIME_ROOT = DATA_ROOT / "funasr_runtime"
RUNTIME_VERSION = "runtime-llamacpp-v0.1.1"
RUNTIME_ZIP_URL = (
    "https://github.com/modelscope/FunASR/releases/download/"
    f"{RUNTIME_VERSION}/funasr-llamacpp-windows-x64.zip"
)

VIDEOS = [
    {
        "slug": "humerus-forward",
        "title": "肱骨前移",
        "path": r"C:\Users\baker\Videos\凯圣王\肱骨前移.mp4",
    },
    {
        "slug": "cubitus-valgus",
        "title": "肘外翻",
        "path": r"C:\Users\baker\Videos\凯圣王\肘外翻.mp4",
    },
]

CANDIDATES = {
    "fw_turbo": {
        "label": "faster-whisper large-v3-turbo",
        "kind": "faster_whisper",
        "model_id": "dropbox-dash/faster-whisper-large-v3-turbo",
        "download_repo": "dropbox-dash/faster-whisper-large-v3-turbo",
        "compute_type": "int8",
    },
    "fw_large_v3": {
        "label": "faster-whisper large-v3",
        "kind": "faster_whisper",
        "model_id": "large-v3",
        "compute_type": "int8",
    },
    "funasr_paraformer": {
        "label": "FunASR Paraformer-zh GGUF",
        "kind": "funasr_runtime",
        "binary_name": "llama-funasr-paraformer.exe",
        "model_repo": "FunAudioLLM/Paraformer-GGUF",
        "model_file": "paraformer-q8.gguf",
    },
    "funasr_sensevoice": {
        "label": "FunASR SenseVoiceSmall GGUF",
        "kind": "funasr_runtime",
        "binary_name": "llama-funasr-sensevoice.exe",
        "model_repo": "FunAudioLLM/SenseVoiceSmall-GGUF",
        "model_file": "sensevoice-small-q8.gguf",
    },
}

VAD_REPO = "FunAudioLLM/fsmn-vad-GGUF"
VAD_FILE = "fsmn-vad.gguf"


@dataclass(frozen=True)
class CandidateResult:
    candidate_id: str
    candidate_label: str
    video_slug: str
    video_title: str
    audio_path: str
    transcript_path: str
    raw_output_path: str
    log_path: str
    elapsed_sec: float
    peak_rss_mb: float
    return_code: int
    transcript_chars: int
    excerpt: str
    model_artifact_path: str | None = None
    model_artifact_size_mb: float | None = None


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--video",
        choices=[video["slug"] for video in VIDEOS],
        action="append",
        help="Run a subset of videos",
    )
    parser.add_argument(
        "--candidate",
        choices=sorted(CANDIDATES),
        action="append",
        help="Run a subset of candidates",
    )
    parser.add_argument(
        "--child-run",
        action="store_true",
        help="Internal mode: execute exactly one candidate for one video",
    )
    parser.add_argument(
        "--force",
        action="store_true",
        help="Re-run candidates even if a result.json already exists",
    )
    parser.add_argument("--child-video", choices=[video["slug"] for video in VIDEOS])
    parser.add_argument("--child-candidate", choices=sorted(CANDIDATES))
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    if args.child_run:
        if not args.child_video or not args.child_candidate:
            raise SystemExit("--child-video and --child-candidate are required with --child-run")
        return run_child(args.child_video, args.child_candidate)

    selected_videos = select_videos(args.video)
    selected_candidates = select_candidates(args.candidate)
    DATA_ROOT.mkdir(parents=True, exist_ok=True)

    all_results: list[CandidateResult] = []
    for video in selected_videos:
        audio_path = ensure_audio(video)
        for candidate_id in selected_candidates:
            result = run_parent_candidate(video, candidate_id, audio_path, force=args.force)
            all_results.append(result)

    summary_path = DATA_ROOT / "summary.json"
    summary_path.write_text(
        json.dumps([asdict(result) for result in all_results], ensure_ascii=False, indent=2),
        encoding="utf-8",
    )
    print(f"Summary written to {summary_path}")
    for result in all_results:
        print(
            f"{result.video_title} | {result.candidate_label} | "
            f"{result.elapsed_sec:.2f}s | peak {result.peak_rss_mb:.2f} MB | "
            f"chars {result.transcript_chars}"
        )
    return 0


def select_videos(slugs: list[str] | None) -> list[dict[str, str]]:
    if not slugs:
        return VIDEOS
    allowed = set(slugs)
    return [video for video in VIDEOS if video["slug"] in allowed]


def select_candidates(candidate_ids: list[str] | None) -> list[str]:
    if not candidate_ids:
        return list(CANDIDATES)
    return candidate_ids


def ensure_audio(video: dict[str, str]) -> Path:
    output_dir = DATA_ROOT / video["slug"]
    output_dir.mkdir(parents=True, exist_ok=True)
    audio_path = output_dir / "audio.wav"
    if audio_path.exists():
        return audio_path

    command = [
        "ffmpeg",
        "-hide_banner",
        "-loglevel",
        "error",
        "-y",
        "-i",
        video["path"],
        "-vn",
        "-ac",
        "1",
        "-ar",
        "16000",
        str(audio_path),
    ]
    subprocess.run(command, check=True)
    return audio_path


def run_parent_candidate(
    video: dict[str, str],
    candidate_id: str,
    audio_path: Path,
    force: bool,
) -> CandidateResult:
    candidate = CANDIDATES[candidate_id]
    output_dir = DATA_ROOT / video["slug"] / candidate_id
    output_dir.mkdir(parents=True, exist_ok=True)
    log_path = output_dir / "run.log"
    result_path = output_dir / "result.json"

    if result_path.exists() and not force:
        payload = json.loads(result_path.read_text(encoding="utf-8"))
        transcript_text = Path(payload["transcript_path"]).read_text(
            encoding="utf-8",
            errors="replace",
        )
        return CandidateResult(
            candidate_id=candidate_id,
            candidate_label=candidate["label"],
            video_slug=video["slug"],
            video_title=video["title"],
            audio_path=str(audio_path),
            transcript_path=payload["transcript_path"],
            raw_output_path=payload["raw_output_path"],
            log_path=str(log_path),
            elapsed_sec=float(payload["elapsed_sec"]),
            peak_rss_mb=float(payload.get("peak_rss_mb", 0.0)),
            return_code=int(payload.get("return_code", 0)),
            transcript_chars=len(transcript_text),
            excerpt=transcript_text[:280],
            model_artifact_path=payload.get("model_artifact_path"),
            model_artifact_size_mb=payload.get("model_artifact_size_mb"),
        )

    child_args = [
        sys.executable,
        str(Path(__file__).resolve()),
        "--child-run",
        "--child-video",
        video["slug"],
        "--child-candidate",
        candidate_id,
    ]

    benchmark_start = time.perf_counter()
    launch_time = time.time()
    peak_rss = 0
    with log_path.open("w", encoding="utf-8", errors="replace") as log_file:
        proc = subprocess.Popen(
            child_args,
            cwd=PROJECT_ROOT,
            stdout=log_file,
            stderr=subprocess.STDOUT,
            text=True,
        )
        parent_proc = psutil.Process(proc.pid)
        while proc.poll() is None:
            peak_rss = max(peak_rss, measure_tree_rss(parent_proc, launch_time))
            time.sleep(0.5)
        peak_rss = max(peak_rss, measure_tree_rss(parent_proc, launch_time))

    elapsed_sec = time.perf_counter() - benchmark_start
    if proc.returncode != 0:
        raise RuntimeError(
            f"{video['title']} / {candidate['label']} failed with code {proc.returncode}. "
            f"See {log_path}"
        )
    payload = json.loads(result_path.read_text(encoding="utf-8"))
    payload["peak_rss_mb"] = round(peak_rss / 1024 / 1024, 2)
    payload["return_code"] = proc.returncode
    result_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
    transcript_text = Path(payload["transcript_path"]).read_text(encoding="utf-8", errors="replace")

    return CandidateResult(
        candidate_id=candidate_id,
        candidate_label=candidate["label"],
        video_slug=video["slug"],
        video_title=video["title"],
        audio_path=str(audio_path),
        transcript_path=payload["transcript_path"],
        raw_output_path=payload["raw_output_path"],
        log_path=str(log_path),
        elapsed_sec=round(elapsed_sec, 2),
        peak_rss_mb=round(peak_rss / 1024 / 1024, 2),
        return_code=proc.returncode,
        transcript_chars=len(transcript_text),
        excerpt=transcript_text[:280],
        model_artifact_path=payload.get("model_artifact_path"),
        model_artifact_size_mb=payload.get("model_artifact_size_mb"),
    )


def measure_tree_rss(parent_proc: psutil.Process, launch_time: float) -> int:
    try:
        procs = [parent_proc, *parent_proc.children(recursive=True)]
    except psutil.Error:
        return 0

    total = 0
    for proc in procs:
        try:
            if proc.create_time() + 2 < launch_time:
                continue
            total += proc.memory_info().rss
        except psutil.Error:
            continue
    return total


def run_child(video_slug: str, candidate_id: str) -> int:
    video = next(video for video in VIDEOS if video["slug"] == video_slug)
    output_dir = DATA_ROOT / video_slug / candidate_id
    output_dir.mkdir(parents=True, exist_ok=True)
    audio_path = ensure_audio(video)
    candidate = CANDIDATES[candidate_id]

    if candidate["kind"] == "faster_whisper":
        payload = run_faster_whisper_candidate(candidate_id, candidate, audio_path, output_dir)
    else:
        payload = run_funasr_runtime_candidate(candidate_id, candidate, audio_path, output_dir)

    result_path = output_dir / "result.json"
    result_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(payload, ensure_ascii=False))
    return 0


def run_faster_whisper_candidate(
    candidate_id: str,
    candidate: dict[str, str],
    audio_path: Path,
    output_dir: Path,
) -> dict[str, object]:
    from faster_whisper import WhisperModel
    from faster_whisper.utils import download_model

    cache_dir = DATA_ROOT / ".cache" / "faster_whisper" / candidate_id
    cache_dir.mkdir(parents=True, exist_ok=True)
    if candidate.get("download_repo"):
        model_path = ensure_ct2_model(candidate["download_repo"], cache_dir)
    else:
        model_path = Path(download_model(candidate["model_id"], output_dir=str(cache_dir)))

    started_at = time.perf_counter()
    model = WhisperModel(
        str(model_path),
        device="cpu",
        compute_type=candidate["compute_type"],
        cpu_threads=os.cpu_count() or 8,
    )
    segments_iter, info = model.transcribe(
        str(audio_path),
        language="zh",
        beam_size=5,
        condition_on_previous_text=False,
        vad_filter=False,
    )
    segments = list(segments_iter)
    elapsed_sec = time.perf_counter() - started_at

    transcript_path = output_dir / "transcript.txt"
    raw_output_path = output_dir / "segments.json"
    transcript_lines = [
        f"[{format_seconds(segment.start)}-{format_seconds(segment.end)}] {segment.text.strip()}"
        for segment in segments
    ]
    transcript_path.write_text("\n".join(transcript_lines), encoding="utf-8")
    raw_output_path.write_text(
        json.dumps(
            [
                {
                    "start": segment.start,
                    "end": segment.end,
                    "text": segment.text,
                }
                for segment in segments
            ],
            ensure_ascii=False,
            indent=2,
        ),
        encoding="utf-8",
    )

    return {
        "candidate_id": candidate_id,
        "candidate_label": candidate["label"],
        "elapsed_sec": round(elapsed_sec, 2),
        "peak_rss_mb": None,
        "return_code": 0,
        "transcript_path": str(transcript_path),
        "raw_output_path": str(raw_output_path),
        "detected_language": info.language,
        "model_artifact_path": str(model_path),
        "model_artifact_size_mb": round(directory_size(model_path) / 1024 / 1024, 2),
    }


def run_funasr_runtime_candidate(
    candidate_id: str,
    candidate: dict[str, str],
    audio_path: Path,
    output_dir: Path,
) -> dict[str, object]:
    runtime_dir = ensure_funasr_runtime()
    model_path = ensure_hf_file(
        candidate["model_repo"],
        candidate["model_file"],
        DATA_ROOT / ".cache" / "funasr_models" / candidate_id,
    )
    vad_path = ensure_hf_file(
        VAD_REPO,
        VAD_FILE,
        DATA_ROOT / ".cache" / "funasr_models" / "vad",
    )
    binary_path = runtime_dir / candidate["binary_name"]

    command = [
        str(binary_path),
        "-m",
        str(model_path),
        "-a",
        str(audio_path),
        "--vad",
        str(vad_path),
    ]
    started_at = time.perf_counter()
    completed = subprocess.run(
        command,
        check=True,
        capture_output=True,
        text=True,
        encoding="utf-8",
        errors="replace",
    )
    elapsed_sec = time.perf_counter() - started_at

    transcript_path = output_dir / "transcript.txt"
    raw_output_path = output_dir / "stdout.txt"
    stdout_text = completed.stdout.strip()
    transcript_path.write_text(stdout_text, encoding="utf-8")
    raw_output_path.write_text(stdout_text, encoding="utf-8")

    return {
        "candidate_id": candidate_id,
        "candidate_label": candidate["label"],
        "elapsed_sec": round(elapsed_sec, 2),
        "peak_rss_mb": None,
        "return_code": 0,
        "transcript_path": str(transcript_path),
        "raw_output_path": str(raw_output_path),
        "model_artifact_path": str(model_path),
        "model_artifact_size_mb": round(model_path.stat().st_size / 1024 / 1024, 2),
    }


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


if __name__ == "__main__":
    raise SystemExit(main())
