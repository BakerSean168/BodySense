import {
  BufferAttribute,
  BufferGeometry,
  Float32BufferAttribute,
  Group,
  Mesh,
  MeshBasicMaterial,
  Texture,
  Uint8BufferAttribute,
  Uint16BufferAttribute,
  Uint32BufferAttribute,
  type InterleavedBufferAttribute,
  type Material,
} from "three";
import { GLTFLoader, type GLTF } from "three/addons/loaders/GLTFLoader.js";
import { MeshoptDecoder } from "three/addons/libs/meshopt_decoder.module.js";

/**
 * Atlas extraction on top of three's GLTFLoader.
 *
 * GLTFLoader owns the actual parsing (including Meshopt decoding); this
 * module keeps the contract the rest of Body Canvas relies on: a flat group
 * with one mesh per glTF primitive, meshes carrying the *original* node
 * names from the file, plain Float32 position/normal attributes, and one
 * shared placeholder material.
 *
 * The original names matter: GLTFLoader sanitizes node names (spaces become
 * underscores, dots are dropped), which would break the shell layer's
 * `.l`/`.r` side detection and the muscle/tendon suffix classification.
 * The names used here always come from the file's own node table.
 */

function toFloat32Attribute(
  attribute: BufferAttribute | InterleavedBufferAttribute,
): Float32BufferAttribute {
  if (
    attribute instanceof BufferAttribute &&
    attribute.array instanceof Float32Array
  ) {
    return new Float32BufferAttribute(attribute.array, attribute.itemSize);
  }
  // Quantized or interleaved attributes (Meshopt-compressed assets) are
  // materialized through the semantic accessors, which dequantize and
  // deinterleave. Downstream baking applies matrices to the raw attribute
  // arrays in place, which cannot dequantize and would corrupt such data;
  // after this step every attribute is a plain Float32 array in node-local
  // space, exactly like an uncompressed source file.
  const { count, itemSize } = attribute;
  const values = new Float32Array(count * itemSize);
  for (let index = 0; index < count; index += 1) {
    const offset = index * itemSize;
    values[offset] = attribute.getX(index);
    if (itemSize > 1) values[offset + 1] = attribute.getY(index);
    if (itemSize > 2) values[offset + 2] = attribute.getZ(index);
    if (itemSize > 3) values[offset + 3] = attribute.getW(index);
  }
  return new Float32BufferAttribute(values, itemSize);
}

function cloneIndexAttribute(index: BufferAttribute): BufferAttribute {
  const array = index.array;
  if (array instanceof Uint8Array) {
    return new Uint8BufferAttribute(array.slice(), 1);
  }
  if (array instanceof Uint16Array) {
    return new Uint16BufferAttribute(array.slice(), 1);
  }
  if (array instanceof Uint32Array) {
    return new Uint32BufferAttribute(array.slice(), 1);
  }
  throw new Error("Vanatome index attribute has an unsupported array type");
}

/**
 * Copies one parsed primitive into the atlas geometry shape: position,
 * normal (computed when the file omits it) and index, nothing else.
 * Returns null for primitives the previous hand-written parser skipped
 * (no position attribute or no index).
 */
function extractPrimitiveGeometry(
  source: BufferGeometry,
): BufferGeometry | null {
  const position = source.getAttribute("position");
  const index = source.getIndex();
  if (!position || !index) return null;
  const geometry = new BufferGeometry();
  geometry.setAttribute("position", toFloat32Attribute(position));
  geometry.setIndex(cloneIndexAttribute(index));
  const normal = source.getAttribute("normal");
  if (normal) {
    geometry.setAttribute("normal", toFloat32Attribute(normal));
  } else {
    geometry.computeVertexNormals();
  }
  return geometry;
}

/**
 * Releases everything GLTFLoader allocated for its own scene graph. The
 * parsed scene is scaffolding: the atlas group keeps its own geometries,
 * and downstream replaces all materials, so nothing from the parsed scene
 * may outlive the extraction.
 */
function disposeParsedGlb(gltf: GLTF): void {
  const geometries = new Set<BufferGeometry>();
  const materials = new Set<Material>();
  const textures = new Set<Texture>();
  gltf.scene.traverse((object) => {
    if (!(object instanceof Mesh)) return;
    geometries.add(object.geometry);
    const list = Array.isArray(object.material)
      ? object.material
      : [object.material];
    for (const material of list) {
      materials.add(material);
      for (const value of Object.values(material)) {
        if (value instanceof Texture) textures.add(value);
      }
    }
  });
  for (const geometry of geometries) geometry.dispose();
  for (const texture of textures) texture.dispose();
  for (const material of materials) material.dispose();
}

export async function parseReferenceAtlasSurface(
  bytes: ArrayBuffer,
): Promise<Group> {
  return parseReferenceAtlasMeshes(bytes, {
    accept: (name) => name.startsWith("body-shell"),
    label: "Vanatome regional body shell / lightweight Body Canvas source",
    emptyMessage: "Vanatome regional GLB contains no body-shell surface",
  });
}

/** Reads every triangle mesh whose node name passes `accept` (flat GLB, no hierarchy). */
export async function parseReferenceAtlasMeshes(
  bytes: ArrayBuffer,
  options: {
    accept: (nodeName: string) => boolean;
    label: string;
    emptyMessage: string;
  },
): Promise<Group> {
  const group = new Group();
  group.name = options.label;
  const sourceMaterial = new MeshBasicMaterial();
  const geometries = new Set<BufferGeometry>();
  let gltf: GLTF | null = null;
  try {
    const loader = new GLTFLoader();
    loader.setMeshoptDecoder(MeshoptDecoder);
    gltf = await loader.parseAsync(bytes, "");
    const parser = gltf.parser;
    gltf.scene.updateMatrixWorld(true);

    // Map each glTF node to the primitive meshes GLTFLoader built for it.
    // The node object (a Mesh, or a Group for multi-primitive meshes)
    // carries its node index in the parser associations; the primitives
    // are the object itself or its direct mesh children.
    const primitivesByNode = new Map<number, Mesh[]>();
    gltf.scene.traverse((object) => {
      const nodeIndex = parser.associations.get(object)?.nodes;
      if (nodeIndex == null) return;
      if (object instanceof Mesh) {
        primitivesByNode.set(nodeIndex, [object]);
        return;
      }
      const primitives = object.children.filter(
        (child): child is Mesh =>
          child instanceof Mesh &&
          parser.associations.get(child)?.nodes == null,
      );
      primitivesByNode.set(nodeIndex, primitives);
    });

    let meshCount = 0;
    const nodes = parser.json.nodes ?? [];
    for (let nodeIndex = 0; nodeIndex < nodes.length; nodeIndex += 1) {
      const node = nodes[nodeIndex];
      if (!node?.name || !options.accept(node.name) || node.mesh == null) {
        continue;
      }
      const primitives = primitivesByNode.get(nodeIndex) ?? [];
      for (const primitive of primitives) {
        const geometry = extractPrimitiveGeometry(primitive.geometry);
        if (!geometry) continue;
        geometries.add(geometry);
        const mesh = new Mesh(geometry, sourceMaterial);
        mesh.name = node.name;
        primitive.matrixWorld.decompose(
          mesh.position,
          mesh.quaternion,
          mesh.scale,
        );
        group.add(mesh);
        meshCount += 1;
      }
    }

    if (!meshCount) {
      throw new Error(options.emptyMessage);
    }
    disposeParsedGlb(gltf);
    gltf = null;
    return group;
  } catch (error) {
    // Includes every extracted geometry, mirroring the ownership the
    // hand-written parser had: nothing parsed may leak on failure.
    for (const geometry of geometries) geometry.dispose();
    if (gltf) disposeParsedGlb(gltf);
    sourceMaterial.dispose();
    group.clear();
    throw error;
  }
}
