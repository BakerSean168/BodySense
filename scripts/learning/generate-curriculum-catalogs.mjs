import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';

const root = process.cwd();
const ledgerDir = path.join(root, 'docs/learning/curriculum/ledger');
const viewDir = path.join(root, 'docs/learning/curriculum/views');
const load = (name) => JSON.parse(fs.readFileSync(path.join(ledgerDir, name), 'utf8'));
const fso = load('full-stack-open.json');
const tech = load('techschool-backend.json');
const agent = load('bodysense-agent.json');

const esc = (value) => String(value ?? '').replaceAll('|', '\\|').replace(/\s+/g, ' ').trim();
const statusLabel = (item) => item.lifecycle;
const exerciseCard = (item) => item.mapping?.exercise_card
  ? `[card](${item.mapping.exercise_card})`
  : '—';

function write(name, lines) {
  const output = path.join(viewDir, name);
  fs.mkdirSync(path.dirname(output), { recursive: true });
  fs.writeFileSync(output, `${lines.join('\n').trimEnd()}\n`);
  console.log(`wrote ${path.relative(root, output)}`);
}

// Full Stack Open catalog: one row per active current-source exercise.
const fsoLines = [
  '# Full Stack Open -> BodySense exercise catalog',
  '',
  '> Generated from `ledger/full-stack-open.json`. This is a human-readable view, not the source of truth.',
  '> Every row is a distinct current-source exercise record. `MAPPED` is not the same as `EXERCISE_READY` or learner mastery.',
  '',
];
for (let part = 0; part <= 14; part++) {
  const rows = fso.items
    .filter((item) => item.kind === 'exercise' && item.source.part === part)
    .sort((a, b) => {
      const an = a.source.exercise_number ?? Number(String(a.source.number ?? '').split('.')[1]);
      const bn = b.source.exercise_number ?? Number(String(b.source.number ?? '').split('.')[1]);
      return (an ?? 9999) - (bn ?? 9999) || a.id.localeCompare(b.id);
    });
  fsoLines.push(`## Part ${part}`, '');
  fsoLines.push('| Source | Mode | BodySense rep | Objective | BodySense adaptation | State |');
  fsoLines.push('|---|---|---|---|---|---|');
  for (const item of rows) {
    const sourceNumber = item.source.exercise_number ?? item.source.number ?? item.id;
    const sourceTitle = item.source.title ? ` · ${item.source.title}` : '';
    const rep = item.mapping?.exercise_id ?? '—';
    fsoLines.push(`| ${esc(sourceNumber)}${esc(sourceTitle)} | ${esc(item.mapping?.mode ?? '—')} | ${esc(rep)} ${exerciseCard(item)} | ${esc(item.mapping?.objective ?? 'source indexed; semantic mapping pending')} | ${esc(item.mapping?.task_summary ?? '—')} | ${statusLabel(item)} |`);
  }
  fsoLines.push('');
}

const conceptRows = fso.items.filter((item) => item.kind === 'concept');
fsoLines.push('## Explicit concept records', '');
fsoLines.push('Numbered exercises do not prove all teaching concepts. These are the section/prose concepts already promoted to explicit semantic records.', '');
fsoLines.push('| Concept | Part | Objective | BodySense adaptation | State |');
fsoLines.push('|---|---:|---|---|---|');
for (const item of conceptRows) {
  fsoLines.push(`| ${esc(item.source.title)} | ${item.source.part} | ${esc(item.mapping?.objective)} | ${esc(item.mapping?.task_summary)} | ${item.lifecycle} |`);
}
fsoLines.push('', `Current section-heading inventory still awaiting exhaustive semantic decomposition: ${fso.source_sections.length + fso.current_mooc_sections.length} active headings. Heading inventory is not reported as knowledge mastery.`);
write('full-stack-open-catalog.md', fsoLines);

// TECH SCHOOL catalog.
const techLines = [
  '# TECH SCHOOL Backend Master Class -> BodySense catalog',
  '',
  '> Generated from `ledger/techschool-backend.json`.',
  '> Source authority is the pinned public README lecture index/title only; video-internal subtopics are not claimed as audited.',
  '',
  '| Lecture | Public title | Mode | BodySense rep | BodySense adaptation | State |',
  '|---:|---|---|---|---|---|',
];
for (const item of tech.items.filter((item) => item.kind === 'lecture').sort((a, b) => a.source.lecture - b.source.lecture)) {
  techLines.push(`| ${item.source.lecture} | ${esc(item.source.title)} | ${esc(item.mapping?.mode)} | ${esc(item.mapping?.exercise_id)} ${exerciseCard(item)} | ${esc(item.mapping?.task_summary)} | ${item.lifecycle} |`);
}
write('techschool-backend-catalog.md', techLines);

// Agent catalog.
const agentLines = [
  '# BodySense Agent Engineering catalog',
  '',
  '> Generated from `ledger/bodysense-agent.json`.',
  '',
  '| Module | Objective | Target / adaptation | State |',
  '|---|---|---|---|',
];
for (const item of agent.items) {
  agentLines.push(`| ${esc(item.mapping?.exercise_id ?? item.id)} ${exerciseCard(item)} | ${esc(item.mapping?.objective)} | ${esc(item.mapping?.task_summary)} | ${item.lifecycle} |`);
}
write('agent-engineering-catalog.md', agentLines);
