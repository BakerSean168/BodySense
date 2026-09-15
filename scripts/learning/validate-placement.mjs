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
const ready = (item) => ['EXERCISE_READY', 'LEARNER_VERIFIED'].includes(item?.lifecycle);
const levels = new Map([['L1',1],['L2',2],['L3',3],['L4',4],['L5',5]]);
const methods = new Set(['placement','exercise','prior-evidence-review']);
const errors = [];
const fail = (message) => errors.push(message);

if (placement.version !== 1) fail(`placement: expected version 1, got ${placement.version}`);
if (!placement.learner) fail('placement: learner is required');
if (!placement.active_track_id || !trackById.has(placement.active_track_id)) fail(`placement: unknown active_track_id ${placement.active_track_id}`);
if (!placement.assessments || typeof placement.assessments !== 'object' || Array.isArray(placement.assessments)) fail('placement: assessments must be an object');

const trackIds = new Set();
for (const track of tracks) {
  if (!track.id) fail(`track ${track.name}: missing stable id`);
  if (trackIds.has(track.id)) fail(`track: duplicate id ${track.id}`);
  trackIds.add(track.id);
  for (const id of track.ids) {
    const item = items.get(id);
    if (!item) fail(`track ${track.id}: unknown item ${id}`);
    else if (!ready(item)) fail(`track ${track.id}: ${id} is ${item.lifecycle}, not ready`);
  }
}

for (const [id, assessment] of Object.entries(placement.assessments ?? {})) {
  const item = items.get(id);
  if (!item) {
    fail(`placement ${id}: unknown curriculum item`);
    continue;
  }
  if (!ready(item)) fail(`placement ${id}: item is ${item.lifecycle}, placement requires ready/verified item`);
  if (!levels.has(assessment.level)) fail(`placement ${id}: invalid level ${assessment.level}`);
  if (!methods.has(assessment.method)) fail(`placement ${id}: invalid method ${assessment.method}`);
  if (!assessment.assessed_at || Number.isNaN(Date.parse(assessment.assessed_at))) fail(`placement ${id}: assessed_at must be an ISO-compatible timestamp`);
  if (!Array.isArray(assessment.evidence) || assessment.evidence.length === 0 || assessment.evidence.some((entry) => typeof entry !== 'string' || !entry.trim())) {
    fail(`placement ${id}: non-empty string evidence is required`);
  }
  const current = item.mastery?.current ?? null;
  if (current !== assessment.level) fail(`placement ${id}: journal level ${assessment.level} != ledger mastery.current ${current}`);
  const requiredRank = levels.get(item.mastery?.required) ?? 999;
  const currentRank = levels.get(assessment.level) ?? 0;
  if (currentRank >= requiredRank && item.lifecycle !== 'LEARNER_VERIFIED') fail(`placement ${id}: ${assessment.level} meets ${item.mastery?.required} but lifecycle is ${item.lifecycle}`);
  if (currentRank < requiredRank && item.lifecycle === 'LEARNER_VERIFIED') fail(`placement ${id}: ${assessment.level} is below ${item.mastery?.required} but lifecycle is LEARNER_VERIFIED`);
  if (!Array.isArray(item.mastery?.evidence) || item.mastery.evidence.length === 0) fail(`placement ${id}: ledger mastery evidence must be retained`);
}

for (const item of items.values()) {
  const current = item.mastery?.current ?? null;
  if (current && !placement.assessments?.[item.id]) fail(`${item.id}: ledger mastery.current=${current} has no placement journal entry`);
  if (item.lifecycle === 'LEARNER_VERIFIED' && !placement.assessments?.[item.id]) fail(`${item.id}: LEARNER_VERIFIED requires a placement journal entry`);
}

if (errors.length) {
  console.error(`Placement validation failed with ${errors.length} issue(s):`);
  for (const error of errors) console.error(`- ${error}`);
  process.exit(1);
}

const assessed = Object.keys(placement.assessments ?? {}).length;
const verified = Object.entries(placement.assessments ?? {}).filter(([id, row]) => {
  const item = items.get(id);
  return (levels.get(row.level) ?? 0) >= (levels.get(item?.mastery?.required) ?? 999);
}).length;
console.log(`Placement validation OK (${assessed} assessed, ${verified} verified, active track: ${placement.active_track_id})`);
