import {
  Box3,
  type BufferGeometry,
  FrontSide,
  Mesh,
  MeshStandardMaterial,
  Object3D,
  SkinnedMesh,
  Sphere,
  Vector3,
} from "three";
import { mergeGeometries } from "three/addons/utils/BufferGeometryUtils.js";
import type { ReferenceRig } from "./referenceRigJoints";
import { bakeSkinnedGeometry } from "./referenceRigSkinning";
import {
  addMuscleAttributes,
  createMuscleMaterial,
  MUSCLE_COLOR,
} from "./referenceMuscleChart";

const BONE_COLOR = "#e8dfc8";

export interface ReferenceLayer {
  mesh: SkinnedMesh<BufferGeometry, MeshStandardMaterial>;
  dispose: () => void;
}
export type ReferenceMuscles = ReferenceLayer;

/**
 * Adds an atlas layer (muscles, bones) to the rig, skinned to the same skeleton so
 * it follows every reference pose. Display only: picking still reads the skin regions.
 */
export function attachReferenceLayer(
  rig: ReferenceRig,
  source: Object3D,
  options: { name: string; color: string; muscleChart?: boolean },
): ReferenceLayer {
  const geometries: BufferGeometry[] = [];
  source.updateMatrixWorld(true);
  source.traverse((object) => {
    if (!(object instanceof Mesh)) return;
    const geometry = bakeSkinnedGeometry(object);
    if (options.muscleChart)
      addMuscleAttributes(geometry, object.name.endsWith("__tendon"));
    geometries.push(geometry);
  });
  const geometry = geometries.length
    ? mergeGeometries(geometries, false)
    : null;
  for (const item of geometries) item.dispose();
  if (!geometry) throw new Error(`参考图谱 ${options.name} 无法合并`);
  const material = options.muscleChart
    ? createMuscleMaterial()
    : new MeshStandardMaterial({
        color: options.color,
        roughness: 0.7,
        metalness: 0,
        side: FrontSide,
      });
  const mesh = new SkinnedMesh(geometry, material);
  mesh.name = options.name;
  mesh.renderOrder = 1;
  mesh.frustumCulled = false;
  mesh.boundingSphere = new Sphere(new Vector3(0, 0.7, 0), 3);
  mesh.boundingBox = new Box3(new Vector3(-3, -3, -3), new Vector3(3, 4, 3));
  mesh.raycast = () => {};
  mesh.bind(rig.skeleton);
  rig.group.add(mesh);
  rig.group.updateMatrixWorld(true);
  return {
    mesh,
    dispose() {
      rig.group.remove(mesh);
      geometry.dispose();
      material.dispose();
    },
  };
}

/** Full-body Z-Anatomy muscles (parts named `…__muscle` / `…__tendon`), shaded like an anatomical chart. */
export function attachReferenceMuscles(
  rig: ReferenceRig,
  source: Object3D,
): ReferenceMuscles {
  return attachReferenceLayer(rig, source, {
    name: "reference-muscles",
    color: MUSCLE_COLOR,
    muscleChart: true,
  });
}

export function attachReferenceSkeleton(
  rig: ReferenceRig,
  source: Object3D,
): ReferenceLayer {
  return attachReferenceLayer(rig, source, {
    name: "reference-skeleton",
    color: BONE_COLOR,
  });
}
