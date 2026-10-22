import {
  BufferGeometry,
  Float32BufferAttribute,
  Group,
  Mesh,
  MeshBasicMaterial,
  Quaternion,
} from "three";
import {
  parseGlb,
  readFloatVec3,
  readIndices,
  MODE_TRIANGLES,
  type GltfNode,
} from "./referenceAtlasGlb";

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
  const geometries = new Set<BufferGeometry>();
  try {
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
        geometries.add(geometry);
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
            new Float32BufferAttribute(
              readFloatVec3(parsed, normalAccessor),
              3,
            ),
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
      throw new Error(options.emptyMessage);
    }
    return group;
  } catch (error) {
    // Includes the geometry whose accessor failed before it could be attached.
    for (const geometry of geometries) geometry.dispose();
    sourceMaterial.dispose();
    group.clear();
    throw error;
  }
}
