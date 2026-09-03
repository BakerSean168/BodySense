# Feature Spec: Health Document Ingestion & Evidence Review

- Status: Target product contract / implementation planned
- Date: 2026-09-01
- Architecture: [`architecture/health-document-evidence-pipeline.md`](./architecture/health-document-evidence-pipeline.md)
- Decision: [`adr/0013-adopt-versioned-health-document-evidence-pipeline.md`](./adr/0013-adopt-versioned-health-document-evidence-pipeline.md)
- Existing admission contract: [`adr/0011-adopt-ocr-report-indicator-evidence-admissibility.md`](./adr/0011-adopt-ocr-report-indicator-evidence-admissibility.md)

## 1. Product goal

Allow a user to contribute health-document evidence to BodySense with minimal manual entry while preserving a clear boundary between:

```text
machine extracted
user confirmed
available to Assessment
medical interpretation
```

The user should be able to answer:

> “这个数值是从我哪一份报告、哪一页、哪一行识别出来的？BodySense 有没有把它当作事实使用？如果识别错了，我能不能改？”

## 2. Supported first-class inputs

Target v1:

- JPEG / PNG / WebP report images;
- born-digital PDF reports;
- scanned PDF reports;
- mixed PDFs containing both native text and scanned pages.

Primary scenarios:

- blood/laboratory reports;
- physical examination reports;
- vitamin/micronutrient reports;
- imaging **text report** pages;
- outpatient/inpatient text summaries.

Not a goal:

- direct interpretation of X-ray/CT/MRI image pixels as radiology diagnosis;
- handwritten medical notes as a guaranteed supported format;
- arbitrary document question answering.

## 3. User flow

```text
Upload report
  -> Processing
  -> Extraction complete
  -> Indicator review summary
       ├─ Ready for assessment
       ├─ Needs confirmation
       └─ Could not read
  -> User optionally reviews questionable values
  -> Assessment can use only authorized evidence
```

## 4. Upload UX

The user selects “健康报告 / 病历资料” rather than an implementation-specific “OCR” option.

Accepted copy example:

```text
上传健康报告
支持体检单、检验报告、检查文字报告和扫描 PDF。
BodySense 会先识别内容，再单独判断哪些信息可用于健康评估。
```

Avoid copy such as:

```text
AI 会自动诊断你的报告
```

because document ingestion does not own Diagnosis authority.

## 5. Processing states

User-visible states:

```text
queued        等待处理
processing    正在读取报告
completed     已完成识别
needs_review  有内容需要确认
failed        处理失败
```

The UI should not expose implementation details such as “Tesseract”, “RapidOCR” or “PP-OCRv6” in normal product copy. Exact mechanism identity belongs in diagnostics/audit details.

## 6. Result summary

Preferred summary:

```text
已识别 12 个指标

8 个可用于评估
3 个需要确认
1 个无法可靠识别
```

Do not say:

```text
已确认 12 个健康指标
```

unless they have actually been confirmed/authorized.

## 7. Indicator card/table

Each candidate should show:

```text
指标名称
识别值
单位
参考范围（如果报告中明确存在）
来源页码
状态
```

Example:

```text
25-羟维生素D
17.8 ng/mL
参考范围：30-100
来源：第 2 页
状态：需要确认
```

## 8. Source-grounded review

For `needs_review`, the user can open the source context.

Target UI:

```text
[原始报告第 2 页局部截图 / 高亮区域]

机器识别：17.8 ng/mL

[确认无误]
[修改]
[不是这个指标]
```

The product must prefer showing source context over asking the user to trust reconstructed text.

## 9. Review semantics

### Confirm

```text
machine candidate
  + user confirm
  -> append review record
```

### Correct

```text
machine candidate remains historical
  + corrected value/unit/range
  -> append review record
```

### Reject

```text
machine candidate remains historical
  + reject review
  -> unavailable to reasoning
```

No review action mutates or deletes the original machine extraction artifact.

## 10. Evidence states

Product states map to domain states approximately as:

```text
可用于评估
  -> admissible or reviewed-confirmed

需要确认
  -> needs_review

已排除
  -> rejected
```

“可用于评估” means only that the source may participate in Assessment. It does not mean medically normal/abnormal and does not mean BodySense has diagnosed anything.

## 11. Born-digital PDF experience

Users should not notice whether the backend used native text or OCR. However, the system should preserve the higher-fidelity path automatically.

```text
PDF with good embedded text
  -> fast processing

scanned PDF
  -> OCR processing
```

The UI may show a generic progress state; engine-specific progress is diagnostics-only.

## 12. Failed/partial processing

### Whole-document failure

Example copy:

```text
这份文件暂时无法读取。原文件仍已保存，你可以重新处理或上传更清晰的版本。
```

### One-page failure

Do not discard the entire document if other pages succeeded. Show:

```text
第 1、2 页已读取
第 3 页需要重新上传或确认
```

### Zero indicators

A valid document may contain no supported structured indicators.

```text
报告文字已读取，但暂未识别出可结构化的健康指标。
```

This is not an OCR job failure.

## 13. Assessment interaction

Assessment receives only evidence that satisfies the current evidence contract.

```text
raw document                X direct authority
raw OCR text                X direct authority
needs_review candidate      X
admissible candidate        ✓
reviewed confirmed fact     ✓
```

Assessment should preserve source references so future UI can explain which document supported an observation.

## 14. Longitudinal behavior

A report is a historical evidence source, not a permanent timeless truth.

Target metadata should retain:

```text
report/upload date
measurement/report date when explicitly available
source document identity
extraction run identity
review history
```

If the user uploads a newer report, BodySense should not delete the old one. Longitudinal BodyState can reason over time-aware evidence.

## 15. Reprocessing UX

When a better extraction engine/configuration becomes available:

```text
重新识别
```

creates a new extraction run.

The UI should not silently replace already reviewed values. If a reprocess changes a previously confirmed value, show a conflict requiring explicit review.

Example:

```text
之前确认：17.8 ng/mL
新识别结果：77.8 ng/mL

来源结果存在冲突，请确认原报告。
```

## 16. Privacy

- documents are private user data;
- raw report text is not exposed in public logs;
- source preview URLs must use the existing authenticated upload-access boundary;
- benchmark and diagnostics never copy identifiable production documents into Git/CI artifacts;
- deletion and retention follow BodySense privacy-erasure/upload-storage contracts.

## 17. Accessibility

Source highlighting must not rely on color alone. Review state needs text labels and accessible controls.

For table values, screen readers should announce:

```text
indicator name -> value -> unit -> reference range -> review status
```

## 18. Acceptance criteria

### Upload and processing

- [ ] image and PDF report uploads enter durable JobRuntime processing;
- [ ] born-digital PDF uses native text when quality gate accepts it;
- [ ] scanned pages use the selected OCR Champion;
- [ ] mixed PDFs can use mixed extraction strategies;
- [ ] processing failures are explicit and retryable.

### Evidence grounding

- [ ] every new automatically admissible indicator has source evidence refs;
- [x] user can see the persisted source page/region for review-required current-pipeline indicators;
- [x] current result shows extraction/review status with explicit non-diagnostic wording.

### Review

- [x] user can confirm/correct/reject through authenticated exact-run actions;
- [x] machine candidate remains immutable;
- [x] review creates append-only history and exposes the effective latest state;
- [x] reviewed state participates in evidence authority through versioned Assessment v5 / `assessment-evidence-contract-v4`.

### Longitudinal/replay

- [ ] reprocessing creates a new extraction run;
- [ ] old run remains available for audit;
- [ ] engine/parser configuration IDs are durable;
- [ ] conflicting reprocess output does not silently overwrite confirmed evidence.

### Safety

- [ ] raw OCR completion alone never implies evidence authority;
- [ ] VLM rescue output cannot bypass the same evidence/review contract;
- [ ] medical interpretation remains in Assessment/Diagnosis, not document parsing.
