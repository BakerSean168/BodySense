import assert from "node:assert/strict";
import test from "node:test";
import { createCandidate } from "./candidate-manifest.mjs";
import { reusableCandidateFailures } from "./nightly-reuse.mjs";

const sha = "a".repeat(40);
const ciId = 777;
const candidateRunId = 999;
const artifactName = "candidate-set-" + sha;

function fixture() {
  const images = Object.fromEntries(
    ["web", "api", "aiService", "runtime"].map((component) => [
      component,
      {
        repository: "registry.example.test/bodysense-" + component,
        tag: "sha-" + sha,
        digest: "sha256:" + "e".repeat(64),
        revision: sha,
      },
    ]),
  );
  return {
    revision: sha,
    artifact: {
      name: artifactName,
      expired: false,
      workflow_run: { id: candidateRunId },
    },
    candidateRun: {
      id: candidateRunId,
      name: "Publish Nightly Candidate",
      event: "workflow_run",
      head_branch: "main",
      conclusion: "success",
    },
    sourceCiRun: {
      id: ciId,
      name: "CI",
      event: "schedule",
      head_branch: "main",
      head_sha: sha,
      conclusion: "success",
    },
    candidate: createCandidate({
      gitSha: sha,
      ciRunId: String(ciId),
      deliveryManifestDigest: "sha256:" + "f".repeat(64),
      migrationHead: 27,
      images,
      staticAssets: {
        enabled: true,
        revision: sha,
        assetBase: "https://cdn.example.test/revision/" + sha,
        atlasCatalogUrl: "https://cdn.example.test/atlas/catalog.json",
      },
    }),
  };
}

test("reuse succeeds for immutable same-SHA candidate sourced from scheduled Full CI", () => {
  assert.deepEqual(reusableCandidateFailures(fixture()), []);
});

test("legacy push-based candidate cannot bypass scheduled Nightly on same SHA", () => {
  const item = fixture();
  item.sourceCiRun.event = "push";
  assert.match(
    reusableCandidateFailures(item).join("; "),
    /successful scheduled Main Full CI/,
  );
});

test("reuse rejects incomplete or mismatched provenance", () => {
  const variants = [
    (item) => {
      item.candidate.ciRunId = "888";
    },
    (item) => {
      item.candidate.gitSha = "b".repeat(40);
    },
    (item) => {
      item.artifact.workflow_run.id = 123;
    },
    (item) => {
      item.artifact.expired = true;
    },
    (item) => {
      item.sourceCiRun.conclusion = "failure";
    },
    (item) => {
      item.sourceCiRun.head_sha = "b".repeat(40);
    },
    (item) => {
      item.candidateRun.conclusion = "failure";
    },
    (item) => {
      item.candidateRun.event = "workflow_dispatch";
    },
    (item) => {
      item.candidate.images.web.digest = "sha256:" + "0".repeat(64);
    },
  ];
  for (const mutate of variants) {
    const item = fixture();
    mutate(item);
    assert.ok(reusableCandidateFailures(item).length > 0);
  }
});

test("reused candidate is uploaded into the current Nightly run for manual Stable selection", async () => {
  const fs = await import("node:fs");
  const workflow = fs.readFileSync(
    ".github/workflows/candidate-publish.yml",
    "utf8",
  );
  assert.match(workflow, /nightly-reuse\.mjs/);
  assert.match(
    workflow,
    /candidate-set-\$\{\{ steps\.meta\.outputs\.revision \}\}/,
  );
  assert.match(workflow, /steps\.decision\.outputs\.publish == 'false'/);
  assert.match(workflow, /reports\/candidate\/reuse\/candidate-set-v1\.json/);
  assert.match(workflow, /actions\/upload-artifact@/);
});

test("reuse CLI carries a verified manifest into this run, and rebuilds for a push source", async () => {
  const fs = await import("node:fs");
  const os = await import("node:os");
  const path = await import("node:path");
  const { execFileSync } = await import("node:child_process");
  const item = fixture();
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "nightly-reuse-test-"));
  try {
    const mock = [
      "#!/bin/sh",
      'if [ "$1" = "api" ]; then',
      '  case "$2" in',
      '    *"/actions/artifacts?"*) cat "$MOCK_DIR/artifacts.json";;',
      '    *"/actions/runs/999") cat "$MOCK_DIR/candidate-run.json";;',
      '    *"/actions/runs/777") cat "$MOCK_DIR/ci-run.json";;',
      "    *) exit 42;;",
      "  esac",
      'elif [ "$1" = "run" ] && [ "$2" = "download" ]; then',
      '  while [ "$#" -gt 0 ]; do',
      '    if [ "$1" = "-D" ]; then dest="$2"; break; fi',
      "    shift",
      "  done",
      '  cp "$MOCK_DIR/candidate.json" "$dest/candidate-set-v1.json"',
      "else exit 43; fi",
      "",
    ].join("\n");
    fs.writeFileSync(path.join(dir, "gh"), mock, { mode: 0o755 });
    const json = (name, value) =>
      fs.writeFileSync(path.join(dir, name), JSON.stringify(value));
    json("artifacts.json", { artifacts: [item.artifact] });
    json("candidate-run.json", item.candidateRun);
    json("candidate.json", item.candidate);
    json("ci-run.json", item.sourceCiRun);
    const output = path.join(dir, "gh-output");
    const env = {
      ...process.env,
      PATH: dir + ":" + process.env.PATH,
      MOCK_DIR: dir,
      GITHUB_OUTPUT: output,
      GITHUB_STEP_SUMMARY: path.join(dir, "summary"),
    };
    const invoke = (manifest) =>
      execFileSync(
        process.execPath,
        [
          "scripts/delivery/nightly-reuse.mjs",
          "--revision",
          sha,
          "--repository",
          "BakerSean168/BodySense",
          "--output-manifest",
          manifest,
        ],
        { env, encoding: "utf8" },
      );
    const accepted = path.join(dir, "reused", "candidate-set-v1.json");
    invoke(accepted);
    assert.match(fs.readFileSync(output, "utf8"), /publish=false/);
    assert.deepEqual(
      JSON.parse(fs.readFileSync(accepted, "utf8")),
      item.candidate,
    );

    json("ci-run.json", { ...item.sourceCiRun, event: "push" });
    fs.writeFileSync(output, "");
    const rejected = path.join(dir, "should-not-be-reused.json");
    invoke(rejected);
    assert.match(fs.readFileSync(output, "utf8"), /publish=true/);
    assert.equal(fs.existsSync(rejected), false);
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});
