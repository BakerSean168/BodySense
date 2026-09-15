import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';

const root = process.cwd();
const ledgerDir = path.join(root, 'docs/learning/curriculum/ledger');
const ledger = JSON.parse(fs.readFileSync(path.join(ledgerDir, 'full-stack-open.json'), 'utf8'));
const audit = JSON.parse(fs.readFileSync(path.join(ledgerDir, 'full-stack-open-concept-audit.json'), 'utf8'));
const out = path.join(root, 'docs/learning/curriculum/views/concept-audit-queue.md');
const activeSections = [...(ledger.source_sections ?? []), ...(ledger.current_mooc_sections ?? [])];
const sectionById = new Map(activeSections.map((item) => [item.id, item]));
const auditById = new Map(audit.sections.map((item) => [item.section_id, item]));

const counts = {};
for (const row of audit.sections) counts[row.status] = (counts[row.status] ?? 0) + 1;
const disposed = audit.sections.length - (counts.PENDING ?? 0);

const lines = [
  '# Full Stack Open concept semantic-audit queue',
  '',
  '> Generated from `ledger/full-stack-open-concept-audit.json` and the active source-section inventory.',
  '> A section heading is a review unit, not proof that every paragraph-level concept is covered.',
  '',
  `Active section-heading review units: **${audit.sections.length}**.`,
  `Semantically dispositioned section units: **${disposed}**.`,
  `- concepts mapped: **${counts.REVIEWED_CONCEPTS_MAPPED ?? 0}**`,
  `- reviewed non-engineering/course logistics: **${counts.REVIEWED_NON_ENGINEERING ?? 0}**`,
  `- reviewed redundant: **${counts.REVIEWED_REDUNDANT ?? 0}**`,
  `- pending: **${counts.PENDING ?? 0}**`,
  '',
];

for (let part = 0; part <= 14; part++) {
  const rows = audit.sections.filter((item) => item.part === part);
  if (!rows.length) continue;
  const partDisposed = rows.filter((item) => item.status !== 'PENDING').length;
  lines.push(`## Part ${part} — ${partDisposed}/${rows.length} section units dispositioned`, '');
  lines.push('| Section source | Heading | Semantic disposition | Explicit concept records |');
  lines.push('|---|---|---|---|');
  for (const row of rows) {
    const section = sectionById.get(row.section_id);
    const source = section?.source ?? {};
    const sourceLoc = source.path
      ? `${source.path}:${source.line}`
      : `${source.page_path ?? 'MOOC page'}${source.heading_level ? ` · h${source.heading_level}` : ''}`;
    lines.push(`| \`${row.section_id}\` · ${String(sourceLoc).replaceAll('|', '\\|')} | ${String(row.title ?? source.title ?? '').replaceAll('|', '\\|')} | ${row.status} | ${row.linked_concept_ids.length ? row.linked_concept_ids.map((id) => `\`${id}\``).join(', ') : '—'} |`);
  }
  lines.push('');
}

lines.push(
  '## Promotion rule',
  '',
  '- `REVIEWED_CONCEPTS_MAPPED`: the section was read semantically and reusable engineering concepts are represented by explicit mapped `kind: concept` records.',
  '- `REVIEWED_NON_ENGINEERING`: course administration/study logistics were inspected and retained explicitly but do not become BodySense engineering material.',
  '- `REVIEWED_REDUNDANT`: semantics are intentionally covered by explicit concept records attached elsewhere; the audit note must explain the relationship.',
  '- `PENDING`: no semantic coverage claim.',
  '',
  'Even after every heading is dispositioned, a final prose-level audit is still required to catch important concepts taught inside a section without a dedicated heading.',
);

fs.writeFileSync(out, `${lines.join('\n').trimEnd()}\n`);
console.log(`wrote ${path.relative(root, out)}`);
