
/**
 * Person/Profile is a presentation projection over authoritative sources. It
 * owns no identity or health data of its own; each category reads from the
 * source that already owns it:
 *
 * | category | authoritative source                        |
 * | -------- | ------------------------------------------- |
 * | records  | BodyState facts / observations              |
 * | equipment| EquipmentAsset inventory (/api/v1/equipment) |
 * | media    | user uploads with a photo file type          |
 * | reports  | user uploads with file type report + OCR     |
 * | habits   | lifestyle onboarding snapshot                |
 *
 * Per the current Penpot V5.5 design the top Tab already names the category, so
 * no category renders a duplicate body heading or generic explanation.
 */
export type ProfileCategoryId = "records" | "equipment" | "media" | "reports" | "habits";

export interface ProfileCategoryMeta {
  id: ProfileCategoryId;
  /** Tab label; the only place the category name is displayed. */
  label: string;
  /** Screen-reader region name for the category panel. */
  regionLabel: string;
}

export const PROFILE_CATEGORIES: readonly ProfileCategoryMeta[] = [
  { id: "records", label: "身体记录", regionLabel: "身体记录" },
  { id: "equipment", label: "装备", regionLabel: "装备清单" },
  { id: "media", label: "影像资料", regionLabel: "影像资料" },
  { id: "reports", label: "医学报告", regionLabel: "医学报告" },
  { id: "habits", label: "习性", regionLabel: "生活习性" },
] as const;

export const PROFILE_CATEGORY_IDS: readonly ProfileCategoryId[] =
  PROFILE_CATEGORIES.map((category) => category.id);

export function isProfileCategoryId(value: string): value is ProfileCategoryId {
  return (PROFILE_CATEGORY_IDS as readonly string[]).includes(value);
}

export function getProfileCategoryMeta(id: ProfileCategoryId): ProfileCategoryMeta {
  return (
    PROFILE_CATEGORIES.find((category) => category.id === id) ?? PROFILE_CATEGORIES[0]
  );
}

export type UploadCategory = "media" | "reports";

export interface ProfileUploadLike {
  id: string;
  file_type: string;
  original_name: string;
  file_size: number;
  mime_type: string;
  ocr_status: string;
  analysis_status: string;
  created_at: string;
}

export function selectUploadsByCategory<T extends ProfileUploadLike>(
  uploads: readonly T[] | null | undefined,
  category: UploadCategory,
): T[] {
  return (uploads ?? []).filter((upload) =>
    category === "reports" ? upload.file_type === "report" : upload.file_type !== "report",
  );
}

export function formatFileSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "—";
  if (bytes < 1024) return `${Math.round(bytes)} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

const OCR_STATUS_LABELS: Record<string, string> = {
  pending: "等待识别",
  processing: "识别中",
  completed: "已识别",
  failed: "识别失败",
};

export function formatOcrStatus(status: string): string {
  return OCR_STATUS_LABELS[status] ?? status;
}

/** Habits reuses the lifestyle API sections; each section is user-editable. */
export type HabitSectionKey =
  "activity" | "sleep" | "exercise" | "nutrition" | "substances" | "recovery";

export interface HabitSectionMeta {
  key: HabitSectionKey;
  label: string;
}

export const HABIT_SECTIONS: readonly HabitSectionMeta[] = [
  { key: "activity", label: "日常活动" },
  { key: "sleep", label: "睡眠" },
  { key: "exercise", label: "运动" },
  { key: "nutrition", label: "饮食" },
  { key: "substances", label: "物质使用" },
  { key: "recovery", label: "恢复" },
] as const;

export interface HabitSectionValue {
  summary: string;
  updated_at?: string;
}

/** A habit row with no recorded summary is an explicit empty state, not a value. */
export function hasHabitValue(value: HabitSectionValue | undefined): boolean {
  return Boolean(value?.summary && value.summary.trim().length > 0);
}
