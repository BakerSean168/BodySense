# BodySense V2 · Frozen Main Baseline and Writer Admission

Status: **SUPERSEDED BASELINE (2026-10-15).** The frozen writer baseline is now `e4160a9c755455ffe9277af2caba88f62df1a032` as recorded in `docs/plan/tickets/v2/queue.json` and the [2026-10-15 alignment audit](./2026-10-15-v2-implementation-alignment-audit.md). The admission rules below still apply; only the SHA moved. V2 domain/release gates remain open.
Owner: ChatGPT Web, GCP Agent Fabric. Observed on 2026-10-14 UTC.

## 1. Immutable execution base

The execution baseline for **all newly admitted, separate ticket worktrees** is
`6bbc36acbb7ed842776098a60e5e8479765d0ee3`.

- PR [#255](https://github.com/BakerSean168/BodySense/pull/255) merged normally at 2026-10-14T05:53:39Z.
- Main CI [38519099798](https://github.com/BakerSean168/BodySense/actions/runs/38519099798) completed SUCCESS on that exact SHA, including real Browser E2E, Web quality, Go/Postgres18 and governance oracles.
- [Publish Main Candidate 38519934344](https://github.com/BakerSean168/BodySense/actions/runs/38519934344): SUCCESS. Immutable Web/assets, four images, API/runtime and candidate manifest were published, with verification against the exact SHA.
- The same workflow's `Promote coherent staging channel` job **SKIPPED** (job 115708618040). `staging-latest` was not automatically promoted.
- [Release Publish 38520429638](https://github.com/BakerSean168/BodySense/actions/runs/38520429638): top-level SUCCESS, but all release-tag, GitHub Release and production promotion jobs **SKIPPED**. Do not misreport it as a release.
- Formal `v2-handoff-state.json` contains five product domains with incomplete six-gate acceptance; it remains untouched. The staged freeze job checks that register **and** explicit operator opt-in, otherwise mutable channel promotion is denied.

## 2. Source provenance: do not replay merged commits

The following committed exact SHAs are ancestors of this main baseline:

| Workstream                       | Included commit                            |
| -------------------------------- | ------------------------------------------ |
| Schema / Episode                 | `0e287ab128bb09a8e8fae354c16bf5e49acbe7bf` |
| Design                           | `a3b8db87e1ca190af08d95f648b540486022962f` |
| React                            | `1c779b5e04224fb38073b15bdfc426a3ce39b9ac` |
| Observation V11                  | `c2076d1dd39d64afa0ec616813e722a94db657ad` |
| Observation V12 committed source | `2396b1ebb77ef62919a993f8871f2ff553f46a4d` |
| Penpot UI source code            | `3597a5f29bc1085f319c91f6b51c1bdaa481f084` |

The full Domain commit `e823931390cee700891b35c73cf26bc4b2374e46` and QA commit
`a89a8090000058363b5b308a212ae0ca80dc0285` are **not exact ancestors**.
Some of their earlier content was individually cherry-picked; use patch-level
review rather than automatically replaying entire old branches.

## 3. Preserve existing writer ownership

- Domain checkout `.chatgpt-worktrees/bodysense-v2-domain` owns staged Training/Safety handoff, plans and two pure-model tests.
- Backend checkout `.chatgpt-worktrees/bodysense-observation-v12` owns a dirty Go HTTP test and the existing `ChatGPT-Web/ENG-BS-04` writer lock.
- QA checkout `.chatgpt-worktrees/bodysense-v2-qa` owns a staged test-harness edit. The backend/QA untracked virtualenvs are not change-request artifacts.
- Canonical source checkout contains untracked `.zcode/`; do not reset/clean it.
- Other design, React and Penpot worktrees hold older accepted committed snapshots and remain unchanged.

Do not force rebase, clean or cherry-pick owner-uncommitted files. Their owner
must commit and hand off or explicitly relinquish the writer lease.

## 4. New parallel ticket admission

1. Only dependency-satisfied, disjoint tickets are eligible.
2. Each new implementation ticket gets **one persistent named branch and worktree** rooted at exact main SHA `6bbc36acbb7ed842776098a60e5e8479765d0ee3`. Do not launch in the canonical shared root or old source worktrees.
3. The active queue validator requires each `in_progress` ticket's `baseSha` to match `mainBaselineSha`. Record absolute checkout, ticket ID and exclusive resources before the writer starts.
4. Any T3 thread must use an explicitly registered native project whose root equals that ticket checkout. The running Worker must visibly allow that ID; picker inventory alone cannot establish healthy model access or authorization.
5. Keep no more than three independent writers, with no overlap in domain contracts, migrations, public OpenAPI, generated clients, handoff JSON or test resources.
6. ChatGPT Web reviews exact SHA/diff, native tests and CI before accepting an implementation or merging; no staging/production action is implied.

Safe first wave: `INT-002` (stacked PR CI) alongside `OPS-001` (T3 admission).
Domain/Backend/QA owner-dirty work remains protected until owner handoff.
This is an execution-control document, not a replacement for `docs/product`
or the unique domain readiness register.
