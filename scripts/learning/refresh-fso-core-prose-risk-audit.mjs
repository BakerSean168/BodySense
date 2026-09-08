import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';
import { execFileSync } from 'node:child_process';

const root = process.cwd();
const sourceRoot = process.argv[2] || process.env.FSO_CORE_SOURCE_DIR;
if (!sourceRoot) {
  console.error('usage: node scripts/learning/refresh-fso-core-prose-risk-audit.mjs <full-stack-open checkout>');
  console.error('or set FSO_CORE_SOURCE_DIR');
  process.exit(2);
}

const selections = [
  ['FSO-P2-SECTION-018', 'React effect timing/dependencies are lifecycle-sensitive and easy to misunderstand from heading-only mapping.'],
  ['FSO-P3-SECTION-014', 'Same-origin/CORS is a browser security boundary with important request/response semantics.'],
  ['FSO-P3-SECTION-032', 'HTTP error mapping often contains multiple failure classes and transport/domain boundary details.'],
  ['FSO-P4-SECTION-022', 'Token expiry, revocation and server-side sessions materially affect authentication authority.'],
  ['FSO-P5-SECTION-006', 'Browser credential persistence changes XSS exposure, reload behavior and logout/session semantics.'],
  ['FSO-P5-SECTION-017', 'Testing Library query semantics encode user-observable/accessibility-oriented testing practice.'],
  ['FSO-P6-SECTION-020', 'Remote-cache ownership, query lifecycle and refetch behavior are core frontend server-state semantics.'],
  ['FSO-P6-SECTION-021', 'Mutation reconciliation/invalidation is a concurrency and cache-consistency boundary.'],
  ['FSO-P7-SECTION-002', 'Memoization is frequently cargo-culted; prose contains dependency/identity/performance caveats beyond the heading.'],
  ['FSO-P7-SECTION-020', 'Security prose combines injection, XSS, dependency supply chain, access control and defense-in-depth guidance.'],
];

const source = path.resolve(sourceRoot);
const ledgerPath = path.join(root, 'docs/learning/curriculum/ledger/full-stack-open.json');
const outPath = path.join(root, 'docs/learning/curriculum/ledger/full-stack-open-core-prose-risk-audit.json');
const ledger = JSON.parse(fs.readFileSync(ledgerPath, 'utf8'));
const expectedCommit = ledger.baseline.indexed_snapshot_commit;
const actualCommit = execFileSync('git', ['-C', source, 'rev-parse', 'HEAD'], { encoding: 'utf8' }).trim();
if (actualCommit !== expectedCommit) throw new Error(`source checkout commit ${actualCommit} != pinned core snapshot ${expectedCommit}`);

let previous = { rows: [] };
if (fs.existsSync(outPath)) previous = JSON.parse(fs.readFileSync(outPath, 'utf8'));
const previousById = new Map((previous.rows ?? []).map((row) => [row.section_id, row]));
const sectionById = new Map((ledger.source_sections ?? []).map((section) => [section.id, section]));

function extractSectionFragment(section) {
  const rel = section.source.path;
  const full = path.join(source, rel);
  const lines = fs.readFileSync(full, 'utf8').split(/\r?\n/);
  const start = section.source.line - 1;
  if (start < 0 || start >= lines.length) throw new Error(`${section.id}: invalid source line ${section.source.line}`);
  if (!lines[start].trimStart().startsWith('### ')) throw new Error(`${section.id}: source line is not h3: ${lines[start]}`);
  let fence = null;
  let end = lines.length;
  for (let i = start + 1; i < lines.length; i++) {
    const trimmed = lines[i].trimStart();
    const fenceMatch = trimmed.match(/^(```+|~~~+)/);
    if (fenceMatch) {
      const token = fenceMatch[1][0];
      fence = fence === null ? token : fence === token ? null : fence;
      continue;
    }
    if (fence === null && /^###\s+/.test(trimmed)) {
      end = i;
      break;
    }
  }
  const fragment = lines.slice(start, end).join('\n').trimEnd() + '\n';
  return {
    start_line: start + 1,
    end_line: end,
    bytes: Buffer.byteLength(fragment),
    sha256: crypto.createHash('sha256').update(fragment).digest('hex'),
  };
}

const rows = selections.map(([sectionId, selectionReason]) => {
  const section = sectionById.get(sectionId);
  if (!section) throw new Error(`unknown core section ${sectionId}`);
  const fragment = extractSectionFragment(section);
  const old = previousById.get(sectionId);
  const unchanged = old?.source_fragment_sha256 === fragment.sha256;
  return {
    section_id: sectionId,
    part: section.source.part,
    title: section.source.title,
    source: {
      path: section.source.path,
      start_line: fragment.start_line,
      end_line: fragment.end_line,
      snapshot_commit: expectedCommit,
    },
    source_fragment_sha256: fragment.sha256,
    source_fragment_bytes: fragment.bytes,
    selection_reason: selectionReason,
    status: unchanged ? old.status : 'PENDING',
    linked_concept_ids: unchanged ? (old.linked_concept_ids ?? []) : [],
    newly_exposed_concept_ids: unchanged ? (old.newly_exposed_concept_ids ?? []) : [],
    reviewed_at: unchanged ? (old.reviewed_at ?? null) : null,
    note: unchanged ? (old.note ?? null) : null,
  };
});

const state = rows.map((row) => ({ section_id: row.section_id, source_fragment_sha256: row.source_fragment_sha256 }));
const sourceStateSha256 = crypto.createHash('sha256').update(JSON.stringify(state)).digest('hex');
const out = {
  version: 1,
  source: {
    course: 'full-stack-open',
    scope: 'targeted high-risk unheaded prose/example audit for selected pinned-core h3 sections',
    snapshot_commit: expectedCommit,
    source_state_sha256: sourceStateSha256,
  },
  policy: 'Selection is risk-based, not exhaustive. REVIEWED means the complete h3 fragment was inspected for engineering semantics hidden below headings; it does not claim paragraph parity for unselected sections.',
  rows,
};
fs.writeFileSync(outPath, `${JSON.stringify(out, null, 2)}\n`);
console.log(`wrote ${path.relative(root, outPath)}`);
console.log(`prose-risk rows: ${rows.length}; source state sha256: ${sourceStateSha256}`);
