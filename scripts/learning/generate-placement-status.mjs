import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';
import { tracks, trackById } from './lib/curriculum-tracks.mjs';

const root = process.cwd();
const ledgerDir = path.join(root, 'docs/learning/curriculum/ledger');
const load = (name) => JSON.parse(fs.readFileSync(path.join(ledgerDir, name), 'utf8'));
const ledgers = [load('full-stack-open.json'), load('techschool-backend.json'), load('bodysense-agent.json')];
const placement = load('learner-placement.json');
const items = new Map(ledgers.flatMap((ledger) => ledger.items).map((item) => [item.id, item]));
const active = trackById.get(placement.active_track_id);
if (!active) throw new Error(`unknown active track: ${placement.active_track_id}`);
const rank = new Map([['L1',1],['L2',2],['L3',3],['L4',4],['L5',5]]);
const label = (item) => item?.mapping?.exercise_id ?? item?.id ?? 'unknown';

function orderedClosure(ids) {
  const focus = new Set(ids);
  const out = [];
  const seen = new Set();
  const visit = (id) => {
    if (seen.has(id)) return;
    seen.add(id);
    const item = items.get(id);
    for (const prereq of item?.mapping?.prerequisites ?? []) visit(prereq);
    if (!focus.has(id)) out.push(id);
  };
  ids.forEach(visit);
  return out;
}

function topoForActive(track) {
  const wanted = new Set([...orderedClosure(track.ids), ...track.ids]);
  const seen = new Set();
  const out = [];
  const visit = (id) => {
    if (seen.has(id) || !wanted.has(id)) return;
    seen.add(id);
    const item = items.get(id);
    for (const prereq of item?.mapping?.prerequisites ?? []) visit(prereq);
    out.push(id);
  };
  track.ids.forEach(visit);
  return out;
}

function state(item) {
  const current = item.mastery?.current ?? null;
  const required = item.mastery?.required ?? 'L4';
  if (!current) return 'UNASSESSED';
  if ((rank.get(current) ?? 0) >= (rank.get(required) ?? 999)) return `${current} VERIFIED`;
  return `${current} GAP`;
}

const closure = orderedClosure(active.ids);
const order = topoForActive(active);
const focus = new Set(active.ids);
const verified = order.filter((id) => state(items.get(id)).endsWith('VERIFIED'));
const gaps = order.filter((id) => state(items.get(id)).endsWith('GAP'));
const unassessed = order.filter((id) => state(items.get(id)) === 'UNASSESSED');
const nextId = order.find((id) => !state(items.get(id)).endsWith('VERIFIED')) ?? null;
const nextItem = nextId ? items.get(nextId) : null;

const lines = [
  '# Learner placement status',
  '',
  '> Generated from curriculum ledgers + `ledger/learner-placement.json`. Do not hand-edit this view.',
  '> Placement never infers mastery from production code alone; every assessed level requires recorded learner evidence.',
  '',
  '## Active track',
  '',
  `**${active.name}** (\`${active.id}\`)`,
  '',
  active.goal,
  '',
  `Relevant ready nodes including prerequisite closure: **${order.length}**; verified: **${verified.length}**; gaps: **${gaps.length}**; unassessed: **${unassessed.length}**.`,
  '',
];

if (nextItem) {
  const card = nextItem.mapping?.exercise_card;
  const rendered = card ? `[${label(nextItem)}](${card})` : `\`${label(nextItem)}\``;
  lines.push(
    '## Next placement target',
    '',
    `${rendered} · required **${nextItem.mastery?.required ?? 'L4'}** · current **${nextItem.mastery?.current ?? 'unassessed'}**.`,
    '',
    'Use the exercise card in placement mode: make the prediction without reading the implementation path, select discriminating evidence, then explain what would falsify the conclusion. Record the observed level only after that learner evidence exists.',
    '',
  );
} else {
  lines.push('## Next placement target', '', 'All nodes in the active track and its prerequisite closure meet their required mastery gate.', '');
}

lines.push('## Placement queue', '', '| # | Scope | Exercise | Required | Current | Placement state | Evidence journal |', '|---:|---|---|---|---|---|---|');
order.forEach((id, index) => {
  const item = items.get(id);
  const card = item.mapping?.exercise_card;
  const rendered = card ? `[${label(item)}](${card})` : `\`${label(item)}\``;
  const assessment = placement.assessments?.[id];
  const evidence = assessment ? `${assessment.method}; ${assessment.evidence.length} evidence item(s)` : '—';
  lines.push(`| ${index + 1} | ${focus.has(id) ? 'track' : 'prerequisite'} | ${rendered} | ${item.mastery?.required ?? 'L4'} | ${item.mastery?.current ?? '—'} | ${state(item)} | ${evidence} |`);
});

lines.push('', '## Available tracks', '', '| Track ID | Track | Nodes | Active |', '|---|---|---:|---|');
for (const track of tracks) lines.push(`| \`${track.id}\` | ${track.name} | ${track.ids.length} | ${track.id === active.id ? 'yes' : ''} |`);

lines.push(
  '',
  '## Recording rule',
  '',
  'Use `node scripts/learning/record-placement.mjs <ITEM_ID> <L1|L2|L3|L4|L5> --evidence "..."` only after the learner has actually produced the corresponding evidence. `L1-L3` records a gap and leaves the node exercise-ready; meeting the required gate promotes it to `LEARNER_VERIFIED`.',
  '',
  `Prerequisite closure outside the active track: ${closure.length ? closure.map((id) => `\`${label(items.get(id))}\``).join(', ') : 'none'}.`,
  '',
);

const out = path.join(root, 'docs/learning/curriculum/views/placement-status.md');
fs.writeFileSync(out, `${lines.join('\n')}\n`);
console.log(`wrote ${path.relative(root, out)}`);
