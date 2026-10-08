export const VANATOME_ATLAS_RELEASE = "1.4.0" as const;
export const VANATOME_ATLAS_BUILD_ID = "994e6cc8ffbb212e" as const;
export const VANATOME_ATLAS_CATALOG_URL =
  "https://atlas.vanatome.vixotic.in/releases/1.4.0/catalog.json" as const;
export const VANATOME_INITIAL_SYSTEM_ID = "regional-anatomy" as const;

interface VanatomeCatalogSystem {
  id: string;
  bundleId: string;
}

interface VanatomeCatalogBundle {
  id: string;
  modelUrl: string;
  bytes?: number;
  sha256?: string;
}

interface MinimalVanatomeCatalog {
  atlas: {
    version: string;
    buildId: string;
  };
  systems: VanatomeCatalogSystem[];
  bundles: VanatomeCatalogBundle[];
}

export interface PinnedVanatomeSystemSource {
  catalogUrl: string;
  modelUrl: string;
  bytes: number | null;
  sha256: string | null;
}

export function resolveVanatomeCatalogUrl(override?: string): string {
  const configured =
    override?.trim() ||
    import.meta.env.VITE_BODYSENSE_ANATOMY_CATALOG_URL?.trim();
  return configured || VANATOME_ATLAS_CATALOG_URL;
}

function absoluteUrl(value: string, base: string): string {
  const documentBase =
    typeof globalThis.location?.href === "string"
      ? globalThis.location.href
      : "http://localhost/";
  return new URL(value, new URL(base, documentBase)).href;
}

/**
 * Lightweight catalog resolution used by the Body Canvas home surface.
 *
 * Fine Anatomy continues to use the full Vanatome loader. The body-first canvas
 * needs only the pinned regional shell model URL, so importing/instantiating the
 * complete atlas loader here would add unnecessary work before first interaction.
 */
export async function resolvePinnedVanatomeSystemSource(
  systemId: string,
  options: {
    signal?: AbortSignal;
    catalogUrl?: string;
    fetchImpl?: typeof fetch;
  } = {},
): Promise<PinnedVanatomeSystemSource> {
  const catalogUrl = resolveVanatomeCatalogUrl(options.catalogUrl);
  const fetchImpl = options.fetchImpl ?? fetch;
  const response = await fetchImpl(catalogUrl, {
    signal: options.signal,
    cache: "force-cache",
  });
  if (!response.ok) {
    throw new Error(
      `Vanatome catalog failed with HTTP ${response.status}: ${catalogUrl}`,
    );
  }

  const value: unknown = await response.json();
  if (!value || typeof value !== "object") {
    throw new Error("Vanatome catalog is not an object");
  }
  const catalog = value as Partial<MinimalVanatomeCatalog>;
  if (
    catalog.atlas?.version !== VANATOME_ATLAS_RELEASE ||
    catalog.atlas?.buildId !== VANATOME_ATLAS_BUILD_ID
  ) {
    throw new Error(
      `Unexpected Vanatome atlas ${catalog.atlas?.version ?? "unknown"}/${catalog.atlas?.buildId ?? "unknown"}; expected ${VANATOME_ATLAS_RELEASE}/${VANATOME_ATLAS_BUILD_ID}`,
    );
  }
  if (!Array.isArray(catalog.systems) || !Array.isArray(catalog.bundles)) {
    throw new Error("Vanatome catalog is missing systems or bundles");
  }

  const system = catalog.systems.find(
    (candidate) =>
      candidate &&
      typeof candidate === "object" &&
      candidate.id === systemId &&
      typeof candidate.bundleId === "string",
  );
  if (!system) {
    throw new Error(`Vanatome system is missing: ${systemId}`);
  }
  const bundle = catalog.bundles.find(
    (candidate) =>
      candidate &&
      typeof candidate === "object" &&
      candidate.id === system.bundleId &&
      typeof candidate.modelUrl === "string",
  );
  if (!bundle) {
    throw new Error(`Vanatome bundle is missing: ${system.bundleId}`);
  }

  return {
    catalogUrl,
    modelUrl: absoluteUrl(bundle.modelUrl, catalogUrl),
    bytes:
      typeof bundle.bytes === "number" && Number.isFinite(bundle.bytes)
        ? bundle.bytes
        : null,
    sha256: typeof bundle.sha256 === "string" ? bundle.sha256 : null,
  };
}
