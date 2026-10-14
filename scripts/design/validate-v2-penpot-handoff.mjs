import { readFileSync } from "node:fs";

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const M = new URL(
  "../../docs/design/v2/penpot-scene-manifest.json",
  import.meta.url,
);
const F = new URL(
  "../../docs/design/v2/penpot-flow-contract.json",
  import.meta.url,
);

export function validatePenpotHandoff(manifest, spec) {
  const errors = [];
  const check = (ok, message) => {
    if (!ok) errors.push(message);
  };
  check(UUID.test(manifest.fileId), "Penpot file locator missing");
  check(UUID.test(manifest.pageId), "Penpot page locator missing");
  check(
    manifest.fileId === spec.fileId && manifest.pageId === spec.pageId,
    "Flow contract uses a different Penpot file/page",
  );
  check(
    Array.isArray(manifest.scenes) && manifest.scenes.length >= 35,
    "Insufficient scene manifest for V2",
  );
  check(
    Object.keys(manifest.components || {}).length >= 14,
    "Reusable component inventory missing",
  );
  check(
    spec.status === "DESIGN_FLOW_SPEC_NOT_PENPOT_VIEWER_SIGNOFF",
    "Never silently mark unverified flows as signed off",
  );
  const keys = new Set();
  const ids = new Set();
  for (const scene of manifest.scenes || []) {
    check(
      scene.key && !keys.has(scene.key),
      "Duplicate or unnamed scene " + scene.key,
    );
    check(
      UUID.test(scene.id) && !ids.has(scene.id),
      "Scene ID missing/duplicated: " + scene.key,
    );
    keys.add(scene.key);
    ids.add(scene.id);
  }
  for (const [name, id] of Object.entries(manifest.components || {}))
    (check(
      name.startsWith("Action /") ||
        name.startsWith("Icon button /") ||
        name.startsWith("Status /") ||
        name.startsWith("Notice /") ||
        name.startsWith("Choice /") ||
        name.startsWith("Evidence /"),
      "Unknown component family: " + name,
    ),
      check(UUID.test(id), "Invalid component ID: " + name));
  const requiredFlows = new Set([
    "PD-BASE-01",
    "PD-BS-01",
    "PD-PC-01",
    "PD-DX-01",
    "PD-TP-01",
    "PD-ASSET-01",
  ]);
  let transitions = 0;
  const flows = new Set();
  for (const flow of spec.flows || []) {
    check(
      requiredFlows.has(flow.id) && !flows.has(flow.id),
      "Unexpected or duplicate flow " + flow.id,
    );
    flows.add(flow.id);
    check(keys.has(flow.start), "Missing flow start: " + flow.start);
    check(
      Array.isArray(flow.steps) && flow.steps.length > 0,
      "Flow missing steps: " + flow.id,
    );
    for (const step of flow.steps || []) {
      transitions++;
      check(
        keys.has(step.from) && keys.has(step.to),
        "Transition references a missing editable scene: " +
          step.from +
          " -> " +
          step.to,
      );
      check(
        typeof step.trigger === "string" && step.trigger.length > 0,
        "Transition needs a user-readable trigger",
      );
      check(
        ["local", "conditional", "command"].includes(step.kind),
        "Unknown transition class: " + step.kind,
      );
      if (step.kind === "command") {
        check(
          typeof step.command === "string" &&
            /^[a-z_]+\.[a-z_]+$/.test(step.command),
          "Command must name a typed operation: " + step.command,
        );
        check(
          typeof step.requires === "string" && step.requires.length > 0,
          "Persisting command must specify a server acknowledgement precondition: " +
            step.from,
        );
      }
      if (step.kind === "conditional")
        check(
          typeof step.requires === "string" && step.requires.length > 0,
          "Conditional branch requires explicit backend state",
        );
      if (
        /Receipt$|Updated$|Set recorded$|Erasure completed$|Current two$/.test(
          step.to,
        )
      ) {
        check(
          step.kind !== "local" && typeof step.requires === "string",
          "Never treat saved/erased/diagnosed as navigation alone: " + step.to,
        );
      }
    }
  }
  for (const f of requiredFlows)
    check(flows.has(f), "Missing required design flow: " + f);
  check(transitions >= 30, "Business flow spec is too incomplete");
  check(
    manifest.overlayQa?.nextActions?.includes("close-overlay") &&
      manifest.overlayQa?.nextActions?.includes("open-overlay"),
    "Overlay QA was not recorded",
  );
  return {
    ok: errors.length === 0,
    errors,
    scenes: keys.size,
    components: Object.keys(manifest.components || {}).length,
    flows: flows.size,
    transitions,
  };
}

if (
  process.argv[1] &&
  import.meta.url === new URL("file://" + process.argv[1]).href
) {
  const result = validatePenpotHandoff(
    JSON.parse(readFileSync(M, "utf8")),
    JSON.parse(readFileSync(F, "utf8")),
  );
  if (!result.ok) {
    console.error("V2_PENPOT_HANDOFF=FAIL\n" + result.errors.join("\n"));
    process.exitCode = 1;
  } else
    console.log(
      "V2_PENPOT_HANDOFF=PASS (" +
        result.scenes +
        " scenes, " +
        result.components +
        " components, " +
        result.flows +
        " flows, " +
        result.transitions +
        " conditional/local commands; scene/flow viewer acceptance remains open)",
    );
}
