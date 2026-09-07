import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';

const root = process.cwd();
const ledger = JSON.parse(fs.readFileSync(path.join(root, 'docs/learning/curriculum/ledger/full-stack-open.json'), 'utf8'));
const out = path.join(root, 'docs/learning/curriculum/views/concept-audit-queue.md');

const activeSections = [...(ledger.source_sections ?? []), ...(ledger.current_mooc_sections ?? [])];
const concepts = ledger.items.filter((item) => item.kind === 'concept');
const conceptsBySection = new Map();
for (const concept of concepts) {
  for (const sectionId of concept.source?.section_ids ?? []) {
    const list = conceptsBySection.get(sectionId) ?? [];
    list.push(concept);
    conceptsBySection.set(sectionId, list);
  }
}

const lines = [
  '# Full Stack Open concept semantic-audit queue',
  '',
  '> Generated from the active source-section inventory and explicit concept records.',
  '> A section heading is a review unit, not proof that every paragraph-level concept is covered. `REVIEWED` below only means the section has at least one explicit semantic concept record linked to it.',
  '',
  `Active section-heading review units: **${activeSections.length}**.`,
  `Sections with at least one explicit semantic concept record: **${[...conceptsBySection.keys()].length}**.`,
  `Pending section review units: **${activeSections.length - conceptsBySection.size}**.`,
  '',
];

for (let part = 0; part <= 14; part++) {
  const sections = activeSections.filter((item) => item.source?.part === part);
  if (!sections.length) continue;
  const reviewed = sections.filter((section) => conceptsBySection.has(section.id));
  lines.push(`## Part ${part} — ${reviewed.length}/${sections.length} section units linked to explicit concepts`, '');
  lines.push('| Section source | Heading | Semantic audit | Explicit concept records |');
  lines.push('|---|---|---|---|');
  for (const section of sections) {
    const refs = conceptsBySection.get(section.id) ?? [];
    const sourceLoc = section.source.path
      ? `${section.source.path}:${section.source.line}`
      : `${section.source.page_path ?? 'MOOC page'}${section.source.heading_level ? ` · h${section.source.heading_level}` : ''}`;
    lines.push(`| \`${section.id}\` · ${sourceLoc.replaceAll('|', '\\|')} | ${String(section.source.title ?? '').replaceAll('|', '\\|')} | ${refs.length ? 'REVIEWED' : 'PENDING'} | ${refs.length ? refs.map((item) => `\`${item.id}\``).join(', ') : '—'} |`);
  }
  lines.push('');
}

lines.push(
  '## Promotion rule',
  '',
  'A heading moves out of this queue only after the section has been read semantically and any reusable engineering concepts are represented by one or more explicit `kind: concept` ledger records with BodySense mapping. Administrative/submission-only headings can instead receive an explicit reviewed-no-concept disposition in a future concept-audit ledger; they must not disappear silently.',
  '',
  'Even after every heading is dispositioned, complete prose-level parity still requires a final audit that checks for important concepts taught inside a section without their own heading.',
);

fs.writeFileSync(out, `${lines.join('\n').trimEnd()}\n`);
console.log(`wrote ${path.relative(root, out)}`);
