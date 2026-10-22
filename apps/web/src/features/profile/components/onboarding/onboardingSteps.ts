/**
 * Onboarding step model, following the Penpot "02 Onboarding" boards
 * (Onboarding / Desktop / Step 01 … Step 16).
 *
 * Penpot asks one short question per screen; the API keeps its existing shape
 * (stable profile + BodyState baseline + lifestyle sections). This module owns the
 * mapping between the two so the wizard stays presentational.
 */
import type {
  DiscomfortAreaCode,
  OnboardingContextPayload,
} from "../../services/onboardingContextService";

export interface ChoiceOption {
  value: string;
  label: string;
}

export type OnboardingSection = "基础信息" | "身体状况" | "生活习惯" | "完成";

export interface OnboardingAnswers {
  gender: "" | "male" | "female";
  birth_date: string;
  height_cm: number | undefined;
  weight_kg: number | undefined;
  /**
   * Step 05. Sent as `discomfort_areas`, a self-report the API keeps outside BodyState
   * (not a symptom record or diagnosis).
   */
  discomfort_regions: string[];
  injury: string;
  activity_pattern: string;
  sitting_hours: string;
  break_interval: string;
  sleep_tags: string[];
  exercise_habit: string;
  exercise_type: string;
  exercise_frequency: string;
  nutrition: string;
  substances: string;
}

export const INITIAL_ANSWERS: OnboardingAnswers = {
  gender: "",
  birth_date: "",
  height_cm: undefined,
  weight_kg: undefined,
  discomfort_regions: [],
  injury: "",
  activity_pattern: "",
  sitting_hours: "",
  break_interval: "",
  sleep_tags: [],
  exercise_habit: "",
  exercise_type: "",
  exercise_frequency: "",
  nutrition: "",
  substances: "",
};

export const GENDER_OPTIONS: ChoiceOption[] = [
  { value: "male", label: "男" },
  { value: "female", label: "女" },
];

export const NO_DISCOMFORT = "none";
export const DISCOMFORT_OPTIONS: ChoiceOption[] = [
  { value: "neck", label: "颈部" },
  { value: "shoulder", label: "肩部" },
  { value: "upper_back", label: "上背部" },
  { value: "lower_back", label: "腰部" },
  { value: "hip", label: "臀髋部" },
  { value: "leg", label: "腿部" },
  { value: "arm_wrist", label: "手臂 / 手腕" },
  { value: NO_DISCOMFORT, label: "都没有不适" },
];

export const INJURY_OPTIONS: ChoiceOption[] = [
  { value: "none", label: "无" },
  { value: "cervical", label: "颈椎问题" },
  { value: "lumbar", label: "腰椎问题" },
  { value: "shoulder_knee", label: "肩 / 膝损伤" },
];

export const ACTIVITY_OPTIONS: ChoiceOption[] = [
  { value: "sedentary_office", label: "久坐办公" },
  { value: "light", label: "轻度活动" },
  { value: "manual", label: "体力劳动" },
  { value: "active", label: "经常运动" },
];

export const SITTING_OPTIONS: ChoiceOption[] = [
  { value: "lt4", label: "4 小时以内" },
  { value: "4-6", label: "4–6 小时" },
  { value: "6-8", label: "6–8 小时" },
  { value: "gt8", label: "8 小时以上" },
];

export const BREAK_OPTIONS: ChoiceOption[] = [
  { value: "lt30m", label: "半小时内" },
  { value: "30m-1h", label: "半小时–1 小时" },
  { value: "1-2h", label: "1–2 小时" },
  { value: "gt2h", label: "2 小时以上" },
];

export const SLEEP_OPTIONS: ChoiceOption[] = [
  { value: "regular", label: "作息比较规律" },
  { value: "late", label: "经常晚睡" },
  { value: "shift", label: "轮班" },
  { value: "irregular_bedtime", label: "入睡时间不固定" },
  { value: "interrupted", label: "睡眠经常中断" },
  { value: "short", label: "明显缺觉" },
];

export const EXERCISE_HABIT_OPTIONS: ChoiceOption[] = [
  { value: "regular", label: "有规律地锻炼" },
  { value: "occasional", label: "偶尔锻炼" },
  { value: "rare", label: "很少锻炼" },
  { value: "almost_never", label: "几乎不锻炼" },
];

export const EXERCISE_TYPE_OPTIONS: ChoiceOption[] = [
  { value: "running_walking", label: "跑步 / 快走" },
  { value: "strength", label: "力量训练" },
  { value: "yoga_stretching", label: "瑜伽 / 拉伸" },
  { value: "ball_other", label: "球类 / 其他" },
];

export const EXERCISE_FREQUENCY_OPTIONS: ChoiceOption[] = [
  { value: "never", label: "几乎不运动" },
  { value: "occasional", label: "偶尔" },
  { value: "1-2", label: "每周 1–2 次" },
  { value: "3+", label: "每周 3 次以上" },
];

export const NUTRITION_OPTIONS: ChoiceOption[] = [
  { value: "regular_meals", label: "规律三餐" },
  { value: "takeout", label: "经常外卖" },
  { value: "dieting", label: "节食 / 控糖" },
];

export const SUBSTANCES_OPTIONS: ChoiceOption[] = [
  { value: "none", label: "都不碰" },
  { value: "occasional", label: "偶尔" },
  { value: "frequent", label: "经常" },
];

export type StepKind = "single" | "multi" | "date" | "number" | "upload";

export interface StepDef {
  id: string;
  section: OnboardingSection;
  title: string;
  subtitle?: string;
  kind: StepKind;
  /** Optional single-choice steps offer 跳过 (Penpot marks them 选填). */
  optional?: boolean;
}

export const STEPS: StepDef[] = [
  {
    id: "gender",
    section: "基础信息",
    title: "您的性别是？",
    kind: "single",
  },
  {
    id: "birth_date",
    section: "基础信息",
    title: "您的出生日期是？",
    kind: "date",
  },
  {
    id: "height",
    section: "基础信息",
    title: "您的身高是？",
    kind: "number",
  },
  {
    id: "weight",
    section: "基础信息",
    title: "您的体重是？",
    kind: "number",
  },
  {
    id: "discomfort",
    section: "身体状况",
    title: "目前哪些部位有不适感？",
    subtitle: "可多选。",
    kind: "multi",
  },
  {
    id: "injury",
    section: "身体状况",
    title: "有无运动损伤或确诊的脊柱问题？",
    subtitle: "如椎间盘突出、脊柱侧弯等。",
    kind: "single",
    optional: true,
  },
  {
    id: "activity",
    section: "身体状况",
    title: "您日常的活动模式是？",
    kind: "single",
  },
  {
    id: "sitting",
    section: "生活习惯",
    title: "工作日平均每天坐着的时间？",
    subtitle: "含通勤、会议、在家坐着的时间。",
    kind: "single",
  },
  {
    id: "break",
    section: "生活习惯",
    title: "连续坐多久会起身活动一下？",
    kind: "single",
  },
  {
    id: "sleep",
    section: "生活习惯",
    title: "睡眠与作息情况",
    kind: "multi",
  },
  {
    id: "exercise_habit",
    section: "生活习惯",
    title: "平时有锻炼习惯吗？",
    kind: "single",
  },
  {
    id: "exercise_type",
    section: "生活习惯",
    title: "平时主要做什么运动？",
    kind: "single",
  },
  {
    id: "exercise_frequency",
    section: "生活习惯",
    title: "运动频率是？",
    kind: "single",
  },
  {
    id: "nutrition",
    section: "生活习惯",
    title: "饮食习惯是？",
    kind: "single",
    optional: true,
  },
  {
    id: "substances",
    section: "生活习惯",
    title: "烟酒情况？",
    kind: "single",
    optional: true,
  },
  {
    id: "upload",
    section: "完成",
    title: "上传体检报告或体态照片",
    kind: "upload",
  },
];

export const TOTAL_STEPS = STEPS.length;

export const HEIGHT_RANGE = { min: 50, max: 250 } as const;
export const WEIGHT_RANGE = { min: 20, max: 300 } as const;

/** "19950615" / "1995-06-15" → "1995 / 06 / 15" (partial input keeps its digits). */
export function formatBirthInput(raw: string): string {
  const digits = raw.replace(/\D/g, "").slice(0, 8);
  return [digits.slice(0, 4), digits.slice(4, 6), digits.slice(6, 8)]
    .filter(Boolean)
    .join(" / ");
}

/** Masked input → ISO date when it is a real calendar day inside the bounds. */
export function parseBirthInput(
  masked: string,
  bounds: { min: string; max: string },
): string {
  const digits = masked.replace(/\D/g, "");
  if (digits.length !== 8) return "";
  const iso = `${digits.slice(0, 4)}-${digits.slice(4, 6)}-${digits.slice(6, 8)}`;
  const date = new Date(`${iso}T00:00:00Z`);
  if (Number.isNaN(date.getTime()) || date.toISOString().slice(0, 10) !== iso)
    return "";
  return iso >= bounds.min && iso <= bounds.max ? iso : "";
}

const DISCOMFORT_CODES = new Set<string>(
  DISCOMFORT_OPTIONS.map((option) => option.value),
);

/**
 * Step 05 chips → the API's coarse area codes in the board order. "都没有不适" stands
 * alone; nothing chosen means the field is omitted (the API leaves it untouched).
 */
export function toDiscomfortAreas(
  regions: string[],
): DiscomfortAreaCode[] | undefined {
  if (regions.includes(NO_DISCOMFORT)) return [NO_DISCOMFORT];
  const chosen = DISCOMFORT_OPTIONS.map((option) => option.value).filter(
    (value) =>
      value !== NO_DISCOMFORT &&
      regions.includes(value) &&
      DISCOMFORT_CODES.has(value),
  );
  return chosen.length > 0 ? (chosen as DiscomfortAreaCode[]) : undefined;
}

const labelOf = (options: ChoiceOption[], value: string) =>
  options.find((option) => option.value === value)?.label ?? "";

const compact = (record: Record<string, string | string[]>) =>
  Object.fromEntries(
    Object.entries(record).filter(([, v]) =>
      Array.isArray(v) ? v.length > 0 : v !== "",
    ),
  );

/** Whether the step's answer allows moving on (single steps advance on choice). */
export function isStepComplete(
  stepIndex: number,
  a: OnboardingAnswers,
): boolean {
  const step = STEPS[stepIndex];
  switch (step?.id) {
    case "gender":
      return a.gender !== "";
    case "birth_date":
      return a.birth_date !== "";
    case "height":
      return a.height_cm !== undefined;
    case "weight":
      return a.weight_kg !== undefined;
    case "discomfort":
      return a.discomfort_regions.length > 0;
    case "sleep":
      return a.sleep_tags.length > 0;
    case "upload":
      return true;
    case "injury":
      return true;
    case "nutrition":
      return true;
    case "substances":
      return true;
    case "activity":
      return a.activity_pattern !== "";
    case "sitting":
      return a.sitting_hours !== "";
    case "break":
      return a.break_interval !== "";
    case "exercise_habit":
      return a.exercise_habit !== "";
    case "exercise_type":
      return a.exercise_type !== "";
    case "exercise_frequency":
      return a.exercise_frequency !== "";
    default:
      return false;
  }
}

/** Map the Penpot answers onto the unchanged onboarding-context command. */
export function buildOnboardingPayload(
  a: OnboardingAnswers,
): OnboardingContextPayload {
  if (!a.gender || !a.birth_date) {
    throw new Error("请先填写性别和出生日期");
  }
  if (a.height_cm === undefined || a.weight_kg === undefined) {
    throw new Error("请先填写身高和体重");
  }

  const activityParts = [
    labelOf(ACTIVITY_OPTIONS, a.activity_pattern),
    a.sitting_hours
      ? `工作日每天坐着约 ${labelOf(SITTING_OPTIONS, a.sitting_hours)}`
      : "",
    a.break_interval
      ? `连续坐 ${labelOf(BREAK_OPTIONS, a.break_interval)}会起身活动`
      : "",
  ].filter(Boolean);

  const sleepLabels = a.sleep_tags
    .map((tag) => labelOf(SLEEP_OPTIONS, tag))
    .filter(Boolean);

  const exerciseType = labelOf(EXERCISE_TYPE_OPTIONS, a.exercise_type);
  const exerciseFrequency = labelOf(
    EXERCISE_FREQUENCY_OPTIONS,
    a.exercise_frequency,
  );
  const exerciseParts = [
    labelOf(EXERCISE_HABIT_OPTIONS, a.exercise_habit),
    exerciseType,
    exerciseFrequency ? `频率：${exerciseFrequency}` : "",
  ].filter(Boolean);

  const discomfortAreas = toDiscomfortAreas(a.discomfort_regions);

  const injury =
    a.injury === "none"
      ? "无运动损伤或确诊的脊柱问题"
      : labelOf(INJURY_OPTIONS, a.injury);

  return {
    expected_body_state_revision: 0,
    profile: { gender: a.gender, birth_date: a.birth_date },
    body_metrics: { height_cm: a.height_cm, weight_kg: a.weight_kg },
    lifestyle: {
      activity: {
        summary: activityParts.join("；"),
        details: compact({
          pattern: a.activity_pattern,
          sitting_hours: a.sitting_hours,
          break_interval: a.break_interval,
        }),
      },
      sleep: {
        summary: sleepLabels.join("；"),
        details: compact({ tags: a.sleep_tags }),
      },
      exercise: {
        summary: exerciseParts.join("；"),
        details: compact({
          habit: a.exercise_habit,
          type: exerciseType,
          type_code: a.exercise_type,
          frequency: a.exercise_frequency,
        }),
      },
      nutrition: {
        summary: labelOf(NUTRITION_OPTIONS, a.nutrition),
        details: compact({ pattern: a.nutrition }),
      },
      substances: {
        summary: labelOf(SUBSTANCES_OPTIONS, a.substances),
        details: compact({ pattern: a.substances }),
      },
      recovery: { summary: "" },
    },
    injury_history: injury,
    ...(discomfortAreas ? { discomfort_areas: discomfortAreas } : {}),
  };
}
