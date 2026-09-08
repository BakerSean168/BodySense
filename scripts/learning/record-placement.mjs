import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';

const root = process.cwd();
const ledgerDir = path.join(root, 'docs/learning/curriculum/ledger');
const ledgerFiles = ['full-stack-open.json','techschool-backend.json','bodysense-agent.json'];
const levels = new Map([['L1',1],['L2',2],['L3',3],['L4',4],['L5',5]]);
const methods = new Set(['placement','exercise','prior-evidence-review']);

const [itemId, level, ...rest] = process.argv.slice(2);
if (!itemId || !levels.has(level)) {
  console.error('usage: node scripts/learning/record-placement.mjs <ITEM_ID> <L1|L2|L3|L4|L5> --evidence "..." [--evidence "..."] [--method placement|exercise|prior-evidence-review] [--note "..."] [--dry-run]');
  process.exit(2);
}

const evidence = [];
let method = 'placement';
let note = null;
let dryRun = false;
for (let i = 0; i < rest.length; i++) {
  const token = rest[i];
  if (token === '--evidence') evidence.push(rest[++i] ?? '');
  else if (token === '--method') method = rest[++i] ?? '';
  else if (token === '--note') note = rest[++i] ?? '';
  else if (token === '--dry-run') dryRun = true;
  else throw new Error(`unknown argument: ${token}`);
}
if (evidence.length === 0 || evidence.some((entry) => !entry.trim())) throw new Error('at least one non-empty --evidence value is required');
if (!methods.has(method)) throw new Error(`invalid method: ${method}`);

let owning = null;
for (const filename of ledgerFiles) {
  const filePath = path.join(ledgerDir, filename);
  const ledger = JSON.parse(fs.readFileSync(filePath, 'utf8'));
  const index = ledger.items.findIndex((item) => item.id === itemId);
  if (index !== -1) {
    owning = { filename, filePath, ledger, index };
    break;
  }
}
if (!owning) throw new Error(`unknown curriculum item: ${itemId}`);
const item = owning.ledger.items[owning.index];
if (!['EXERCISE_READY','LEARNER_VERIFIED'].includes(item.lifecycle)) throw new Error(`${itemId} is ${item.lifecycle}; placement requires an exercise-ready node`);

const previous = item.mastery?.current ?? null;
if (previous && (levels.get(level) ?? 0) < (levels.get(previous) ?? 0)) {
  throw new Error(`refusing mastery downgrade ${previous} -> ${level}; correct the journal/ledger explicitly if prior evidence was invalid`);
}
const required = item.mastery?.required ?? 'L4';
item.mastery ??= { required, current: null, evidence: [] };
item.mastery.current = level;
item.mastery.evidence ??= [];
for (const entry of evidence) if (!item.mastery.evidence.includes(entry)) item.mastery.evidence.push(entry);
item.lifecycle = (levels.get(level) ?? 0) >= (levels.get(required) ?? 999) ? 'LEARNER_VERIFIED' : 'EXERCISE_READY';

const placementPath = path.join(ledgerDir, 'learner-placement.json');
const placement = JSON.parse(fs.readFileSync(placementPath, 'utf8'));
const assessedAt = new Date().toISOString();
placement.assessments ??= {};
placement.assessments[itemId] = {
  level,
  assessed_at: assessedAt,
  method,
  evidence,
  note: note || null,
};
placement.updated_at = assessedAt;

if (!dryRun) {
  fs.writeFileSync(owning.filePath, `${JSON.stringify(owning.ledger, null, 2)}\n`);
  fs.writeFileSync(placementPath, `${JSON.stringify(placement, null, 2)}\n`);
}
console.log(`${dryRun ? '[dry-run] ' : ''}${itemId}: ${previous ?? 'unassessed'} -> ${level}; lifecycle=${item.lifecycle}; required=${required}; ledger=${owning.filename}`);
