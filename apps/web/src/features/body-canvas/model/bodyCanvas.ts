import type { ConsultationSpatialContext } from "@/features/consultation/types/consultation";
import {
  BODY_REGION_IDS,
  getBodyRegionDefinition,
  type BodyRegionId,
} from "@/features/body-explorer/model/bodyRegionOntology";

export type CanvasTool = "orbit" | "brush" | "box";
export type CanvasPanel = "state" | "diagnosis" | "treatment" | "progress" | "equipment" | "anatomy" | "help" | "motions";
export type MotionId = "stand" | "run" | "jump" | "sit" | "arm_raise" | "calf_raise" | "hip_hinge" | "bridge";

export interface ReferenceMotion {
  id: MotionId;
  label: string;
  duration: number;
  description: string;
  regions: readonly BodyRegionId[];
}

/** Reference demonstrations, never prescriptions, observations or diagnoses. */
export const REFERENCE_MOTIONS: readonly ReferenceMotion[] = [
  { id: "stand", label: "站立", duration: 5, description: "静立参考位。旋转视角，选择需要查看的区域。", regions: [] },
  { id: "run", label: "跑步", duration: 1.2, description: "交替摆臂与下肢运动的通用示意，不是你的跑姿评估。", regions: ["hip.left", "hip.right", "knee.left", "knee.right"] },
  { id: "jump", label: "跳跃", duration: 2.8, description: "准备、起跳与落地阶段示意。可暂停查看任意阶段。", regions: ["thigh.left", "thigh.right", "calf.left", "calf.right"] },
  { id: "sit", label: "坐姿", duration: 5, description: "坐姿情境参考，不推断组织负荷或个人体态问题。", regions: ["neck", "lower_back", "hip.left", "hip.right"] },
  { id: "arm_raise", label: "抬臂观察", duration: 6, description: "观察双臂抬起与下降的阶段。执行要求以已接受方案为准。", regions: ["shoulder.left", "shoulder.right", "scapular.left", "scapular.right"] },
  { id: "calf_raise", label: "提踵", duration: 4, description: "提起与回落的动作示意，不自动给出次数、负重或适用性。", regions: ["calf.left", "calf.right", "ankle.left", "ankle.right"] },
  { id: "hip_hinge", label: "髋铰链", duration: 5, description: "髋部折叠与恢复站立的参考动作。", regions: ["hip.left", "hip.right", "lower_back"] },
  { id: "bridge", label: "臀桥", duration: 5, description: "仰卧屈膝与抬髋的通用示意；不是基于你的病历生成的训练建议。", regions: ["gluteal.left", "gluteal.right", "pelvis"] },
];

export function getReferenceMotion(id: MotionId): ReferenceMotion {
  return REFERENCE_MOTIONS.find((motion) => motion.id === id) ?? REFERENCE_MOTIONS[0];
}

export function demonstrationForTitle(title: string): MotionId | null {
  // Deliberately conservative: an unrelated movement is worse than no demo.
  if (/臀桥|glute\s*bridge/i.test(title)) return "bridge";
  if (/提踵|calf\s*raise/i.test(title)) return "calf_raise";
  if (/髋铰链|hip\s*hinge/i.test(title)) return "hip_hinge";
  if (/抬臂观察|arm\s*raise/i.test(title)) return "arm_raise";
  return null;
}

export interface CanvasSelection {
  regions: BodyRegionId[];
  history: BodyRegionId[][];
}
export type SelectionAction =
  | { type: "select"; regions: readonly BodyRegionId[]; append?: boolean }
  | { type: "remove"; region: BodyRegionId }
  | { type: "clear" }
  | { type: "undo" };
export const EMPTY_SELECTION: CanvasSelection = { regions: [], history: [] };

export function selectionReducer(state: CanvasSelection, action: SelectionAction): CanvasSelection {
  if (action.type === "undo") {
    const previous = state.history.at(-1);
    return previous ? { regions: previous, history: state.history.slice(0, -1) } : state;
  }
  const next = action.type === "clear" ? [] : action.type === "remove"
    ? state.regions.filter((id) => id !== action.region)
    : [...new Set([...(action.append ? state.regions : []), ...action.regions])].filter((id) => BODY_REGION_IDS.includes(id));
  if (next.length === state.regions.length && next.every((id, index) => id === state.regions[index])) return state;
  return { regions: next, history: [...state.history.slice(-19), state.regions] };
}

export function buildCanvasContext(
  regions: readonly BodyRegionId[],
  motion: MotionId | null,
  phase: number,
  playing: boolean,
): ConsultationSpatialContext {
  const unique = [...new Set(regions)].slice(0, BODY_REGION_IDS.length);
  const primary = unique[0];
  const referenceMotion = motion
    ? (() => {
        const definition = getReferenceMotion(motion);
        return {
          id: motion,
          label: definition.label,
          phase: Number.isFinite(phase) ? Math.min(1, Math.max(0, phase)) : 0,
          paused: !playing,
          source: "reference_animation" as const,
        };
      })()
    : undefined;
  return {
    ...(primary ? { body_region_id: primary, body_region_label: getBodyRegionDefinition(primary).labels["zh-CN"], body_region_ids: unique } : {}),
    ...(referenceMotion ? { reference_motion: referenceMotion } : {}),
  };
}

export function isEditingTarget(target: EventTarget | null): boolean {
  return target instanceof HTMLElement && Boolean(target.closest("input, textarea, select, [contenteditable='true'], [role='textbox'], [role='combobox'], [role='slider']"));
}

export type CanvasShortcut =
  | { type: "tool"; tool: CanvasTool }
  | { type: "panel"; panel: CanvasPanel }
  | { type: "motion"; motion: MotionId }
  | { type: "pause" | "reset" | "chat" | "escape" | "undo" };

export function canvasShortcut(event: Pick<KeyboardEvent, "key" | "ctrlKey" | "metaKey" | "altKey" | "isComposing" | "repeat" | "target" | "defaultPrevented">): CanvasShortcut | null {
  if (event.defaultPrevented || event.isComposing || event.repeat || event.ctrlKey || event.metaKey || event.altKey || isEditingTarget(event.target)) return null;
  const key = event.key.toLowerCase();
  if (key === "escape") return { type: "escape" };
  if (key === " " || key === "f") return { type: "pause" };
  if (key === "r") return { type: "reset" };
  if (key === "/") return { type: "chat" };
  if (key === "z") return { type: "undo" };
  const tools: Record<string, CanvasTool> = { v: "orbit", b: "brush", s: "box" };
  if (tools[key]) return { type: "tool", tool: tools[key] };
  const panels: Record<string, CanvasPanel> = { p: "treatment", d: "diagnosis", c: "state", e: "equipment", h: "progress", x: "anatomy", "?": "help" };
  if (panels[key]) return { type: "panel", panel: panels[key] };
  const motions: Record<string, MotionId> = { "1": "stand", "2": "run", "3": "jump", "4": "sit" };
  return motions[key] ? { type: "motion", motion: motions[key] } : null;
}

/** A mesh's name is an atlas visual hint, not a new durable anatomical ID. */
export function atlasSurfaceRegion(name: string, center: readonly [number, number, number]): BodyRegionId {
  const text = name.toLowerCase().replace(/[_-]/g, " ");
  const [x, y, z] = center;
  const side = /(?:\.l|\.left)$/.test(name) ? "left" : /(?:\.r|\.right)$/.test(name) ? "right" : x >= 0 ? "left" : "right";
  if (/scapul/.test(text)) return `scapular.${side}`;
  if (/deltoid|shoulder|acromial|axilla/.test(text)) return `shoulder.${side}`;
  if (/elbow|cubital|olecranon/.test(text)) return `elbow.${side}`;
  if (/forearm|antebrach/.test(text)) return `forearm.${side}`;
  if (/wrist|carpal/.test(text)) return `wrist.${side}`;
  if (/hand|finger|pollex|thumb|digit|minimus|palmar|palm|thenar|metacarp/.test(text) && y > .6) return `hand.${side}`;
  if (/region of arm|brachial/.test(text)) return `upper_arm.${side}`;
  if (/gluteal|gluteus|buttock/.test(text)) return `gluteal.${side}`;
  if (/knee|patell|popliteal/.test(text)) return `knee.${side}`;
  if (/ankle|malleol/.test(text)) return `ankle.${side}`;
  if (/foot|toe|plantar|calcaneal|tarsal|metatars/.test(text)) return `foot.${side}`;
  if (/region of leg|sural/.test(text)) return `calf.${side}`;
  if (/thigh|femoral/.test(text)) return `thigh.${side}`;
  if (/hip|trochanter/.test(text)) return `hip.${side}`;
  if (/lumbar|sacral/.test(text)) return "lower_back";
  if (/cervical|neck|carotid/.test(text)) return "neck";
  if (/thoracic|pectoral|mammary|sternal/.test(text)) return z < -.035 ? "upper_back" : "chest";
  if (/abdomin|umbilical|hypochondri/.test(text)) return "abdomen";
  if (y >= 1.51) return "head";
  if (y >= 1.445) return "neck";
  if (Math.abs(x) > .18 && y > .66) {
    if (y > 1.30) return `shoulder.${side}`;
    if (y > 1.15) return `upper_arm.${side}`;
    if (y > 1.065) return `elbow.${side}`;
    if (y > .885) return `forearm.${side}`;
    if (y > .82) return `wrist.${side}`;
    return `hand.${side}`;
  }
  if (y > 1.16) return z < -.025 ? "upper_back" : "chest";
  if (y > .96) return z < -.025 ? "lower_back" : "abdomen";
  if (y > .87) return Math.abs(x) < .065 ? "pelvis" : z < -.035 ? `gluteal.${side}` : `hip.${side}`;
  if (y > .51) return `thigh.${side}`;
  if (y > .385) return `knee.${side}`;
  if (y > .14) return `calf.${side}`;
  if (y > .075) return `ankle.${side}`;
  return `foot.${side}`;
}
