#!/usr/bin/env node
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { validateCandidate } from "./candidate-manifest.mjs";

const CANDIDATE_WORKFLOW = "Publish Nightly Candidate";

export function reusableCandidateFailures({
  artifact,
  candidateRun,
  sourceCiRun,
  candidate,
  revision,
}) {
  const failures = [];
  if (!/^[0-9a-f]{40}$/.test(revision ?? ""))
    failures.push("invalid target revision");
  if (artifact?.expired !== false)
    failures.push("candidate artifact is expired");
  if (artifact?.name !== "candidate-set-" + revision)
    failures.push("candidate artifact name does not match revision");
  if (
    String(artifact?.workflow_run?.id ?? "") !== String(candidateRun?.id ?? "")
  )
    failures.push("artifact owner workflow run mismatch");
  if (
    candidateRun?.name !== CANDIDATE_WORKFLOW ||
    candidateRun?.event !== "workflow_run" ||
    candidateRun?.head_branch !== "main" ||
    candidateRun?.conclusion !== "success"
  )
    failures.push("candidate workflow was not a successful main CI downstream");
  failures.push(...validateCandidate(candidate));
  if (candidate?.gitSha !== revision)
    failures.push("candidate manifest source SHA mismatch");
  if (String(candidate?.ciRunId ?? "") !== String(sourceCiRun?.id ?? ""))
    failures.push("candidate source CI run ID mismatch");
  if (
    sourceCiRun?.name !== "CI" ||
    sourceCiRun?.event !== "schedule" ||
    sourceCiRun?.head_branch !== "main" ||
    sourceCiRun?.head_sha !== revision ||
    sourceCiRun?.conclusion !== "success"
  )
    failures.push(
      "candidate source must be a successful scheduled Main Full CI",
    );
  return failures;
}

function gh(args) {
  return execFileSync("gh", args, {
    encoding: "utf8",
    maxBuffer: 8 * 1024 * 1024,
    stdio: ["ignore", "pipe", "pipe"],
    timeout: 90_000,
  });
}

function githubJson(endpoint) {
  return JSON.parse(gh(["api", endpoint]));
}

function listCandidateArtifacts(repository, revision) {
  const name = "candidate-set-" + revision;
  const artifacts = [];
  for (let page = 1; page <= 20; page++) {
    const response = githubJson(
      "repos/" +
        repository +
        "/actions/artifacts?name=" +
        name +
        "&per_page=100&page=" +
        page,
    );
    const items = response.artifacts;
    if (!Array.isArray(items))
      throw new Error("GitHub artifacts API has no artifacts array");
    artifacts.push(...items);
    if (items.length < 100) return artifacts;
  }
  throw new Error("candidate artifact lookup exceeded pagination limit");
}

function note(message) {
  console.log(message);
  if (process.env.GITHUB_STEP_SUMMARY)
    fs.appendFileSync(process.env.GITHUB_STEP_SUMMARY, message + "\n");
}

function decision(publish) {
  if (process.env.GITHUB_OUTPUT)
    fs.appendFileSync(process.env.GITHUB_OUTPUT, "publish=" + publish + "\n");
  else console.log("publish=" + publish);
}

function downloadCandidate(repository, runId, name, destination) {
  gh([
    "run",
    "download",
    String(runId),
    "-R",
    repository,
    "-n",
    name,
    "-D",
    destination,
  ]);
  return JSON.parse(
    fs.readFileSync(path.join(destination, "candidate-set-v1.json"), "utf8"),
  );
}

export function resolveReusableCandidate({
  repository,
  revision,
  outputManifest,
}) {
  if (!/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(repository ?? ""))
    throw new Error("invalid repository");
  if (!/^[0-9a-f]{40}$/.test(revision ?? ""))
    throw new Error("invalid revision");
  if (!outputManifest) throw new Error("output manifest is required");

  const artifacts = listCandidateArtifacts(repository, revision);
  for (const artifact of artifacts) {
    if (artifact.expired || !artifact.workflow_run?.id) continue;
    const candidateRun = githubJson(
      "repos/" + repository + "/actions/runs/" + artifact.workflow_run.id,
    );
    if (
      candidateRun.name !== CANDIDATE_WORKFLOW ||
      candidateRun.event !== "workflow_run" ||
      candidateRun.head_branch !== "main" ||
      candidateRun.conclusion !== "success"
    )
      continue;
    const tempDir = fs.mkdtempSync(
      path.join(os.tmpdir(), "bodysense-nightly-reuse-"),
    );
    try {
      const candidate = downloadCandidate(
        repository,
        candidateRun.id,
        artifact.name,
        tempDir,
      );
      if (validateCandidate(candidate).length) {
        note(
          "Ignored invalid prior candidate manifest from run " +
            candidateRun.id,
        );
        continue;
      }
      const sourceCiRun = githubJson(
        "repos/" + repository + "/actions/runs/" + candidate.ciRunId,
      );
      const failures = reusableCandidateFailures({
        artifact,
        candidateRun,
        sourceCiRun,
        candidate,
        revision,
      });
      if (failures.length) {
        note(
          "Ignored unqualified prior candidate run " +
            candidateRun.id +
            ": " +
            failures.join("; "),
        );
        continue;
      }
      fs.mkdirSync(path.dirname(outputManifest), { recursive: true });
      fs.copyFileSync(
        path.join(tempDir, "candidate-set-v1.json"),
        outputManifest,
      );
      note(
        "Reusing qualified scheduled Nightly candidate from run " +
          candidateRun.id +
          " (source CI " +
          candidate.ciRunId +
          ")",
      );
      decision(false);
      return false;
    } finally {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
  }
  note(
    "No fully qualified scheduled Nightly candidate exists for " +
      revision +
      "; building a new candidate.",
  );
  decision(true);
  return true;
}

function main() {
  const args = {};
  for (let index = 2; index < process.argv.length; index += 2) {
    const flag = process.argv[index];
    const value = process.argv[index + 1];
    if (!flag?.startsWith("--") || !value || value.startsWith("--"))
      throw new Error("expected --name value pairs");
    args[flag.slice(2)] = value;
  }
  resolveReusableCandidate({
    repository: args.repository,
    revision: args.revision,
    outputManifest: args["output-manifest"],
  });
}

if (
  process.argv[1] &&
  fileURLToPath(import.meta.url) === path.resolve(process.argv[1])
) {
  try {
    main();
  } catch (error) {
    console.error("Nightly candidate reuse failed: " + error.message);
    process.exitCode = 1;
  }
}
