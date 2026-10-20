import {
  BufferAttribute,
  BufferGeometry,
  Float32BufferAttribute,
  Group,
  Mesh,
  MeshBasicMaterial,
  Quaternion,
  Uint16BufferAttribute,
  Uint32BufferAttribute,
  Uint8BufferAttribute,
} from "three";
import {
  VANATOME_INITIAL_SYSTEM_ID,
  resolvePinnedVanatomeSystemSource,
} from "@/features/body-explorer/model/vanatomePin";

const GLB_MAGIC = 0x46546c67;
const GLB_VERSION = 2;
const GLB_JSON_CHUNK = 0x4e4f534a;
const GLB_BIN_CHUNK = 0x004e4942;
const COMPONENT_FLOAT = 5126;
const COMPONENT_UNSIGNED_BYTE = 5121;
const COMPONENT_UNSIGNED_SHORT = 5123;
const COMPONENT_UNSIGNED_INT = 5125;
const MODE_TRIANGLES = 4;

interface GltfAccessor {
  bufferView?: number;
  byteOffset?: number;
  componentType: number;
  count: number;
  type: string;
  sparse?: unknown;
}

interface GltfBufferView {
  buffer: number;
  byteOffset?: number;
  byteLength: number;
  byteStride?: number;
}

interface GltfPrimitive {
  attributes: Record<string, number>;
  indices?: number;
  mode?: number;
}

interface GltfMesh {
  primitives: GltfPrimitive[];
}

interface GltfNode {
  name?: string;
  mesh?: number;
  matrix?: number[];
  translation?: number[];
  rotation?: number[];
  scale?: number[];
}

interface MinimalGltf {
  accessors: GltfAccessor[];
  bufferViews: GltfBufferView[];
  meshes: GltfMesh[];
  nodes: GltfNode[];
}

interface ParsedGlb {
  json: MinimalGltf;
  binaryOffset: number;
  binaryLength: number;
  bytes: ArrayBuffer;
}

export interface ReferenceAtlasSurface {
  group: Group;
  modelUrl: string;
  dispose: () => void;
}

function parseGlb(bytes: ArrayBuffer): ParsedGlb {
  const view = new DataView(bytes);
  if (view.byteLength < 20) throw new Error("Vanatome regional GLB is truncated");
  if (view.getUint32(0, true) !== GLB_MAGIC) {
    throw new Error("Vanatome regional model is not a GLB file");
  }
  if (view.getUint32(4, true) !== GLB_VERSION) {
    throw new Error("Vanatome regional GLB version is not supported");
  }
  if (view.getUint32(8, true) > view.byteLength) {
    throw new Error("Vanatome regional GLB declares an invalid length");
  }

  let offset = 12;
  let json: MinimalGltf | null = null;
  let binaryOffset = -1;
  let binaryLength = 0;
  const decoder = new TextDecoder();
  while (offset + 8 <= view.byteLength) {
    const chunkLength = view.getUint32(offset, true);
    const chunkType = view.getUint32(offset + 4, true);
    const contentOffset = offset + 8;
    const contentEnd = contentOffset + chunkLength;
    if (contentEnd > view.byteLength) {
      throw new Error("Vanatome regional GLB has an invalid chunk");
    }
    if (chunkType === GLB_JSON_CHUNK) {
      const raw = decoder
        .decode(new Uint8Array(bytes, contentOffset, chunkLength))
        .trim();
      json = JSON.parse(raw) as MinimalGltf;
    } else if (chunkType === GLB_BIN_CHUNK) {
      binaryOffset = contentOffset;
      binaryLength = chunkLength;
    }
    offset = contentEnd;
  }
  if (!json || binaryOffset < 0) {
    throw new Error("Vanatome regional GLB is missing JSON or binary data");
  }
  if (
    !Array.isArray(json.accessors) ||
    !Array.isArray(json.bufferViews) ||
    !Array.isArray(json.meshes) ||
    !Array.isArray(json.nodes)
  ) {
    throw new Error("Vanatome regional GLB schema is incomplete");
  }
  return { json, binaryOffset, binaryLength, bytes };
}

function accessorStart(
  parsed: ParsedGlb,
  accessor: GltfAccessor,
): { start: number; stride: number } {
  if (accessor.bufferView == null || accessor.sparse) {
    throw new Error("Sparse or bufferless Vanatome accessors are not supported");
  }
  const bufferView = parsed.json.bufferViews[accessor.bufferView];
  if (!bufferView || bufferView.buffer !== 0) {
    throw new Error("Vanatome accessor references an unsupported buffer");
  }
  const start =
    parsed.binaryOffset +
    (bufferView.byteOffset ?? 0) +
    (accessor.byteOffset ?? 0);
  const stride = bufferView.byteStride ?? 0;
  if (
    start < parsed.binaryOffset ||
    start >= parsed.binaryOffset + parsed.binaryLength
  ) {
    throw new Error("Vanatome accessor points outside the GLB binary chunk");
  }
  return { start, stride };
}

function readFloatVec3(
  parsed: ParsedGlb,
  accessorIndex: number,
): Float32Array {
  const accessor = parsed.json.accessors[accessorIndex];
  if (
    !accessor ||
    accessor.componentType !== COMPONENT_FLOAT ||
    accessor.type !== "VEC3" ||
    !Number.isInteger(accessor.count) ||
    accessor.count < 1
  ) {
    throw new Error("Vanatome shell position/normal accessor is unsupported");
  }
  const { start, stride } = accessorStart(parsed, accessor);
  const byteStride = stride || 12;
  if (byteStride < 12) {
    throw new Error("Vanatome VEC3 accessor has an invalid byte stride");
  }
  const requiredEnd = start + (accessor.count - 1) * byteStride + 12;
  if (requiredEnd > parsed.binaryOffset + parsed.binaryLength) {
    throw new Error("Vanatome VEC3 accessor exceeds the GLB binary chunk");
  }

  const source = new DataView(parsed.bytes);
  const output = new Float32Array(accessor.count * 3);
  for (let index = 0; index < accessor.count; index += 1) {
    const inputOffset = start + index * byteStride;
    const outputOffset = index * 3;
    output[outputOffset] = source.getFloat32(inputOffset, true);
    output[outputOffset + 1] = source.getFloat32(inputOffset + 4, true);
    output[outputOffset + 2] = source.getFloat32(inputOffset + 8, true);
  }
  return output;
}

function readIndices(
  parsed: ParsedGlb,
  accessorIndex: number,
): BufferAttribute {
  const accessor = parsed.json.accessors[accessorIndex];
  if (
    !accessor ||
    accessor.type !== "SCALAR" ||
    !Number.isInteger(accessor.count) ||
    accessor.count < 3
  ) {
    throw new Error("Vanatome shell index accessor is unsupported");
  }
  const { start, stride } = accessorStart(parsed, accessor);
  const bytesPerComponent =
    accessor.componentType === COMPONENT_UNSIGNED_BYTE
      ? 1
      : accessor.componentType === COMPONENT_UNSIGNED_SHORT
        ? 2
        : accessor.componentType === COMPONENT_UNSIGNED_INT
          ? 4
          : 0;
  if (!bytesPerComponent) {
    throw new Error("Vanatome shell index component type is unsupported");
  }
  const byteStride = stride || bytesPerComponent;
  if (byteStride < bytesPerComponent) {
    throw new Error("Vanatome index accessor has an invalid byte stride");
  }
  const requiredEnd =
    start + (accessor.count - 1) * byteStride + bytesPerComponent;
  if (requiredEnd > parsed.binaryOffset + parsed.binaryLength) {
    throw new Error("Vanatome index accessor exceeds the GLB binary chunk");
  }

  const source = new DataView(parsed.bytes);
  if (accessor.componentType === COMPONENT_UNSIGNED_BYTE) {
    const values = new Uint8Array(accessor.count);
    for (let index = 0; index < accessor.count; index += 1) {
      values[index] = source.getUint8(start + index * byteStride);
    }
    return new Uint8BufferAttribute(values, 1);
  }
  if (accessor.componentType === COMPONENT_UNSIGNED_SHORT) {
    const values = new Uint16Array(accessor.count);
    for (let index = 0; index < accessor.count; index += 1) {
      values[index] = source.getUint16(start + index * byteStride, true);
    }
    return new Uint16BufferAttribute(values, 1);
  }
  const values = new Uint32Array(accessor.count);
  for (let index = 0; index < accessor.count; index += 1) {
    values[index] = source.getUint32(start + index * byteStride, true);
  }
  return new Uint32BufferAttribute(values, 1);
}

function applyNodeTransform(mesh: Mesh, node: GltfNode): void {
  if (Array.isArray(node.matrix) && node.matrix.length === 16) {
    mesh.matrix.fromArray(node.matrix);
    mesh.matrix.decompose(mesh.position, mesh.quaternion, mesh.scale);
    return;
  }
  if (Array.isArray(node.translation) && node.translation.length === 3) {
    mesh.position.fromArray(node.translation);
  }
  if (Array.isArray(node.rotation) && node.rotation.length === 4) {
    mesh.quaternion.copy(
      new Quaternion(
        node.rotation[0],
        node.rotation[1],
        node.rotation[2],
        node.rotation[3],
      ),
    );
  }
  if (Array.isArray(node.scale) && node.scale.length === 3) {
    mesh.scale.fromArray(node.scale);
  }
}

export function parseReferenceAtlasSurface(bytes: ArrayBuffer): Group {
  return parseReferenceAtlasMeshes(bytes, {
    accept: (name) => name.startsWith("body-shell"),
    label: "Vanatome regional body shell / lightweight Body Canvas source",
    emptyMessage: "Vanatome regional GLB contains no body-shell surface",
  });
}

/** Reads every triangle mesh whose node name passes `accept` (flat GLB, no hierarchy). */
export function parseReferenceAtlasMeshes(
  bytes: ArrayBuffer,
  options: {
    accept: (nodeName: string) => boolean;
    label: string;
    emptyMessage: string;
  },
): Group {
  const parsed = parseGlb(bytes);
  const group = new Group();
  group.name = options.label;
  const sourceMaterial = new MeshBasicMaterial();
  let meshCount = 0;

  for (const node of parsed.json.nodes) {
    if (
      !node.name ||
      !options.accept(node.name) ||
      node.mesh == null ||
      !parsed.json.meshes[node.mesh]
    ) {
      continue;
    }
    const meshDefinition = parsed.json.meshes[node.mesh];
    for (const primitive of meshDefinition.primitives) {
      if ((primitive.mode ?? MODE_TRIANGLES) !== MODE_TRIANGLES) {
        continue;
      }
      const positionAccessor = primitive.attributes.POSITION;
      if (positionAccessor == null || primitive.indices == null) {
        continue;
      }
      const geometry = new BufferGeometry();
      geometry.setAttribute(
        "position",
        new Float32BufferAttribute(
          readFloatVec3(parsed, positionAccessor),
          3,
        ),
      );
      const normalAccessor = primitive.attributes.NORMAL;
      if (normalAccessor != null) {
        geometry.setAttribute(
          "normal",
          new Float32BufferAttribute(readFloatVec3(parsed, normalAccessor), 3),
        );
      } else {
        geometry.computeVertexNormals();
      }
      geometry.setIndex(readIndices(parsed, primitive.indices));

      const mesh = new Mesh(geometry, sourceMaterial);
      mesh.name = node.name;
      applyNodeTransform(mesh, node);
      group.add(mesh);
      meshCount += 1;
    }
  }

  if (!meshCount) {
    sourceMaterial.dispose();
    group.clear();
    throw new Error(options.emptyMessage);
  }
  return group;
}

export function disposeReferenceAtlasSurface(group: Group): void {
  const materials = new Set<MeshBasicMaterial>();
  group.traverse((object) => {
    if (!(object instanceof Mesh)) return;
    object.geometry.dispose();
    const list = Array.isArray(object.material) ? object.material : [object.material];
    for (const material of list) {
      if (material instanceof MeshBasicMaterial) materials.add(material);
    }
  });
  for (const material of materials) material.dispose();
  group.clear();
}

async function fetchPinnedSystemBytes(
  systemId: string,
  options: {
    signal?: AbortSignal;
    catalogUrl?: string;
    fetchImpl?: typeof fetch;
  },
): Promise<{ bytes: ArrayBuffer; modelUrl: string }> {
  const fetchImpl = options.fetchImpl ?? fetch;
  const source = await resolvePinnedVanatomeSystemSource(systemId, {
    signal: options.signal,
    catalogUrl: options.catalogUrl,
    fetchImpl,
  });
  const response = await fetchImpl(source.modelUrl, {
    signal: options.signal,
    cache: "force-cache",
  });
  if (!response.ok) {
    throw new Error(
      `Vanatome ${systemId} model failed with HTTP ${response.status}: ${source.modelUrl}`,
    );
  }
  const bytes = await response.arrayBuffer();
  if (source.bytes != null && bytes.byteLength !== source.bytes) {
    throw new Error(
      `Vanatome ${systemId} model length mismatch: expected ${source.bytes}, got ${bytes.byteLength}`,
    );
  }
  // Let the shell/chrome paint before the bounded geometry extraction begins.
  await new Promise<void>((resolve) => {
    if (typeof requestAnimationFrame === "function") {
      requestAnimationFrame(() => resolve());
    } else {
      setTimeout(resolve, 0);
    }
  });
  return { bytes, modelUrl: source.modelUrl };
}

export async function loadPinnedReferenceAtlasSurface(options: {
  signal?: AbortSignal;
  catalogUrl?: string;
  fetchImpl?: typeof fetch;
} = {}): Promise<ReferenceAtlasSurface> {
  const { bytes, modelUrl } = await fetchPinnedSystemBytes(
    VANATOME_INITIAL_SYSTEM_ID,
    options,
  );
  const group = parseReferenceAtlasSurface(bytes);
  return {
    group,
    modelUrl,
    dispose: () => disposeReferenceAtlasSurface(group),
  };
}

/**
 * Full-body muscular layer (≈6.6 MB): superficial and intermediate muscles plus
 * tendons from Z-Anatomy (CC BY-SA 4.0), exported for BodySense in the same
 * coordinate space as Atlas 1.4.0 (whose own `muscular` bundle has no limb
 * muscles). Nodes are named `<structure>__muscle` / `<structure>__tendon`.
 * Loaded after the skin so the body paints first.
 *
 * Served same-origin from the Web image (`apps/web/public/anatomy/`). Only the
 * hashed `dist/assets/` files are published to the static CDN under BASE_URL,
 * so a BASE_URL-relative path 404s when the CDN base is configured.
 */
export const REFERENCE_MUSCLES_URL = "/anatomy/z-anatomy-muscles-v1.glb";

export async function loadPinnedReferenceMuscles(options: {
  signal?: AbortSignal;
  fetchImpl?: typeof fetch;
  /** Download now, but parse only after this settles (e.g. the skin is built). */
  parseAfter?: Promise<unknown>;
} = {}): Promise<ReferenceAtlasSurface> {
  const fetchImpl = options.fetchImpl ?? fetch;
  const response = await fetchImpl(REFERENCE_MUSCLES_URL, { signal: options.signal, cache: "force-cache" });
  if (!response.ok) {
    throw new Error(`Reference muscles failed with HTTP ${response.status}: ${REFERENCE_MUSCLES_URL}`);
  }
  const bytes = await response.arrayBuffer();
  await options.parseAfter?.catch(() => undefined);
  const group = parseReferenceAtlasMeshes(bytes, {
    accept: (name) => /__(muscle|tendon)$/.test(name),
    label: "Z-Anatomy muscular layer / Body Canvas",
    emptyMessage: "Reference muscles GLB contains no meshes",
  });
  return {
    group,
    modelUrl: REFERENCE_MUSCLES_URL,
    dispose: () => disposeReferenceAtlasSurface(group),
  };
}

/** The pinned skeletal layer (3.9 MB), fetched only when the 骨骼 layer is turned on. */
export function loadPinnedReferenceSkeleton(options: {
  signal?: AbortSignal;
  catalogUrl?: string;
  fetchImpl?: typeof fetch;
} = {}): Promise<ReferenceAtlasSurface> {
  return loadPinnedReferenceSystem("skeletal", options);
}

async function loadPinnedReferenceSystem(
  systemId: "skeletal",
  options: {
    signal?: AbortSignal;
    catalogUrl?: string;
    fetchImpl?: typeof fetch;
    parseAfter?: Promise<unknown>;
  },
): Promise<ReferenceAtlasSurface> {
  const { bytes, modelUrl } = await fetchPinnedSystemBytes(systemId, options);
  await options.parseAfter?.catch(() => undefined);
  const group = parseReferenceAtlasMeshes(bytes, {
    accept: () => true,
    label: `Vanatome ${systemId} layer / Body Canvas`,
    emptyMessage: `Vanatome ${systemId} GLB contains no meshes`,
  });
  return {
    group,
    modelUrl,
    dispose: () => disposeReferenceAtlasSurface(group),
  };
}
