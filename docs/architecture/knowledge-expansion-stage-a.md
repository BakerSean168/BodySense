# Knowledge Expansion — Stage A Research: Baseline, Needs, and Reference Architectures

> Status: Stage A research output for the knowledge-expansion parent task
> (GitHub Issue #499). This document is a research/decision record, not a task
> ledger: it fixes the verifiable baseline, the knowledge-need priorities, and
> the recommended expansion architecture so Stage B can start from explicit
> entry criteria. Product semantics remain owned by
> [docs/product](../product/README.md); lifecycle mechanics remain owned by
> [Knowledge Lifecycle Architecture](./knowledge-lifecycle-architecture.md).
>
> Method: the baseline in §1–§2 is verified from this repository only, at
> `main` commit `106824d`. Anything that exists only in a database or a running
> environment is listed as an explicit gap in §3 — no number in this document
> is an estimate of production data. External facts in §5 were checked against
> public sources on 2026-11-03 (references inline). Statements marked
> *(inference)* are judgement, not measurements.

## 1. Current system summary

BodySense already has a governed knowledge lifecycle; it does **not** have a
broad knowledge base yet. The pipeline is:

```text
register source (Go KnowledgeSourceRegistry, operator-protected)
  -> ingest job (Go KnowledgeIngestionService, knowledge.ingest_video only)
  -> Python AI service (ASR -> split -> AI curate -> embed)
  -> generated/curated packs (file artifacts, schema-versioned)
  -> human review + claim review + external-evidence review
  -> publish (Go KnowledgePublicationService, transactional, rollback-able)
  -> published-only retrieval (Python KnowledgeLibrary, pgvector)
  -> citation + answer attribution + publication observations (deny-first gate)
```

Component inventory (paths verified in-repo):

| Concern | Implementation |
| --- | --- |
| Source registry | `apps/api/internal/service/knowledge_source_registry.go`; model `apps/api/internal/model/knowledge_source.go` (`knowledge_sources`) |
| Ingestion | `apps/api/internal/service/knowledge_ingestion_service.go` — single job type `knowledge.ingest_video`; rejects non-`video` registered sources and any video path that differs from the registered source path |
| Publication | `apps/api/internal/service/knowledge_publication_service.go`; CLI `apps/api/cmd/knowledge-publication-manager` (`publish-reviewed`, `rollback`, `observe-eval`, `status`); source CLI `apps/api/cmd/knowledge-source-manager` (`register-thought-forest`) |
| Retrieval (online) | `apps/ai-service/src/rag/knowledge_library.py` (async pgvector pool); HTTP surface `apps/ai-service/src/api/routes/knowledge.py` (`POST /search`, `GET /sources`, `GET /stats`); agent tool `apps/ai-service/src/services/agent/tools/search_knowledge.py` |
| Ingestion-side RAG | `apps/ai-service/src/rag/`: `video_pipeline.py`, `splitter.py` / `ai_splitter.py`, `ai_curator.py`, `embedding.py`, `curated_source.py`, `knowledge_pack.py`, `clip_exporter.py` |
| Review-side RAG | `apps/ai-service/src/rag/claim_review.py`, `external_evidence.py` (canonicalises DOI / PMID / URL references), `thought_forest_snapshot.py` (Markdown note snapshot → claim candidates) |
| Data model | `knowledge_sources -> knowledge_segments -> knowledge_units (+ knowledge_clips)`, `knowledge_publications`, `knowledge_publication_observations`; canonical schema is the migration history (baseline `apps/api/migrations/000001_vnext_baseline.up.sql` plus later migrations cited in the lifecycle doc §6) |
| Eval | `apps/ai-service/scripts/run_published_knowledge_eval.py` + datasets and thresholds under `docs/knowledges/eval/` |

Governance properties that any expansion must preserve (all enforced in code,
not just documented):

- **License is a closed vocabulary.** `knowledge_source_registry.go` accepts
  exactly `verified_reuse`, `citation_only`, `owned`, `public_domain`.
  Registration with any other value — including an unreviewed `unknown` —
  fails. The DB column default is `unknown`, so a source that bypassed the
  registry can exist but `FindIngestible` refuses it.
- **Source identity is immutable and hash-pinned.** Registration requires a
  SHA-256 `content_hash`, non-empty provenance, `source_version` (default
  `v1`). `RegisterOrValidate` is idempotent only for byte-identical identity
  (key, type, title, author, problem slug, path, language, license, hash,
  version, canonical URL, provenance); any drift returns a conflict instead of
  mutating the registered source. The ingestion job re-validates the pinned
  hash at execution time (`validatePinnedSource`).
- **Publication is a hard gate, not a flag flip.**
  `ValidateKnowledgeUnitForPublication` enforces quality ≥ 0.90
  (`KnowledgePublicationMinQualityScore`), reviewed lifecycle/review status,
  content-hash freshness, external-evidence support whose license is not
  `rejected`, and source license not `rejected`. `PublishReviewedBatch`
  derives the unit set from an immutable reviewed snapshot and cross-checks
  content/claim/review/evidence/locator identity inside the publication
  transaction; drift aborts before any publication row exists.
- **Rollback is identity-checked.** `RollbackBatch` re-verifies the unit set,
  lifecycle/version, and content of every unit before writing the rollback
  publication; drift aborts.
- **Retrieval is published-only by default**, with the citation trust boundary
  and deterministic grounding/attribution checks described in the lifecycle
  doc §11.4–§11.6, and a deny-first observation gate (§11.5): identity,
  provenance, citation, or grounding failures recommend rollback; the gate
  never executes state changes itself.
- **Ingestion is video-only today.** There is no text/API source adapter in
  the Go ingestion service; the only non-video path that exists is the
  Thought Forest Markdown snapshot flow in Python plus the reviewed-artifact
  publish path. This is the single biggest structural gap for expansion (§6).

## 2. Verified in-repo baseline (examples, not production data)

Everything under `docs/knowledges/` is **example / evaluation material**
(its README states it holds reusable technical references and eval/evidence
inputs, not live data). Counts below were taken directly from the files.

### 2.1 `docs/knowledges/**` inventory

16 files: 5 narrative references (incl. the Issue #13 video pipeline doc and
manual ingestion runbook), 1 glossary, 1 testing guide, 1 AI-agent pipeline
reference, 4 eval datasets, 6 evidence artifacts.

### 2.2 Eval cohorts (`docs/knowledges/eval/`)

| Dataset | Cases | Shape |
| --- | --- | --- |
| `published-knowledge-pilot.jsonl` | 6 | 3 `hit` (pain / nociception definitions), 3 `no_result` (ankle sprain care, anterior pelvic tilt, off-topic PostgreSQL) |
| `published-knowledge-pain-nociception-cohort.jsonl` | 9 | 6 `hit`, 3 `no_result`; same negative set pattern |
| `…-cohort.thresholds.json` | — | Qualification gate: pass rate 1.0, ≥6 positive hits, ≥3 negative rejections, ≥6 citation-valid, ≥6 grounding-supported, publication identity required, dataset pinned by SHA-256 |
| `thought-forest-unpublished.jsonl` | 12 | Unpublished-need probes across 10 problem slugs (see §4) — these are *demand signals*, not coverage |

### 2.3 Evidence samples (`docs/knowledges/evidence/`)

| Artifact pair | Units | Claim decisions | External sources |
| --- | --- | --- | --- |
| pilot | 1 reviewed unit | 1 approved | 1 — IASP pain terminology page |
| pain-nociception cohort | 3 reviewed units | 3 approved | 2 — IASP terminology page + IASP revised pain definition announcement |

Both snapshots are pinned to Thought Forest commits (`4990c58…`, `8dbe766…`).
The lifecycle doc §11.7 additionally records an isolated GCP-dev PostgreSQL
vertical slice over a source snapshot of **11 notes / 115 units**, of which 3
were reviewed and published as the cohort, with rollback verified. That is a
*documented development-environment result*, not a repository artifact and
not production data.

### 2.4 What the baseline actually proves

- The full governance chain works end-to-end at **pilot scale**: 4 reviewed
  units total in-repo, all on one topic (pain / nociception definitions),
  backed by one publisher (IASP).
- The eval design is sound: positive exact-unit hits, negative rejection, and
  an unrelated-domain negative (`PostgreSQL 索引`) are all represented, and
  qualification thresholds are hash-pinned.
- Coverage breadth is effectively **one domain deep**. Every other clinical
  domain in §4 currently has zero reviewed units in the in-repo evidence.
  *(Inference, from §2.2–§2.3: the two `no_result` cases for ankle sprain and
  anterior pelvic tilt are deliberate negatives today, but they are also
  exactly the kind of questions users will ask first.)*

## 3. Explicit gap: numbers only an operator can produce

This environment has **no database or production access**, and Stage A does
not invent the missing numbers. The following baseline values are currently
**unknown** and must be produced by an operator with read-only access to each
environment (dev / staging / production separately) before Stage B sizes any
import:

| # | Missing number | Why it matters |
| --- | --- | --- |
| G1 | Registered sources per environment, by `source_type` and `license_status` | Tells whether the video pipeline or the Thought Forest path produced what exists, and how much content sits in non-ingestible license states |
| G2 | Units per environment, by `lifecycle_status` × `review_status` | The reviewed→published funnel; shows where content stalls |
| G3 | Published units and publication batches (count, latest version, rollback history) | The true live corpus size — the only "how much knowledge do we have" number that counts |
| G4 | Video-ingested sources vs Thought Forest sources actually present | Validates the §1 claim that ingestion is video-only *in practice*, not just in code |
| G5 | Publication observations: window size, hit / miss / grounding distribution per publication | Whether the §11.5 gate has a qualifying `continue` window or is still at `hold` (0 runtime samples) in production |
| G6 | Retrieval hit rate per environment on real traffic (not the eval cohorts) | Baseline for judging whether expansion improves real answers |
| G7 | Embedding provider/model actually configured per environment | The hashing-vs-semantic distinction changes retrieval policy (lifecycle doc §11.3); in-repo config defaults do not prove the deployed value |

Exact read-only queries (column names verified against
`apps/api/migrations/000001_vnext_baseline.up.sql` and the Go models):

```sql
-- G1
SELECT source_type, license_status, ingest_status, count(*)
FROM knowledge_sources GROUP BY 1, 2, 3 ORDER BY 1, 2, 3;

-- G2
SELECT lifecycle_status, review_status, count(*)
FROM knowledge_units GROUP BY 1, 2 ORDER BY 1, 2;

-- G3
SELECT status, count(*), sum(unit_count) FROM knowledge_publications
GROUP BY 1 ORDER BY 1;
SELECT publication_key, published_version, unit_count, published_at
FROM knowledge_publications ORDER BY published_at DESC LIMIT 10;

-- G5 (observation kinds are recorded per publication; see the
-- idx_knowledge_publication_observations_publication_kind_created index)
SELECT observation_kind, count(*) FROM knowledge_publication_observations
GROUP BY 1 ORDER BY 1;
```

Equivalent supported interfaces (preferred over raw SQL where they exist):
`GET /api/knowledge/stats` and `GET /api/knowledge/sources` on the AI service
(per-table counts and the source list, implemented in
`apps/ai-service/src/rag/knowledge_library.py`), and
`knowledge-publication-manager status` for publication state. Hit-rate style
numbers (G5/G6) come from `knowledge_publication_observations` and the
observation gate output (`observe-eval`), not from the stats endpoint.

**Stage A position:** until G1–G3 are reported per environment, all planning
uses the in-repo example baseline (§2) and treats live-corpus size as unknown,
not as zero and not as the pilot numbers.

## 4. Product knowledge-need matrix

Demand side is grounded in `docs/product`: Diagnosis holds named medical
concepts per user (domain/diagnosis.md), TrainingPlan/Trainer prescribe and
progress exercise (domain/training.md), SafetyState gates training on
structured risk screening — the shipped Go screening checks five signals
(trauma, radiating pain, numbness, weakness, dizziness; domain/safety.md
§2.4) — and BodyState records symptom episodes against the Body Region
Ontology. Frequency ranking below is *(inference)* from the product's core
loop (posture / pain self-report → diagnosis → training) plus the demand
signals already encoded in the eval probes; thinness is grounded in §2.

| # | Knowledge domain | Product anchor | Expected frequency | In-repo evidence today | Priority |
| --- | --- | --- | --- | --- | --- |
| N1 | Pain science & nociception basics | Ordinary consultation (requirements §2.1); every diagnosis explanation | High | **Only covered domain**: 4 reviewed units, IASP-backed, eval cohort + thresholds | P0 — maintain, extend carefully |
| N2 | Contraindications, red flags, referral advice | SafetyState screening (5 signals), scenario F risk blocking, `urgent` semantics | Lower frequency, highest harm if wrong | One unpublished probe (`musculoskeletal-safety`); safety doc explicitly says clinical thresholds/red-flag rules are **not yet validated** and need independent versioned, professionally reviewed policy | P0 — but governance-first, not volume-first (see §7) |
| N3 | Neck & shoulder | Body Region Ontology regions; scenario B (shoulder pain laterality) | High *(inference: desk-work posture product)* | None reviewed; lifecycle §11.2 example query set mentions shoulder self-test only | P1 |
| N4 | Thoracolumbar (low back) | Core posture loop; pelvic-tilt probes link here | High *(inference)* | None reviewed; 2 unpublished probes are pelvis/back-adjacent | P1 |
| N5 | Pelvis & hip | Thought Forest probes: anterior pelvic tilt ×2, gluteus medius, glute training | High (probes show anticipated demand) | 0 published — pilot eval expects `no_result` for 骨盆前倾是什么？ today | P1 |
| N6 | Knee & ankle | Probes: medial knee pain, IT band syndrome; pilot negative: ankle sprain care | Medium–high | None reviewed | P1 |
| N7 | Nerve-referral symptoms (radiating pain, numbness, weakness) | Safety screening signals; scenario D (new leg numbness changes plan advice) | Medium, safety-adjacent | None reviewed | P1 (with N2 review path) |
| N8 | Anatomy & kinesiology basics | Body Region Ontology, 3D anatomy assets, ROM probe | Medium — enabling layer for N3–N6 | Probe only (`range-of-motion-assessment`, `human-movement-biomechanics`, `movement-variability`) | P2 — small, stable, cheap to review well |
| N9 | Posture & function assessment | BodyState observations; self-test style queries | Medium | Probe only (ROM) | P2 |
| N10 | Exercise prescription & progressive loading | TrainingPlan/Trainer; probes: glute progression, resistance-training dose | High — every plan user hits this | 2 unpublished probes; no reviewed units | P1 |

Reading of the matrix: the corpus is inverted relative to demand. The one
covered domain (N1) is foundational but narrow; the domains users will ask
about most (N3–N6, N10) have **zero** reviewed coverage, and the
highest-risk domain (N2/N7) is deliberately gated behind professional review
that the product spec says does not exist yet. Stage B must therefore expand
breadth for low-risk explanatory content while keeping safety content on the
slow path (§7).

## 5. Reference architecture comparison

Four reference systems were compared. None is a drop-in for BodySense: each
solves a different layer of the problem (retrieval evaluation, terminology
governance, guideline computability, in-EHR rule execution).

### 5.1 MedRAG / MIRAGE (retrieval & evaluation reference)

What it is: a research toolkit + benchmark (Xiong et al., 2024) that
systematically varies corpora, retrievers, and LLMs for medical QA. Corpora:
PubMed (23.9M abstracts/snippets), StatPearls (9.3k articles → 301.2k
snippets, via NCBI Bookshelf), 18 medical textbooks (125.8k snippets),
Wikipedia (29.9M snippets), and the combined MedCorp (54.2M snippets);
retrievers include BM25 and MedCPT. MIRAGE holds 7,663 questions from five
medical QA datasets; RAG improved LLM accuracy by up to 18% in their setup.

- **Borrow:** the *evaluation discipline* — vary one retrieval variable at a
  time against a frozen question set, exactly like the hash-pinned thresholds
  file BodySense already has; and the snippet-with-provenance corpus shape
  (source, chunk, stable identity), which maps onto the existing unit +
  locator model.
- **Do not copy:** the corpora or the scale. MedCorp's 54.2M unreviewed
  snippets are the opposite of BodySense's reviewed-unit governance; exam-QA
  accuracy (MedQA-style) is not evidence of safe lay-user rehabilitation
  answers; there is no publication/rollback/license lifecycle at all.
- **Licensing traps:** MedRAG is a *benchmark distribution*, not a license
  grant. Its components have heterogeneous rights — PubMed abstracts are
  individually copyrighted by publishers/journals (NCBI distributes them;
  reuse terms vary by article), StatPearls content is NCBI Bookshelf material
  with its own terms, the 18 textbooks are commercial works, Wikipedia is
  CC BY-SA with share-alike consequences. **Copying any MedRAG corpus into
  BodySense without a per-source license check would fail the registry's own
  license gate** — every imported item must enter with one of the four
  registry license values, justified per source, or stay `citation_only`.

### 5.2 OpenMRS Concept Dictionary + Open Concept Lab (terminology governance reference)

What it is: OpenMRS (MPL-2.0 with healthcare disclaimer) is built on a
*concept dictionary* — every clinical data item is a governed concept, so new
content is metadata, not schema change. The CIEL dictionary (Columbia
International eHealth Laboratory) is the widely reused reference dictionary,
released on a near-monthly cadence. Open Concept Lab (OCL) is the terminology
service around it: versioned *sources* (repositories of concepts and
mappings), *collections* (curated subsets), subscriptions, and exports; it
hosts reference terminologies (e.g. ICD-10) and placeholders for external
systems it cannot host (e.g. SNOMED CT).

- **Borrow:** (1) the source/collection split — BodySense's
  `knowledge_sources` ≈ OCL sources, and a *published cohort* is effectively
  an OCL collection; Stage B adapters should think in those terms.
  (2) Versioned releases with subscription-style diffing as the model for
  keeping imported subsets current, instead of one-off dumps.
  (3) Mapping discipline: concepts carry typed mappings to external codes
  rather than embedding foreign identifiers as truth — BodySense already has
  the analogous stance in its Body Region Ontology (concept coding is
  separable from the medical name; diagnosis.md §3.0).
- **Do not copy:** the EMR data-model gravity (concepts as *recordable data
  items* for clinical documentation) — BodySense concepts serve retrieval and
  explanation, not charting; nor OCL as a runtime dependency.
- **Licensing traps:** CIEL is distributed under a community *trust model*
  (contribute-back expectations, terms set by Columbia), not a plain open
  license — reusing CIEL content wholesale needs its terms checked, not
  assumed. Terminologies OCL merely *references* keep their own regimes:
  SNOMED CT requires an Affiliate Licence — free in SNOMED International
  member territories and low-income countries, potentially fee-bearing
  elsewhere — so BodySense must not treat SNOMED CT as freely reusable for
  its deployment territories without a licence decision on file. ICD/LOINC
  have their own terms. A mapping *to* a code is lower-risk than reproducing the
  terminology's descriptions.

### 5.3 WHO SMART Guidelines (guideline computability reference)

What it is: WHO's framework (Standards-based, Machine-readable, Adaptive,
Requirements-based, Testable) for turning narrative guidelines into digital
systems through layers L1 narrative → L2 operational (Digital Adaptation
Kits: personas, data dictionary, decision logic, requirements, test
scenarios) → L3 machine-readable (FHIR R4 implementation guides, value sets,
CQL logic) → L4 executable reference software → L5 dynamic/AI-optimised.

- **Borrow:** the *layering discipline* for safety-relevant content: keep the
  narrative source (L1), an operational, software-neutral statement of the
  rule and its requirements (L2), and only then any computable form (L3).
  BodySense's safety domain already demands "independently versioned,
  professionally reviewed" red-flag policy — an L1→L2→L3 trail is exactly
  how to make such a policy auditable. Also borrow DAK-style **test
  scenarios as part of the content package**: BodySense eval cohorts already
  play this role and should ship alongside any safety content set.
- **Do not copy:** FHIR/CQL as a runtime stack. BodySense has no FHIR
  surface, no clinical-record exchange requirement, and a Go/Python contract
  boundary of its own; adopting FHIR to store rehabilitation knowledge would
  be architecture tourism. WHO DAKs also target national programme
  implementation (antenatal care, HIV, …), not MSK rehabilitation — there is
  no WHO SMART DAK to import for BodySense's domains.
- **Licensing traps:** WHO guideline content carries WHO copyright/terms
  (adaptation must preserve fidelity and attribution; it is not public
  domain), and SMART artifacts inherit both WHO terms and the terms of any
  terminology they bind (see §5.2 on SNOMED CT). "Standards-based" does not
  mean "rights-cleared".

### 5.4 OpenEMR Clinical Decision Rules (execution reference, brief)

What it is: OpenEMR (GPL) ships a Clinical Decision Rules engine inside the
EHR: configurable patient/physician reminders, clinical quality measure
calculation, and rule-driven alerts evaluated against live patient records.

- **Borrow:** one idea only — rules are *data with an evaluation/audit loop*
  (measures computed continuously, reminders logged), which parallels
  BodySense's publication-observation gate. If N2 safety content ever becomes
  computable, it should ship with that kind of measured feedback, not as
  static text alone.
- **Do not copy:** the engine or its coupling. OpenEMR rules execute inside a
  clinician-facing EHR against structured chart data; BodySense's safety
  gating is a product-level Go authority over user-reported facts
  (SafetyState), and its spec explicitly forbids the model or free text from
  deciding risk levels. GPL code reuse is also off the table for this
  codebase's licensing posture.
- **Licensing traps:** GPL copyleft for code; bundled rule content derives
  from US quality-measure programmes whose content terms are separate from
  the code license.

### 5.5 Comparison summary

| Reference | Layer it informs | Key borrow | Hard stop |
| --- | --- | --- | --- |
| MedRAG / MIRAGE | Retrieval evaluation | Frozen-set, one-variable evaluation; snippet provenance shape | No corpus import without per-source license adjudication |
| OpenMRS / OCL + CIEL | Terminology & source governance | Source/collection split, versioned releases, typed mappings | No EMR data model; CIEL/SNOMED terms are not "open by default" |
| WHO SMART Guidelines | Safety-content computability | L1→L2→L3 layering + test scenarios packaged with content | No FHIR/CQL adoption; no MSK DAK exists to import |
| OpenEMR CDR | Rule execution feedback | Rules-as-data with measured outcomes | No engine/code reuse (GPL, EHR-coupled) |

## 6. Options for expansion

### Option 1 — Extend the existing KB with per-source adapters (recommended)

Add narrow **source adapters** beside the existing pipelines, one per source
family, each emitting the artifacts the current governance already consumes
(registered source → snapshot/pack → claim + external-evidence review →
reviewed snapshot → `publish-reviewed`). Candidate Stage B source families,
chosen because their license posture can actually satisfy the registry:

| Source family | Access | License posture (checked 2026-11-03) | Registry mapping |
| --- | --- | --- | --- |
| IASP (already in use) | Public pages | Publisher content; current use is definition-level support with attribution | `citation_only` (support relations, not bulk copy) |
| Europe PMC / PMC Open Access subset | REST API, bulk OA subset | Per-article license is carried in the article record; only the OA subset is cleared for text mining/reuse, and each article's own CC terms govern | `verified_reuse` only for items whose record shows a reusable CC license; otherwise `citation_only` |
| PubMed (NCBI E-utilities) | API | Metadata/abstracts are retrievable programmatically, but abstracts remain publisher-copyrighted; free access ≠ reuse rights | `citation_only` by default |
| Thought Forest expansion (owned notes) | In-house authoring | Owned content, the existing pilot path | `owned` |

Pros: reuses every existing gate (license vocabulary, hash-pinned identity,
claim review, artifact-bound publication, rollback, observation gate);
incremental per domain; no new runtime dependency; failure of one adapter
does not affect retrieval. Cons: throughput is bounded by human claim review
— this is a deliberate property, and §7 sizes Stage B accordingly.
Explicitly **excluded** as bulk sources: NICE guidelines (UK open content
licence is UK-only; AI use requires a separate written licence and fee —
incompatible with unattended import), Cochrane reviews (paywalled,
copyrighted), commercial textbooks and JOSPT/APTA guideline full texts
unless a specific licence is obtained; these may be cited as external
evidence relations where the current review flow already supports that.

### Option 2 — External real-time query at answer time

Query Europe PMC / PubMed live during consultation and ground answers on
fresh abstracts.

Pros: always current; no local corpus to govern. Cons: breaks the system's
core contract — answers would rest on **unreviewed** material with no
publication identity, no claim review, no rollback, and no stable citation
(the exact failure modes the lifecycle was built to remove); adds latency,
rate-limit, and availability coupling to the consultation path; makes the
observation gate meaningless. Verdict: **rejected as a retrieval path.**
A restricted variant is acceptable: real-time query as an *authoring aid*
inside the review pipeline (candidate discovery for human reviewers), where
results still enter through Option 1's gates.

### Option 3 — Bulk import a large corpus (MedCorp-style)

Import tens of thousands+ of snippets and rely on retrieval ranking.

Pros: instant breadth. Cons: license adjudication at that scale is not
credible per-source (§5.1); quality ≥ 0.90 + human claim review cannot be
honestly applied; the eval cohorts are calibrated for exact-unit governance,
not corpus-scale ranking; a bad batch cannot be reviewed or rolled back
meaningfully. Verdict: **rejected.** Bulk *metadata* harvesting (titles,
abstracts, license fields) to prioritise what humans review is fine — that is
Option 1's discovery step, not an import.

## 7. Recommended minimal incremental architecture

Recommendation: **Option 1, one new adapter shape, no new platform.**

1. **Generalise the snapshot path, don't add a second lifecycle.** The
   Thought Forest flow (Markdown snapshot → claim candidates → external
   evidence review → claim review → reviewed snapshot → artifact-bound
   publish) already is the text-source lifecycle. Stage B adds *source
   adapters* that produce snapshot-shaped inputs from external families
   (Europe PMC OA items first), plus a `source_type` value for API-sourced
   text so the Go registry/ingestion side can register and pin them the same
   way it pins Thought Forest commits today (canonical URL + version +
   content hash are already registry fields).
2. **Keep Go as the only publication authority.** Adapters never write
   `knowledge_units` directly; they stop at reviewed-snapshot artifacts.
   Publication, rollback, and observation stay exactly as implemented.
3. **Expand by domain cohorts, mirroring the pain cohort pattern.** One
   domain = one reviewed cohort = one eval dataset + hash-pinned thresholds
   file + one publication. Suggested Stage B order from §4: N5/N6
   (pelvis/hip, knee/ankle — demand already probed, low-risk explanatory
   content), then N3/N4 (neck/shoulder, thoracolumbar), then N10/N8
   (exercise dosing, anatomy basics). The existing `no_result` eval cases for
   ankle sprain and anterior pelvic tilt flip to positive cases only when
   their cohort actually publishes — the eval change is part of the cohort's
   definition of done.
4. **Safety content (N2/N7) stays on the slow path.** No red-flag or referral
   content publishes until the product's own precondition exists: an
   independently versioned, professionally reviewed safety policy
   (docs/product/domain/safety.md §2.8). Stage B may prepare L1/L2 material
   (narrative + operational form, per §5.3) but must not publish it as
   retrieval knowledge before that review exists. This is a product gate,
   not an engineering preference.
5. **Terminology stays lightweight.** Reuse the OCL lesson (§5.2) as
   *practice*, not platform: problem slugs remain BodySense's concept keys;
   external codes (ICD etc.) are recorded as typed mapping metadata on the
   source/unit where a reviewer verified them, never as imported terminology
   content. No SNOMED CT content without a licence decision on file.

Rough cost / effort *(inference, engineering time only, excluding reviewer
availability)*: adapter for one API source family ≈ 1–2 weeks including
tests and registry changes; per-domain cohort ≈ 2–5 days of authoring +
review-cycle time for 10–30 units at the pain cohort's review depth;
eval/threshold updates ≈ 0.5 day per cohort. No infrastructure cost beyond
existing PostgreSQL/pgvector; Europe PMC/PubMed APIs are free under their
terms of use (rate limits apply). The bottleneck is qualified human review,
not engineering — Stage B planning must staff review before adapters.

## 8. Stage B entry criteria

Stage B (first expansion implementation) may start when **all** of the
following hold:

1. **Operator baseline delivered.** G1–G3 from §3 are reported for the
   target environment(s) and recorded on the parent issue, so live-corpus
   size is a fact, not an assumption. (G5–G7 may follow, but the first
   cohort's environment must at least report G7, its embedding provider.)
2. **First cohort chosen and bounded.** One domain from §7 step 3, a named
   problem-slug list, and a target unit range (suggested 10–30 units)
   recorded on the Stage B issue before any adapter code is written.
3. **License adjudication per source, in writing.** For the chosen source
   family: the registry license value it will use, the evidence for that
   value (license text/record field), and the `citation_only` fallback for
   everything else — recorded on the issue. No adjudication, no import.
4. **Reviewer assigned.** A named human reviewer for claim review and
   external-evidence review of the cohort (the existing gates require human
   decisions; an unstaffed pipeline is a backlog generator, not progress).
5. **Eval contract defined up front.** The cohort's positive/negative cases
   and thresholds file are drafted with the cohort, including which existing
   `no_result` cases (if any) intentionally flip to `hit`.
6. **Safety exclusion respected.** The first cohort contains no red-flag,
   referral, or contraindication content; if the chosen domain forces such
   content, it is split out and held to the §7 step 4 precondition.
7. **No regression to existing gates.** Adapter work is accepted only with
   tests showing the §1 governance properties still fail closed
   (license vocabulary, hash pinning, publication quality gate), following
   the repo's T3 small-step validation in AGENTS.md.

## References

In-repo: `docs/architecture/knowledge-lifecycle-architecture.md`;
`docs/knowledges/README.md` and the `eval/` + `evidence/` artifacts;
`apps/api/internal/service/knowledge_source_registry.go`,
`knowledge_ingestion_service.go`, `knowledge_publication_service.go`;
`apps/ai-service/src/rag/` and `src/api/routes/knowledge.py`;
`docs/product/domain/{diagnosis,training,safety,bodystate}.md`,
`docs/product/requirements.md`.

External (accessed 2026-11-03): Xiong et al., *Benchmarking
Retrieval-Augmented Generation for Medicine* (MedRAG/MIRAGE, arXiv
2402.13178) and the MedRAG corpus documentation; OpenMRS / Open Concept Lab
documentation (OCL sources & collections, CIEL distribution notes) and the
OpenMRS public licence summary; SNOMED CT licensing via NLM/SNOMED
International affiliate-licence terms; WHO SMART Guidelines framework and
SMART Base / DAK implementation guides (smart.who.int) and Mehl et al.,
*Lancet Digital Health* 2021; OpenEMR feature documentation (Clinical
Decision Rules); NCBI PMC developer guidance (API-only automated retrieval,
per-article license variance) and Europe PMC RESTful API documentation;
NICE terms, UK Open Content Licence, and AI-use permission policy.
