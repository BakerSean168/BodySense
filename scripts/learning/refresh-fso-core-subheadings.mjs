import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';
import { execFileSync } from 'node:child_process';

const root = process.cwd();
const sourceRoot = process.argv[2] || process.env.FSO_CORE_SOURCE_DIR;
if (!sourceRoot) {
  console.error('usage: node scripts/learning/refresh-fso-core-subheadings.mjs <full-stack-open checkout>');
  console.error('or set FSO_CORE_SOURCE_DIR');
  process.exit(2);
}
const source = path.resolve(sourceRoot);
const ledgerPath = path.join(root, 'docs/learning/curriculum/ledger/full-stack-open.json');
const outPath = path.join(root, 'docs/learning/curriculum/ledger/full-stack-open-core-subheading-audit.json');
const ledger = JSON.parse(fs.readFileSync(ledgerPath, 'utf8'));
const expectedCommit = ledger.baseline.indexed_snapshot_commit;
const actualCommit = execFileSync('git', ['-C', source, 'rev-parse', 'HEAD'], { encoding: 'utf8' }).trim();
if (actualCommit !== expectedCommit) {
  throw new Error(`source checkout commit ${actualCommit} != pinned core snapshot ${expectedCommit}`);
}

let previous = { rows: [] };
if (fs.existsSync(outPath)) previous = JSON.parse(fs.readFileSync(outPath, 'utf8'));
const previousByKey = new Map(previous.rows.map((row) => [`${row.source.path}:${row.source.line}:${row.source.level}:${row.title}`, row]));

const rows = [];
for (let part = 0; part <= 7; part++) {
  const dir = path.join(source, `src/content/${part}/en`);
  const files = fs.readdirSync(dir).filter((name) => new RegExp(`^part${part}.*\\.md$`).test(name)).sort();
  let sequence = 0;
  for (const file of files) {
    const full = path.join(dir, file);
    const rel = path.relative(source, full).split(path.sep).join('/');
    const lines = fs.readFileSync(full, 'utf8').split(/\r?\n/);
    let fence = null;
    let parentHeading = null;
    for (let index = 0; index < lines.length; index++) {
      const raw = lines[index];
      const trimmed = raw.trimStart();
      const fenceMatch = trimmed.match(/^(```+|~~~+)/);
      if (fenceMatch) {
        const token = fenceMatch[1][0];
        fence = fence === null ? token : fence === token ? null : fence;
        continue;
      }
      if (fence !== null) continue;
      const heading = trimmed.match(/^(#{3,6})\s+(.+?)\s*$/);
      if (!heading) continue;
      const level = heading[1].length;
      const title = heading[2].trim();
      if (level === 3) {
        parentHeading = { title, line: index + 1 };
        continue;
      }
      if (level < 4) continue;
      sequence += 1;
      const id = `FSO-P${part}-SUBHEADING-${String(sequence).padStart(3, '0')}`;
      const sourceRecord = {
        course: 'full-stack-open',
        part,
        snapshot_commit: expectedCommit,
        path: rel,
        line: index + 1,
        level,
        parent_heading: parentHeading?.title ?? null,
        parent_heading_line: parentHeading?.line ?? null,
      };
      const key = `${rel}:${index + 1}:${level}:${title}`;
      const old = previousByKey.get(key);
      rows.push({
        id,
        part,
        title,
        source: sourceRecord,
        status: old?.status ?? 'PENDING',
        linked_item_ids: old?.linked_item_ids ?? [],
        reviewed_at: old?.reviewed_at ?? null,
        note: old?.note ?? null,
      });
    }
  }
}

const normalized = rows.map((row) => ({
  part: row.part,
  title: row.title,
  path: row.source.path,
  line: row.source.line,
  level: row.source.level,
  parent_heading: row.source.parent_heading,
}));
const sourceStateSha256 = crypto.createHash('sha256').update(JSON.stringify(normalized)).digest('hex');
const out = {
  version: 1,
  source: {
    course: 'full-stack-open',
    scope: 'pinned core Parts 0-7 markdown headings level 4-6 outside fenced code blocks',
    snapshot_commit: expectedCommit,
    source_state_sha256: sourceStateSha256,
  },
  rows,
};
fs.writeFileSync(outPath, `${JSON.stringify(out, null, 2)}\n`);
console.log(`wrote ${path.relative(root, outPath)}`);
console.log(`subheading rows: ${rows.length}; source state sha256: ${sourceStateSha256}`);
