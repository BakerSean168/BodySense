import { describe, expect, it } from "vitest";
import type { BodyStateFact, BodyStateProjection } from "@/features/consultation/types/consultation";
import {
  formatFileSize,
  formatOcrStatus,
  hasHabitValue,
  PROFILE_CATEGORIES,
  selectProfileRecords,
  selectUploadsByCategory,
} from "./profileCategories";

function fact(overrides: Partial<BodyStateFact> = {}): BodyStateFact {
  return {
    id: "fact-1",
    kind: "discomfort",
    value: "抬高手臂时右肩疼",
    origin: "user_reported",
    review_state: "confirmed",
    lifecycle_state: "active",
    trend: "stable",
    updated_revision: 1,
    ...overrides,
  };
}

function snapshot(facts: BodyStateFact[]): BodyStateProjection {
  return { current_revision: 1, safety_state: {}, facts, observations: [] };
}

describe("profile categories", () => {
  it("exposes the five established categories in order", () => {
    expect(PROFILE_CATEGORIES.map((category) => category.id)).toEqual([
      "records",
      "equipment",
      "media",
      "reports",
      "habits",
    ]);
    expect(PROFILE_CATEGORIES.map((category) => category.label)).toEqual([
      "身体记录",
      "装备",
      "影像资料",
      "医学报告",
      "习性",
    ]);
  });

  it("excludes rejected and inactive records", () => {
    const data = snapshot([
      fact({ id: "keep" }),
      fact({ id: "rejected", review_state: "rejected" }),
      fact({ id: "inactive", lifecycle_state: "inactive" }),
    ]);
    expect(selectProfileRecords(data).map((item) => item.id)).toEqual(["keep"]);
  });

  it("returns empty selections for a missing snapshot", () => {
    expect(selectProfileRecords(null)).toEqual([]);
  });

  it("splits uploads into media and reports by file type", () => {
    const uploads = [
      { id: "a", file_type: "photo_front" },
      { id: "b", file_type: "report" },
      { id: "c", file_type: "consultation_photo" },
    ].map((upload) => ({
      ...upload,
      original_name: `${upload.id}.jpg`,
      file_size: 1024,
      mime_type: "image/jpeg",
      ocr_status: "completed",
      analysis_status: "none",
      created_at: "2026-10-11T00:00:00Z",
    }));
    expect(selectUploadsByCategory(uploads, "reports").map((u) => u.id)).toEqual(["b"]);
    expect(selectUploadsByCategory(uploads, "media").map((u) => u.id)).toEqual([
      "a",
      "c",
    ]);
  });

  it("formats sizes and OCR status without inventing values", () => {
    expect(formatFileSize(0)).toBe("—");
    expect(formatFileSize(512)).toBe("512 B");
    expect(formatFileSize(2048)).toBe("2 KB");
    expect(formatFileSize(3 * 1024 * 1024)).toBe("3.0 MB");
    expect(formatOcrStatus("completed")).toBe("已识别");
    expect(formatOcrStatus("weird")).toBe("weird");
  });

  it("treats a blank habit summary as absent", () => {
    expect(hasHabitValue({ summary: "每晚 7 小时" })).toBe(true);
    expect(hasHabitValue({ summary: "   " })).toBe(false);
    expect(hasHabitValue(undefined)).toBe(false);
  });
});
