// Map of body part names to SVG group IDs
const BODY_PART_MAP: Record<string, string[]> = {
  头部: ["head"],
  颈椎: ["neck"],
  肩部: ["shoulder-left", "shoulder-right"],
  左肩: ["shoulder-left"],
  右肩: ["shoulder-right"],
  胸部: ["chest"],
  背部: ["back"],
  腰部: ["lower-back"],
  手臂: ["arm-left", "arm-right"],
  左手: ["arm-left"],
  右手: ["arm-right"],
  髋部: ["hip-left", "hip-right"],
  骨盆: ["pelvis"],
  膝盖: ["knee-left", "knee-right"],
  左膝: ["knee-left"],
  右膝: ["knee-right"],
  腿部: ["leg-left", "leg-right"],
  脚踝: ["ankle-left", "ankle-right"],
};

export function highlightedBodySvgIds(parts: readonly string[]): Set<string> {
  return new Set(parts.flatMap((part) => BODY_PART_MAP[part] ?? []));
}
