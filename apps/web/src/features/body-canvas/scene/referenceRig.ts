import {
  Bone,
  Box3,
  type BufferAttribute,
  type BufferGeometry,
  FrontSide,
  Group,
  Mesh,
  MeshStandardMaterial,
  Object3D,
  Skeleton,
  SkinnedMesh,
  Sphere,
  Vector3,
} from "three";
import { mergeGeometries } from "three/addons/utils/BufferGeometryUtils.js";
import type { BodyRegionId } from "@/features/body-explorer/model/bodyRegionOntology";
import { atlasSurfaceRegion } from "../model/bodyCanvas";
import {
  JOINTS,
  type ReferenceBones,
  type ReferenceRig,
} from "./referenceRigJoints";
import {
  bakeSkinnedGeometry,
  SKIN_COLOR,
  SKIN_OPACITY,
} from "./referenceRigSkinning";

export { JOINTS } from "./referenceRigJoints";
export type {
  JointId,
  ReferenceBones,
  ReferenceRig,
} from "./referenceRigJoints";
export {
  SKIN_COLOR,
  SKIN_OPACITY,
  SKIN_SELECTED_OPACITY,
  skinInfluence,
} from "./referenceRigSkinning";
export {
  attachReferenceLayer,
  attachReferenceMuscles,
  attachReferenceSkeleton,
} from "./referenceRigLayers";
export type { ReferenceLayer, ReferenceMuscles } from "./referenceRigLayers";
export { applyReferencePose, sampleReferencePose } from "./referenceRigPose";
export type { ReferencePose } from "./referenceRigPose";

export function buildReferenceRig(source: Object3D): ReferenceRig {
  const group = new Group();
  group.name = "BodySense / Reference animation / NOT a patient measurement";
  const entries = JOINTS.map(([id]) => [id, new Bone()] as const);
  const bones = Object.fromEntries(entries) as ReferenceBones;
  JOINTS.forEach(([id, parent, x, y, z]) => {
    const bone = bones[id];
    bone.name = id;
    const origin = JOINTS.find(([key]) => key === parent);
    bone.position.set(
      x - (origin?.[2] ?? 0),
      y - (origin?.[3] ?? 0),
      z - (origin?.[4] ?? 0),
    );
    if (parent) bones[parent].add(bone);
    else group.add(bone);
  });
  group.updateMatrixWorld(true);
  const skeleton = new Skeleton(JOINTS.map(([id]) => bones[id]));
  skeleton.calculateInverses();
  const regions = new Map<BodyRegionId, BufferGeometry[]>();
  source.updateMatrixWorld(true);
  source.traverse((object) => {
    if (!(object instanceof Mesh) || !object.name.startsWith("body-shell"))
      return;
    const geometry = bakeSkinnedGeometry(object);
    geometry.computeBoundingBox();
    const center = geometry.boundingBox!.getCenter(new Vector3());
    const region = atlasSurfaceRegion(object.name, [
      center.x,
      center.y,
      center.z,
    ]);
    const list = regions.get(region) ?? [];
    list.push(geometry);
    regions.set(region, list);
  });
  if (!regions.size) {
    skeleton.dispose();
    throw new Error("参考图谱缺少可识别的身体表面");
  }
  const meshes: ReferenceRig["meshes"] = [];
  for (const [region, geometries] of regions) {
    const geometry = mergeGeometries(geometries, false);
    for (const item of geometries) item.dispose();
    if (!geometry) throw new Error("参考图谱表面无法合并");
    // Translucent paper skin over the muscular layer: the figure stays whole,
    // surface detail is not shown. depthWrite off so the muscles read through.
    const material = new MeshStandardMaterial({
      color: SKIN_COLOR,
      roughness: 0.85,
      metalness: 0,
      side: FrontSide,
      transparent: true,
      opacity: SKIN_OPACITY,
      depthWrite: false,
    });
    const mesh = new SkinnedMesh(geometry, material);
    mesh.renderOrder = 2;
    mesh.name = region;
    mesh.userData.bodyRegionId = region;
    mesh.frustumCulled = false;
    // Conservative bounds cover every permitted reference pose; exact hits still use skinned triangles.
    mesh.boundingSphere = new Sphere(new Vector3(0, 0.7, 0), 3);
    mesh.boundingBox = new Box3(new Vector3(-3, -3, -3), new Vector3(3, 4, 3));
    mesh.bind(skeleton);
    group.add(mesh);
    meshes.push(mesh);
  }
  group.updateMatrixWorld(true);
  return {
    group,
    bones,
    skeleton,
    meshes,
    dispose() {
      meshes.forEach((mesh) => {
        mesh.geometry.dispose();
        mesh.material.dispose();
      });
      skeleton.dispose();
      group.clear();
    },
  };
}

/** Exposed for asset qualification tests without WebGL. */
export function referenceBounds(rig: ReferenceRig): Box3 {
  const bounds = new Box3();
  const point = new Vector3();
  for (const mesh of rig.meshes) {
    const attribute: BufferAttribute = mesh.geometry.getAttribute(
      "position",
    ) as BufferAttribute;
    for (let i = 0; i < attribute.count; i++)
      bounds.expandByPoint(mesh.getVertexPosition(i, point));
  }
  return bounds;
}
