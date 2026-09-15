import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';

const root = process.cwd();
const ledgerDir = path.join(root, 'docs/learning/curriculum/ledger');
const load = (name) => JSON.parse(fs.readFileSync(path.join(ledgerDir, name), 'utf8'));
const ledgers = [load('full-stack-open.json'), load('techschool-backend.json'), load('bodysense-agent.json')];
const items = new Map(ledgers.flatMap((ledger) => ledger.items).map((item) => [item.id, item]));
const ready = [...items.values()].filter((item) => ['EXERCISE_READY','LEARNER_VERIFIED'].includes(item.lifecycle));
const readyIds = new Set(ready.map((item) => item.id));

const indegree = new Map(ready.map((item) => [item.id, 0]));
const outgoing = new Map(ready.map((item) => [item.id, []]));
for (const item of ready) {
  for (const prereq of item.mapping?.prerequisites ?? []) {
    if (!readyIds.has(prereq)) throw new Error(`${item.id} depends on non-ready ${prereq}`);
    indegree.set(item.id, (indegree.get(item.id) ?? 0) + 1);
    outgoing.get(prereq).push(item.id);
  }
}
const queue = [...ready.filter((item) => indegree.get(item.id) === 0).map((item) => item.id)].sort();
const topo=[];
while (queue.length) {
  const id=queue.shift();
  topo.push(id);
  for (const next of outgoing.get(id) ?? []) {
    indegree.set(next, indegree.get(next)-1);
    if (indegree.get(next) === 0) {
      queue.push(next);
      queue.sort();
    }
  }
}
if (topo.length !== ready.length) throw new Error('ready prerequisite graph contains a cycle');

const mermaidId = (id) => `n_${id.replace(/[^A-Za-z0-9_]/g,'_')}`;
const label = (item) => item.mapping?.exercise_id ?? item.id;
const nodeLines = ready.map((item) => `  ${mermaidId(item.id)}["${label(item)}"]`);
const edgeLines=[];
for (const item of ready) {
  for (const prereq of item.mapping?.prerequisites ?? []) {
    edgeLines.push(`  ${mermaidId(prereq)} --> ${mermaidId(item.id)}`);
  }
}
const roots = ready.filter((item)=> (item.mapping?.prerequisites ?? []).length===0);
const view = `# Exercise-ready prerequisite spine\n\n> Generated from ledger prerequisites. Do not hand-edit the graph.\n> Only \`EXERCISE_READY\` / \`LEARNER_VERIFIED\` nodes are shown. The generator fails if a ready node depends on a non-ready node or if the ready graph contains a cycle.\n\n## Ready roots\n\n${roots.map((item)=>`- \`${label(item)}\``).join('\n')}\n\n## Dependency graph\n\n\`\`\`mermaid\ngraph TD\n${nodeLines.join('\n')}\n${edgeLines.join('\n')}\n\`\`\`\n\n## Topological study order\n\nThe graph allows parallel branches; this is one valid topological order:\n\n${topo.map((id,index)=>`${index+1}. \`${label(items.get(id))}\``).join('\n')}\n\nA placement audit can skip a node only when prior evidence independently satisfies that node's L4/L5 acceptance card. Skipping a prerequisite because production code already exists is not sufficient.\n`;
const out=path.join(root,'docs/learning/curriculum/views/prerequisite-spine.md');
fs.writeFileSync(out,view);
console.log(`wrote ${path.relative(root,out)}`);
