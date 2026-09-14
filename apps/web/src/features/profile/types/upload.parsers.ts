import { z } from "zod";
import type { OCRResult, PostureAnalysis } from "./upload.types";

const ocrConfidenceSchema = z.enum(["high", "medium", "low", "unknown"]);
const indicatorAdmissibilitySchema = z
  .object({
    status: z.enum(["admissible", "needs_review", "rejected"]),
    policy_revision: z.string().min(1),
    reason_codes: z.array(z.string()),
  })
  .strip();
const healthIndicatorSchema = z
  .object({
    name: z.string().min(1),
    value: z.string().min(1),
    unit: z.string().nullable(),
    reference_range: z.string().nullable(),
    confidence: ocrConfidenceSchema,
    evidence_admissibility: indicatorAdmissibilitySchema.optional(),
  })
  .strip();
const ocrResultSchema = z
  .object({
    raw_text: z.string(),
    indicators: z.array(healthIndicatorSchema),
    confidence: ocrConfidenceSchema,
  })
  .strip();

const postureConfidenceSchema = z.enum(["high", "medium", "low"]);
const postureMetricSchema = z
  .object({
    name: z.string().min(1),
    value: z.number(),
    unit: z.string(),
  })
  .strip();
const postureFindingSchema = z
  .object({
    key: z.string().min(1),
    label: z.string().min(1),
    severity: z.enum(["mild", "moderate", "marked"]),
    confidence: postureConfidenceSchema,
    evidence: z.string(),
    metric: postureMetricSchema.nullable(),
  })
  .strip();
const postureRedFlagSchema = z
  .object({ category: z.string(), message: z.string() })
  .strip();
const postureAnalysisSchema = z
  .object({
    schema_version: z.number().int(),
    view: z.enum(["front", "side", "back"]),
    overall_confidence: postureConfidenceSchema,
    findings: z.array(postureFindingSchema),
    red_flags: z.array(postureRedFlagSchema),
    summary_markdown: z.string(),
    disclaimer: z.string(),
  })
  .strip();

export function parseOCRResult(input: unknown): OCRResult | null {
  if (input === undefined || input === null) return null;
  return ocrResultSchema.parse(input);
}

export function parsePostureAnalysis(input: unknown): PostureAnalysis | null {
  if (input === undefined || input === null) return null;
  return postureAnalysisSchema.parse(input);
}
