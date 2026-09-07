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
const mooc = load('full-stack-open-current-mooc.json');

const rank = { SOURCE_INDEXED: 1, MAPPED: 2, EXERCISE_READY: 3, LEARNER_VERIFIED: 4 };
const atLeast = (items, state) => items.filter((item) => rank[item.lifecycle] >= rank[state]).length;
const fsoExercises = fso.items.filter((item) => item.kind === 'exercise');
const core = fsoExercises.filter((item) => item.source.part <= 7);
const currentMoocExercises = fsoExercises.filter((item) => item.source.part >= 8 && item.source.part <= 14);
const fsoConcepts = fso.items.filter((item) => item.kind === 'concept');
const techLectures = tech.items.filter((item) => item.kind === 'lecture');
const all = [...fso.items, ...tech.items, ...agent.items];
const ready = all.filter((item) => ['EXERCISE_READY', 'LEARNER_VERIFIED'].includes(item.lifecycle));
const verified = all.filter((item) => item.lifecycle === 'LEARNER_VERIFIED');

const lines = [
  '# BodySense Master Course · Coverage Status',
  '',
  '> Generated from machine-readable ledgers. Do not hand-edit counts in this file.',
  '> Baseline date: 2026-09-07',
  '',
  '## What the numbers mean',
  '',
  'Lifecycle is monotonic:',
  '',
  '`SOURCE_INDEXED -> MAPPED -> EXERCISE_READY -> LEARNER_VERIFIED`',
  '',
  '- **SOURCE_INDEXED**: source item has a stable identifier/location, but no coverage claim beyond source inventory.',
  '- **MAPPED**: source objective has a BodySense DIRECT / COMPARE / OPTIONAL mapping.',
  '- **EXERCISE_READY**: the exercise has concrete targets, prerequisites, prediction, task, failure case, verification evidence and an L4 gate.',
  '- **LEARNER_VERIFIED**: the learner has actually met the required mastery level with recorded evidence.',
  '',
  'These states intentionally separate source integrity, curriculum mapping, executable practice and actual learning.',
  '',
  '## Full Stack Open',
  '',
  `Active current-source exercise inventory: **${fsoExercises.length} records across Parts 0-14**.`,
  '',
  '| Part | Current source exercises | Mapped | Exercise ready | Learner verified | Source authority |',
  '|---:|---:|---:|---:|---:|---|',
];

for (let part = 0; part <= 14; part++) {
  const rows = fsoExercises.filter((item) => item.source.part === part);
  const authority = part <= 7 ? 'pinned course-repository snapshot' : 'current courses.mooc.fi API snapshot';
  lines.push(`| ${part} | ${rows.length} | ${atLeast(rows, 'MAPPED')} | ${atLeast(rows, 'EXERCISE_READY')} | ${atLeast(rows, 'LEARNER_VERIFIED')} | ${authority} |`);
}

lines.push(
  '',
  `Core Parts 0-7: **${atLeast(core, 'MAPPED')}/${core.length} numbered source exercises semantically mapped** against pinned repository snapshot \`${fso.baseline.indexed_snapshot_commit}\`. This is mapping coverage, not completed learning.`,
  '',
  `Advanced Parts 8-14: **${atLeast(currentMoocExercises, 'MAPPED')}/${currentMoocExercises.length} current exercise records semantically mapped** after indexing from the public courses.mooc.fi Course Material API. Snapshot retrieval: \`${mooc.retrieved_at}\`; source-state SHA-256: \`${mooc.source_state_sha256}\`. Mapping is complete at exercise-objective level; exercise readiness and concept-level parity remain separate.`,
  '',
  `The previous repository snapshot's Parts 8-11 are retained only as **${(fso.historical_items ?? []).length} historical exercise records** for comparison; they are not counted as current-course parity.`,
  '',
  `Current source-section inventory: **${fso.source_sections.length} core headings (Parts 0-7)** + **${fso.current_mooc_sections.length} current MOOC headings (Parts 8-14)**. **${fsoConcepts.length} explicit section-derived concepts** have already been semantically decomposed and mapped. Heading inventory improves omission detection but does not equal exhaustive paragraph-level semantic parity.`,
  '',
  'The MOOC metadata snapshot intentionally stores only identifiers, page/chapter metadata, headings and short exercise titles; it does not copy exercise assignments, answers or course prose.',
  '',
  '## TECH SCHOOL Backend Master Class',
  '',
  `Public README baseline: \`${tech.baseline.public_repo_commit}\`.`,
  '',
  `- Public lecture IDs/titles indexed: **${techLectures.length}/78**.`,
  `- Title-level BodySense mappings: **${atLeast(techLectures, 'MAPPED')}/78**.`,
  `- Exercise-ready lecture reps: **${atLeast(techLectures, 'EXERCISE_READY')}/78**.`,
  `- Learner-verified lecture reps: **${atLeast(techLectures, 'LEARNER_VERIFIED')}/78**.`,
  '',
  'This does **not** claim that paid/video-internal teaching semantics were audited. The source authority is the public README title/index only.',
  '',
  '## BodySense Agent extension',
  '',
  `- Project-defined modules mapped: **${atLeast(agent.items, 'MAPPED')}/${agent.items.length}**.`,
  `- Exercise-ready: **${atLeast(agent.items, 'EXERCISE_READY')}/${agent.items.length}**.`,
  `- Learner-verified: **${atLeast(agent.items, 'LEARNER_VERIFIED')}/${agent.items.length}**.`,
  '',
  '## Exercise-ready spine',
  '',
  `There are currently **${ready.length}** executable cards and **${verified.length}** learner-verified cards.`,
  '',
);

for (const item of ready) {
  lines.push(`- \`${item.mapping.exercise_id}\` -> [card](${item.mapping.exercise_card})`);
}

lines.push(
  '',
  'The next curriculum milestone is concept-level semantic audit plus expansion of `EXERCISE_READY` coverage while preserving prerequisite closure. Placement assessment starts only on ready prerequisite slices.',
);

fs.mkdirSync(path.dirname(viewPath), { recursive: true });
fs.writeFileSync(viewPath, `${lines.join('\n')}\n`);
console.log(`wrote ${path.relative(root, viewPath)}`);
