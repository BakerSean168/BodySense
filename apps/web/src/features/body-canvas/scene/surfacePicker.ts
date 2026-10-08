import {
  Camera, Color, DoubleSide, MeshBasicMaterial, NearestFilter, NoColorSpace,
  RGBAFormat, Scene, SkinnedMesh, UnsignedByteType, WebGLRenderer, WebGLRenderTarget,
} from "three";
import type { BodyRegionId } from "@/features/body-explorer/model/bodyRegionOntology";
import type { ReferenceRig } from "./referenceRig";

export interface ScreenRect { x: number; y: number; width: number; height: number }

/** A depth-tested ID pass selects visible skin, never projected geometry hidden behind it. */
export class SurfacePicker {
  private readonly scene = new Scene();
  private readonly target = new WebGLRenderTarget(1, 1, {
    format: RGBAFormat, type: UnsignedByteType,
    minFilter: NearestFilter, magFilter: NearestFilter,
    depthBuffer: true, stencilBuffer: false,
  });
  private readonly regions: BodyRegionId[];
  private readonly materials: MeshBasicMaterial[] = [];
  private width = 1;
  private height = 1;
  private screenWidth = 1;
  private screenHeight = 1;

  constructor(private readonly renderer: WebGLRenderer, rig: ReferenceRig) {
    this.target.texture.colorSpace = NoColorSpace;
    this.scene.background = new Color(0, 0, 0);
    this.regions = rig.meshes.map((mesh) => mesh.userData.bodyRegionId as BodyRegionId);
    rig.meshes.forEach((source, index) => {
      const material = new MeshBasicMaterial({ color: new Color((index + 1) / 255, 0, 0), side: DoubleSide, toneMapped: false, fog: false });
      this.materials.push(material);
      const mesh = new SkinnedMesh(source.geometry, material);
      mesh.bind(source.skeleton, source.bindMatrix);
      mesh.frustumCulled = false;
      this.scene.add(mesh);
    });
  }

  render(camera: Camera, width: number, height: number): void {
    this.screenWidth = Math.max(1, width); this.screenHeight = Math.max(1, height);
    const scale = Math.min(1, 768 / this.screenWidth, 1024 / this.screenHeight);
    const w = Math.max(1, Math.round(width * scale)), h = Math.max(1, Math.round(height * scale));
    if (w !== this.width || h !== this.height) { this.width = w; this.height = h; this.target.setSize(w, h); }
    const previous = this.renderer.getRenderTarget();
    const color = this.renderer.getClearColor(new Color());
    const alpha = this.renderer.getClearAlpha();
    const autoClear = this.renderer.autoClear;
    try {
      this.renderer.autoClear = true;
      this.renderer.setRenderTarget(this.target);
      this.renderer.setClearColor(0, 1);
      this.renderer.clear();
      this.renderer.render(this.scene, camera);
    } finally {
      this.renderer.setRenderTarget(previous);
      this.renderer.setClearColor(color, alpha);
      this.renderer.autoClear = autoClear;
    }
  }

  read(rect: ScreenRect, circle = false): BodyRegionId[] {
    const sx = this.width / this.screenWidth, sy = this.height / this.screenHeight;
    const left = Math.max(0, Math.floor(rect.x * sx));
    const right = Math.min(this.width, Math.ceil((rect.x + Math.max(1, rect.width)) * sx));
    const top = Math.max(0, Math.floor(rect.y * sy));
    const bottom = Math.min(this.height, Math.ceil((rect.y + Math.max(1, rect.height)) * sy));
    const width = right - left, height = bottom - top;
    if (width <= 0 || height <= 0) return [];
    const pixels = new Uint8Array(width * height * 4);
    this.renderer.readRenderTargetPixels(this.target, left, this.height - bottom, width, height, pixels);
    const found = new Set<BodyRegionId>();
    for (let i = 0; i < pixels.length; i += 4) {
      if (circle) {
        const x = ((i / 4) % width + .5) / width - .5;
        const y = (Math.floor(i / 4 / width) + .5) / height - .5;
        if (x * x + y * y > .25) continue;
      }
      const region = this.regions[pixels[i] - 1];
      if (region) found.add(region);
    }
    return [...found];
  }

  dispose(): void {
    this.target.dispose(); this.materials.forEach((material) => material.dispose()); this.scene.clear();
  }
}
