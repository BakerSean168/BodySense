import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';
import { trackById, tracks } from './lib/curriculum-tracks.mjs';

const root = process.cwd();
const trackId = process.argv[2];
if (!trackId || !trackById.has(trackId)) {
  console.error('usage: node scripts/learning/select-placement-track.mjs <TRACK_ID>');
  console.error(`available: ${tracks.map((track) => track.id).join(', ')}`);
  process.exit(2);
}
const file = path.join(root, 'docs/learning/curriculum/ledger/learner-placement.json');
const placement = JSON.parse(fs.readFileSync(file, 'utf8'));
const before = placement.active_track_id ?? null;
placement.active_track_id = trackId;
placement.updated_at = new Date().toISOString();
fs.writeFileSync(file, `${JSON.stringify(placement, null, 2)}\n`);
console.log(`active placement track: ${before ?? 'none'} -> ${trackId}`);
