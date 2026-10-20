import {
  Bone, Box3, BufferAttribute, BufferGeometry, Color, Float32BufferAttribute, FrontSide,
  type InterleavedBufferAttribute,
  Group, Mesh, MeshStandardMaterial, Object3D, Skeleton, SkinnedMesh, Sphere,
  Uint16BufferAttribute, Vector3,
} from "three";
import { mergeGeometries } from "three/addons/utils/BufferGeometryUtils.js";
import type { BodyRegionId } from "@/features/body-explorer/model/bodyRegionOntology";
import { atlasSurfaceRegion, type MotionId } from "../model/bodyCanvas";

export const JOINTS = [
  ["pelvis", null, 0, .94, 0], ["spine", "pelvis", 0, 1.08, 0],
  ["chest", "spine", 0, 1.30, 0], ["neck", "chest", 0, 1.46, 0], ["head", "neck", 0, 1.55, 0],
  ["upperArmL", "chest", .182, 1.36, -.008], ["forearmL", "upperArmL", .222, 1.11, -.008], ["handL", "forearmL", .265, .847, .03],
  ["upperArmR", "chest", -.182, 1.36, -.008], ["forearmR", "upperArmR", -.222, 1.11, -.008], ["handR", "forearmR", -.265, .847, .03],
  ["thighL", "pelvis", .086, .91, 0], ["calfL", "thighL", .088, .441, .005], ["footL", "calfL", .088, .087, -.025],
  ["thighR", "pelvis", -.086, .91, 0], ["calfR", "thighR", -.088, .441, .005], ["footR", "calfR", -.088, .087, -.025],
] as const;
export type JointId = typeof JOINTS[number][0];
export type ReferenceBones = Record<JointId, Bone>;
export interface ReferenceRig {
  group: Group;
  bones: ReferenceBones;
  skeleton: Skeleton;
  meshes: Array<SkinnedMesh<BufferGeometry, MeshStandardMaterial>>;
  dispose: () => void;
}
const jointIndex = new Map<JointId, number>(JOINTS.map(([id], index) => [id, index]));
const clamp = (value: number) => Math.max(0, Math.min(1, value));
const smooth = (value: number) => { const t = clamp(value); return t * t * (3 - 2 * t); };

function restoreMirroredTriangleWinding(geometry: BufferGeometry): void {
  // Several right-side Vanatome shell nodes are mirrored with a negative scale.
  // Baking that matrix into geometry reverses triangle winding. Keep the source
  // indexed representation and swap the second/third index of every triangle;
  // this avoids tripling the vertex payload just to repair face orientation.
  const index = geometry.getIndex();
  if (index) {
    for (let offset = 0; offset + 2 < index.count; offset += 3) {
      const second = index.getX(offset + 1);
      index.setX(offset + 1, index.getX(offset + 2));
      index.setX(offset + 2, second);
    }
    index.needsUpdate = true;
    return;
  }

  // Defensive fallback for a future non-indexed atlas build.
  for (const attribute of Object.values(geometry.attributes)) {
    if (!(attribute instanceof BufferAttribute)) continue;
    for (let vertex = 0; vertex + 2 < attribute.count; vertex += 3) {
      for (let component = 0; component < attribute.itemSize; component += 1) {
        const second = attribute.getComponent(vertex + 1, component);
        attribute.setComponent(vertex + 1, component, attribute.getComponent(vertex + 2, component));
        attribute.setComponent(vertex + 2, component, second);
      }
    }
    attribute.needsUpdate = true;
  }
}

/** Deterministic artist-authored skinning for this pinned reference atlas, not measured anatomy. */
export function skinInfluence(x: number, y: number): [number, number, number] {
  const side = x >= 0 ? "L" : "R";
  const blend = (a: JointId, b: JointId, weight: number): [number, number, number] => [jointIndex.get(a)!, jointIndex.get(b)!, smooth(weight)];
  const arm = Math.abs(x) > .185 && y > .63 && y < 1.45;
  if (arm) {
    if (y > 1.30) return blend("chest", `upperArm${side}`, (Math.abs(x) - .155) / .09);
    if (y > 1.065) return blend(`forearm${side}`, `upperArm${side}`, (y - 1.065) / .09);
    return blend(`hand${side}`, `forearm${side}`, (y - .81) / .075);
  }
  if (y < .975) {
    if (y > .83) return blend(`thigh${side}`, "pelvis", (y - .83) / .145);
    if (y > .38) return blend(`calf${side}`, `thigh${side}`, (y - .38) / .12);
    return blend(`foot${side}`, `calf${side}`, (y - .055) / .08);
  }
  if (y > 1.495) return blend("neck", "head", (y - 1.495) / .05);
  if (y > 1.415) return blend("chest", "neck", (y - 1.415) / .08);
  if (y > 1.18) return blend("spine", "chest", (y - 1.18) / .10);
  return blend("pelvis", "spine", (y - .99) / .14);
}

export const SKIN_COLOR = "#f4f1e8";
export const SKIN_OPACITY = .22;
export const SKIN_SELECTED_OPACITY = .58;
/** Classic anatomical-chart red; tendons and aponeuroses a muted rose so hands/feet do not read white. */
const MUSCLE_COLOR = "#a3271f";
const TENDON_COLOR = "#b86259";
const BONE_COLOR = "#e8dfc8";

/** Bakes an atlas mesh into rig space with the deterministic reference skinning. */
function bakeSkinnedGeometry(object: Mesh): BufferGeometry {
  const mirrored = object.matrixWorld.determinant() < 0;
  const geometry = object.geometry.clone().applyMatrix4(object.matrixWorld);
  if (mirrored) {
    // The left-side Vanatome meshes reuse the right-side meshes under a
    // negative X scale. applyMatrix4 already reflects their normals through
    // the normal matrix, so they stay outward; only the triangle winding
    // flips. Restore the winding and leave the normals alone: inverting them
    // as well lit the whole left half from inside (dark, split down the
    // midline).
    restoreMirroredTriangleWinding(geometry);
  }
  for (const key of Object.keys(geometry.attributes)) if (key !== "position" && key !== "normal") geometry.deleteAttribute(key);
  if (!geometry.getAttribute("normal")) geometry.computeVertexNormals();
  const position = geometry.getAttribute("position");
  const indices = new Uint16Array(position.count * 4);
  const weights = new Float32Array(position.count * 4);
  for (let i = 0; i < position.count; i++) {
    const [a, b, weight] = skinInfluence(position.getX(i), position.getY(i));
    indices[i * 4] = a; indices[i * 4 + 1] = b;
    weights[i * 4] = 1 - weight; weights[i * 4 + 1] = weight;
  }
  geometry.setAttribute("skinIndex", new Uint16BufferAttribute(indices, 4));
  geometry.setAttribute("skinWeight", new Float32BufferAttribute(weights, 4));
  return geometry;
}

export interface ReferenceLayer {
  mesh: SkinnedMesh<BufferGeometry, MeshStandardMaterial>;
  dispose: () => void;
}
export type ReferenceMuscles = ReferenceLayer;

/** Principal axis of a muscle part (its fibre direction, roughly), by power iteration on the covariance. */
function principalAxis(position: BufferAttribute | InterleavedBufferAttribute): Vector3 {
  const n = position.count;
  let mx = 0, my = 0, mz = 0;
  for (let i = 0; i < n; i++) { mx += position.getX(i); my += position.getY(i); mz += position.getZ(i); }
  mx /= n; my /= n; mz /= n;
  let xx = 0, xy = 0, xz = 0, yy = 0, yz = 0, zz = 0;
  for (let i = 0; i < n; i++) {
    const x = position.getX(i) - mx, y = position.getY(i) - my, z = position.getZ(i) - mz;
    xx += x * x; xy += x * y; xz += x * z; yy += y * y; yz += y * z; zz += z * z;
  }
  const axis = new Vector3(.3, 1, .2).normalize();
  for (let k = 0; k < 24; k++) {
    axis.set(xx * axis.x + xy * axis.y + xz * axis.z, xy * axis.x + yy * axis.y + yz * axis.z, xz * axis.x + yz * axis.y + zz * axis.z);
    if (axis.lengthSq() < 1e-20) return new Vector3(0, 1, 0);
    axis.normalize();
  }
  return axis;
}

/** Per-vertex fibre direction and tissue (0 muscle, 1 tendon) for the chart shading. */
function addMuscleAttributes(geometry: BufferGeometry, tendon: boolean): void {
  const position = geometry.getAttribute("position");
  const axis = tendon ? new Vector3(0, 1, 0) : principalAxis(position);
  const fiber = new Float32Array(position.count * 3);
  const tissue = new Float32Array(position.count).fill(tendon ? 1 : 0);
  for (let i = 0; i < position.count; i++) { fiber[i * 3] = axis.x; fiber[i * 3 + 1] = axis.y; fiber[i * 3 + 2] = axis.z; }
  geometry.setAttribute("fiber", new Float32BufferAttribute(fiber, 3));
  geometry.setAttribute("tissue", new Float32BufferAttribute(tissue, 1));
}

/**
 * Red muscle with fine fibre striations running along each muscle's main axis,
 * pale tendons. Striations fade out with distance (fwidth) so they never alias.
 */
function createMuscleMaterial(): MeshStandardMaterial {
  const material = new MeshStandardMaterial({ color: MUSCLE_COLOR, roughness: .52, metalness: 0, side: FrontSide });
  material.onBeforeCompile = (shader) => {
    shader.uniforms.tendonColor = { value: new Color(TENDON_COLOR) };
    shader.vertexShader = shader.vertexShader
      .replace("#include <common>", `#include <common>
attribute vec3 fiber;
attribute float tissue;
varying float vStripe;
varying float vBundle;
varying float vTissue;`)
      .replace("#include <begin_vertex>", `#include <begin_vertex>
vec3 bsAcross = cross(normalize(fiber), normalize(objectNormal));
bsAcross = dot(bsAcross, bsAcross) > 1e-6 ? normalize(bsAcross) : vec3(1.0, 0.0, 0.0);
vStripe = dot(position, bsAcross);
vBundle = dot(position, normalize(fiber));
vTissue = tissue;`);
    shader.fragmentShader = shader.fragmentShader
      .replace("#include <common>", `#include <common>
uniform vec3 tendonColor;
varying float vStripe;
varying float vBundle;
varying float vTissue;`)
      .replace("#include <color_fragment>", `#include <color_fragment>
{
  float fine = vStripe * 1400.0;
  float coarse = vStripe * 260.0 + sin(vBundle * 40.0) * 1.4;
  float fadeFine = clamp(1.0 - fwidth(fine) / 2.2, 0.0, 1.0);
  float fadeCoarse = clamp(1.0 - fwidth(coarse) / 2.2, 0.0, 1.0);
  float striation = 1.0 - (0.13 * fadeFine * (0.5 + 0.5 * sin(fine)) + 0.16 * fadeCoarse * smoothstep(0.55, 1.0, 0.5 + 0.5 * sin(coarse)));
  float muscle = 1.0 - smoothstep(0.25, 0.75, vTissue);
  diffuseColor.rgb = mix(tendonColor, diffuseColor.rgb * striation, muscle);
}`);
  };
  material.customProgramCacheKey = () => "bodysense-muscle-chart-v1";
  return material;
}

/**
 * Adds an atlas layer (muscles, bones) to the rig, skinned to the same skeleton so
 * it follows every reference pose. Display only: picking still reads the skin regions.
 */
export function attachReferenceLayer(rig: ReferenceRig, source: Object3D, options: { name: string; color: string; muscleChart?: boolean }): ReferenceLayer {
  const geometries: BufferGeometry[] = [];
  source.updateMatrixWorld(true);
  source.traverse((object) => {
    if (!(object instanceof Mesh)) return;
    const geometry = bakeSkinnedGeometry(object);
    if (options.muscleChart) addMuscleAttributes(geometry, /__tendon$/.test(object.name));
    geometries.push(geometry);
  });
  const geometry = geometries.length ? mergeGeometries(geometries, false) : null;
  for (const item of geometries) item.dispose();
  if (!geometry) throw new Error(`参考图谱 ${options.name} 无法合并`);
  const material = options.muscleChart
    ? createMuscleMaterial()
    : new MeshStandardMaterial({ color: options.color, roughness: .7, metalness: 0, side: FrontSide });
  const mesh = new SkinnedMesh(geometry, material);
  mesh.name = options.name;
  mesh.renderOrder = 1;
  mesh.frustumCulled = false;
  mesh.boundingSphere = new Sphere(new Vector3(0, .7, 0), 3);
  mesh.boundingBox = new Box3(new Vector3(-3, -3, -3), new Vector3(3, 4, 3));
  mesh.raycast = () => {};
  mesh.bind(rig.skeleton);
  rig.group.add(mesh);
  rig.group.updateMatrixWorld(true);
  return {
    mesh,
    dispose() { rig.group.remove(mesh); geometry.dispose(); material.dispose(); },
  };
}

/** Full-body Z-Anatomy muscles (parts named `…__muscle` / `…__tendon`), shaded like an anatomical chart. */
export function attachReferenceMuscles(rig: ReferenceRig, source: Object3D): ReferenceMuscles {
  return attachReferenceLayer(rig, source, { name: "reference-muscles", color: MUSCLE_COLOR, muscleChart: true });
}

export function attachReferenceSkeleton(rig: ReferenceRig, source: Object3D): ReferenceLayer {
  return attachReferenceLayer(rig, source, { name: "reference-skeleton", color: BONE_COLOR });
}

export function buildReferenceRig(source: Object3D): ReferenceRig {
  const group = new Group();
  group.name = "BodySense / Reference animation / NOT a patient measurement";
  const entries = JOINTS.map(([id]) => [id, new Bone()] as const);
  const bones = Object.fromEntries(entries) as ReferenceBones;
  JOINTS.forEach(([id, parent, x, y, z]) => {
    const bone = bones[id];
    bone.name = id;
    const origin = JOINTS.find(([key]) => key === parent);
    bone.position.set(x - (origin?.[2] ?? 0), y - (origin?.[3] ?? 0), z - (origin?.[4] ?? 0));
    if (parent) bones[parent].add(bone); else group.add(bone);
  });
  group.updateMatrixWorld(true);
  const skeleton = new Skeleton(JOINTS.map(([id]) => bones[id]));
  skeleton.calculateInverses();
  const regions = new Map<BodyRegionId, BufferGeometry[]>();
  source.updateMatrixWorld(true);
  source.traverse((object) => {
    if (!(object instanceof Mesh) || !object.name.startsWith("body-shell")) return;
    const geometry = bakeSkinnedGeometry(object);
    geometry.computeBoundingBox();
    const center = geometry.boundingBox!.getCenter(new Vector3());
    const region = atlasSurfaceRegion(object.name, [center.x, center.y, center.z]);
    const list = regions.get(region) ?? [];
    list.push(geometry); regions.set(region, list);
  });
  if (!regions.size) { skeleton.dispose(); throw new Error("参考图谱缺少可识别的身体表面"); }
  const meshes: ReferenceRig["meshes"] = [];
  for (const [region, geometries] of regions) {
    const geometry = mergeGeometries(geometries, false);
    for (const item of geometries) item.dispose();
    if (!geometry) throw new Error("参考图谱表面无法合并");
    // Translucent paper skin over the muscular layer: the figure stays whole,
    // surface detail is not shown. depthWrite off so the muscles read through.
    const material = new MeshStandardMaterial({
      color: SKIN_COLOR, roughness: .85, metalness: 0, side: FrontSide,
      transparent: true, opacity: SKIN_OPACITY, depthWrite: false,
    });
    const mesh = new SkinnedMesh(geometry, material);
    mesh.renderOrder = 2;
    mesh.name = region;
    mesh.userData.bodyRegionId = region;
    mesh.frustumCulled = false;
    // Conservative bounds cover every permitted reference pose; exact hits still use skinned triangles.
    mesh.boundingSphere = new Sphere(new Vector3(0, .7, 0), 3);
    mesh.boundingBox = new Box3(new Vector3(-3, -3, -3), new Vector3(3, 4, 3));
    mesh.bind(skeleton);
    group.add(mesh); meshes.push(mesh);
  }
  group.updateMatrixWorld(true);
  return {
    group, bones, skeleton, meshes,
    dispose() {
      meshes.forEach((mesh) => { mesh.geometry.dispose(); mesh.material.dispose(); });
      skeleton.dispose(); group.clear();
    },
  };
}

export interface ReferencePose {
  offset: [number, number, number];
  rotations: Partial<Record<JointId, readonly [number, number, number]>>;
}

export function sampleReferencePose(motion: MotionId, inputPhase: number): ReferencePose {
  const phase = Number.isFinite(inputPhase) ? clamp(inputPhase) : 0;
  const a = phase * Math.PI * 2;
  const wave = Math.sin(a);
  const u = (1 - Math.cos(a)) / 2;
  const pose: ReferencePose = { offset: [0, 0, 0], rotations: {} };
  const setX = (id: JointId, x: number) => { pose.rotations[id] = [x, 0, 0]; };
  if (motion === "stand") { setX("chest", wave * .008); return pose; }
  if (motion === "run") {
    pose.offset[1] = Math.abs(wave) * .035;
    setX("spine", .10);
    setX("thighL", -.78 * wave); setX("thighR", .78 * wave);
    setX("calfL", .54 + .54 * Math.sin(a - 1.1)); setX("calfR", .54 - .54 * Math.sin(a - 1.1));
    setX("upperArmL", .60 * wave); setX("upperArmR", -.60 * wave);
    setX("forearmL", -1.18); setX("forearmR", -1.18);
  } else if (motion === "sit") {
    pose.offset[1] = -.43;
    setX("spine", .12); setX("neck", .08);
    for (const side of ["L", "R"] as const) {
      setX(`thigh${side}`, -1.46); setX(`calf${side}`, 1.46);
      setX(`upperArm${side}`, -.58); setX(`forearm${side}`, -.92);
    }
  } else if (motion === "jump") {
    const crouch = phase < .25 ? Math.sin(phase / .25 * Math.PI) : phase > .75 ? Math.sin((phase - .75) / .25 * Math.PI) * .65 : 0;
    const flight = phase >= .25 && phase <= .75 ? Math.sin((phase - .25) / .5 * Math.PI) : 0;
    pose.offset[1] = -.15 * crouch + .26 * flight;
    setX("spine", .15 * crouch);
    for (const side of ["L", "R"] as const) {
      setX(`thigh${side}`, -.62 * crouch); setX(`calf${side}`, 1.15 * crouch);
      setX(`upperArm${side}`, -.9 * flight + .18 * crouch);
    }
  } else if (motion === "arm_raise") {
    pose.rotations.upperArmL = [-.15 * u, 0, 2.18 * u];
    pose.rotations.upperArmR = [-.15 * u, 0, -2.18 * u];
    setX("forearmL", -.18 * u); setX("forearmR", -.18 * u);
  } else if (motion === "calf_raise") {
    pose.offset[1] = .045 * u;
    setX("footL", -.18 * u); setX("footR", -.18 * u);
  } else if (motion === "hip_hinge") {
    pose.offset[1] = -.12 * u; pose.offset[2] = -.12 * u;
    setX("pelvis", .45 * u); setX("spine", .30 * u);
    setX("thighL", -.62 * u); setX("thighR", -.62 * u);
    setX("calfL", .30 * u); setX("calfR", .30 * u);
    setX("upperArmL", -.55 * u); setX("upperArmR", -.55 * u);
  } else if (motion === "bridge") {
    pose.offset = [0, -.72 + .18 * u, .08];
    setX("pelvis", -Math.PI / 2 - .12 * u);
    for (const side of ["L", "R"] as const) {
      setX(`thigh${side}`, -1.08 + .30 * u); setX(`calf${side}`, 1.62);
    }
  }
  return pose;
}

export function applyReferencePose(rig: ReferenceRig, motion: MotionId, phase: number): void {
  const pose = sampleReferencePose(motion, phase);
  for (const [id] of JOINTS) {
    const value = pose.rotations[id] ?? [0, 0, 0];
    rig.bones[id].rotation.set(...value);
  }
  rig.bones.pelvis.position.set(pose.offset[0], .94 + pose.offset[1], pose.offset[2]);
  rig.group.updateMatrixWorld(true);
  rig.skeleton.update();
}

/** Exposed for asset qualification tests without WebGL. */
export function referenceBounds(rig: ReferenceRig): Box3 {
  const bounds = new Box3();
  const point = new Vector3();
  for (const mesh of rig.meshes) {
    const attribute: BufferAttribute = mesh.geometry.getAttribute("position") as BufferAttribute;
    for (let i = 0; i < attribute.count; i++) bounds.expandByPoint(mesh.getVertexPosition(i, point));
  }
  return bounds;
}
