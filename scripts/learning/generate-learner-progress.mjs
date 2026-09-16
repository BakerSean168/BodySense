import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';

const root = process.cwd();
const ledgerDir = path.join(root, 'docs/learning/curriculum/ledger');
const load = (name) => JSON.parse(fs.readFileSync(path.join(ledgerDir, name), 'utf8'));
const fso = load('full-stack-open.json');
const tech = load('techschool-backend.json');
const agent = load('bodysense-agent.json');
const placement = load('learner-placement.json');

const isVerified = (item) => item?.lifecycle === 'LEARNER_VERIFIED';
const isAssessed = (item) => Boolean(item?.mastery?.current);
const label = (item) => item?.mapping?.exercise_id ?? item?.id ?? 'unknown';
const shortTitle = (item) => item?.mapping?.objective ?? item?.source?.title ?? item?.id ?? 'unknown';

const partTopics = new Map([
  [0, 'Fundamentals of Web apps'],
  [1, 'Introduction to React'],
  [2, 'Communicating with server'],
  [3, 'Programming a server with NodeJS and Express'],
  [4, 'Testing Express servers, user administration'],
  [5, 'Testing React apps'],
  [6, 'Advanced state management'],
  [7, 'React router, custom hooks, tooling and advanced frontend'],
  [8, 'GraphQL'],
  [9, 'TypeScript'],
  [10, 'React Native'],
  [11, 'CI/CD'],
  [12, 'Containers'],
  [13, 'Relational databases'],
  [14, 'Next.js'],
]);

const lines = [
  '# Learner progress by canonical source',
  '',
  '> Generated from canonical curriculum ledgers and `learner-placement.json`. Do not hand-edit mastery counts here.',
  '> Progress is keyed by source item IDs and canonical Full Stack Open Parts, not by conversational labels such as “第五章/第六章/第七章”.',
  '',
  `Active learning track: **${placement.active_track_id}**.`,
  '',
  '## Full Stack Open progress',
  '',
  '| Part | Canonical topic | Assessed | Verified | Gaps |',
  '|---:|---|---:|---:|---:|',
];

for (let part = 0; part <= 14; part += 1) {
  const items = fso.items.filter((item) => item?.source?.part === part && isAssessed(item));
  const verified = items.filter(isVerified).length;
  lines.push(`| ${part} | ${partTopics.get(part)} | ${items.length} | ${verified} | ${items.length - verified} |`);
}

const assessedParts = [...new Set(fso.items.filter(isAssessed).map((item) => item?.source?.part).filter((part) => Number.isInteger(part)))].sort((a, b) => a - b);
for (const part of assessedParts) {
  const items = fso.items.filter((item) => item?.source?.part === part && isAssessed(item));
  lines.push('', `### Part ${part} · ${partTopics.get(part)}`, '');
  for (const item of items) {
    const state = isVerified(item) ? 'VERIFIED' : 'GAP';
    lines.push(`- **${item.mastery.current} ${state}** · \`${label(item)}\` · ${shortTitle(item)}`);
  }
}

const summarizeOther = (name, ledger) => {
  const items = ledger.items.filter(isAssessed);
  const verified = items.filter(isVerified);
  lines.push('', `## ${name}`, '', `Assessed: **${items.length}** · verified: **${verified.length}** · gaps: **${items.length - verified.length}**.`, '');
  for (const item of items) {
    const state = isVerified(item) ? 'VERIFIED' : 'GAP';
    lines.push(`- **${item.mastery.current} ${state}** · \`${label(item)}\` · ${shortTitle(item)}`);
  }
};

summarizeOther('TECH SCHOOL Backend', tech);
summarizeOther('BodySense Agent extension', agent);

const placementRows = Object.entries(placement.assessments ?? {})
  .map(([id, assessment]) => ({ id, ...assessment }))
  .sort((a, b) => new Date(a.assessed_at) - new Date(b.assessed_at));

const formatShanghai = (iso) => new Intl.DateTimeFormat('sv-SE', {
  timeZone: 'Asia/Shanghai',
  year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit',
  hour12: false,
}).format(new Date(iso));

lines.push('', '## Recent assessment timeline', '', '> Times are Asia/Shanghai. This timeline is useful for reconciling old conversational chapter labels with the canonical item IDs that actually received mastery evidence.', '');
for (const row of placementRows.slice(-40)) {
  lines.push(`- ${formatShanghai(row.assessed_at)} · **${row.level}** · \`${row.id}\` · ${row.method}`);
}

lines.push('', '## Reconciliation rule', '', 'If an old conversation called a study block “Chapter N” but the recorded item IDs belong to another canonical Part, keep the mastery evidence on those canonical IDs and treat the old chapter label as a navigation error. Do **not** duplicate the evidence onto the incorrectly named Part and do **not** force the learner to repeat verified work.', '');

const out = path.join(root, 'docs/learning/curriculum/views/learner-progress.md');
fs.writeFileSync(out, `${lines.join('\n')}\n`);
console.log(`wrote ${path.relative(root, out)}`);
