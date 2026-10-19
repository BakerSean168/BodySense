import { BoxGeometry, Group, Mesh, MeshBasicMaterial, Vector3 } from "three";
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

  it("keeps mirrored shell normals outward and in step with the winding", () => {
    // Vanatome's left-side shells are the right-side meshes under scale.x = -1.
    // Both halves must light the same way: normals outward, faces front-facing.
    const outward = (mirrored: boolean) => {
      const source = new Group();
      const sourceGeometry = new BoxGeometry(0.12, 0.1, 0.08);
      const shell = new Mesh(sourceGeometry, new MeshBasicMaterial());
      shell.name = `body-shell__Deltoid region.${mirrored ? "l" : "r"}`;
      shell.position.set(mirrored ? 0.2 : -0.2, 1.35, 0);
      if (mirrored) shell.scale.set(-1, 1, 1);
      source.add(shell);
      const rig = buildReferenceRig(source);
      try {
        const geometry = rig.meshes[0].geometry;
        const position = geometry.getAttribute("position");
        const normal = geometry.getAttribute("normal");
        const index = geometry.getIndex()!;
        const center = new Vector3(mirrored ? 0.2 : -0.2, 1.35, 0);
        const p = (i: number) => new Vector3().fromBufferAttribute(position, i);
        const n = (i: number) => new Vector3().fromBufferAttribute(normal, i);
        for (let i = 0; i < position.count; i++) {
          expect(n(i).dot(p(i).sub(center))).toBeGreaterThan(0);
        }
        for (let t = 0; t < index.count; t += 3) {
          const [a, b, c] = [index.getX(t), index.getX(t + 1), index.getX(t + 2)];
          const face = p(b).sub(p(a)).cross(p(c).sub(p(a)));
          expect(face.dot(n(a))).toBeGreaterThan(0);
        }
      } finally {
        rig.dispose();
        sourceGeometry.dispose();
        shell.material.dispose();
      }
    };
    outward(false);
    outward(true);
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
