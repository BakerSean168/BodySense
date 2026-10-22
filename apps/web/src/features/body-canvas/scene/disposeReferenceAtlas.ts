import { Group, Mesh, MeshBasicMaterial } from "three";

export function disposeReferenceAtlasSurface(group: Group): void {
  const materials = new Set<MeshBasicMaterial>();
  group.traverse((object) => {
    if (!(object instanceof Mesh)) return;
    object.geometry.dispose();
    const list = Array.isArray(object.material)
      ? object.material
      : [object.material];
    for (const material of list) {
      if (material instanceof MeshBasicMaterial) materials.add(material);
    }
  });
  for (const material of materials) material.dispose();
  group.clear();
}
