import { Component, useEffect, type ReactNode } from "react";
import { useGLTF } from "@react-three/drei";
import { Mesh, MeshLambertMaterial, type Object3D } from "three";
import type { VanatomeViewerAtlas } from "@vixotic/vanatome-atlas";

const BODY_SHELL_ID = "body-shell";
const TRANSLUCENT_SKIN_FLAG = "bodysenseTranslucentSkin";

function anatomyIdOf(object: Object3D | null): string | null {
  for (let current = object; current; current = current.parent) {
    const id: unknown = current.userData?.anatomyId;
    if (typeof id === "string") return id;
  }
  return null;
}

export function atlasHasBodyShell(atlas: VanatomeViewerAtlas): boolean {
  return atlas.structures.some((structure) => structure.id === BODY_SHELL_ID);
}

/**
 * Default look: muscles over a translucent paper skin. Vanatome 0.1.6 restyles
 * every MeshStandardMaterial (the body shell is forced to an opaque/wireframe
 * look), so before the viewer clones the cached GLB scene the shell meshes get
 * a Lambert material it leaves alone. Skin detail stays unreadable; visibility,
 * picking and layer controls are unchanged.
 */
export function TranslucentShellPrep({
  url,
  onReady,
}: {
  url: string;
  onReady: (url: string) => void;
}) {
  const gltf = useGLTF(url);
  useEffect(() => {
    gltf.scene.traverse((object) => {
      if (!(object instanceof Mesh) || object.userData[TRANSLUCENT_SKIN_FLAG])
        return;
      if (anatomyIdOf(object) !== BODY_SHELL_ID) return;
      const previous = Array.isArray(object.material)
        ? object.material
        : [object.material];
      object.material = new MeshLambertMaterial({
        color: "#f4f1e8",
        transparent: true,
        opacity: 0.3,
        depthWrite: false,
      });
      previous.forEach((material) => material.dispose());
      object.renderOrder = 10;
      object.userData[TRANSLUCENT_SKIN_FLAG] = true;
    });
    onReady(url);
  }, [gltf, url, onReady]);
  return null;
}

/** A prep failure must not block the viewer; it reports its own load errors. */
export class ShellPrepBoundary extends Component<
  { url: string; onReady: (url: string) => void; children: ReactNode },
  { failed: boolean }
> {
  override state = { failed: false };
  static getDerivedStateFromError() {
    return { failed: true };
  }
  override componentDidCatch() {
    this.props.onReady(this.props.url);
  }
  override render() {
    return this.state.failed ? null : this.props.children;
  }
}
