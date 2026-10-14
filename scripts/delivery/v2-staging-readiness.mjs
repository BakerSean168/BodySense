import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

export const REQUIRED_DOMAINS = Object.freeze([
  "body-state",
  "conversation-person",
  "diagnosis",
  "training-safety",
  "person-assets",
]);
export const REQUIRED_GATES = Object.freeze([
  "business",
  "view_model",
  "database",
  "design",
  "engineering",
  "integration",
]);
export const REQUIRED_FOUNDATIONS = Object.freeze([
  "F-PRODUCT",
  "F-BODYFACT",
  "F-CONTRACT",
  "F-PENPOT",
  "F-ACCEPTANCE",
]);

// This gate governs only mutable staging-latest promotion. Immutable exact-SHA
// candidate publication and main CI always proceed independently.
export function evaluateV2StagingReadiness({ enabled, handoff }) {
  const blockers = [];
  if (enabled !== "true") {
    blockers.push(
      "BODYSENSE_V2_STAGING_PROMOTION_ENABLED is not explicitly true",
    );
  }
  if (!handoff || typeof handoff !== "object" || handoff.version !== 1) {
    blockers.push("approved V2 handoff register is missing or invalid");
    return { eligible: false, blockers };
  }
  const domains = Array.isArray(handoff.domains) ? handoff.domains : [];
  if (
    domains.length !== REQUIRED_DOMAINS.length ||
    new Set(domains.map((domain) => domain?.id)).size !==
      REQUIRED_DOMAINS.length ||
    REQUIRED_DOMAINS.some((id) => !domains.some((domain) => domain?.id === id))
  ) {
    blockers.push("expected five unique approved V2 domains");
  }
  for (const id of REQUIRED_DOMAINS) {
    const domain = domains.find((item) => item?.id === id);
    if (!domain || !domain.gates || typeof domain.gates !== "object") {
      blockers.push(`${id}: missing six-gate handoff`);
      continue;
    }
    for (const gate of REQUIRED_GATES) {
      const entry = domain.gates[gate];
      if (
        entry?.state !== "ready" ||
        !Array.isArray(entry?.proof) ||
        entry.proof.length === 0
      ) {
        blockers.push(`${id}.${gate}: not ready with reproducible proof`);
      }
    }
  }
  const foundations = Array.isArray(handoff.foundations)
    ? handoff.foundations
    : [];
  for (const id of REQUIRED_FOUNDATIONS) {
    const matching = foundations.filter((entry) => entry?.id === id);
    if (
      matching.length !== 1 ||
      matching[0].state !== "ready" ||
      !Array.isArray(matching[0].proof) ||
      matching[0].proof.length === 0
    ) {
      blockers.push(`${id}: not ready with proof`);
    }
  }
  return { eligible: blockers.length === 0, blockers };
}

const script = fileURLToPath(import.meta.url);
if (process.argv[1] && path.resolve(process.argv[1]) === script) {
  const root = path.resolve(path.dirname(script), "../..");
  let handoff;
  try {
    handoff = JSON.parse(
      fs.readFileSync(
        path.join(root, "docs/plan/active/v2-handoff-state.json"),
        "utf8",
      ),
    );
  } catch (error) {
    console.error(
      "V2 staging admission: unreadable readiness register",
      error.message,
    );
  }
  const result = evaluateV2StagingReadiness({
    enabled: process.env.BODYSENSE_V2_STAGING_PROMOTION_ENABLED,
    handoff,
  });
  const label = result.eligible ? "PASS" : "BLOCKED";
  console.log(`V2_STAGING_PROMOTION=${label}`);
  for (const reason of result.blockers) console.log(`BLOCKER: ${reason}`);
  if (process.argv.includes("--github-output")) {
    const outputFile = process.env.GITHUB_OUTPUT;
    if (!outputFile) throw new Error("GITHUB_OUTPUT is required");
    fs.appendFileSync(outputFile, `eligible=${result.eligible}\n`);
    if (process.env.GITHUB_STEP_SUMMARY) {
      fs.appendFileSync(
        process.env.GITHUB_STEP_SUMMARY,
        `### V2 staging-latest promotion: ${label}\n\n` +
          (result.eligible
            ? "All domain/foundation gates have proof and operator opt-in is enabled.\n"
            : "Immutable main candidate still publishes; mutable staging channel remains unchanged.\n" +
              result.blockers.map((reason) => `- ${reason}\n`).join("")),
      );
    }
  }
}
