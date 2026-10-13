import { BoxGeometry, Group, Mesh, MeshBasicMaterial } from "three";
import { describe, expect, it } from "vitest";
import {
  buildReferenceRig,
  referenceBounds,
  sampleReferencePose,
  skinInfluence,
} from "./referenceRig";

describe("Body Canvas reference rig", () => {
  it("keeps the pinned atlas surface indexed instead of tripling vertices", () => {
    const source = new Group();
    const sourceGeometry = new BoxGeometry(0.12, 0.1, 0.08);
    const originalVertexCount = sourceGeometry.getAttribute("position").count;
    expect(sourceGeometry.getIndex()).not.toBeNull();

    const shell = new Mesh(sourceGeometry, new MeshBasicMaterial());
    shell.name = "body-shell__Deltoid region.r";
    shell.position.set(-0.2, 1.35, 0);
    shell.scale.set(-1, 1, 1);
    source.add(shell);

    const rig = buildReferenceRig(source);
    try {
      expect(rig.meshes).toHaveLength(1);
      expect(rig.meshes[0].geometry.getIndex()).not.toBeNull();
      expect(rig.meshes[0].geometry.getAttribute("position").count).toBe(
        originalVertexCount,
      );
      const bounds = referenceBounds(rig);
      expect(bounds.isEmpty()).toBe(false);
      expect(Number.isFinite(bounds.min.x)).toBe(true);
      expect(Number.isFinite(bounds.max.y)).toBe(true);
    } finally {
      rig.dispose();
      sourceGeometry.dispose();
      shell.material.dispose();
    }
  });

  it("clamps skin blending and reference pose phases", () => {
    const [, , armWeight] = skinInfluence(0.24, 1.33);
    expect(armWeight).toBeGreaterThanOrEqual(0);
    expect(armWeight).toBeLessThanOrEqual(1);

    expect(sampleReferencePose("arm_raise", -3)).toEqual(
      sampleReferencePose("arm_raise", 0),
    );
    expect(sampleReferencePose("arm_raise", 7)).toEqual(
      sampleReferencePose("arm_raise", 1),
    );
  });
});
