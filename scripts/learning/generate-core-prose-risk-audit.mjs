import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';

const root = process.cwd();
const auditPath = path.join(root, 'docs/learning/curriculum/ledger/full-stack-open-core-prose-risk-audit.json');
const outPath = path.join(root, 'docs/learning/curriculum/views/core-prose-risk-audit.md');
const audit = JSON.parse(fs.readFileSync(auditPath, 'utf8'));
const rows = audit.rows ?? [];
const reviewed = rows.filter((row) => row.status === 'REVIEWED');
const newConcepts = rows.flatMap((row) => row.newly_exposed_concept_ids ?? []);

const lines = [
  '# Full Stack Open pinned-core targeted prose-risk audit',
  '',
  '> Generated from `ledger/full-stack-open-core-prose-risk-audit.json`.',
  '> This is deliberately **risk-selected**, not an exhaustive paragraph-by-paragraph inventory. A reviewed row means the complete pinned h3 fragment was read for important unheaded engineering semantics.',
  '',
  `Pinned source: \`${audit.source.snapshot_commit}\`; selected-fragment fingerprint: \`${audit.source.source_state_sha256}\`.`,
  `Selected high-risk h3 fragments: **${rows.length}**; reviewed: **${reviewed.length}**; pending: **${rows.length - reviewed.length}**.`,
  `New explicit concepts exposed by prose review: **${newConcepts.length}**${newConcepts.length ? ` (${newConcepts.map((id) => `\`${id}\``).join(', ')})` : ''}.`,
  '',
  '| Section | Source fragment | Why selected | Linked concepts | Newly exposed | Review note |',
  '|---|---|---|---|---|---|',
];

for (const row of rows) {
  const source = `${row.source.path}:${row.source.start_line}-${row.source.end_line}`;
  const linked = (row.linked_concept_ids ?? []).map((id) => `\`${id}\``).join(', ') || '—';
  const exposed = (row.newly_exposed_concept_ids ?? []).map((id) => `\`${id}\``).join(', ') || '—';
  const clean = (value) => String(value ?? '').replaceAll('|', '\\|').replaceAll('\n', ' ');
  lines.push(`| \`${row.section_id}\` · ${clean(row.title)} | \`${source}\` · \`${row.source_fragment_sha256.slice(0, 12)}…\` | ${clean(row.selection_reason)} | ${linked} | ${exposed} | ${clean(row.note || row.status)} |`);
}

lines.push(
  '',
  '## Interpretation',
  '',
  '- Primary h3 and nested h4-h6 audits answer **“did we inspect the known headings?”**.',
  '- This risk audit answers **“did selected high-risk sections hide important semantics inside ordinary prose/examples?”**.',
  '- A newly exposed concept is promoted into the same canonical FSO ledger and must satisfy the same mapping/readiness/mastery rules.',
  '- Unselected prose remains an explicit non-claim; this file must never be used to advertise exhaustive paragraph parity.',
  '',
);

fs.writeFileSync(outPath, `${lines.join('\n')}\n`);
console.log(`wrote ${path.relative(root, outPath)}`);
