import {
  VANATOME_INITIAL_SYSTEM_ID,
  resolvePinnedVanatomeSystemSource,
} from "@/features/body-explorer/model/vanatomePin";

import {
  parseReferenceAtlasSurface,
  parseReferenceAtlasMeshes,
} from "./referenceAtlasParser";
import { disposeReferenceAtlasSurface } from "./disposeReferenceAtlas";
import type { ReferenceAtlasSurface } from "./referenceAtlas.types";
export {
  parseReferenceAtlasSurface,
  parseReferenceAtlasMeshes,
} from "./referenceAtlasParser";
export { disposeReferenceAtlasSurface } from "./disposeReferenceAtlas";
export type { ReferenceAtlasSurface } from "./referenceAtlas.types";

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
  options.signal?.throwIfAborted();
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
  options.signal?.throwIfAborted();
  return { bytes, modelUrl: source.modelUrl };
}

export async function loadPinnedReferenceAtlasSurface(
  options: {
    signal?: AbortSignal;
    catalogUrl?: string;
    fetchImpl?: typeof fetch;
  } = {},
): Promise<ReferenceAtlasSurface> {
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

export async function loadPinnedReferenceMuscles(
  options: {
    signal?: AbortSignal;
    fetchImpl?: typeof fetch;
    /** Download now, but parse only after this settles (e.g. the skin is built). */
    parseAfter?: Promise<unknown>;
  } = {},
): Promise<ReferenceAtlasSurface> {
  const fetchImpl = options.fetchImpl ?? fetch;
  const response = await fetchImpl(REFERENCE_MUSCLES_URL, {
    signal: options.signal,
    cache: "force-cache",
  });
  if (!response.ok) {
    throw new Error(
      `Reference muscles failed with HTTP ${response.status}: ${REFERENCE_MUSCLES_URL}`,
    );
  }
  const bytes = await response.arrayBuffer();
  options.signal?.throwIfAborted();
  await options.parseAfter?.catch(() => undefined);
  options.signal?.throwIfAborted();
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
export function loadPinnedReferenceSkeleton(
  options: {
    signal?: AbortSignal;
    catalogUrl?: string;
    fetchImpl?: typeof fetch;
  } = {},
): Promise<ReferenceAtlasSurface> {
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
  options.signal?.throwIfAborted();
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
