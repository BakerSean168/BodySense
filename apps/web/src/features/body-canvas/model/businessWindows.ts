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
 * 5. At most one business window is *visible* at a time. Dock/Open/Active are
 *    mutually exclusive: opening or restoring a window minimizes the others
 *    instead of destroying them, so two windows can never overlap and steal each
 *    other's controls while the other keeps its task state for restore.
 */

export type BusinessWindowId =
  "chat" | "person" | "diagnosis" | "training" | "trainer";

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
    dockLabel: "对话",
    key: "/",
  },
  {
    id: "person",
    title: "个人档案",
    description: "当前个人资料、身体记录、器材与资料。",
    dockLabel: "个人",
    key: "M",
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

/**
 * Legacy deep-link aliases. The previous surface exposed `?view=state|progress`
 * and those URLs must keep resolving to the same business object after the
 * window refactor, so old links are not silently broken.
 */
const LEGACY_WINDOW_ALIASES: Record<string, BusinessWindowId> = {
  state: "person",
  progress: "person",
};

/** Resolve a `?view=` value to a business window, including legacy aliases. */
export function resolveWindowFromView(
  value: string | null | undefined,
): BusinessWindowId | null {
  if (!value) return null;
  if (isBusinessWindowId(value)) return value;
  return LEGACY_WINDOW_ALIASES[value] ?? null;
}

export function getBusinessWindowMeta(
  id: BusinessWindowId,
): BusinessWindowMeta {
  return (
    BUSINESS_WINDOWS.find((window) => window.id === id) ?? BUSINESS_WINDOWS[0]
  );
}

export function isBusinessWindowId(
  value: string | null | undefined,
): value is BusinessWindowId {
  return (
    typeof value === "string" &&
    (BUSINESS_WINDOW_IDS as readonly string[]).includes(value)
  );
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
    case "open":
    case "restore": {
      // Opening an unopened window and restoring a minimized one are the same
      // operation: focus it and fold every other visible window away.
      const order = toFront(state.order, action.window);
      const minimized = [
        ...new Set([
          ...without(state.minimized, action.window),
          ...without(state.order, action.window),
        ]),
      ];
      return { order, minimized };
    }
    case "minimize": {
      if (
        !state.order.includes(action.window) ||
        state.minimized.includes(action.window)
      ) {
        return state;
      }
      return {
        order: state.order,
        minimized: [...state.minimized, action.window],
      };
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
        // Toggling away the active window reveals the body but keeps the window
        // mounted and recoverable rather than discarding its task state.
        return businessWindowReducer(state, {
          type: "minimize",
          window: action.window,
        });
      }
      return businessWindowReducer(state, {
        type: "open",
        window: action.window,
      });
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
 * Penpot V2 desktop dock geometry, read from every "Desktop / <Window> / …"
 * board (1440 x 900, no app bar): each window is docked 40px from one side of
 * the screen at y 92 and is 700px tall, ending at y 792 above the Dock, so the
 * body stays visible beside it. Person and Chat dock left; Diagnosis, Training
 * and Trainer (and Training / Safety restrictions) dock right. CSS bounds the
 * sizes to the real stage. Mobile uses a 358px sheet in the 390px viewport.
 */
export interface BusinessWindowSize {
  width: number;
  height: number;
}

/** The side of the desktop stage a window docks to. */
export type BusinessWindowDock = "left" | "right";

export interface BusinessWindowLayout {
  desktop: BusinessWindowSize;
  /**
   * Narrower desktop width for the window's detail scenes, when Penpot draws
   * them narrower than the home scene (Person: Body / Episode, Body / Region
   * and Assets are 560 wide, the category lists 860).
   */
  sceneWidth?: number;
  dock: BusinessWindowDock;
  mobile: BusinessWindowSize;
  /** Upper bound applied when the stage is short. */
  maxHeight: number;
}

export const BUSINESS_WINDOW_LAYOUT: Record<
  BusinessWindowId,
  BusinessWindowLayout
> = {
  chat: {
    desktop: { width: 560, height: 700 },
    dock: "left",
    mobile: { width: 358, height: 704 },
    maxHeight: 700,
  },
  person: {
    desktop: { width: 860, height: 700 },
    sceneWidth: 560,
    dock: "left",
    mobile: { width: 358, height: 704 },
    maxHeight: 700,
  },
  diagnosis: {
    desktop: { width: 600, height: 700 },
    dock: "right",
    mobile: { width: 358, height: 704 },
    maxHeight: 700,
  },
  training: {
    desktop: { width: 600, height: 700 },
    dock: "right",
    mobile: { width: 358, height: 704 },
    maxHeight: 700,
  },
  trainer: {
    desktop: { width: 600, height: 700 },
    dock: "right",
    mobile: { width: 358, height: 704 },
    maxHeight: 700,
  },
};

/** Distance from the docked side of the stage (Penpot x 40 / 1440 - 40). */
export const DESKTOP_WINDOW_BASE_LEFT = 40;
/** Penpot screen y 92: the stage is full height (no app bar), below the ribbon. */
export const DESKTOP_WINDOW_BASE_TOP = 92;
/** Space kept below a window for the Dock: Penpot 900 - 792. */
export const DESKTOP_WINDOW_BOTTOM_RESERVE = 108;
export const DESKTOP_WINDOW_STACK_STEP = 26;
const DESKTOP_WINDOW_MAX_STACK = 3;

/**
 * Offset of a desktop window from its docked side and the stage top. CSS
 * applies `left` as the margin on whichever side the window docks to. The
 * cascade is kept for keyboard/stage positioning and future window stacking.
 */
export function desktopWindowOffset(visibleIndex: number): {
  left: number;
  top: number;
} {
  const bounded = Number.isFinite(visibleIndex)
    ? Math.max(0, Math.min(Math.floor(visibleIndex), DESKTOP_WINDOW_MAX_STACK))
    : 0;
  return {
    left: DESKTOP_WINDOW_BASE_LEFT + bounded * DESKTOP_WINDOW_STACK_STEP,
    top: DESKTOP_WINDOW_BASE_TOP + bounded * DESKTOP_WINDOW_STACK_STEP,
  };
}

/**
 * Placement index of a window. With mutual exclusion only the visible window
 * matters, so this is always 0 for the active window and -1 when hidden.
 */
export function businessWindowStackIndex(
  state: BusinessWindowState,
  id: BusinessWindowId,
): number {
  return isBusinessWindowVisible(state, id) ? 0 : -1;
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
export function computeBmi(
  metrics: BodyMetricsInput | null | undefined,
): number | null {
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
  const finite =
    typeof value === "number" && Number.isFinite(value) ? value : null;
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
