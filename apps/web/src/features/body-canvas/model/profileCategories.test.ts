import { describe, expect, it } from "vitest";
import type { BodyStateFact, BodyStateProjection } from "@/features/consultation/types/consultation";
import {
  AVAILABLE_EQUIPMENT_KIND,
  formatFileSize,
  formatOcrStatus,
  hasHabitValue,
  PROFILE_CATEGORIES,
  selectAvailableEquipment,
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

  it("keeps equipment out of records so a category is not shown twice", () => {
    const data = snapshot([
      fact({ id: "record-1" }),
      fact({
        id: "equipment-1",
        kind: AVAILABLE_EQUIPMENT_KIND,
        value: "弹力带",
        concern_key: "resources:equipment",
      }),
    ]);
    expect(selectProfileRecords(data).map((item) => item.id)).toEqual(["record-1"]);
    expect(selectAvailableEquipment(data).map((item) => item.id)).toEqual([
      "equipment-1",
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

  it("only treats confirmed, active equipment as available", () => {
    const data = snapshot([
      fact({ id: "confirmed", kind: AVAILABLE_EQUIPMENT_KIND, concern_key: "resources:equipment" }),
      fact({
        id: "unverified",
        kind: AVAILABLE_EQUIPMENT_KIND,
        concern_key: "resources:equipment",
        review_state: "unverified",
      }),
      fact({
        id: "excluded",
        kind: AVAILABLE_EQUIPMENT_KIND,
        concern_key: "resources:equipment",
        excluded_from_reasoning: true,
      }),
    ]);
    expect(selectAvailableEquipment(data).map((item) => item.id)).toEqual([
      "confirmed",
    ]);
  });

  it("returns empty selections for a missing snapshot", () => {
    expect(selectProfileRecords(null)).toEqual([]);
    expect(selectAvailableEquipment(undefined)).toEqual([]);
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
