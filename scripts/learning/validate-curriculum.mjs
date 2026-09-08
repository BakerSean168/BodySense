import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';

const root = process.cwd();
const ledgerDir = path.join(root, 'docs/learning/curriculum/ledger');
const load = (name) => JSON.parse(fs.readFileSync(path.join(ledgerDir, name), 'utf8'));
const fso = load('full-stack-open.json');
const tech = load('techschool-backend.json');
const agent = load('bodysense-agent.json');
const fsoConceptAudit = load('full-stack-open-concept-audit.json');
const fsoCoreSubheadingAudit = load('full-stack-open-core-subheading-audit.json');
const fsoCoreProseRiskAudit = load('full-stack-open-core-prose-risk-audit.json');

const errors = [];
const fail = (message) => errors.push(message);
const lifecycleRank = new Map([
  ['SOURCE_INDEXED', 1],
  ['MAPPED', 2],
  ['EXERCISE_READY', 3],
  ['LEARNER_VERIFIED', 4],
]);
const allowedModes = new Set(['DIRECT', 'COMPARE', 'OPTIONAL']);

function stableStringify(value) {
  if (Array.isArray(value)) return `[${value.map(stableStringify).join(',')}]`;
  if (value && typeof value === 'object') {
    return `{${Object.keys(value).sort().map((key) => `${JSON.stringify(key)}:${stableStringify(value[key])}`).join(',')}}`;
  }
  return JSON.stringify(value);
}

function sourceStateDigest(mooc) {
  const state = {
    schema_version: mooc.schema_version,
    platform: mooc.platform,
    organization: mooc.organization,
    api_base: mooc.api_base,
    courses: mooc.courses,
  };
  return crypto.createHash('sha256').update(stableStringify(state)).digest('hex');
}

const masteryRank = new Map([['L1',1],['L2',2],['L3',3],['L4',4],['L5',5]]);

function allItems() {
  return [...fso.items, ...tech.items, ...agent.items];
}

function validateUniqueIds() {
  const seen = new Set();
  for (const item of allItems()) {
    if (seen.has(item.id)) fail(`duplicate curriculum id: ${item.id}`);
    seen.add(item.id);
  }
}

function validateLifecycle(item, ledgerPath) {
  if (!lifecycleRank.has(item.lifecycle)) fail(`${item.id}: invalid lifecycle ${item.lifecycle}`);
  const rank = lifecycleRank.get(item.lifecycle) ?? 0;
  if (rank >= 2) {
    if (!item.mapping) return fail(`${item.id}: ${item.lifecycle} requires mapping`);
    if (!allowedModes.has(item.mapping.mode)) fail(`${item.id}: invalid mode ${item.mapping.mode}`);
    if (!item.mapping.exercise_id) fail(`${item.id}: mapped item requires exercise_id`);
    if (!item.mapping.objective) fail(`${item.id}: mapped item requires objective`);
    if (!item.mapping.task_summary) fail(`${item.id}: mapped item requires task_summary`);
    if (!['UNMODELED','REVIEWED'].includes(item.mapping.dependency_audit)) fail(`${item.id}: mapping requires dependency_audit`);
    if (rank < lifecycleRank.get('EXERCISE_READY') && item.mapping.dependency_audit === 'UNMODELED' && (item.mapping.prerequisites?.length ?? 0) > 0) fail(`${item.id}: unmodeled prerequisites must remain empty`);
  }
  if (rank >= 3) {
    const m = item.mapping;
    if (m.dependency_audit !== 'REVIEWED') fail(`${item.id}: EXERCISE_READY requires reviewed prerequisites`);
    if (!Array.isArray(m.target_files) || m.target_files.length === 0) fail(`${item.id}: EXERCISE_READY requires target_files`);
    if (!Array.isArray(m.completion_evidence_plan) || m.completion_evidence_plan.length === 0) fail(`${item.id}: EXERCISE_READY requires completion_evidence_plan`);
    if (!m.exercise_card) {
      fail(`${item.id}: EXERCISE_READY requires exercise_card`);
    } else {
      const cardPath = path.resolve(path.dirname(ledgerPath), m.exercise_card);
      if (!fs.existsSync(cardPath)) fail(`${item.id}: exercise card not found: ${cardPath}`);
      else {
        const card = fs.readFileSync(cardPath, 'utf8');
        for (const heading of ['## Concept','## Prerequisites','## BodySense target files','## Prediction before reading/running','## Task','## Failure case','## Verification command / evidence','## Explain-back questions','## Production change','## L4 acceptance']) {
          if (!card.includes(heading)) fail(`${item.id}: exercise card missing ${heading}`);
        }
      }
    }
    for (const target of m.target_files) {
      const targetPath = path.join(root, target);
      if (!fs.existsSync(targetPath)) fail(`${item.id}: target path does not exist: ${target}`);
    }
  }
  if (rank >= 4) {
    if (!item.mastery?.current) fail(`${item.id}: LEARNER_VERIFIED requires current mastery`);
    const current = masteryRank.get(item.mastery?.current) ?? 0;
    const required = masteryRank.get(item.mastery?.required) ?? 999;
    if (current < required) fail(`${item.id}: current mastery ${item.mastery?.current} below required ${item.mastery?.required}`);
    if (!Array.isArray(item.mastery?.evidence) || item.mastery.evidence.length === 0) fail(`${item.id}: LEARNER_VERIFIED requires evidence`);
  }
}

function validatePrerequisites() {
  const ids = new Set(allItems().map((item) => item.id));
  for (const item of allItems()) {
    const prereqs = item.mapping?.prerequisites ?? [];
    for (const prereq of prereqs) {
      if (!ids.has(prereq)) fail(`${item.id}: unknown prerequisite ${prereq}`);
      if (prereq === item.id) fail(`${item.id}: cannot depend on itself`);
    }
  }
}


function validateDependencyGraph() {
  const items = new Map(allItems().map((item) => [item.id, item]));
  const visiting = new Set();
  const visited = new Set();
  function dfs(id, chain=[]) {
    if (visited.has(id)) return;
    if (visiting.has(id)) {
      fail(`dependency cycle: ${[...chain, id].join(' -> ')}`);
      return;
    }
    visiting.add(id);
    const item = items.get(id);
    for (const prereq of item?.mapping?.prerequisites ?? []) dfs(prereq, [...chain, id]);
    visiting.delete(id);
    visited.add(id);
  }
  for (const id of items.keys()) dfs(id);

  // An exercise-ready item is not truly executable if one of its declared
  // prerequisites is still only indexed/mapped. Keep the ready spine closed.
  for (const item of items.values()) {
    if ((lifecycleRank.get(item.lifecycle) ?? 0) < lifecycleRank.get('EXERCISE_READY')) continue;
    for (const prereqId of item.mapping?.prerequisites ?? []) {
      const prereq = items.get(prereqId);
      if ((lifecycleRank.get(prereq?.lifecycle) ?? 0) < lifecycleRank.get('EXERCISE_READY')) {
        fail(`${item.id}: ready item depends on non-ready prerequisite ${prereqId} (${prereq?.lifecycle})`);
      }
    }
  }
}

function validateFSO() {
  const expectedCurrentCounts = {0:6,1:14,2:20,3:22,4:23,5:31,6:22,7:20,8:30,9:35,10:30,11:24,12:25,13:28,14:26};
  const expectedHistoricalAdvancedCounts = {8:26,9:30,10:27,11:21};
  const exercises = fso.items.filter((item) => item.kind === 'exercise');
  if (exercises.length !== 356) fail(`FSO: expected 356 active current-source exercise records for Parts 0-14, got ${exercises.length}`);

  for (const [partText, count] of Object.entries(expectedCurrentCounts)) {
    const part = Number(partText);
    const rows = exercises.filter((item) => item.source.part === part);
    if (rows.length !== count) fail(`FSO Part ${part}: expected ${count} active exercise records, got ${rows.length}`);
    const ids = new Set(rows.map((item) => item.source.platform_exercise_id ?? item.source.number ?? item.id));
    if (ids.size !== rows.length) fail(`FSO Part ${part}: duplicate source exercise identities`);

    if (part <= 7) {
      for (const item of rows) {
        if ((lifecycleRank.get(item.lifecycle) ?? 0) < lifecycleRank.get('MAPPED')) fail(`${item.id}: core Part ${part} must be at least MAPPED`);
        if (item.source.authority !== 'current-site-snapshot') fail(`${item.id}: Part ${part} should use current-site-snapshot authority`);
        if (item.mapping.semantic_audit !== 'reviewed-against-source-exercise') fail(`${item.id}: core mapping must record source exercise semantic review`);
      }
    } else {
      for (const item of rows) {
        if (item.source.authority !== 'current-mooc-api') fail(`${item.id}: Part ${part} must use current-mooc-api authority`);
        if (item.source.verification !== 'VERIFIED_CURRENT_MOOC_API_INDEX') fail(`${item.id}: Part ${part} must record verified current MOOC API indexing`);
        if (item.source.source_snapshot_sha256 !== fso.baseline.current_mooc?.source_state_sha256) fail(`${item.id}: Part ${part} source digest differs from current MOOC baseline`);
        if ((lifecycleRank.get(item.lifecycle) ?? 0) < lifecycleRank.get('MAPPED')) fail(`${item.id}: current MOOC Part ${part} must be at least MAPPED after semantic audit`);
        if (item.mapping?.semantic_audit !== 'reviewed-against-current-mooc-exercise') fail(`${item.id}: current MOOC mapping must record semantic review`);
        if (item.mapping?.dependency_audit !== 'UNMODELED' && (lifecycleRank.get(item.lifecycle) ?? 0) < lifecycleRank.get('EXERCISE_READY')) fail(`${item.id}: mapped current MOOC dependency graph must remain UNMODELED until ready`);
      }
    }
  }

  if (fso.baseline.indexed_snapshot_commit !== '0711aef8a451c4458263e5587ccda85f08fd7a96') fail('FSO: unexpected indexed core snapshot commit');
  if (!fso.baseline.current_mooc?.source_state_sha256) fail('FSO: current MOOC source digest missing');
  if (fso.baseline.current_mooc?.semantic_mapping?.exercise_records_mapped !== 198) fail('FSO: current MOOC semantic mapping baseline must record 198 mapped exercise records');

  const moocIndexPath = path.join(ledgerDir, 'full-stack-open-current-mooc.json');
  if (!fs.existsSync(moocIndexPath)) {
    fail('FSO: current MOOC metadata snapshot file missing');
  } else {
    const mooc = JSON.parse(fs.readFileSync(moocIndexPath, 'utf8'));
    if (mooc.source_state_sha256 !== sourceStateDigest(mooc)) fail('FSO: MOOC metadata snapshot content does not match its source_state_sha256');
    if (mooc.source_state_sha256 !== fso.baseline.current_mooc?.source_state_sha256) fail('FSO: MOOC snapshot digest does not match ledger baseline');
    const moocExercises = mooc.courses.flatMap((course) => course.exercises ?? []);
    if (moocExercises.length !== 198) fail(`FSO: expected 198 current MOOC exercise records for Parts 8-14, got ${moocExercises.length}`);
    for (const course of mooc.courses) {
      const expected = expectedCurrentCounts[course.part];
      if (!expected) fail(`FSO: unexpected current MOOC part ${course.part}`);
      if ((course.exercises ?? []).length !== expected) fail(`FSO Part ${course.part}: MOOC index expected ${expected}, got ${(course.exercises ?? []).length}`);
    }
  }

  const historical = fso.historical_items ?? [];
  if (historical.length !== 104) fail(`FSO: expected 104 archived snapshot exercises for historical Parts 8-11, got ${historical.length}`);
  for (const [partText, count] of Object.entries(expectedHistoricalAdvancedCounts)) {
    const part = Number(partText);
    const rows = historical.filter((item) => item.source?.part === part && item.kind === 'exercise');
    if (rows.length !== count) fail(`FSO historical Part ${part}: expected ${count}, got ${rows.length}`);
  }

  const activeSectionIds = new Set([...(fso.source_sections ?? []), ...(fso.current_mooc_sections ?? [])].map((item) => item.id));
  for (const concept of fso.items.filter((item) => item.kind === 'concept')) {
    for (const sectionId of concept.source?.section_ids ?? []) {
      if (!activeSectionIds.has(sectionId)) fail(`${concept.id}: references unknown/inactive source section ${sectionId}`);
    }
  }

  const conceptIds = [
    'FSO-P2-CONCEPT-ASYNC-RUNTIME','FSO-P2-CONCEPT-PROMISES','FSO-P2-CONCEPT-EFFECTS',
    'FSO-P7-CONCEPT-USEMEMO','FSO-P7-CONCEPT-REACT-MEMO','FSO-P7-CONCEPT-USECALLBACK',
    'FSO-P7-CONCEPT-SQL-INJECTION','FSO-P7-CONCEPT-XSS','FSO-P7-CONCEPT-DEPENDENCY-SECURITY','FSO-P7-CONCEPT-BROKEN-AUTHZ'
  ];
  const ids = new Set(fso.items.map((item) => item.id));
  for (const id of conceptIds) if (!ids.has(id)) fail(`FSO: audited concept missing: ${id}`);

  if (!Array.isArray(fso.source_sections) || fso.source_sections.length !== 279) fail(`FSO: expected 279 active core section headings for Parts 0-7, got ${fso.source_sections?.length}`);
  if (!Array.isArray(fso.current_mooc_sections) || fso.current_mooc_sections.length !== 385) fail(`FSO: expected 385 current MOOC section headings for Parts 8-14, got ${fso.current_mooc_sections?.length}`);
  if (!Array.isArray(fso.historical_source_sections) || fso.historical_source_sections.length !== 132) fail(`FSO: expected 132 archived section headings for historical Parts 8-11, got ${fso.historical_source_sections?.length}`);
}


function validateFSOConceptAudit() {
  const activeSections = [...(fso.source_sections ?? []), ...(fso.current_mooc_sections ?? [])];
  const activeIds = new Set(activeSections.map((item) => item.id));
  const auditRows = fsoConceptAudit.sections ?? [];
  const auditIds = new Set(auditRows.map((item) => item.section_id));
  if (auditRows.length !== activeSections.length) fail(`FSO concept audit: expected ${activeSections.length} rows, got ${auditRows.length}`);
  if (auditIds.size !== auditRows.length) fail('FSO concept audit: duplicate section_id');
  for (const id of activeIds) if (!auditIds.has(id)) fail(`FSO concept audit: missing active section ${id}`);
  for (const id of auditIds) if (!activeIds.has(id)) fail(`FSO concept audit: stale/inactive section ${id}`);

  if (fsoConceptAudit.active_source_fingerprint?.core_snapshot_commit !== fso.baseline.indexed_snapshot_commit) fail('FSO concept audit: core snapshot fingerprint mismatch');
  if (fsoConceptAudit.active_source_fingerprint?.current_mooc_source_state_sha256 !== fso.baseline.current_mooc?.source_state_sha256) fail('FSO concept audit: current MOOC fingerprint mismatch');

  const conceptById = new Map(fso.items.filter((item) => item.kind === 'concept').map((item) => [item.id, item]));
  const allowed = new Set(['PENDING','REVIEWED_CONCEPTS_MAPPED','REVIEWED_NON_ENGINEERING','REVIEWED_REDUNDANT']);
  const pendingRows = auditRows.filter((row) => row.status === 'PENDING');
  if (fso.baseline.semantic_section_audit_checked_at && pendingRows.length > 0) {
    fail(`FSO concept audit: baseline claims completed section audit at ${fso.baseline.semantic_section_audit_checked_at}, but ${pendingRows.length} rows are PENDING`);
  }
  if (fso.baseline.current_mooc?.semantic_mapping?.section_units_dispositioned !== 385) {
    fail('FSO concept audit: current MOOC baseline must record 385 dispositioned section units');
  }
  for (const row of auditRows) {
    if (!allowed.has(row.status)) fail(`FSO concept audit ${row.section_id}: invalid status ${row.status}`);
    const linked = row.linked_concept_ids ?? [];
    if (row.status === 'REVIEWED_CONCEPTS_MAPPED' && linked.length === 0) fail(`FSO concept audit ${row.section_id}: mapped status requires concept links`);
    if (row.status === 'PENDING' && row.reviewed_at) fail(`FSO concept audit ${row.section_id}: pending row must not have reviewed_at`);
    for (const conceptId of linked) {
      const concept = conceptById.get(conceptId);
      if (!concept) fail(`FSO concept audit ${row.section_id}: unknown linked concept ${conceptId}`);
      if (!(concept?.source?.section_ids ?? []).includes(row.section_id)) fail(`FSO concept audit ${row.section_id}: concept ${conceptId} lacks reciprocal section link`);
    }
  }
}


function validateFSOCoreSubheadingAudit() {
  const audit = fsoCoreSubheadingAudit;
  const rows = audit.rows ?? [];
  const baseline = fso.baseline.core_subheading_audit;
  if (!baseline) {
    fail('FSO core subheading audit: missing baseline metadata');
    return;
  }
  if (audit.source?.snapshot_commit !== fso.baseline.indexed_snapshot_commit) fail('FSO core subheading audit: pinned snapshot commit mismatch');
  if (baseline.snapshot_commit !== audit.source?.snapshot_commit) fail('FSO core subheading audit: baseline snapshot mismatch');
  if (baseline.source_state_sha256 !== audit.source?.source_state_sha256) fail('FSO core subheading audit: source-state fingerprint mismatch');
  if (baseline.rows !== rows.length) fail(`FSO core subheading audit: baseline rows ${baseline.rows} != ${rows.length}`);
  if (rows.length !== 196) fail(`FSO core subheading audit: expected 196 pinned h4-h6 rows, got ${rows.length}`);
  const ids = new Set(rows.map((row) => row.id));
  if (ids.size !== rows.length) fail('FSO core subheading audit: duplicate row id');
  const allowed = new Set(['PENDING','COVERED_BY_EXERCISE','ALTERNATIVE_EXERCISE_VARIANT','REVIEWED_CONCEPTS_MAPPED','REVIEWED_REDUNDANT','REVIEWED_NON_ENGINEERING','REMOVED_SOURCE_TRACK']);
  const itemById = new Map(fso.items.map((item) => [item.id, item]));
  let dispositioned = 0;
  for (const row of rows) {
    if (!allowed.has(row.status)) fail(`FSO core subheading audit ${row.id}: invalid status ${row.status}`);
    if (row.source?.snapshot_commit !== fso.baseline.indexed_snapshot_commit) fail(`FSO core subheading audit ${row.id}: source commit mismatch`);
    if (row.source?.level < 4 || row.source?.level > 6) fail(`FSO core subheading audit ${row.id}: level must be 4..6`);
    const linked = row.linked_item_ids ?? [];
    if (row.status !== 'PENDING') dispositioned += 1;
    if (row.status === 'PENDING' && row.reviewed_at) fail(`FSO core subheading audit ${row.id}: pending row must not have reviewed_at`);
    if (['COVERED_BY_EXERCISE','ALTERNATIVE_EXERCISE_VARIANT','REVIEWED_CONCEPTS_MAPPED','REVIEWED_REDUNDANT'].includes(row.status) && linked.length === 0) {
      fail(`FSO core subheading audit ${row.id}: ${row.status} requires linked items`);
    }
    let exerciseLinks = 0;
    let conceptLinks = 0;
    for (const linkedId of linked) {
      const item = itemById.get(linkedId);
      if (!item) {
        fail(`FSO core subheading audit ${row.id}: unknown linked item ${linkedId}`);
        continue;
      }
      if (item.kind === 'exercise') exerciseLinks += 1;
      if (item.kind === 'concept') conceptLinks += 1;
    }
    if (row.status === 'COVERED_BY_EXERCISE' && exerciseLinks === 0) fail(`FSO core subheading audit ${row.id}: covered-by-exercise requires exercise link`);
    if (row.status === 'ALTERNATIVE_EXERCISE_VARIANT' && (exerciseLinks === 0 || conceptLinks === 0)) fail(`FSO core subheading audit ${row.id}: alternative variant requires exercise + concept links`);
    if (row.status === 'REVIEWED_CONCEPTS_MAPPED' && conceptLinks === 0) fail(`FSO core subheading audit ${row.id}: concept-mapped requires concept link`);
    if (['REVIEWED_NON_ENGINEERING','REMOVED_SOURCE_TRACK'].includes(row.status) && linked.length > 0) fail(`FSO core subheading audit ${row.id}: ${row.status} should not claim active linked coverage`);
  }
  if (baseline.dispositioned !== dispositioned) fail(`FSO core subheading audit: baseline dispositioned ${baseline.dispositioned} != ${dispositioned}`);
  if (baseline.reviewed_at && dispositioned !== rows.length) fail(`FSO core subheading audit: baseline claims reviewed at ${baseline.reviewed_at} but ${rows.length - dispositioned} rows remain pending`);
}

function validateFSOCoreProseRiskAudit() {
  const audit = fsoCoreProseRiskAudit;
  const rows = audit.rows ?? [];
  const baseline = fso.baseline.core_prose_risk_audit;
  if (!baseline) {
    fail('FSO core prose-risk audit: missing baseline metadata');
    return;
  }
  if (audit.source?.snapshot_commit !== fso.baseline.indexed_snapshot_commit) fail('FSO core prose-risk audit: pinned snapshot commit mismatch');
  if (baseline.snapshot_commit !== audit.source?.snapshot_commit) fail('FSO core prose-risk audit: baseline snapshot mismatch');
  const state = rows.map((row) => ({ section_id: row.section_id, source_fragment_sha256: row.source_fragment_sha256 }));
  const calculatedState = crypto.createHash('sha256').update(JSON.stringify(state)).digest('hex');
  if (audit.source?.source_state_sha256 !== calculatedState) fail('FSO core prose-risk audit: row fingerprints do not match source-state hash');
  if (baseline.source_state_sha256 !== audit.source?.source_state_sha256) fail('FSO core prose-risk audit: baseline source-state fingerprint mismatch');
  if (baseline.rows !== rows.length) fail(`FSO core prose-risk audit: baseline rows ${baseline.rows} != ${rows.length}`);
  const rowIds = new Set(rows.map((row) => row.section_id));
  if (rowIds.size !== rows.length) fail('FSO core prose-risk audit: duplicate section_id');
  const coreSections = new Map((fso.source_sections ?? []).map((section) => [section.id, section]));
  const conceptById = new Map(fso.items.filter((item) => item.kind === 'concept').map((item) => [item.id, item]));
  let dispositioned = 0;
  let newlyExposed = 0;
  for (const row of rows) {
    if (!['PENDING','REVIEWED'].includes(row.status)) fail(`FSO core prose-risk audit ${row.section_id}: invalid status ${row.status}`);
    const section = coreSections.get(row.section_id);
    if (!section) fail(`FSO core prose-risk audit ${row.section_id}: unknown/non-core section`);
    if (row.source?.snapshot_commit !== fso.baseline.indexed_snapshot_commit) fail(`FSO core prose-risk audit ${row.section_id}: source commit mismatch`);
    if (!/^[a-f0-9]{64}$/.test(row.source_fragment_sha256 ?? '')) fail(`FSO core prose-risk audit ${row.section_id}: invalid fragment sha256`);
    if (row.status === 'PENDING') {
      if (row.reviewed_at) fail(`FSO core prose-risk audit ${row.section_id}: pending row must not have reviewed_at`);
      continue;
    }
    dispositioned += 1;
    if (!row.reviewed_at || !row.note) fail(`FSO core prose-risk audit ${row.section_id}: reviewed row requires reviewed_at and note`);
    const linked = row.linked_concept_ids ?? [];
    if (linked.length === 0) fail(`FSO core prose-risk audit ${row.section_id}: reviewed row requires linked concepts`);
    for (const conceptId of linked) {
      const concept = conceptById.get(conceptId);
      if (!concept) {
        fail(`FSO core prose-risk audit ${row.section_id}: unknown concept ${conceptId}`);
        continue;
      }
      if (!(concept.source?.section_ids ?? []).includes(row.section_id)) fail(`FSO core prose-risk audit ${row.section_id}: concept ${conceptId} lacks reciprocal section link`);
    }
    for (const conceptId of row.newly_exposed_concept_ids ?? []) {
      newlyExposed += 1;
      if (!linked.includes(conceptId)) fail(`FSO core prose-risk audit ${row.section_id}: newly exposed ${conceptId} must also be linked`);
      const concept = conceptById.get(conceptId);
      if (concept?.mapping?.semantic_audit !== 'reviewed-against-source-prose-risk') fail(`FSO core prose-risk audit ${row.section_id}: new concept ${conceptId} must record prose-risk semantic audit`);
    }
  }
  if (baseline.dispositioned !== dispositioned) fail(`FSO core prose-risk audit: baseline dispositioned ${baseline.dispositioned} != ${dispositioned}`);
  if (baseline.newly_exposed_concepts !== newlyExposed) fail(`FSO core prose-risk audit: baseline new concepts ${baseline.newly_exposed_concepts} != ${newlyExposed}`);
  if (baseline.reviewed_at && dispositioned !== rows.length) fail(`FSO core prose-risk audit: baseline claims reviewed but ${rows.length - dispositioned} rows remain pending`);
}

function validateTech() {
  if (tech.baseline.public_repo_commit !== '97f000fe58ad01a0774179ffa8884ac7784cf263') fail('TECH: unexpected public README baseline commit');
  const lectures = tech.items.filter((item) => item.kind === 'lecture');
  if (lectures.length !== 78) fail(`TECH: expected 78 lectures, got ${lectures.length}`);
  const nums = lectures.map((item) => item.source.lecture).sort((a,b)=>a-b);
  for (let i=0;i<78;i++) if (nums[i] !== i) fail(`TECH: lecture index is not exactly 0..77 at position ${i}`);
  for (const item of lectures) {
    if (item.source.verification !== 'VERIFIED_PUBLIC_README_TITLE_ONLY') fail(`${item.id}: must not imply video-internal audit`);
    if ((lifecycleRank.get(item.lifecycle) ?? 0) < lifecycleRank.get('MAPPED')) fail(`${item.id}: public lecture should have title-level BodySense mapping`);
  }
}

function validateAgent() {
  if (agent.items.length !== 8) fail(`Agent extension: expected 8 items, got ${agent.items.length}`);
  for (const item of agent.items) if ((lifecycleRank.get(item.lifecycle) ?? 0) < 2) fail(`${item.id}: Agent extension must be mapped`);
}

validateUniqueIds();
validateFSO();
validateFSOConceptAudit();
validateFSOCoreSubheadingAudit();
validateFSOCoreProseRiskAudit();
validateTech();
validateAgent();
for (const [ledger, filename] of [[fso,'full-stack-open.json'],[tech,'techschool-backend.json'],[agent,'bodysense-agent.json']]) {
  const ledgerPath = path.join(ledgerDir, filename);
  for (const item of ledger.items) validateLifecycle(item, ledgerPath);
}
validatePrerequisites();
validateDependencyGraph();

if (errors.length) {
  console.error(`Curriculum validation failed with ${errors.length} issue(s):`);
  for (const error of errors) console.error(`- ${error}`);
  process.exit(1);
}

function counts(items) {
  const out = {SOURCE_INDEXED:0,MAPPED:0,EXERCISE_READY:0,LEARNER_VERIFIED:0};
  for (const item of items) out[item.lifecycle] = (out[item.lifecycle] ?? 0) + 1;
  return out;
}
console.log('Curriculum validation OK');
console.log('FSO', counts(fso.items));
console.log('TECH', counts(tech.items));
console.log('Agent', counts(agent.items));
