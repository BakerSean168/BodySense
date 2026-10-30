import { useEffect, useRef, useState, type ReactNode } from "react";
import { useNavigate } from "react-router";
import { toast } from "sonner";
import { useProfileStore } from "@/stores/profileStore";
import { assessmentApi } from "@/features/assessment/services/assessmentService";
import { onboardingContextService } from "../../services/onboardingContextService";
import {
  OnboardingLayout,
  OnboardingPrimaryButton,
  OnboardingSkipButton,
} from "./OnboardingLayout";
import { ChipGrid, ChoiceList, ValueBox } from "./OnboardingControls";
import { UploadStep } from "./steps/UploadStep";
import {
  ACTIVITY_OPTIONS,
  BREAK_OPTIONS,
  DISCOMFORT_OPTIONS,
  EXERCISE_FREQUENCY_OPTIONS,
  EXERCISE_HABIT_OPTIONS,
  EXERCISE_TYPE_OPTIONS,
  GENDER_OPTIONS,
  HEIGHT_RANGE,
  INITIAL_ANSWERS,
  INJURY_OPTIONS,
  NO_DISCOMFORT,
  NUTRITION_OPTIONS,
  SITTING_OPTIONS,
  SLEEP_OPTIONS,
  STEPS,
  SUBSTANCES_OPTIONS,
  TOTAL_STEPS,
  WEIGHT_RANGE,
  buildOnboardingPayload,
  formatBirthInput,
  parseBirthInput,
  isStepComplete,
  type ChoiceOption,
  type OnboardingAnswers,
} from "./onboardingSteps";

type SingleKey =
  | "gender"
  | "injury"
  | "activity_pattern"
  | "sitting_hours"
  | "break_interval"
  | "exercise_habit"
  | "exercise_type"
  | "exercise_frequency"
  | "nutrition"
  | "substances";

const SINGLE_STEPS: Record<
  string,
  { key: SingleKey; options: ChoiceOption[] }
> = {
  gender: { key: "gender", options: GENDER_OPTIONS },
  injury: { key: "injury", options: INJURY_OPTIONS },
  activity: { key: "activity_pattern", options: ACTIVITY_OPTIONS },
  sitting: { key: "sitting_hours", options: SITTING_OPTIONS },
  break: { key: "break_interval", options: BREAK_OPTIONS },
  exercise_habit: { key: "exercise_habit", options: EXERCISE_HABIT_OPTIONS },
  exercise_type: { key: "exercise_type", options: EXERCISE_TYPE_OPTIONS },
  exercise_frequency: {
    key: "exercise_frequency",
    options: EXERCISE_FREQUENCY_OPTIONS,
  },
  nutrition: { key: "nutrition", options: NUTRITION_OPTIONS },
  substances: { key: "substances", options: SUBSTANCES_OPTIONS },
};

/** Short pause so the chosen row's check is seen before the next question. */
const ADVANCE_DELAY_MS = 220;

const LOADING_PHRASES = [
  "正在整理您已提交的身体资料...",
  "正在读取已完成的体态分析结果...",
  "正在读取体检报告中已识别的指标...",
  "正在根据当前可用证据生成健康评估...",
];

function prefersReducedMotion() {
  return (
    typeof window !== "undefined" &&
    typeof window.matchMedia === "function" &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches
  );
}

function rangeError(
  raw: string,
  range: { min: number; max: number },
  name: string,
  unit: string,
) {
  if (raw.trim() === "") return "";
  const num = Number(raw);
  if (!Number.isFinite(num)) return `请输入有效的${name}`;
  if (num < range.min || num > range.max) {
    return `${name}必须在 ${range.min}-${range.max} ${unit} 之间`;
  }
  return "";
}

function dateBounds() {
  const today = new Date();
  const max = new Date(today.getFullYear(), today.getMonth(), today.getDate());
  const min = new Date(max);
  min.setFullYear(min.getFullYear() - 150);
  const format = (value: Date) =>
    `${value.getFullYear()}-${String(value.getMonth() + 1).padStart(2, "0")}-${String(value.getDate()).padStart(2, "0")}`;
  return { min: format(min), max: format(max) };
}

export function OnboardingWizard() {
  const navigate = useNavigate();
  const { fetchProfile } = useProfileStore();

  const [stepIndex, setStepIndex] = useState(0);
  const [answers, setAnswers] = useState<OnboardingAnswers>(INITIAL_ANSWERS);
  const [birthInput, setBirthInput] = useState("");
  const [heightInput, setHeightInput] = useState("");
  const [weightInput, setWeightInput] = useState("");
  const [isGenerating, setIsGenerating] = useState(false);
  const [phraseIndex, setPhraseIndex] = useState(0);
  const advanceTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Keep a successful context write across retries when a later step fails.
  const contextSaved = useRef(false);

  useEffect(
    () => () => {
      if (advanceTimer.current) clearTimeout(advanceTimer.current);
    },
    [],
  );

  useEffect(() => {
    if (!isGenerating) return;
    const interval = setInterval(
      () => setPhraseIndex((prev) => (prev + 1) % LOADING_PHRASES.length),
      2500,
    );
    return () => clearInterval(interval);
  }, [isGenerating]);

  const step = STEPS[stepIndex];
  const canProceed = isStepComplete(stepIndex, answers);

  const update = <K extends keyof OnboardingAnswers>(
    key: K,
    value: OnboardingAnswers[K],
  ) => setAnswers((prev) => ({ ...prev, [key]: value }));

  const goNext = () =>
    setStepIndex((prev) => Math.min(prev + 1, TOTAL_STEPS - 1));
  const goBack = () => {
    if (advanceTimer.current) clearTimeout(advanceTimer.current);
    setStepIndex((prev) => Math.max(prev - 1, 0));
  };

  const choose = (key: SingleKey, value: string) => {
    update(key, value as never);
    if (advanceTimer.current) clearTimeout(advanceTimer.current);
    if (prefersReducedMotion()) {
      goNext();
    } else {
      advanceTimer.current = setTimeout(goNext, ADVANCE_DELAY_MS);
    }
  };

  const skip = (key: SingleKey) => {
    update(key, "");
    goNext();
  };

  const toggleDiscomfort = (value: string) =>
    setAnswers((prev) => {
      const has = prev.discomfort_regions.includes(value);
      let next: string[];
      if (value === NO_DISCOMFORT) next = has ? [] : [NO_DISCOMFORT];
      else
        next = has
          ? prev.discomfort_regions.filter((v) => v !== value)
          : [
              ...prev.discomfort_regions.filter((v) => v !== NO_DISCOMFORT),
              value,
            ];
      return { ...prev, discomfort_regions: next };
    });

  const toggleSleep = (value: string) =>
    setAnswers((prev) => ({
      ...prev,
      sleep_tags: prev.sleep_tags.includes(value)
        ? prev.sleep_tags.filter((v) => v !== value)
        : [...prev.sleep_tags, value],
    }));

  const onMetric = (
    raw: string,
    setRaw: (value: string) => void,
    key: "height_cm" | "weight_kg",
    range: { min: number; max: number },
  ) => {
    setRaw(raw);
    const num = Number(raw);
    const valid =
      raw.trim() !== "" &&
      Number.isFinite(num) &&
      num >= range.min &&
      num <= range.max;
    update(key, valid ? num : undefined);
  };

  const handleSubmit = async () => {
    if (isGenerating) return;
    setIsGenerating(true);
    try {
      // Persist identity + BodyState first. An unavailable LLM must not turn a
      // committed onboarding into a failed wizard or duplicate its writes.
      if (!contextSaved.current) {
        await onboardingContextService.submit(buildOnboardingPayload(answers));
        contextSaved.current = true;
      }
      await fetchProfile();
    } catch (err) {
      console.error("Onboarding context save or refresh failed:", err);
      setIsGenerating(false);
      toast.error(
        err instanceof Error ? err.message : "保存身体档案失败，请稍后重试",
      );
      return;
    }

    try {
      // Assessment is derived and optional; the committed BodyState is the
      // durable source of truth even when the model provider is unavailable.
      await assessmentApi.generate();
      toast.success("初始身体状态已建立");
    } catch (err) {
      console.warn("Onboarding saved but assessment generation failed:", err);
      toast.warning("身体档案已保存，AI 评估暂不可用，可稍后重试");
    }
    navigate("/consultation?view=state", { replace: true });
  };

  if (isGenerating) {
    return (
      <div
        data-testid="onboarding-generating"
        className="flex min-h-[100dvh] flex-col items-center justify-center bg-white p-6 text-[#1A1A1A]"
      >
        <div
          aria-hidden="true"
          className="h-14 w-14 rounded-full border-4 border-[#E8F0EA] border-t-[#2C694F] motion-safe:animate-spin"
        />
        <h1 className="mt-6 text-xl font-bold">正在生成您的健康评估</h1>
        <p
          role="status"
          aria-live="polite"
          className="mt-2 h-6 text-sm text-[#6B7280]"
        >
          {LOADING_PHRASES[phraseIndex]}
        </p>
      </div>
    );
  }

  const single = SINGLE_STEPS[step.id];
  const bounds = dateBounds();
  const birthError =
    birthInput.replace(/\D/g, "").length === 8 && !answers.birth_date
      ? "请输入有效的出生日期"
      : "";
  const heightError = rangeError(heightInput, HEIGHT_RANGE, "身高", "cm");
  const weightError = rangeError(weightInput, WEIGHT_RANGE, "体重", "kg");
  const nextIfReady = () => {
    if (canProceed) goNext();
  };

  let body: ReactNode = null;
  let footer: ReactNode = null;

  if (single) {
    body = (
      <ChoiceList
        options={single.options}
        value={answers[single.key]}
        onChoose={(value) => choose(single.key, value)}
      />
    );
    if (step.optional)
      footer = <OnboardingSkipButton onClick={() => skip(single.key)} />;
  } else if (step.id === "birth_date") {
    body = (
      <ValueBox
        id="birthDate"
        label="出生日期"
        type="date"
        value={birthInput}
        placeholder="YYYY / MM / DD"
        error={birthError}
        onChange={(raw) => {
          const masked = formatBirthInput(raw);
          setBirthInput(masked);
          update("birth_date", parseBirthInput(masked, bounds));
        }}
        onEnter={nextIfReady}
      />
    );
  } else if (step.id === "height") {
    body = (
      <ValueBox
        id="height"
        label="身高（cm）"
        type="number"
        unit="cm"
        value={heightInput}
        min={HEIGHT_RANGE.min}
        max={HEIGHT_RANGE.max}
        step={0.1}
        placeholder="170"
        error={heightError}
        onChange={(raw) =>
          onMetric(raw, setHeightInput, "height_cm", HEIGHT_RANGE)
        }
        onEnter={nextIfReady}
      />
    );
  } else if (step.id === "weight") {
    body = (
      <ValueBox
        id="weight"
        label="体重（kg）"
        type="number"
        unit="kg"
        value={weightInput}
        min={WEIGHT_RANGE.min}
        max={WEIGHT_RANGE.max}
        step={0.1}
        placeholder="65"
        error={weightError}
        onChange={(raw) =>
          onMetric(raw, setWeightInput, "weight_kg", WEIGHT_RANGE)
        }
        onEnter={nextIfReady}
      />
    );
  } else if (step.id === "discomfort") {
    body = (
      <ChipGrid
        options={DISCOMFORT_OPTIONS}
        values={answers.discomfort_regions}
        onToggle={toggleDiscomfort}
      />
    );
  } else if (step.id === "sleep") {
    body = (
      <ChipGrid
        options={SLEEP_OPTIONS}
        values={answers.sleep_tags}
        onToggle={toggleSleep}
      />
    );
  } else if (step.id === "upload") {
    body = <UploadStep />;
  }

  if (!single) {
    const isLast = stepIndex === TOTAL_STEPS - 1;
    footer = (
      <OnboardingPrimaryButton
        onClick={isLast ? handleSubmit : goNext}
        disabled={!canProceed}
      >
        {isLast ? "完成" : "继续"}
      </OnboardingPrimaryButton>
    );
  }

  return (
    <OnboardingLayout
      stepIndex={stepIndex}
      totalSteps={TOTAL_STEPS}
      section={step.section}
      title={step.title}
      subtitle={step.subtitle}
      onBack={goBack}
      footer={footer}
    >
      <div key={step.id}>{body}</div>
    </OnboardingLayout>
  );
}
