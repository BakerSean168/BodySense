import { readFileSync } from "node:fs";

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const LIVE = new URL(
  "../../docs/design/v2/penpot-live-scenes-manifest.json",
  import.meta.url,
);
const BASE = new URL(
  "../../docs/design/v2/penpot-scene-manifest.json",
  import.meta.url,
);

const PRIVILEGED_OUTCOME =
  /(?:receipt|erasure completed|set recorded|updated diagnosis|diagnosis committed|current two)$/i;

export function validateLivePenpotScenes(live, original) {
  const errors = [];
  const requireProof = (passed, message) => {
    if (!passed) errors.push(message);
  };
  requireProof(
    live.schemaVersion === 2,
    "Live scene manifest needs schemaVersion 2",
  );
  requireProof(
    live.fileId === original.fileId && live.pageId === original.pageId,
    "Live scene references a different Penpot file or page",
  );
  requireProof(
    typeof live.snapshotAtUtc === "string" &&
      !Number.isNaN(Date.parse(live.snapshotAtUtc)),
    "Live scene snapshot timestamp missing",
  );
  requireProof(
    live.phase === "PILOT_LIVE_INTERACTIONS_NOT_FULL_SIGNOFF",
    "Live scene inventory cannot claim unverified design acceptance",
  );

  const scenes = new Map();
  const ids = new Set();
  for (const s of live.scenes || []) {
    requireProof(
      typeof s.key === "string" && !scenes.has(s.key),
      "Duplicate/unnamed live scene " + s.key,
    );
    requireProof(
      UUID.test(s.id) && !ids.has(s.id),
      "Invalid/duplicate live scene shape ID for " + s.key,
    );
    requireProof(
      UUID.test(s.canvasId),
      "Missing shared Canvas shape for " + s.key,
    );
    requireProof(
      s.kind === (s.key?.startsWith("Mobile /") ? "mobile" : "desktop"),
      "Scene device classification mismatch: " + s.key,
    );
    requireProof(
      /\/ Home(?: \/ .+)?$/.test(s.key)
        ? s.windowId === null
        : UUID.test(s.windowId),
      "Floating window reference inconsistent: " + s.key,
    );
    ids.add(s.id);
    scenes.set(s.key, s);
  }
  for (const key of [
    "Desktop / Home",
    "Desktop / Person / Records",
    "Desktop / Diagnosis / Current",
    "Desktop / Training / Accepted",
    "Desktop / Chat / Draft",
    "Mobile / Home",
    "Mobile / Person / Records",
    "Mobile / Diagnosis / Current",
    "Mobile / Training / Accepted",
    "Mobile / Chat / Draft",
    "Mobile / Chat / Ask User",
  ]) {
    requireProof(scenes.has(key), "Missing essential combined scene " + key);
  }

  const sources = new Set();
  for (const edge of live.localInteractions || []) {
    requireProof(
      edge.kind === "local",
      "Only navigation/selection may be hardwired in Penpot; persistent commands require backend acknowledgement",
    );
    requireProof(
      scenes.has(edge.from) && scenes.has(edge.to),
      "Navigation references a nonexistent combined scene: " +
        edge.from +
        " -> " +
        edge.to,
    );
    requireProof(
      UUID.test(edge.sourceId) && !sources.has(edge.sourceId),
      "Duplicate/invalid interactive hotspot: " + edge.sourceId,
    );
    requireProof(
      UUID.test(edge.targetId) && edge.targetId === scenes.get(edge.to)?.id,
      "Navigation destination ID does not match saved scene",
    );
    requireProof(
      typeof edge.control === "string" && edge.control.length > 0,
      "Navigation must name its trigger",
    );
    requireProof(
      !PRIVILEGED_OUTCOME.test(edge.to),
      "Medical, training, or deletion success must never be navigated to without server acknowledgement",
    );
    sources.add(edge.sourceId);
  }

  const starts = new Set();
  for (const f of live.flowStarts || []) {
    requireProof(
      typeof f.name === "string" && !starts.has(f.name),
      "Flow name missing/duplicated",
    );
    requireProof(
      scenes.has(f.startKey) && f.startId === scenes.get(f.startKey)?.id,
      "Flow start must point at an actual combined scene",
    );
    starts.add(f.name);
  }
  for (const kind of ["Desktop / Home", "Mobile / Home"])
    requireProof(
      (live.flowStarts || []).some((f) => f.startKey === kind),
      "Missing actual Canvas flow start: " + kind,
    );

  for (const test of live.viewerQa || []) {
    requireProof(
      test.status === "passed" || test.status === "blocked",
      "Viewer QA result must be explicitly passed or blocked",
    );
    requireProof(
      scenes.has(test.from) && scenes.has(test.to),
      "Viewer QA points outside the saved scenes",
    );
    if (test.status === "passed") {
      requireProof(
        typeof test.evidence === "string" &&
          test.evidence.includes("Viewer") &&
          typeof test.observedAtUtc === "string" &&
          !Number.isNaN(Date.parse(test.observedAtUtc)),
        "Passing Viewer QA requires timestamped real click evidence",
      );
      requireProof(
        (live.localInteractions || []).some(
          (edge) =>
            edge.from === test.from &&
            edge.to === test.to &&
            edge.control === test.control,
        ),
        "Viewer QA claim cannot exist without corresponding saved navigation",
      );
    }
  }

  requireProof(
    Array.isArray(live.outstanding) && live.outstanding.length > 0,
    "Incomplete business design cannot be declared finished",
  );
  return {
    ok: errors.length === 0,
    errors,
    scenes: scenes.size,
    localInteractions: (live.localInteractions || []).length,
    viewerPassed: (live.viewerQa || []).filter((q) => q.status === "passed")
      .length,
    flowStarts: starts.size,
  };
}

if (
  process.argv[1] &&
  import.meta.url === new URL("file://" + process.argv[1]).href
) {
  const live = JSON.parse(readFileSync(LIVE, "utf8"));
  const base = JSON.parse(readFileSync(BASE, "utf8"));
  const r = validateLivePenpotScenes(live, base);
  if (!r.ok) {
    console.error("V2_PENPOT_LIVE=FAIL\n" + r.errors.join("\n"));
    process.exitCode = 1;
  } else {
    console.log(
      `V2_PENPOT_LIVE=PASS (${r.scenes} scenes, ${r.localInteractions} saved local interactions, ${r.viewerPassed} timestamped Viewer clicks, ${r.flowStarts} starts; complete domain sign-off remains open)`,
    );
  }
}
