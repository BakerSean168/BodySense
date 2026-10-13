import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

export const REPO_ROOT = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../..",
);
const STATES = new Set(["ready", "in_progress", "not_started", "blocked"]);
const TRACKS = [
  "domain_schema",
  "product_design",
  "engineering",
  "integration_release",
];
const GATES = [
  "business",
  "view_model",
  "database",
  "design",
  "engineering",
  "integration",
];

function checkFile(root, relative, label, errors) {
  if (
    typeof relative !== "string" ||
    !relative ||
    path.isAbsolute(relative) ||
    relative.includes("\\") ||
    relative.split("/").some((part) => !part || part === "." || part === "..")
  ) {
    errors.push(`${label}: invalid relative repository path`);
    return;
  }
  const resolved = path.resolve(root, relative);
  if (!resolved.startsWith(root + path.sep) || !fs.existsSync(resolved)) {
    errors.push(`${label}: missing or outside repository: ${relative}`);
  }
}

function validateGate(root, gate, label, errors) {
  if (!gate || typeof gate !== "object" || !STATES.has(gate.state)) {
    errors.push(`${label}: invalid/missing gate state`);
    return;
  }
  if (gate.state === "ready") {
    if (!Array.isArray(gate.proof) || gate.proof.length === 0) {
      errors.push(`${label}: ready without artifact proof`);
      return;
    }
    for (const [index, proof] of gate.proof.entries()) {
      checkFile(root, proof, `${label}.proof[${index}]`, errors);
    }
  } else if (
    typeof gate.next !== "string" ||
    !/^(DS|PD|ENG|IR)-[A-Z0-9-]+$/.test(gate.next)
  ) {
    errors.push(`${label}: pending gate without executable next ticket ID`);
  }
}

export function validateHandoff(handoff, root = REPO_ROOT) {
  const errors = [];
  if (!handoff || handoff.version !== 1)
    return ["handoff: unsupported version"];
  if (
    !handoff.baseline ||
    !["enabled", "disabled"].includes(handoff.baseline.hourlyAutomation)
  ) {
    errors.push("baseline: missing automation state");
  }
  checkFile(root, handoff.masterPlan, "masterPlan", errors);
  const master = handoff.masterPlan;
  if (typeof master === "string" && fs.existsSync(path.resolve(root, master))) {
    const body = fs.readFileSync(path.resolve(root, master), "utf8");
    if (!/^Status: ACTIVE \/ PRIMARY PROGRAM/m.test(body)) {
      errors.push("masterPlan: missing explicit ACTIVE primary program status");
    }
  }
  if (!handoff.tracks || typeof handoff.tracks !== "object") {
    errors.push("tracks: missing");
  } else {
    if (
      Object.keys(handoff.tracks).sort().join(",") !==
      [...TRACKS].sort().join(",")
    ) {
      errors.push("tracks: expected exactly four workstreams");
    }
    for (const id of TRACKS) {
      const loc = handoff.tracks[id];
      checkFile(root, loc, `tracks.${id}`, errors);
      if (typeof loc === "string" && fs.existsSync(path.resolve(root, loc))) {
        const body = fs.readFileSync(path.resolve(root, loc), "utf8");
        if (!/^Status: ACTIVE \/ TRACK [ABCD]/m.test(body)) {
          errors.push(`tracks.${id}: not marked ACTIVE`);
        }
        if (!body.includes("2026-10-13-v2-parallel-master.md")) {
          errors.push(`tracks.${id}: missing master plan link`);
        }
      }
    }
  }
  const index = path.join(root, "docs/plan/active/README.md");
  if (
    !fs.existsSync(index) ||
    !fs.readFileSync(index, "utf8").includes("2026-10-13-v2-parallel-master.md")
  ) {
    errors.push("active index: missing primary program link");
  }
  if (!Array.isArray(handoff.foundations) || !handoff.foundations.length) {
    errors.push("foundations: missing baseline status");
  } else {
    const known = new Set();
    for (const foundation of handoff.foundations) {
      if (typeof foundation.id !== "string" || known.has(foundation.id)) {
        errors.push(`foundations: duplicated/missing ID ${foundation.id}`);
      }
      known.add(foundation.id);
      validateGate(root, foundation, `foundation.${foundation.id}`, errors);
    }
  }
  if (!Array.isArray(handoff.domains) || handoff.domains.length !== 5) {
    errors.push("domains: expected five product-domain cohorts");
    return errors;
  }
  const domainIds = new Set();
  for (const domain of handoff.domains) {
    if (typeof domain.id !== "string" || domainIds.has(domain.id)) {
      errors.push(`domains: duplicate/missing ID ${domain.id}`);
    }
    domainIds.add(domain.id);
    if (!Array.isArray(domain.product) || !domain.product.length) {
      errors.push(`domain.${domain.id}: missing product authority`);
    } else {
      for (const [index, source] of domain.product.entries()) {
        checkFile(
          root,
          source,
          `domain.${domain.id}.product[${index}]`,
          errors,
        );
      }
    }
    if (
      !Array.isArray(domain.sf) ||
      !domain.sf.length ||
      domain.sf.some((id) => !/^SF-[0-9]{2}$/.test(id))
    ) {
      errors.push(`domain.${domain.id}: missing original SF ticket mapping`);
    }
    if (
      !domain.gates ||
      Object.keys(domain.gates).sort().join(",") !== [...GATES].sort().join(",")
    ) {
      errors.push(`domain.${domain.id}: expected six independent gates`);
      continue;
    }
    for (const key of GATES)
      validateGate(
        root,
        domain.gates[key],
        `domain.${domain.id}.${key}`,
        errors,
      );
    const ready = (key) => domain.gates[key]?.state === "ready";
    if (ready("view_model") && !ready("business")) {
      errors.push(`domain.${domain.id}: view_model signed off before business`);
    }
    if (ready("database") && !ready("business")) {
      errors.push(`domain.${domain.id}: database signed off before business`);
    }
    if (ready("design") && !ready("view_model")) {
      errors.push(`domain.${domain.id}: design signed off before view_model`);
    }
    if (ready("engineering") && (!ready("view_model") || !ready("database"))) {
      errors.push(
        `domain.${domain.id}: engineering signed off before view_model/database`,
      );
    }
    if (
      ready("integration") &&
      GATES.slice(0, -1).some((gate) => !ready(gate))
    ) {
      errors.push(
        `domain.${domain.id}: integration signed off before all prerequisite gates`,
      );
    }
  }
  return errors;
}

export function validateRepository(root = REPO_ROOT) {
  try {
    const state = JSON.parse(
      fs.readFileSync(
        path.join(root, "docs/plan/active/v2-handoff-state.json"),
        "utf8",
      ),
    );
    return validateHandoff(state, root);
  } catch (error) {
    return [`handoff: cannot parse v2-handoff-state.json: ${error.message}`];
  }
}

if (
  process.argv[1] &&
  path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  const errors = validateRepository();
  if (errors.length) {
    for (const error of errors)
      process.stderr.write(`V2_PLAN_ERROR: ${error}\n`);
    process.exitCode = 1;
  } else {
    process.stdout.write(
      "V2_ACTIVE_PLANS=PASS (one master, four tracks, five domains, six gates)\n",
    );
  }
}
