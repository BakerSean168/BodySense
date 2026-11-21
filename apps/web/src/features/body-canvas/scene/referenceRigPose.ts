import type { MotionId } from "../model/bodyCanvas";
import {
  clamp,
  JOINTS,
  type JointId,
  type ReferenceRig,
} from "./referenceRigJoints";

export interface ReferencePose {
  offset: [number, number, number];
  rotations: Partial<Record<JointId, readonly [number, number, number]>>;
}

export function sampleReferencePose(
  motion: MotionId,
  inputPhase: number,
): ReferencePose {
  const phase = Number.isFinite(inputPhase) ? clamp(inputPhase) : 0;
  const a = phase * Math.PI * 2;
  const wave = Math.sin(a);
  const u = (1 - Math.cos(a)) / 2;
  const pose: ReferencePose = { offset: [0, 0, 0], rotations: {} };
  const setX = (id: JointId, x: number) => {
    pose.rotations[id] = [x, 0, 0];
  };
  if (motion === "stand") {
    setX("chest", wave * 0.008);
    return pose;
  }
  if (motion === "run") {
    pose.offset[1] = Math.abs(wave) * 0.035;
    setX("spine", 0.1);
    setX("thighL", -0.78 * wave);
    setX("thighR", 0.78 * wave);
    setX("calfL", 0.54 + 0.54 * Math.sin(a - 1.1));
    setX("calfR", 0.54 - 0.54 * Math.sin(a - 1.1));
    setX("upperArmL", 0.6 * wave);
    setX("upperArmR", -0.6 * wave);
    setX("forearmL", -1.18);
    setX("forearmR", -1.18);
  } else if (motion === "sit") {
    pose.offset[1] = -0.43;
    setX("spine", 0.12);
    setX("neck", 0.08);
    for (const side of ["L", "R"] as const) {
      setX(`thigh${side}`, -1.46);
      setX(`calf${side}`, 1.46);
      setX(`upperArm${side}`, -0.58);
      setX(`forearm${side}`, -0.92);
    }
  } else if (motion === "jump") {
    const crouch =
      phase < 0.25
        ? Math.sin((phase / 0.25) * Math.PI)
        : phase > 0.75
          ? Math.sin(((phase - 0.75) / 0.25) * Math.PI) * 0.65
          : 0;
    const flight =
      phase >= 0.25 && phase <= 0.75
        ? Math.sin(((phase - 0.25) / 0.5) * Math.PI)
        : 0;
    pose.offset[1] = -0.15 * crouch + 0.26 * flight;
    setX("spine", 0.15 * crouch);
    for (const side of ["L", "R"] as const) {
      setX(`thigh${side}`, -0.62 * crouch);
      setX(`calf${side}`, 1.15 * crouch);
      setX(`upperArm${side}`, -0.9 * flight + 0.18 * crouch);
    }
  } else if (motion === "arm_raise") {
    pose.rotations.upperArmL = [-0.15 * u, 0, 2.18 * u];
    pose.rotations.upperArmR = [-0.15 * u, 0, -2.18 * u];
    setX("forearmL", -0.18 * u);
    setX("forearmR", -0.18 * u);
  } else if (motion === "calf_raise") {
    pose.offset[1] = 0.045 * u;
    setX("footL", -0.18 * u);
    setX("footR", -0.18 * u);
  } else if (motion === "hip_hinge") {
    pose.offset[1] = -0.12 * u;
    pose.offset[2] = -0.12 * u;
    setX("pelvis", 0.45 * u);
    setX("spine", 0.3 * u);
    setX("thighL", -0.62 * u);
    setX("thighR", -0.62 * u);
    setX("calfL", 0.3 * u);
    setX("calfR", 0.3 * u);
    setX("upperArmL", -0.55 * u);
    setX("upperArmR", -0.55 * u);
  } else if (motion === "bridge") {
    pose.offset = [0, -0.72 + 0.18 * u, 0.08];
    setX("pelvis", -Math.PI / 2 - 0.12 * u);
    for (const side of ["L", "R"] as const) {
      setX(`thigh${side}`, -1.08 + 0.3 * u);
      setX(`calf${side}`, 1.62);
    }
  }
  return pose;
}

export function applyReferencePose(
  rig: ReferenceRig,
  motion: MotionId,
  phase: number,
): void {
  const pose = sampleReferencePose(motion, phase);
  for (const [id] of JOINTS) {
    const value = pose.rotations[id] ?? [0, 0, 0];
    rig.bones[id].rotation.set(...value);
  }
  rig.bones.pelvis.position.set(
    pose.offset[0],
    0.94 + pose.offset[1],
    pose.offset[2],
  );
  rig.group.updateMatrixWorld(true);
  rig.skeleton.update();
}
