/**
 * Business window identity and lifecycle for the body-first workspace.
 *
 * BodySense is not a split-screen product: one interactive 3D body stage stays
 * mounted while independently openable business windows (Chat, Person,
 * Diagnosis, TrainingPlan) float above it. This module owns the *set* of open
 * windows and its invariants so the React surface only renders derived state.
 *
 * Invariants enforced here:
 * 1. `order` contains each window id at most once, so a plan variant switch can
 *    never leave a second Training window behind.
 * 2. Only one TrainingPlan window identity exists at all (`training`); the
 *    selectable plan is separate state, not a second window.
 * 3. `minimized` is always a subset of `order`; closing a window clears it.
 * 4. `active` is the most recently opened/restored/focused window.
 */

export type BusinessWindowId =
  | "chat"
  | "person"
  | "diagnosis"
  | "training"
  | "trainer";

export interface BusinessWindowMeta {
  id: BusinessWindowId;
  /** Single level-1 business title. Layout renders it once, without subtitle. */
  title: string;
  /** Accessible description for the window region. */
  description: string;
  /** Short Dock label. */
  dockLabel: string;
  /** Canvas shortcut key, matching `canvasShortcut`. */
  key: string;
}

export const BUSINESS_WINDOWS: readonly BusinessWindowMeta[] = [
  {
    id: "chat",
    title: "对话",
    description: "围绕身体记录提问、核对建议并确认结果。",
    dockLabel: "助手",
    key: "/",
  },
  {
    id: "person",
    title: "个人档案",
    description: "当前个人资料、身体记录、器材与资料。",
    dockLabel: "档案",
    key: "I",
  },
  {
    id: "diagnosis",
    title: "综合诊断",
    description: "当前唯一综合诊断中已经确认的病情。",
    dockLabel: "诊断",
    key: "D",
  },
  {
    id: "training",
    title: "训练计划",
    description: "当前计划已接受的热身、训练与放松动作。",
    dockLabel: "训练",
    key: "P",
  },
  {
    id: "trainer",
    title: "训练执行",
    description: "执行当前计划、记录完成情况与身体变化。",
    dockLabel: "执行",
    key: "T",
  },
] as const;

export const BUSINESS_WINDOW_IDS: readonly BusinessWindowId[] =
  BUSINESS_WINDOWS.map((window) => window.id);

export function getBusinessWindowMeta(id: BusinessWindowId): BusinessWindowMeta {
  return BUSINESS_WINDOWS.find((window) => window.id === id) ?? BUSINESS_WINDOWS[0];
}

export function isBusinessWindowId(value: string | null | undefined): value is BusinessWindowId {
  return typeof value === "string" && (BUSINESS_WINDOW_IDS as readonly string[]).includes(value);
}

/**
 * Rendering order. `order` is open-window stacking (last = active);
 * `minimized` keeps a window recoverable without occupying the stage.
 */
export interface BusinessWindowState {
  order: BusinessWindowId[];
  minimized: BusinessWindowId[];
}

export type BusinessWindowAction =
  | { type: "open"; window: BusinessWindowId }
  | { type: "close"; window: BusinessWindowId }
  | { type: "toggle"; window: BusinessWindowId }
  | { type: "minimize"; window: BusinessWindowId }
  | { type: "restore"; window: BusinessWindowId }
  | { type: "closeAll" };

export const EMPTY_BUSINESS_WINDOWS: BusinessWindowState = {
  order: [],
  minimized: [],
};

function without(
  values: readonly BusinessWindowId[],
  target: BusinessWindowId,
): BusinessWindowId[] {
  return values.filter((value) => value !== target);
}

/** Bring to the top of the stack without duplicating the identity. */
function toFront(
  values: readonly BusinessWindowId[],
  target: BusinessWindowId,
): BusinessWindowId[] {
  return [...without(values, target), target];
}

export function businessWindowReducer(
  state: BusinessWindowState,
  action: BusinessWindowAction,
): BusinessWindowState {
  switch (action.type) {
    case "open": {
      const order = toFront(state.order, action.window);
      const minimized = without(state.minimized, action.window);
      return { order, minimized };
    }
    case "restore": {
      // Restoring an unopened window is an open; restoring a visible window just
      // focuses it. Both keep exactly one entry per identity.
      if (!state.order.includes(action.window)) {
        return { order: toFront(state.order, action.window), minimized: state.minimized };
      }
      return {
        order: toFront(state.order, action.window),
        minimized: without(state.minimized, action.window),
      };
    }
    case "minimize": {
      if (!state.order.includes(action.window) || state.minimized.includes(action.window)) {
        return state;
      }
      return { order: state.order, minimized: [...state.minimized, action.window] };
    }
    case "close": {
      if (!state.order.includes(action.window)) return state;
      return {
        order: without(state.order, action.window),
        minimized: without(state.minimized, action.window),
      };
    }
    case "toggle": {
      if (isBusinessWindowVisible(state, action.window)) {
        return businessWindowReducer(state, { type: "close", window: action.window });
      }
      return businessWindowReducer(state, { type: "open", window: action.window });
    }
    case "closeAll":
      return state.order.length === 0 ? state : EMPTY_BUSINESS_WINDOWS;
    default:
      return state;
  }
}

export function isBusinessWindowOpen(
  state: BusinessWindowState,
  id: BusinessWindowId,
): boolean {
  return state.order.includes(id);
}

/** Open and not minimized — i.e. actually occupying stage space. */
export function isBusinessWindowVisible(
  state: BusinessWindowState,
  id: BusinessWindowId,
): boolean {
  return state.order.includes(id) && !state.minimized.includes(id);
}

export function visibleBusinessWindows(
  state: BusinessWindowState,
): BusinessWindowId[] {
  return state.order.filter((id) => !state.minimized.includes(id));
}

/** The most recently focused visible window, or null when the body is clear. */
export function activeBusinessWindow(
  state: BusinessWindowState,
): BusinessWindowId | null {
  for (let index = state.order.length - 1; index >= 0; index -= 1) {
    const candidate = state.order[index];
    if (!state.minimized.includes(candidate)) return candidate;
  }
  return null;
}

export function minimizedBusinessWindows(
  state: BusinessWindowState,
): BusinessWindowId[] {
  return state.order.filter((id) => state.minimized.includes(id));
}

/**
 * Escape/close semantics: first leave a transient surface (window), then fall
 * back to clearing the body selection. Returning null means "no window left to
 * close", so callers keep the selection-clearing fallback intact.
 */
export function topmostClosableWindow(
  state: BusinessWindowState,
): BusinessWindowId | null {
  return activeBusinessWindow(state);
}

/**
 * Desktop geometry per Penpot V5.5 (Diagnosis 420x574, TrainingPlan 420x615,
 * Chat 374x528). The prototype also froze absolute x/y so its 878px Viewer had
 * no clipping; production must adapt to the real viewport, so only the preferred
 * offset is derived here and CSS clamps it into the visible stage.
 */
export interface BusinessWindowSize {
  width: number;
  height: number;
}

export interface BusinessWindowLayout {
  desktop: BusinessWindowSize;
  mobile: BusinessWindowSize;
  /** Upper bound applied when the stage is short. */
  maxHeight: number;
}

export const BUSINESS_WINDOW_LAYOUT: Record<BusinessWindowId, BusinessWindowLayout> = {
  chat: {
    desktop: { width: 374, height: 528 },
    mobile: { width: 390, height: 650 },
    maxHeight: 560,
  },
  person: {
    desktop: { width: 1060, height: 720 },
    mobile: { width: 390, height: 650 },
    maxHeight: 780,
  },
  diagnosis: {
    desktop: { width: 420, height: 574 },
    mobile: { width: 390, height: 548 },
    maxHeight: 606,
  },
  training: {
    desktop: { width: 420, height: 615 },
    mobile: { width: 390, height: 650 },
    maxHeight: 640,
  },
  trainer: {
    desktop: { width: 460, height: 620 },
    mobile: { width: 390, height: 650 },
    maxHeight: 660,
  },
};

export const DESKTOP_WINDOW_BASE_LEFT = 430;
export const DESKTOP_WINDOW_BASE_TOP = 18;
export const DESKTOP_WINDOW_STACK_STEP = 26;
const DESKTOP_WINDOW_MAX_STACK = 3;

/**
 * Cascade offset so several windows stay distinguishable while the body stays
 * visible. Deliberately not a fixed absolute position: the consuming CSS clamps
 * the result against the current stage width.
 */
export function desktopWindowOffset(openIndex: number): { left: number; top: number } {
  const bounded = Number.isFinite(openIndex)
    ? Math.max(0, Math.min(Math.floor(openIndex), DESKTOP_WINDOW_MAX_STACK))
    : 0;
  return {
    left: DESKTOP_WINDOW_BASE_LEFT + bounded * DESKTOP_WINDOW_STACK_STEP,
    top: DESKTOP_WINDOW_BASE_TOP + bounded * DESKTOP_WINDOW_STACK_STEP,
  };
}

/** Index of a window inside the open stack; -1 when it is not open. */
export function businessWindowStackIndex(
  state: BusinessWindowState,
  id: BusinessWindowId,
): number {
  return state.order.indexOf(id);
}

/** Body metrics are user-entered values; BMI is only derived when both exist. */
export interface BodyMetricLike {
  value?: number;
  unit?: string;
}

export interface BodyMetricsInput {
  height?: BodyMetricLike | null;
  weight?: BodyMetricLike | null;
  bmi?: number | null;
}

function finitePositive(value: unknown): number | null {
  return typeof value === "number" && Number.isFinite(value) && value > 0
    ? value
    : null;
}

/**
 * BMI uses confirmed height in metres and weight in kilograms. Missing or
 * invalid input yields null so the UI shows a dash instead of an invented value.
 * An explicit server-provided BMI is preferred when it is numeric.
 */
export function computeBmi(metrics: BodyMetricsInput | null | undefined): number | null {
  if (!metrics) return null;
  const explicit = finitePositive(metrics.bmi);
  if (explicit) return Math.round(explicit * 10) / 10;
  const heightCm = finitePositive(metrics.height?.value);
  const weightKg = finitePositive(metrics.weight?.value);
  if (!heightCm || !weightKg) return null;
  const metres = heightCm / 100;
  const bmi = weightKg / (metres * metres);
  if (!Number.isFinite(bmi) || bmi <= 0) return null;
  return Math.round(bmi * 10) / 10;
}

/** `— · —` style value line: never fabricate a placeholder value. */
export function formatMetricValue(
  value: number | null | undefined,
  unit: string,
  options: { digits?: number } = {},
): string {
  const finite = typeof value === "number" && Number.isFinite(value) ? value : null;
  if (finite === null) return unit ? `— ${unit}` : "—";
  const digits = options.digits ?? 0;
  return `${finite.toFixed(digits)} ${unit}`.trim();
}

/** `1998.01.13`-style compact date; blank input stays an explicit dash. */
export function formatCompactDate(value: string | null | undefined): string {
  if (!value) return "—";
  const [year, month, day] = value.slice(0, 10).split("-");
  if (!year || !month || !day) return value;
  return `${year}.${month}.${day}`;
}

export const GENDER_LABELS: Record<string, string> = {
  male: "男",
  female: "女",
};

export function formatGender(value: string | null | undefined): string {
  if (!value) return "—";
  return GENDER_LABELS[value] ?? value;
}
