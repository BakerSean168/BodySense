"""Generate deterministic, non-user health-document benchmark fixtures.

Generated assets live under ``data/benchmarks`` and are gitignored. The committed
corpus spec is the reproducible source of truth; the generated manifest records
asset/font hashes so a run can prove what it used.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import random
import sys
from pathlib import Path
from typing import Any

import fitz
from PIL import Image, ImageDraw, ImageEnhance, ImageFilter, ImageFont

SERVICE_ROOT = Path(__file__).resolve().parents[1]
if str(SERVICE_ROOT) not in sys.path:
    sys.path.insert(0, str(SERVICE_ROOT))

from src.document_pipeline.contracts import CorpusDocument, CorpusIndicatorTruth  # noqa: E402
from src.document_pipeline.corpus import write_manifest  # noqa: E402

SPEC_PATH = SERVICE_ROOT / "benchmarks" / "health_document" / "corpus_spec.json"
DEFAULT_OUTPUT_ROOT = SERVICE_ROOT / "data" / "benchmarks" / "health_document" / "synthetic-v1"
FONT_CANDIDATES = (
    Path("/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"),
    Path("/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc"),
)

INDICATORS = (
    ("vitamin_d", "维生素D", "17.8", "ng/mL", "30-100"),
    ("ferritin", "铁蛋白", "86.4", "ng/mL", "30-400"),
    ("hemoglobin", "血红蛋白", "142", "g/L", "130-175"),
    ("wbc", "白细胞", "6.21", "10^9/L", "3.5-9.5"),
    ("rbc", "红细胞", "4.82", "10^12/L", "4.3-5.8"),
    ("platelet", "血小板", "238", "10^9/L", "125-350"),
    ("glucose", "血糖", "5.18", "mmol/L", "3.9-6.1"),
    ("cholesterol", "胆固醇", "4.36", "mmol/L", "0-5.2"),
    ("triglyceride", "甘油三酯", "1.12", "mmol/L", "0-1.7"),
    ("uric_acid", "尿酸", "398", "umol/L", "208-428"),
    ("creatinine", "肌酐", "78", "umol/L", "57-111"),
    ("bun", "尿素氮", "5.4", "mmol/L", "3.1-8.0"),
)


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output-root", type=Path, default=DEFAULT_OUTPUT_ROOT)
    parser.add_argument("--font", type=Path)
    parser.add_argument("--force", action="store_true")
    return parser.parse_args()


def resolve_font(explicit: Path | None) -> Path:
    env_font = os.getenv("BODYSENSE_BENCHMARK_CJK_FONT", "").strip()
    candidates = [explicit] if explicit is not None else []
    if env_font:
        candidates.append(Path(env_font))
    candidates.extend(FONT_CANDIDATES)
    for candidate in candidates:
        if candidate is not None and candidate.is_file():
            return candidate.resolve()
    raise SystemExit("No CJK benchmark font found. Pass --font or BODYSENSE_BENCHMARK_CJK_FONT.")


def load_spec() -> dict[str, Any]:
    payload = json.loads(SPEC_PATH.read_text(encoding="utf-8"))
    if payload.get("schema_version") != "health-document-synthetic-corpus-spec-v1":
        raise SystemExit("unsupported corpus spec")
    return payload


def rows_for_document(document_index: int, page_count: int) -> list[list[tuple[str, ...]]]:
    # 3-page documents use all 12 indicators exactly once; shorter documents
    # use a deterministic rotation without duplicate indicator IDs per document.
    count = page_count * 4
    start = (document_index * 3) % len(INDICATORS)
    ordered = [INDICATORS[(start + offset) % len(INDICATORS)] for offset in range(count)]
    return [ordered[page * 4 : (page + 1) * 4] for page in range(page_count)]


def load_pil_font(font_path: Path, size: int) -> ImageFont.FreeTypeFont:
    return ImageFont.truetype(str(font_path), size=size, index=0)


def draw_report_page(
    rows: list[tuple[str, ...]],
    *,
    font_path: Path,
    title: str,
    table: bool,
    complex_layout: bool,
    degraded: bool,
    seed: int,
) -> Image.Image:
    image = Image.new("RGB", (1654, 2339), "white")
    draw = ImageDraw.Draw(image)
    title_font = load_pil_font(font_path, 48)
    body_font = load_pil_font(font_path, 34)
    small_font = load_pil_font(font_path, 28)
    draw.text((100, 90), title, fill="black", font=title_font)
    draw.text(
        (100, 165), "Synthetic benchmark fixture / 非真实患者数据", fill="black", font=small_font
    )

    y = 290
    if table:
        columns = (100, 620, 880, 1120)
        headers = ("项目", "结果", "单位", "参考范围")
        for x, header in zip(columns, headers, strict=True):
            draw.text((x, y), header, fill="black", font=body_font)
        y += 65
        draw.line((90, y, 1530, y), fill="black", width=2)
        y += 30
        for row_index, (_, name, value, unit, ref_range) in enumerate(rows, start=1):
            x_shift = 25 if complex_layout and row_index % 2 == 0 else 0
            values = (name, value, unit, ref_range)
            for x, value_text in zip(columns, values, strict=True):
                draw.text((x + x_shift, y), value_text, fill="black", font=body_font)
            y += 82 if complex_layout else 74
            if complex_layout:
                draw.line((90, y - 12, 1530, y - 12), fill=(170, 170, 170), width=1)
    else:
        for _, name, value, unit, ref_range in rows:
            draw.text(
                (100, y),
                f"{name}: {value} {unit} 参考范围: {ref_range}",
                fill="black",
                font=body_font,
            )
            y += 88

    draw.text(
        (100, 2160),
        "Generated by BodySense health-document-synthetic-v1",
        fill="black",
        font=small_font,
    )

    if degraded:
        rng = random.Random(seed)
        image = image.rotate(
            rng.choice((-1.25, -0.75, 0.75, 1.25)), resample=Image.Resampling.BICUBIC
        )
        image = image.filter(ImageFilter.GaussianBlur(radius=0.75))
        image = ImageEnhance.Contrast(image).enhance(0.88)
        image = ImageEnhance.Brightness(image).enhance(0.94)
    return image


def write_native_pdf(
    output: Path,
    pages: list[list[tuple[str, ...]]],
    *,
    font_path: Path,
    table: bool,
    complex_layout: bool,
) -> None:
    doc = fitz.open()
    try:
        for page_index, rows in enumerate(pages, start=1):
            page = doc.new_page(width=595, height=842)
            # Use PyMuPDF's deterministic built-in Simplified Chinese font for
            # born-digital fixtures. Embedding the full Noto CJK font made each
            # synthetic PDF ~16 MiB, exceeding BodySense's real 10 MiB upload
            # contract and turning benchmark transport failures into false OCR
            # failures. Image/scanned fixtures still use the pinned Noto asset.
            page.insert_text((55, 60), "BodySense 合成健康报告", fontname="china-s", fontsize=18)
            page.insert_text(
                (55, 82), f"第 {page_index} 页 / 非真实患者数据", fontname="china-s", fontsize=9
            )
            y = 125
            if table:
                page.insert_text(
                    (55, y),
                    "项目        结果        单位        参考范围",
                    fontname="china-s",
                    fontsize=10,
                )
                y += 28
                for row_index, (_, name, value, unit, ref_range) in enumerate(rows, start=1):
                    spacer = "   " if not complex_layout or row_index % 2 else "      "
                    text = spacer.join((name, value, unit, ref_range))
                    page.insert_text((55, y), text, fontname="china-s", fontsize=10)
                    y += 31
            else:
                for _, name, value, unit, ref_range in rows:
                    page.insert_text(
                        (55, y),
                        f"{name}: {value} {unit} 参考范围: {ref_range}",
                        fontname="china-s",
                        fontsize=11,
                    )
                    y += 34
        doc.save(output, no_new_id=True)
    finally:
        doc.close()


def write_scanned_pdf(output: Path, images: list[Image.Image]) -> None:
    doc = fitz.open()
    try:
        for image in images:
            page = doc.new_page(width=595, height=842)
            tmp = output.with_suffix(".page.jpg")
            image.save(tmp, format="JPEG", quality=90, optimize=True)
            try:
                page.insert_image(page.rect, filename=str(tmp))
            finally:
                tmp.unlink(missing_ok=True)
        doc.save(output, no_new_id=True)
    finally:
        doc.close()


def build_truth(pages: list[list[tuple[str, ...]]]) -> list[CorpusIndicatorTruth]:
    truth: list[CorpusIndicatorTruth] = []
    for page_index, rows in enumerate(pages, start=1):
        for row_index, (indicator_id, name, value, unit, ref_range) in enumerate(rows, start=1):
            truth.append(
                CorpusIndicatorTruth(
                    indicator_id=indicator_id,
                    display_name=name,
                    value=value,
                    unit=unit,
                    reference_range=ref_range,
                    page=page_index,
                    row=row_index,
                    critical_numeric=True,
                )
            )
    return truth


def main() -> int:
    args = parse_args()
    spec = load_spec()
    font_path = resolve_font(args.font)
    output_root = args.output_root.resolve()
    assets_root = output_root / "assets"
    if output_root.exists() and not args.force:
        raise SystemExit(f"output already exists: {output_root}; pass --force to replace")
    if output_root.exists():
        import shutil

        shutil.rmtree(output_root)
    assets_root.mkdir(parents=True, exist_ok=True)

    records: list[CorpusDocument] = []
    global_document_index = 0
    for cohort_spec in spec["cohorts"]:
        cohort = str(cohort_spec["id"])
        document_count = int(cohort_spec["documents"])
        page_count = int(cohort_spec["pages_per_document"])
        for local_index in range(document_count):
            fixture_id = f"{cohort.replace(chr(95), chr(45))}-{local_index + 1:02d}"
            pages = rows_for_document(global_document_index, page_count)
            global_document_index += 1
            is_photo = cohort.startswith("phone_photo")
            degraded = "degraded" in cohort
            table = cohort in {
                "native_pdf_table",
                "chinese_lab_table",
                "mixed_zh_en_units",
                "complex_table",
            }
            complex_layout = cohort == "complex_table"
            images = [
                draw_report_page(
                    rows,
                    font_path=font_path,
                    title=f"BodySense 合成报告 {fixture_id} P{page_index}",
                    table=table,
                    complex_layout=complex_layout,
                    degraded=degraded,
                    seed=int(spec["seed"]) + global_document_index * 17 + page_index,
                )
                for page_index, rows in enumerate(pages, start=1)
            ]

            if is_photo:
                suffix = ".jpg"
                mime_type = "image/jpeg"
                asset = assets_root / f"{fixture_id}{suffix}"
                images[0].save(asset, format="JPEG", quality=92 if not degraded else 78)
            else:
                suffix = ".pdf"
                mime_type = "application/pdf"
                asset = assets_root / f"{fixture_id}{suffix}"
                if cohort in {"native_pdf_simple", "native_pdf_table", "mixed_zh_en_units"}:
                    write_native_pdf(
                        asset,
                        pages,
                        font_path=font_path,
                        table=table,
                        complex_layout=complex_layout,
                    )
                else:
                    write_scanned_pdf(asset, images)

            records.append(
                CorpusDocument(
                    schema_version="health-document-corpus-item-v1",
                    fixture_id=fixture_id,
                    cohort=cohort,
                    asset_path=f"assets/{asset.name}",
                    mime_type=mime_type,
                    page_count=page_count,
                    source_classification="synthetic",
                    contains_real_user_data=False,
                    source_license="BodySense synthetic fixture; no external patient content",
                    generator_revision=str(spec["generator_revision"]),
                    generator_asset_sha256=sha256_file(asset),
                    annotation_state="synthetic_ground_truth",
                    indicators=build_truth(pages),
                )
            )

    manifest = output_root / "corpus_manifest.jsonl"
    write_manifest(manifest, records)
    provenance = {
        "schema_version": "health-document-synthetic-provenance-v1",
        "generator_revision": spec["generator_revision"],
        "spec_sha256": sha256_file(SPEC_PATH),
        "font_path": str(font_path),
        "font_sha256": sha256_file(font_path),
        "font_usage": "benchmark fixture generation only; generated assets are gitignored",
        "contains_real_user_data": False,
        "documents": len(records),
        "pages": sum(record.page_count for record in records),
    }
    (output_root / "corpus_provenance.json").write_text(
        json.dumps(provenance, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    print(json.dumps(provenance, ensure_ascii=False))
    print(f"manifest={manifest}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
