import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';

const root = process.cwd();
const ledgerDir = path.join(root, 'docs/learning/curriculum/ledger');
const load = (name) => JSON.parse(fs.readFileSync(path.join(ledgerDir, name), 'utf8'));
const ledgers = [load('full-stack-open.json'), load('techschool-backend.json'), load('bodysense-agent.json')];
const items = new Map(ledgers.flatMap((ledger) => ledger.items).map((item) => [item.id, item]));
const ready = (item) => ['EXERCISE_READY', 'LEARNER_VERIFIED'].includes(item?.lifecycle);
const label = (item) => item?.mapping?.exercise_id ?? item?.id ?? 'unknown';

const tracks = [
  {
    name: 'Web/browser request foundation',
    goal: 'Build the browser -> React -> HTTP -> API mental model used by every later frontend/full-stack exercise.',
    ids: ['FSO-0.1','FSO-0.3','FSO-0.4','FSO-0.5','FSO-0.6','FSO-2.11','FSO-2.17','FSO-3.1'],
  },
  {
    name: 'TypeScript contracts and runtime trust',
    goal: 'Separate structural static typing from runtime validation, then encode variant/state contracts safely.',
    ids: [
      'FSO-P9-CONCEPT-STRUCTURAL-TYPING','FSO-P9-CONCEPT-TYPE-ERASURE','FSO-P9-CONCEPT-UNKNOWN-NARROWING',
      'FSO-P9-CONCEPT-DISCRIMINATED-UNIONS','FSO-P9-CONCEPT-EXHAUSTIVE-NARROWING',
      'FSO-P9-CONCEPT-TYPED-SERVER-DATA','FSO-P9-CONCEPT-SCHEMA-VALIDATION',
    ],
  },
  {
    name: 'HTTP, authentication and web security',
    goal: 'Understand request semantics and middleware, then prove authentication, revocation, browser token storage and server-side authorization boundaries.',
    ids: [
      'FSO-P3-CONCEPT-HTTP-SAFETY-IDEMPOTENCY','FSO-P3-CONCEPT-MIDDLEWARE-CHAIN','FSO-P3-CONCEPT-SAME-ORIGIN-CORS','FSO-P3-CONCEPT-HTTP-ERROR-TAXONOMY',
      'TECH-20','TECH-21','TECH-22','TECH-37',
      'FSO-P4-CONCEPT-BEARER-AUTHORIZATION','FSO-P4-CONCEPT-TOKEN-REVOCATION','FSO-P5-CONCEPT-BROWSER-TOKEN-PERSISTENCE',
      'FSO-P7-CONCEPT-BROKEN-AUTHZ','FSO-P7-CONCEPT-XSS',
    ],
  },
  {
    name: 'Frontend state, server cache and realtime recovery',
    goal: 'Choose the correct state owner, synchronize server mutations, and recover push streams without treating transport state as durable truth.',
    ids: [
      'FSO-P6-CONCEPT-STATE-OWNERSHIP-CHOICE','FSO-P6-CONCEPT-TANSTACK-QUERY','FSO-P6-CONCEPT-QUERY-MUTATION-INVALIDATION','FSO-P7-CONCEPT-SERVER-PUSH-SYNC','BS-A7',
    ],
  },
  {
    name: 'Go backend, database and concurrency reliability',
    goal: 'Move from schema/repository tests through transaction locks/isolation into HTTP/API errors, authentication and durable jobs.',
    ids: ['TECH-01','TECH-03','TECH-05','TECH-06','TECH-07','TECH-09','TECH-11','TECH-15','TECH-16','TECH-20','TECH-21','TECH-22','TECH-37','TECH-54'],
  },
  {
    name: 'Containers and production delivery',
    goal: 'Understand image/runtime/network persistence first, then make CI/deploy reproducible, gated, recoverable and revision-identifiable.',
    ids: [
      'FSO-P12-CONCEPT-IMAGE-VS-CONTAINER','FSO-P12-CONCEPT-DOCKERFILE','FSO-P12-CONCEPT-DOCKER-COMPOSE','FSO-P12-CONCEPT-DOCKER-NETWORK-DNS','FSO-P12-CONCEPT-DOCKER-VOLUMES',
      'TECH-10','TECH-25',
      'FSO-P11-CONCEPT-REPRODUCIBLE-PIPELINE','FSO-P11-CONCEPT-CI-QUALITY-GATES','FSO-P11-CONCEPT-BRANCH-PROTECTION','FSO-P11-CONCEPT-SAFE-DEPLOYMENT-SYSTEM','FSO-P11-CONCEPT-DEPLOYED-REVISION-PROVENANCE',
    ],
  },
  {
    name: 'Production Agent engineering',
    goal: 'Learn typed execution, durable ownership, evidence/admissibility, deterministic authority, eval/rollout, replay, HITL/recovery and failure attribution as one production system.',
    ids: ['BS-A1','BS-A2','BS-A3','BS-A4','BS-A5','BS-A6','BS-A7','BS-A8'],
  },
];

for (const track of tracks) {
  for (const id of track.ids) {
    const item = items.get(id);
    if (!item) throw new Error(`${track.name}: unknown item ${id}`);
    if (!ready(item)) throw new Error(`${track.name}: ${id} is ${item.lifecycle}, not exercise-ready`);
  }
}

function prerequisiteClosure(ids) {
  const focus = new Set(ids);
  const seen = new Set();
  const visit = (id) => {
    const item = items.get(id);
    for (const prereq of item?.mapping?.prerequisites ?? []) {
      if (seen.has(prereq)) continue;
      seen.add(prereq);
      visit(prereq);
    }
  };
  ids.forEach(visit);
  return [...seen].filter((id) => !focus.has(id));
}

const lines = [
  '# Exercise-ready study tracks',
  '',
  '> Generated from the machine-readable ledgers. This is a curated learner-facing lens over the ready prerequisite graph, not a second source of truth.',
  '> Tracks overlap intentionally. Complete a prerequisite once and reuse the same evidence across every track that depends on it.',
  '',
  `Current executable curriculum: **${[...items.values()].filter(ready).length} ready/verified nodes**.`,
  '',
];

for (const [index, track] of tracks.entries()) {
  const closure = prerequisiteClosure(track.ids);
  lines.push(`## ${index + 1}. ${track.name}`, '', track.goal, '', '| # | Exercise | Required level |', '|---:|---|---|');
  track.ids.forEach((id, i) => {
    const item = items.get(id);
    const card = item.mapping?.exercise_card;
    const rendered = card ? `[${label(item)}](${card})` : `\`${label(item)}\``;
    lines.push(`| ${i + 1} | ${rendered} | ${item.mastery?.required ?? 'L4'} |`);
  });
  lines.push('');
  if (closure.length) {
    const rendered = closure.map((id) => `\`${label(items.get(id))}\``).join(', ');
    lines.push(`Prerequisite closure outside this track: ${rendered}.`, '');
  } else {
    lines.push('Prerequisite closure outside this track: none.', '');
  }
}

lines.push(
  '## How to use the tracks',
  '',
  '1. Do **not** read the target code first. Write the card prediction before opening the implementation/tests.',
  '2. Use the smallest verification slice that can distinguish the prediction from the failure case.',
  '3. A green existing test is evidence about the system, not evidence of learner mastery. L4 requires independent explain-back and falsification criteria.',
  '4. When prior production work already proves a card, use placement evidence; do not mechanically rewrite working code.',
  '5. Production changes are optional and should happen only when the card exposes a real, regression-characterized gap.',
  '',
);

const out = path.join(root, 'docs/learning/curriculum/views/study-tracks.md');
fs.writeFileSync(out, `${lines.join('\n')}\n`);
console.log(`wrote ${path.relative(root, out)}`);
