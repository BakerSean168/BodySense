import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const submit = vi.fn();
const generate = vi.fn();

vi.mock("../../services/onboardingContextService", () => ({
  onboardingContextService: { submit: (...args: unknown[]) => submit(...args) },
}));
vi.mock("@/features/assessment/services/assessmentService", () => ({
  assessmentApi: { generate: (...args: unknown[]) => generate(...args) },
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("./steps/UploadStep", () => ({
  UploadStep: () => <div data-testid="upload-step-stub">upload</div>,
}));

import { useProfileStore } from "@/stores/profileStore";
import { OnboardingWizard } from "./OnboardingWizard";
import { toDiscomfortAreas,
  buildOnboardingPayload,
  formatBirthInput,
  INITIAL_ANSWERS,
  parseBirthInput,
  STEPS,
} from "./onboardingSteps";

function setReducedMotion(reduce: boolean) {
  Object.defineProperty(window, "matchMedia", {
    configurable: true,
    writable: true,
    value: (query: string) => ({
      matches: reduce && query.includes("reduce"),
      media: query,
      addEventListener: () => {},
      removeEventListener: () => {},
      addListener: () => {},
      removeListener: () => {},
      onchange: null,
      dispatchEvent: () => false,
    }),
  });
}

function renderWizard() {
  return render(
    <MemoryRouter initialEntries={["/onboarding"]}>
      <Routes>
        <Route path="/onboarding" element={<OnboardingWizard />} />
        <Route path="/consultation" element={<div>健康工作台</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

const heading = () => screen.getByRole("heading", { level: 1 });
const choose = (name: string) => fireEvent.click(screen.getByRole("button", { name }));
const next = () => fireEvent.click(screen.getByRole("button", { name: "继续" }));

beforeEach(() => {
  setReducedMotion(true);
  submit.mockResolvedValue({ body_state_revision: 1 });
  generate.mockResolvedValue({});
  act(() => useProfileStore.setState({ fetchProfile: vi.fn(async () => {}) }));
});

afterEach(() => {
  vi.useRealTimers();
  submit.mockReset();
  generate.mockReset();
});

describe("OnboardingWizard (Penpot 02 Onboarding)", () => {
  it("follows the 16 Penpot steps and submits the mapped onboarding command", async () => {
    renderWizard();

    expect(heading()).toHaveTextContent("您的性别是？");
    expect(screen.getByText("1/16")).toBeInTheDocument();
    expect(screen.getByText("基础信息")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "上一步" })).not.toBeInTheDocument();
    expect(screen.getByRole("progressbar")).toHaveAttribute("aria-valuenow", "1");
    choose("男");

    expect(heading()).toHaveTextContent("您的出生日期是？");
    expect(screen.getByRole("button", { name: "继续" })).toBeDisabled();
    fireEvent.change(screen.getByLabelText("出生日期"), { target: { value: "19950231" } });
    expect(screen.getByRole("alert")).toHaveTextContent("请输入有效的出生日期");
    fireEvent.change(screen.getByLabelText("出生日期"), { target: { value: "19950615" } });
    expect(screen.getByLabelText("出生日期")).toHaveValue("1995 / 06 / 15");
    next();

    expect(heading()).toHaveTextContent("您的身高是？");
    fireEvent.change(screen.getByLabelText("身高（cm）"), { target: { value: "300" } });
    expect(screen.getByRole("alert")).toHaveTextContent("身高必须在 50-250 cm 之间");
    expect(screen.getByRole("button", { name: "继续" })).toBeDisabled();
    fireEvent.change(screen.getByLabelText("身高（cm）"), { target: { value: "170" } });
    next();

    expect(heading()).toHaveTextContent("您的体重是？");
    fireEvent.change(screen.getByLabelText("体重（kg）"), { target: { value: "65" } });
    next();

    expect(heading()).toHaveTextContent("目前哪些部位有不适感？");
    expect(screen.getByText("身体状况")).toBeInTheDocument();
    choose("颈部");
    choose("腰部");
    expect(screen.getByRole("button", { name: "颈部" })).toHaveAttribute("aria-pressed", "true");
    choose("都没有不适");
    expect(screen.getByRole("button", { name: "颈部" })).toHaveAttribute("aria-pressed", "false");
    expect(screen.getByRole("button", { name: "都没有不适" })).toHaveAttribute("aria-pressed", "true");
    next();

    expect(heading()).toHaveTextContent("有无运动损伤或确诊的脊柱问题？");
    choose("腰椎问题");
    expect(heading()).toHaveTextContent("您日常的活动模式是？");
    expect(screen.queryByRole("button", { name: "跳过" })).not.toBeInTheDocument();
    choose("久坐办公");
    expect(heading()).toHaveTextContent("工作日平均每天坐着的时间？");
    expect(screen.getByText("生活习惯")).toBeInTheDocument();
    choose("4–6 小时");
    expect(heading()).toHaveTextContent("连续坐多久会起身活动一下？");
    choose("半小时–1 小时");

    expect(heading()).toHaveTextContent("睡眠与作息情况");
    choose("经常晚睡");
    choose("轮班");
    next();

    choose("有规律地锻炼");
    choose("跑步 / 快走");
    choose("偶尔");
    expect(heading()).toHaveTextContent("饮食习惯是？");
    fireEvent.click(screen.getByRole("button", { name: "跳过" }));
    expect(heading()).toHaveTextContent("烟酒情况？");
    choose("都不碰");

    expect(heading()).toHaveTextContent("上传体检报告或体态照片");
    expect(screen.getByText("16/16")).toBeInTheDocument();
    expect(screen.getByText("完成", { selector: "span" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "完成" }));

    await waitFor(() => expect(submit).toHaveBeenCalledTimes(1));
    const payload = submit.mock.calls[0][0];
    expect(payload).toMatchObject({
      expected_body_state_revision: 0,
      profile: { gender: "male", birth_date: "1995-06-15" },
      body_metrics: { height_cm: 170, weight_kg: 65 },
      injury_history: "腰椎问题",
    });
    expect(payload.lifestyle.activity).toEqual({
      summary: "久坐办公；工作日每天坐着约 4–6 小时；连续坐 半小时–1 小时会起身活动",
      details: { pattern: "sedentary_office", sitting_hours: "4-6", break_interval: "30m-1h" },
    });
    expect(payload.lifestyle.sleep).toEqual({
      summary: "经常晚睡；轮班",
      details: { tags: ["late", "shift"] },
    });
    expect(payload.lifestyle.exercise.summary).toBe("有规律地锻炼；跑步 / 快走；频率：偶尔");
    expect(payload.lifestyle.exercise.details).toMatchObject({ frequency: "occasional", type: "跑步 / 快走" });
    expect(payload.lifestyle.nutrition).toEqual({ summary: "", details: {} });
    expect(payload.lifestyle.substances).toEqual({ summary: "都不碰", details: { pattern: "none" } });
    expect(payload.lifestyle.recovery).toEqual({ summary: "" });
    // Step 05: "都没有不适" replaced the areas picked before it and is sent alone.
    expect(payload.discomfort_areas).toEqual(["none"]);
    expect(generate).toHaveBeenCalledTimes(1);
    expect(await screen.findByText("健康工作台")).toBeInTheDocument();
  });

  it("goes back with the header arrow and keeps earlier answers", () => {
    renderWizard();
    choose("女");
    expect(heading()).toHaveTextContent("您的出生日期是？");
    fireEvent.click(screen.getByRole("button", { name: "上一步" }));
    expect(heading()).toHaveTextContent("您的性别是？");
    expect(screen.getByRole("button", { name: "女" })).toHaveAttribute("aria-pressed", "true");
  });

  it("waits briefly before advancing when motion is allowed", () => {
    setReducedMotion(false);
    vi.useFakeTimers();
    renderWizard();
    choose("男");
    expect(heading()).toHaveTextContent("您的性别是？");
    expect(screen.getByRole("button", { name: "男" })).toHaveAttribute("aria-pressed", "true");
    act(() => {
      vi.advanceTimersByTime(250);
    });
    expect(heading()).toHaveTextContent("您的出生日期是？");
  });
});

describe("buildOnboardingPayload", () => {
  it("maps 无 to an explicit no-injury statement and leaves skipped sections empty", () => {
    const payload = buildOnboardingPayload({
      ...INITIAL_ANSWERS,
      gender: "female",
      birth_date: "1990-01-01",
      height_cm: 160,
      weight_kg: 50,
      injury: "none",
    });
    expect(payload.injury_history).toBe("无运动损伤或确诊的脊柱问题");
    expect(payload.lifestyle.activity).toEqual({ summary: "", details: {} });
  });

  it("sends step 05 areas as coarse codes in board order and omits an unanswered step", () => {
    const base = { ...INITIAL_ANSWERS, gender: "male" as const, birth_date: "1990-01-01", height_cm: 170, weight_kg: 60 };
    expect(
      buildOnboardingPayload({ ...base, discomfort_regions: ["leg", "neck", "lower_back"] }).discomfort_areas,
    ).toEqual(["neck", "lower_back", "leg"]);
    expect(buildOnboardingPayload({ ...base, discomfort_regions: ["neck", "none"] }).discomfort_areas).toEqual(["none"]);
    expect(buildOnboardingPayload(base)).not.toHaveProperty("discomfort_areas");
    expect(toDiscomfortAreas(["knee"])).toBeUndefined();
  });

  it("refuses to build without the required identity and metrics", () => {
    expect(() => buildOnboardingPayload(INITIAL_ANSWERS)).toThrow("请先填写性别和出生日期");
  });

  it("masks and parses the birth date like the board (1995 / 06 / 15)", () => {
    expect(formatBirthInput("1995-06-15")).toBe("1995 / 06 / 15");
    expect(formatBirthInput("19950")).toBe("1995 / 0");
    const bounds = { min: "1900-01-01", max: "2026-10-19" };
    expect(parseBirthInput("1995 / 06 / 15", bounds)).toBe("1995-06-15");
    expect(parseBirthInput("1995 / 02 / 30", bounds)).toBe("");
    expect(parseBirthInput("2030 / 01 / 01", bounds)).toBe("");
  });

  it("has 16 steps in the Penpot order", () => {
    expect(STEPS.map((s) => s.title)).toHaveLength(16);
    expect(STEPS[15].section).toBe("完成");
  });
});
