import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';

const root = process.cwd();
const ledgerDir = path.join(root, 'docs/learning/curriculum/ledger');
const load = (name) => JSON.parse(fs.readFileSync(path.join(ledgerDir, name), 'utf8'));
const fso = load('full-stack-open.json');
const tech = load('techschool-backend.json');
const agent = load('bodysense-agent.json');

const errors = [];
const fail = (message) => errors.push(message);
const lifecycleRank = new Map([
  ['SOURCE_INDEXED', 1],
  ['MAPPED', 2],
  ['EXERCISE_READY', 3],
  ['LEARNER_VERIFIED', 4],
]);
const allowedModes = new Set(['DIRECT', 'COMPARE', 'OPTIONAL']);
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
  const expectedCounts = {0:6,1:14,2:20,3:22,4:23,5:31,6:22,7:20,8:26,9:30,10:27,11:21};
  const exercises = fso.items.filter((item) => item.kind === 'exercise');
  if (exercises.length !== 262) fail(`FSO: expected 262 indexed exercises for Parts 0-11, got ${exercises.length}`);
  for (const [partText, count] of Object.entries(expectedCounts)) {
    const part = Number(partText);
    const rows = exercises.filter((item) => item.source.part === part);
    if (rows.length !== count) fail(`FSO Part ${part}: expected ${count} exercises, got ${rows.length}`);
    const numbers = new Set(rows.map((item) => item.source.number));
    if (numbers.size !== rows.length) fail(`FSO Part ${part}: duplicate source numbers`);
    if (part <= 7) {
      for (const item of rows) {
        if ((lifecycleRank.get(item.lifecycle) ?? 0) < lifecycleRank.get('MAPPED')) fail(`${item.id}: core Part ${part} must be at least MAPPED`);
        if (item.source.authority !== 'current-site-snapshot') fail(`${item.id}: Part ${part} should use current-site-snapshot authority`);
        if (item.mapping.semantic_audit !== 'reviewed-against-source-exercise') fail(`${item.id}: core mapping must record source exercise semantic review`);
      }
    } else {
      for (const item of rows) {
        if (item.source.authority !== 'historical-snapshot-current-mooc-unverified') fail(`${item.id}: Part ${part} must not claim current MOOC parity`);
      }
    }
  }
  if (fso.baseline.indexed_snapshot_commit !== '0711aef8a451c4458263e5587ccda85f08fd7a96') fail('FSO: unexpected indexed snapshot commit');
  const boundaries = new Map(fso.items.filter((item) => item.kind === 'source_boundary').map((item) => [item.source.part, item]));
  for (const part of [12,13,14]) {
    const item = boundaries.get(part);
    if (!item) fail(`FSO Part ${part}: missing current source boundary record`);
    else if (item.source.verification !== 'UNVERIFIED_CURRENT_MOOC') fail(`FSO Part ${part}: must remain explicitly UNVERIFIED_CURRENT_MOOC until source is obtained`);
  }
  const conceptIds = [
    'FSO-P2-CONCEPT-ASYNC-RUNTIME','FSO-P2-CONCEPT-PROMISES','FSO-P2-CONCEPT-EFFECTS',
    'FSO-P7-CONCEPT-USEMEMO','FSO-P7-CONCEPT-REACT-MEMO','FSO-P7-CONCEPT-USECALLBACK',
    'FSO-P7-CONCEPT-SQL-INJECTION','FSO-P7-CONCEPT-XSS','FSO-P7-CONCEPT-DEPENDENCY-SECURITY','FSO-P7-CONCEPT-BROKEN-AUTHZ'
  ];
  const ids = new Set(fso.items.map((item) => item.id));
  for (const id of conceptIds) if (!ids.has(id)) fail(`FSO: audited concept missing: ${id}`);
  if (!Array.isArray(fso.source_sections) || fso.source_sections.length !== 411) fail(`FSO: expected 411 indexed section headings from snapshot, got ${fso.source_sections?.length}`);
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
