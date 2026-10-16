# PR6 E · Native BodyState closeout

Base: main `72bbe661d59b881b36e68d813595e82e01a80d62` (#297).

## Scope

E1's dead V1 intake binding was merged in #293. Close E's remaining architecture/API/table references and verify the full browser suite on the post-C native BodyState stack. Verify and archive the completed PR6 slice records; retain chronological implementation evidence rather than treating old pending statements as current work.

## Guardrails

Code, baseline and OpenAPI own implementation facts; docs/product owns business rules. Native facts retain user wording and owner/source lineage. Go owns Safety restrictions/resolutions and fail-closed coverage; Agent inputs stay frozen. This slice does not implement pending Diagnosis/Trainer/asset product domains or raise V2 readiness gates. Use only newly created synthetic local PG18/Redis and temporary API/AI processes; no deployment or business database cleanup.

## Plan and acceptance

1. Match current architecture descriptions to native ledgers, report-only Assessment, analysis-scoped evidence audit, exact restriction resolution and native Web projection.
2. Check merged PR identities and update/archive completed slice notes with immutable merge provenance.
3. Run every Playwright spec without retries, including native process-death/lease-expiry recovery, and record results and reproducible runner details. Preserve production Safety semantics.
4. Validate Markdown/local links, plan gates, route/schema references, architecture and Postman checks. One PR; complete CI, latest-main rebase, force-with-lease, exact-head merge without admin.

## Verified merge provenance

GitHub merge identities were read on 2026-10-16 and matched the main ancestry:

- D: #288, `b93ce7c0a2c627748c189f2ce6fac1ef79a623e0`.
- B2b: #290, `d93a7d9a27ca207927058e802ed98c4b968a77b3`.
- B3: #291, `b36364ec69ca41b3bdcdb889ed058c1676cbe918`.
- B4a: #292, `b8ba34fa192dc9a1dd2fa46b93505713b95881d8`.
- E1: #293, `dd740fb650581563e4f0036891484d9ff707d9bc`.
- B4b1: #294, `6705ca3c926739a37c3228d18990bb7d0ac3bf15`.
- B4b2a: #295, `8b2a6e6701e6a88dfa8cefa8e8a4432d11a5fefa`.
- B4b2b: #296, `4d3dc793c9f11cf02e9c9179454e4dac3fd9aaf6`.
- C: #297, `72bbe661d59b881b36e68d813595e82e01a80d62` (complete CI at head `96f3eedc1c716c36933dcc206f01f5690f4b550f`).

## Retirement audit

The executable baseline and OpenAPI are authoritative. The old fact/observation/evidence tables, aggregate GET, temporal/review commands, global evidence/clearance routes and Safety JSON/AI-write paths are removed. Native aggregate identity/revisions, source/owner checks, receipt/outbox transactions and frozen Agent inputs remain. Assessment is report-only; Diagnosis and Treatment use owner-bound analysis evidence. Web consumes native record APIs, rejecting mixed revisions and foreign owners, and its map derives only record-presence states.

Current technical/longitudinal/Safety/3D architecture descriptions now match those boundaries. Historical ADRs and completed execution-ticket examples remain historical evidence, with explicit retirement notes rather than rewritten decisions. Dependency manifests replace the duplicated stale version table. Completed PR6 notes are archived with merge provenance; active program checkpoints identify this bounded retirement without closing V2 readiness.

## Local acceptance evidence

Runtime, schema, public contracts and browser specs are unchanged from source `72bbe661d59b881b36e68d813595e82e01a80d62`; E changes documentation only.

- Full Chromium suite: **25/25 PASS in 10.0 minutes, zero skipped and zero retries**, across every one of the 13 spec files. Real atlas 1.4.0 rendering/fallback, Ask User recovery, owner/source rejection, negative/third-party non-facts, exact amendments, raw observations, Episode links, lifestyle temporal history, native Safety, explicit cancellation and the complete longitudinal loop passed.
- API restart recovery was enabled rather than skipped: expire the synthetic running Run lease, kill only this harness's API PID and restart the same binary/environment in `apps/api`. Normal startup reconciliation produced the durable `execution_lost` event and restored the browser composer. The brief proxy disconnect and shell `Killed` line are the intended process-death probe, not assertion failures.
- The new, labeled synthetic PG18 database contained **zero** legacy fact/observation/evidence tables, **zero** legacy Safety JSON columns and all six checked native authority tables (facts, observations, hypotheses, statement evidence, restrictions, resolutions).
- Focused Go HTTP tests passed: `TestRetiredBodyStateRoutesAbsent` and `TestGetHealthWorkspaceMatchesOpenAPIResponseSchema`.
- Documented native table names match the baseline; 77 listed concrete API operations match canonical OpenAPI. Changed-document local links, Markdown formatting, architecture boundary checks, 107 plan tests and Postman verification (106 operations) passed.
- All temporary test containers/processes were cleaned; no staging/production or business database operation was performed. No business/design/engineering/integration readiness flag was raised.

### Browser reproduction and provenance

Use the isolated PG18/Redis, deterministic Python/Go and upload environment from `scripts/validate-body-fact-vertical.sh` at the source above. For this final acceptance, omit that runner's bounded spec-file list and run the complete suite:

```bash
CI=true E2E_BASE_URL=http://127.0.0.1:28573 \
  E2E_API_BASE_URL=http://127.0.0.1:28571 \
  E2E_RESTART_API_COMMAND="bash /tmp/pr6-e-restart-api.sh" \
  node node_modules/@playwright/test/cli.js test --retries 0 --reporter=list
```

The restart helper uses only the newly created labeled container and owned API PID file. It runs the same lease-expiry/startup-reconciliation probe as `scripts/e2e-expire-run-and-restart-api.sh`, adapted to the isolated native process instead of a Compose environment. It is a test-only local artifact, not a new application runtime or production control path. The full-suite provisioning timeout was 1200 seconds; the pinned atlas catalog matched the existing browser CI configuration. CI's reproducible full suite remains `.github/workflows/ci.yml` → `pnpm e2e`.

Local evidence is retained under `.runtime/schema-first-v2-evidence/vertical-e2e-765881/`: `browser.log`, `api.log`, `ai.log`, `schema-evidence.txt`, `full-suite-runner.sh` and `restart-api.sh` (ignored test artifacts, not committed health data). Runner SHA256: `86f6cdf2ccbbe18d87b25677c6db084aabf7f4b8cab4cd0eb210e7c66d46366d`; restart helper SHA256: `c2e0ef6ad4f5bb7f27e2f3c9b1ef03436407bb1e6ae984820e3f1a458639159e`.

Status: E implementation/documentation and local acceptance COMPLETE. The associated GitHub PR owns exact-head CI and final merge provenance. Complete V2 Diagnosis/Trainer/asset/design/release acceptance remains outside PR6.
