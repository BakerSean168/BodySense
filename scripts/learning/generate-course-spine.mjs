import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';

const root = process.cwd();
const ledgerPath = path.join(root, 'docs/learning/curriculum/ledger/full-stack-open.json');
const ledger = JSON.parse(fs.readFileSync(ledgerPath, 'utf8'));
const ready = (item) => ['EXERCISE_READY', 'LEARNER_VERIFIED'].includes(item.lifecycle);
const verified = (item) => item.lifecycle === 'LEARNER_VERIFIED';

const parts = [
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
];

const notes = new Map([
  [3, 'First substantial backend + persistence practice; the source course uses Node/Express and MongoDB.'],
  [7, 'Realtime/server-push is introduced as an architecture comparison here. BodySense SSE belongs here or in Agent/runtime work, not as the title of Part 8.'],
  [8, 'Canonical Part 8 topic. BodySense uses an isolated GraphQL/Apollo Server + Apollo Client lab plus comparisons with REST, TanStack Query and SSE; production migration is not required.'],
  [9, 'TypeScript/runtime-trust practice.'],
  [13, 'Canonical relational-database part; BodySense maps it to PostgreSQL/GORM/SQL behavior.'],
]);

const lines = [
  '# Full Stack Open source-course spine',
  '',
  '> Generated from the curriculum ledger plus the pinned Part metadata. This file is the learner-facing authority for what “Part N / 第 N 章” means inside the BodySense Master Course.',
  '',
  '## Numbering invariant',
  '',
  '- `Full Stack Open Part N` / “第 N 章” in FSO context always resolves to the source-course part below.',
  '- A `Study Track` is a topical lens over the ready dependency graph. It is **not** a numbered chapter and must be named as a Track.',
  '- `BS-A1..A8` are BodySense Agent extension modules, not FSO Parts.',
  '- A numbered section such as `§8` inside an architecture document is only a document section.',
  '- If the requested FSO part has no exercise-ready node, coaching must say so and prepare that part; it must not silently substitute a different ready topic.',
  '',
  '> **Hard check:** Full Stack Open **Part 8 = GraphQL**. SSE is only relevant there when comparing GraphQL subscriptions/WebSockets with BodySense realtime transport. “Part 8 = SSE event contract design” is invalid navigation.',
  '',
  '| FSO Part | Canonical topic | Current exercise records | Ready/verified learning nodes | Learner verified | BodySense note |',
  '|---:|---|---:|---:|---:|---|',
];

for (const [part, title] of parts) {
  const items = ledger.items.filter((item) => item.source?.part === part);
  const exercises = items.filter((item) => item.kind === 'exercise').length;
  const readyCount = items.filter(ready).length;
  const verifiedCount = items.filter(verified).length;
  lines.push(`| ${part} | ${title} | ${exercises} | ${readyCount} | ${verifiedCount} | ${notes.get(part) ?? ''} |`);
}

lines.push(
  '',
  '## Part 8 executable spine',
  '',
  'The current learner-facing Part 8 track is `fso-part-8-graphql`. Its core sequence is:',
  '',
  '```text',
  'Schema / query selection',
  '-> Apollo Server execution',
  '-> resolver args / request context',
  '-> mutation + domain errors',
  '-> Apollo Client vs TanStack Query',
  '-> variables + normalized cache',
  '-> mutation cache reconciliation',
  '-> auth context',
  '-> subscriptions vs BodySense SSE',
  '-> N+1 / database query cost',
  '```',
  '',
  'The source catalog still contains every mapped Part 8 exercise/section. This spine only identifies the high-value executable path; it does not claim that twelve ready cards replace all source-course material.',
  '',
  '## Database placement',
  '',
  '- Part 3 introduces backend persistence in the original course through MongoDB/Mongoose concepts.',
  '- Part 13 is the dedicated relational-database part. BodySense preserves the concepts while using PostgreSQL/GORM rather than forcing Sequelize into production.',
  '- TECH SCHOOL backend exercises provide the deeper SQL/transaction/lock/isolation path beside FSO Part 13.',
  '',
);

while (lines.at(-1) === '') {
  lines.pop();
}

const out = path.join(root, 'docs/learning/curriculum/views/course-spine.md');
fs.writeFileSync(out, `${lines.join('\n')}\n`);
console.log(`wrote ${path.relative(root, out)}`);
