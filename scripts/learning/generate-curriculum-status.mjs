import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';

const root = process.cwd();
const ledgerDir = path.join(root, 'docs/learning/curriculum/ledger');
const viewPath = path.join(root, 'docs/learning/curriculum/views/coverage-status.md');
const load = (name) => JSON.parse(fs.readFileSync(path.join(ledgerDir, name), 'utf8'));
const fso = load('full-stack-open.json');
const tech = load('techschool-backend.json');
const agent = load('bodysense-agent.json');

const rank = {SOURCE_INDEXED:1,MAPPED:2,EXERCISE_READY:3,LEARNER_VERIFIED:4};
const atLeast = (items, state) => items.filter((item) => rank[item.lifecycle] >= rank[state]).length;
const exact = (items, state) => items.filter((item) => item.lifecycle === state).length;
const fsoExercises = fso.items.filter((item) => item.kind === 'exercise');
const currentCore = fsoExercises.filter((item) => item.source.part <= 7);
const historicalTransfer = fsoExercises.filter((item) => item.source.part >= 8 && item.source.part <= 11);
const fsoConcepts = fso.items.filter((item) => item.kind === 'concept');
const techLectures = tech.items.filter((item) => item.kind === 'lecture');
const all = [...fso.items, ...tech.items, ...agent.items];
const ready = all.filter((item) => item.lifecycle === 'EXERCISE_READY' || item.lifecycle === 'LEARNER_VERIFIED');
const verified = all.filter((item) => item.lifecycle === 'LEARNER_VERIFIED');

const partRows = [];
for (let part=0; part<=11; part++) {
  const rows=fsoExercises.filter((item)=>item.source.part===part);
  const authority=part<=7?'current-site snapshot':'historical snapshot; current MOOC pending';
  partRows.push(`| ${part} | ${rows.length} | ${atLeast(rows,'MAPPED')} | ${atLeast(rows,'EXERCISE_READY')} | ${atLeast(rows,'LEARNER_VERIFIED')} | ${authority} |`);
}

const md = `# BodySense Master Course · Coverage Status\n\n> Generated from machine-readable ledgers. Do not hand-edit counts in this file.\n> Baseline date: 2026-09-07\n\n## What the numbers mean\n\nLifecycle is monotonic:\n\n\`SOURCE_INDEXED -> MAPPED -> EXERCISE_READY -> LEARNER_VERIFIED\`\n\n- **SOURCE_INDEXED**: source item has a stable identifier/location, but no coverage claim beyond that source inventory.\n- **MAPPED**: source objective has a BodySense DIRECT / COMPARE / OPTIONAL mapping.\n- **EXERCISE_READY**: the exercise has concrete targets, prediction, task, failure case, verification evidence and an L4 gate.\n- **LEARNER_VERIFIED**: the learner has actually met the required mastery level with recorded evidence.\n\nThese states intentionally separate source indexing, curriculum design, executable practice and actual learning.\n\n## Full Stack Open\n\nSnapshot exercise inventory for Parts 0-11: **${fsoExercises.length}** distinct numbered exercises.\n\n| Part | Indexed exercises | Mapped | Exercise ready | Learner verified | Source authority |\n|---:|---:|---:|---:|---:|---|\n${partRows.join('\n')}\n\nCurrent-site core (Parts 0-7): **${currentCore.length}/${currentCore.length} source exercises semantically mapped**. This is mapping coverage, not completed learning.\n\nParts 8-11: **${historicalTransfer.length} historical exercise IDs indexed** from commit \`${fso.baseline.indexed_snapshot_commit}\`; current MOOC parity remains unverified and therefore is not reported as current-course coverage.\n\nParts 12-14: current MOOC source is explicitly recorded as **UNVERIFIED_CURRENT_MOOC** until a reproducible source snapshot is obtained.\n\nConcept inventory: **${fso.source_sections.length} source section headings indexed** from the snapshot; **${fsoConcepts.length} high-risk/previously-missed concepts semantically decomposed and mapped** (async runtime, Promise, Effect lifecycle, memoization/reference stability, injection, XSS, dependency security, broken auth/access control). Exhaustive paragraph-level concept parity is **not yet claimed**.\n\n## TECH SCHOOL Backend Master Class\n\nPublic README baseline: \`${tech.baseline.public_repo_commit}\`.\n\n- Public lecture IDs/titles indexed: **${techLectures.length}/78**.\n- Title-level BodySense mappings: **${atLeast(techLectures,'MAPPED')}/78**.\n- Exercise-ready lecture reps: **${atLeast(techLectures,'EXERCISE_READY')}/78**.\n- Learner-verified lecture reps: **${atLeast(techLectures,'LEARNER_VERIFIED')}/78**.\n\nThis does **not** claim that paid/video-internal teaching semantics were audited. The source authority is the public README title/index only.\n\n## BodySense Agent extension\n\n- Project-defined modules mapped: **${atLeast(agent.items,'MAPPED')}/${agent.items.length}**.\n- Exercise-ready: **${atLeast(agent.items,'EXERCISE_READY')}/${agent.items.length}**.\n- Learner-verified: **${atLeast(agent.items,'LEARNER_VERIFIED')}/${agent.items.length}**.\n\n## Exercise-ready spine\n\nThere are currently **${ready.length}** executable cards and **${verified.length}** learner-verified cards.\n\n${ready.map((item)=>`- \`${item.mapping.exercise_id}\` -> [card](${item.mapping.exercise_card})`).join('\n')}\n\nThe next implementation milestone is to expand **EXERCISE_READY** coverage from this spine while continuing the current-MOOC source audit. Placement assessment starts only after the prerequisite slice being assessed is exercise-ready.\n`;

fs.mkdirSync(path.dirname(viewPath), {recursive:true});
fs.writeFileSync(viewPath, md);
console.log(`wrote ${path.relative(root, viewPath)}`);
