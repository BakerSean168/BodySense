import type { ClientDiagnosticAttributeValue } from "@/lib/clientDiagnostics";
import type { LoadedVanatomeAtlas } from "./vanatomeAdapter";

export interface LongTaskMetrics {
  count: number;
  totalMs: number;
  maxMs: number;
}

export function diagnosticNow(): number {
  return typeof performance !== "undefined" ? performance.now() : Date.now();
}

export function roundMetric(value: number): number {
  return Math.round(value * 10) / 10;
}

export function longTaskAttributes(
  metrics: LongTaskMetrics,
): Record<string, ClientDiagnosticAttributeValue> {
  return {
    "main_thread.long_task_count": metrics.count,
    "main_thread.long_task_total_ms": roundMetric(metrics.totalMs),
    "main_thread.long_task_max_ms": roundMetric(metrics.maxMs),
  };
}

export function resourceTimingAttributes(
  resource: string | null | undefined,
): Record<string, ClientDiagnosticAttributeValue> {
  if (!resource || typeof performance === "undefined") return {};
  let absolute: string;
  try {
    absolute = new URL(resource, window.location.href).href;
  } catch {
    return {};
  }
  const entries = performance.getEntriesByName(absolute, "resource");
  const entry = entries.at(-1);
  if (!(entry instanceof PerformanceResourceTiming)) return {};
  return resourceTimingEntryAttributes(entry);
}

export function resourceTimingEntryAttributes(
  entry: PerformanceResourceTiming,
): Record<string, ClientDiagnosticAttributeValue> {
  // Cross-origin Resource Timing is redacted unless the asset host opts in via
  // Timing-Allow-Origin. When responseStart/size fields are zeroed, computing
  // derived TTFB/transfer values creates impossible negative or huge numbers.
  // Keep the coarse duration and explicitly mark the record as restricted.
  const timingRestricted =
    entry.responseStart <= 0 || entry.responseStart < entry.startTime;
  if (timingRestricted) {
    return {
      "resource.duration_ms": roundMetric(entry.duration),
      "resource.timing_restricted": true,
      "network.protocol": entry.nextHopProtocol || "unknown",
    };
  }

  return {
    "resource.duration_ms": roundMetric(entry.duration),
    "resource.ttfb_ms": roundMetric(entry.responseStart - entry.startTime),
    "resource.transfer_ms": roundMetric(
      entry.responseEnd - entry.responseStart,
    ),
    "resource.transfer_size": entry.transferSize,
    "resource.encoded_body_size": entry.encodedBodySize,
    "resource.decoded_body_size": entry.decodedBodySize,
    "resource.cache_hit": entry.transferSize === 0 && entry.decodedBodySize > 0,
    "network.protocol": entry.nextHopProtocol || "unknown",
  };
}

export function absoluteResourceUrl(resource: string): string {
  try {
    return new URL(resource, window.location.href).href;
  } catch {
    return resource;
  }
}

export function expectedModelBytes(
  loaded: LoadedVanatomeAtlas,
  modelUrl: string,
): number {
  let absoluteModelUrl: string;
  let absoluteCatalogUrl: string;
  try {
    absoluteModelUrl = new URL(modelUrl, window.location.href).href;
    absoluteCatalogUrl = new URL(loaded.catalogUrl, window.location.href).href;
  } catch {
    return 0;
  }
  for (const bundle of loaded.catalog.bundles) {
    try {
      if (
        new URL(bundle.modelUrl, absoluteCatalogUrl).href === absoluteModelUrl
      ) {
        return bundle.bytes ?? 0;
      }
    } catch {
      continue;
    }
  }
  return 0;
}
