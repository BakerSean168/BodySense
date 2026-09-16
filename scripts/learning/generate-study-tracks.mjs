import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';
import { tracks } from './lib/curriculum-tracks.mjs';

const root = process.cwd();
const ledgerDir = path.join(root, 'docs/learning/curriculum/ledger');
const load = (name) => JSON.parse(fs.readFileSync(path.join(ledgerDir, name), 'utf8'));
const ledgers = [load('full-stack-open.json'), load('techschool-backend.json'), load('bodysense-agent.json')];
const items = new Map(ledgers.flatMap((ledger) => ledger.items).map((item) => [item.id, item]));
const ready = (item) => ['EXERCISE_READY', 'LEARNER_VERIFIED'].includes(item?.lifecycle);
const learnerVerified = (item) => item?.lifecycle === 'LEARNER_VERIFIED';
const label = (item) => item?.mapping?.exercise_id ?? item?.id ?? 'unknown';

for (const track of tracks) {
  for (const id of track.ids) {
    const item = items.get(id);
    if (!item) throw new Error(`${track.name}: unknown item ${id}`);
    if (!ready(item)) throw new Error(`${track.name}: ${id} is ${item.lifecycle}, not exercise-ready`);
  }
}

const trackedReadyIds = new Set(tracks.flatMap((track) => track.ids));
const untrackedReady = [...items.values()].filter((item) => ready(item) && !trackedReadyIds.has(item.id));
if (untrackedReady.length) {
  throw new Error(`study tracks omit ${untrackedReady.length} ready node(s): ${untrackedReady.map((item) => item.id).join(', ')}`);
}

function prerequisiteClosure(ids) {
  const focus = new Set(ids);
  const seen = new Set();
  const visit = (id) => {
    const item = items.get(id);
    if (learnerVerified(item)) return;
    for (const prereq of item?.mapping?.prerequisites ?? []) {
      if (seen.has(prereq)) continue;
      seen.add(prereq);
      visit(prereq);
    }
  };
  ids.forEach(visit);
  return [...seen].filter((id) => !focus.has(id));
}

const lines = [
  '# Exercise-ready study tracks',
  '',
  '> Generated from the machine-readable ledgers. This is a curated learner-facing lens over the ready prerequisite graph, not a second source of truth.',
  '> Tracks overlap intentionally. Complete a prerequisite once and reuse the same evidence across every track that depends on it.',
  '',
  `Current executable curriculum: **${[...items.values()].filter(ready).length} ready/verified nodes**.`,
  '',
];

for (const track of tracks) {
  const closure = prerequisiteClosure(track.ids);
  lines.push(`## Track: ${track.name}`, '', `Stable track id: \`${track.id}\``, '', track.goal, '', '| # | Exercise | Required level |', '|---:|---|---|');
  track.ids.forEach((id, i) => {
    const item = items.get(id);
    const card = item.mapping?.exercise_card;
    const rendered = card ? `[${label(item)}](${card})` : `\`${label(item)}\``;
    lines.push(`| ${i + 1} | ${rendered} | ${item.mastery?.required ?? 'L4'} |`);
  });
  lines.push('');
  if (closure.length) {
    const rendered = closure.map((id) => `\`${label(items.get(id))}\``).join(', ');
    lines.push(`Prerequisite closure outside this track: ${rendered}.`, '');
  } else {
    lines.push('Prerequisite closure outside this track: none.', '');
  }
}

lines.push(
  '## How to use the tracks',
  '',
  '1. Do **not** read the target code first. Write the card prediction before opening the implementation/tests.',
  '2. Use the smallest verification slice that can distinguish the prediction from the failure case.',
  '3. A green existing test is evidence about the system, not evidence of learner mastery. L4 requires independent explain-back and falsification criteria.',
  '4. When prior production work already proves a card, use placement evidence; do not mechanically rewrite working code.',
  '5. Production changes are optional and should happen only when the card exposes a real, regression-characterized gap.',
  '',
);

const out = path.join(root, 'docs/learning/curriculum/views/study-tracks.md');
fs.writeFileSync(out, `${lines.join('\n')}\n`);
console.log(`wrote ${path.relative(root, out)}`);
