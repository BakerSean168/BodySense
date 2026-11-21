import {
  Bone,
  type BufferGeometry,
  type Group,
  type MeshStandardMaterial,
  type SkinnedMesh,
  type Skeleton,
} from "three";

export const JOINTS = [
  ["pelvis", null, 0, 0.94, 0],
  ["spine", "pelvis", 0, 1.08, 0],
  ["chest", "spine", 0, 1.3, 0],
  ["neck", "chest", 0, 1.46, 0],
  ["head", "neck", 0, 1.55, 0],
  ["upperArmL", "chest", 0.182, 1.36, -0.008],
  ["forearmL", "upperArmL", 0.222, 1.11, -0.008],
  ["handL", "forearmL", 0.265, 0.847, 0.03],
  ["upperArmR", "chest", -0.182, 1.36, -0.008],
  ["forearmR", "upperArmR", -0.222, 1.11, -0.008],
  ["handR", "forearmR", -0.265, 0.847, 0.03],
  ["thighL", "pelvis", 0.086, 0.91, 0],
  ["calfL", "thighL", 0.088, 0.441, 0.005],
  ["footL", "calfL", 0.088, 0.087, -0.025],
  ["thighR", "pelvis", -0.086, 0.91, 0],
  ["calfR", "thighR", -0.088, 0.441, 0.005],
  ["footR", "calfR", -0.088, 0.087, -0.025],
] as const;
export type JointId = (typeof JOINTS)[number][0];
export type ReferenceBones = Record<JointId, Bone>;
export interface ReferenceRig {
  group: Group;
  bones: ReferenceBones;
  skeleton: Skeleton;
  meshes: Array<SkinnedMesh<BufferGeometry, MeshStandardMaterial>>;
  dispose: () => void;
}
export const jointIndex = new Map<JointId, number>(
  JOINTS.map(([id], index) => [id, index]),
);
export const clamp = (value: number) => Math.max(0, Math.min(1, value));
export const smooth = (value: number) => {
  const t = clamp(value);
  return t * t * (3 - 2 * t);
};
