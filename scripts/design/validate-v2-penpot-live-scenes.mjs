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

  requireProof(
    live.flowStartEvidence ===
      "Official penpot.currentPage.flows startingBoard readback",
    "Flow starts must be backed by actual official Penpot page flows",
  );
  const starts = new Set();
  for (const f of live.flowStarts || []) {
    requireProof(
      typeof f.name === "string" && !starts.has(f.name),
      "Flow name missing/duplicated",
    );
    requireProof(
      scenes.has(f.startKey) &&
        f.startId === scenes.get(f.startKey)?.id &&
        f.startName === "V2 / Scene / " + f.startKey,
      "Flow start must point at an actual combined scene",
    );
    starts.add(f.name);
  }
  for (const kind of ["Desktop / Home", "Mobile / Home"])
    requireProof(
      (live.flowStarts || []).some((f) => f.startKey === kind),
      "Missing actual Canvas flow start: " + kind,
    );

  const caseProgress = new Map();
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
        UUID.test(test.sourceId) &&
          UUID.test(test.targetId) &&
          (live.localInteractions || []).some(
            (edge) =>
              edge.sourceId === test.sourceId &&
              edge.targetId === test.targetId &&
              edge.from === test.from &&
              edge.to === test.to &&
              edge.control === test.control,
          ),
        "Viewer QA must identify its actual saved Penpot hotspot and destination IDs",
      );
      const progress = caseProgress.get(test.case);
      requireProof(
        typeof test.case === "string" &&
          test.case.length > 0 &&
          Number.isInteger(test.step) &&
          test.step === (progress?.step || 0) + 1 &&
          (!progress || progress.to === test.from),
        "Viewer QA sequence must be contiguous in its named test case",
      );
      caseProgress.set(test.case, test);
    }
  }

  requireProof(
    caseProgress.size >= 17,
    "The expanded Viewer handoff lacks sufficient distinct tested flows",
  );
  const caseRecords = new Set();
  for (const entry of live.viewerEvidence?.testCases || []) {
    requireProof(
      entry.status === "passed" &&
        !caseRecords.has(entry.name) &&
        caseProgress.get(entry.name)?.step === entry.clickedSteps &&
        caseProgress.get(entry.name)?.observedAtUtc === entry.observedAtUtc,
      "Viewer test-case summary must match the actual ordered click trace: " +
        entry.name,
    );
    caseRecords.add(entry.name);
  }
  requireProof(
    caseRecords.size === caseProgress.size,
    "Every Viewer click series requires exactly one matching test-case record",
  );

  requireProof(
    scenes.size >= 34,
    "Insufficient real composed V2 domain scenes for the expanded handoff",
  );
  requireProof(
    sources.size >= 120,
    "Expanded Penpot navigation inventory is incomplete",
  );
  requireProof(starts.size >= 15, "Essential design Flow starts are missing");

  const geometry = live.coordinateAudit;
  requireProof(
    geometry?.desktopViewportWidth === 1280 &&
      geometry?.mobileViewportWidth === 390 &&
      typeof geometry?.method === "string" &&
      geometry.method.includes("Penpot") &&
      geometry.method.includes("not browser"),
    "Viewport coordinate audit needs real Penpot shape readback and a design-only disclaimer",
  );
  const auditedKeys = new Set();
  for (const frame of geometry?.scenes || []) {
    const originalFrame = scenes.get(frame.key);
    const isMobile = frame.kind === "mobile";
    const viewportWidth = isMobile ? 390 : 1280;
    const viewportHeight = isMobile ? 844 : 900;
    const rightMargin = isMobile ? 16 : 48;
    requireProof(
      Boolean(originalFrame) &&
        !auditedKeys.has(frame.key) &&
        frame.id === originalFrame?.id &&
        frame.windowId === originalFrame?.windowId &&
        frame.kind === originalFrame?.kind &&
        frame.viewerWidth === viewportWidth &&
        frame.viewerHeight === viewportHeight,
      "Coordinate audit does not match a unique saved scene: " + frame.key,
    );
    if (frame.windowId !== null) {
      requireProof(
        Number.isFinite(frame.x) &&
          Number.isFinite(frame.y) &&
          Number.isFinite(frame.windowWidth) &&
          Number.isFinite(frame.windowHeight) &&
          frame.x >= 0 &&
          frame.y >= 0 &&
          frame.windowWidth > 0 &&
          frame.windowHeight > 0 &&
          frame.right === frame.x + frame.windowWidth &&
          frame.bottom === frame.y + frame.windowHeight &&
          frame.right <= viewportWidth - rightMargin &&
          frame.bottom <= viewportHeight,
        "Floating window exceeds bounded Penpot canvas/viewer coordinates: " +
          frame.key,
      );
    }
    auditedKeys.add(frame.key);
  }
  requireProof(
    auditedKeys.size === scenes.size,
    "Every real scene must have one Penpot coordinate audit entry",
  );

  const identity = live.identityOverlay;
  const popup = identity?.overlay;
  requireProof(
    UUID.test(popup?.id) && popup?.closeAction === "close-overlay",
    "Person profile must be a single closable identity overlay, not a sixth facet",
  );
  const identitySources = new Set();
  const expectedFacets = new Set([
    "Desktop / Person / Records",
    "Desktop / Person / Equipment",
    "Desktop / Person / Media",
    "Desktop / Person / Reports",
    "Desktop / Person / Habits",
  ]);
  for (const opener of identity?.openers || []) {
    requireProof(
      expectedFacets.has(opener.from) &&
        opener.type === "open-overlay" &&
        opener.destinationId === popup?.id &&
        UUID.test(opener.sourceId) &&
        !identitySources.has(opener.sourceId) &&
        !sources.has(opener.sourceId) &&
        opener.closeWhenClickOutside === false,
      "Identity overlay opener is not a unique, safe facet-local hotspot: " +
        opener.from,
    );
    identitySources.add(opener.sourceId);
    expectedFacets.delete(opener.from);
  }
  requireProof(
    expectedFacets.size === 0 && identitySources.size === 5,
    "All five Person facets must open the same identity overlay",
  );
  const popupQa = identity?.viewerQa;
  requireProof(
    popupQa?.status === "passed" &&
      typeof popupQa?.observedAtUtc === "string" &&
      !Number.isNaN(Date.parse(popupQa.observedAtUtc)) &&
      popupQa?.baseScene === "Desktop / Person / Records" &&
      popupQa.steps?.length === 2 &&
      popupQa.steps[0].action === "open-overlay" &&
      identitySources.has(popupQa.steps[0].sourceId) &&
      popupQa.steps[1].action === "close-overlay" &&
      popupQa.steps[1].sourceId === popup?.closeSourceId &&
      popupQa.steps.every(
        (step) =>
          step.targetId === popup?.id &&
          step.evidence?.includes("Chrome Penpot Viewer DOM click"),
      ),
    "Identity overlay needs actual Viewer open/close click evidence",
  );

  const guardedIds = new Set();
  for (const control of live.guardedCommands || []) {
    requireProof(
      scenes.has(control.sceneKey) &&
        UUID.test(control.id) &&
        !guardedIds.has(control.id) &&
        !sources.has(control.id) &&
        control.interactions === 0 &&
        /^[a-z][a-z0-9_]*(?:\.[a-z][a-z0-9_]*)+$/.test(control.command) &&
        typeof control.guard === "string" &&
        control.guard.trim().length > 10,
      "A persisting command must have zero unconditional prototype navigation and an authoritative guard: " +
        control.sceneKey,
    );
    guardedIds.add(control.id);
  }
  requireProof(
    guardedIds.size >= 14,
    "Explicit server-gated controls are missing from the safe prototype handoff",
  );

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
    guardedCommands: guardedIds.size,
    overlayQa: popupQa?.status === "passed",
    coordinateAudited: auditedKeys.size,
    viewerCasesPassed: caseProgress.size,
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
