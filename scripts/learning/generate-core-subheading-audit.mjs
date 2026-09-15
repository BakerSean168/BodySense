import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';
const root=process.cwd();
const ledgerDir=path.join(root,'docs/learning/curriculum/ledger');
const audit=JSON.parse(fs.readFileSync(path.join(ledgerDir,'full-stack-open-core-subheading-audit.json'),'utf8'));
const ledger=JSON.parse(fs.readFileSync(path.join(ledgerDir,'full-stack-open.json'),'utf8'));
const itemById=new Map(ledger.items.map((item)=>[item.id,item]));
const counts={};for(const row of audit.rows)counts[row.status]=(counts[row.status]??0)+1;
const disposed=audit.rows.filter((row)=>row.status!=='PENDING').length;
const lines=[
 '# Full Stack Open pinned-core nested-heading audit','',
 '> Generated from `ledger/full-stack-open-core-subheading-audit.json`. Scope: Parts 0-7 markdown h4-h6 outside fenced code blocks at the pinned source commit.','> This layer exists because the primary core section inventory intentionally used h3 teaching sections while numbered exercises were indexed separately. It catches nested teaching points, exercise variants and removed tracks that would otherwise be invisible.','',
 `Pinned source: \`${audit.source.snapshot_commit}\`; nested-heading fingerprint: \`${audit.source.source_state_sha256}\`.`,
 `Nested h4-h6 review units: **${audit.rows.length}**; dispositioned: **${disposed}**; pending: **${counts.PENDING??0}**.`,'',
 `- covered by current numbered exercise: **${counts.COVERED_BY_EXERCISE??0}**`,
 `- current alternative exercise variant: **${counts.ALTERNATIVE_EXERCISE_VARIANT??0}**`,
 `- nested technical concept mapped: **${counts.REVIEWED_CONCEPTS_MAPPED??0}**`,
 `- reviewed redundant with existing concepts: **${counts.REVIEWED_REDUNDANT??0}**`,
 `- reviewed non-engineering/course administration: **${counts.REVIEWED_NON_ENGINEERING??0}**`,
 `- explicitly removed source track: **${counts.REMOVED_SOURCE_TRACK??0}**`,'',
];
for(let part=0;part<=7;part++){
 const rows=audit.rows.filter((row)=>row.part===part);if(!rows.length)continue;
 lines.push(`## Part ${part} — ${rows.filter((r)=>r.status!=='PENDING').length}/${rows.length}`,'','| Source | Parent h3 | Nested heading | Disposition | Linked coverage |','|---|---|---|---|---|');
 for(const row of rows){
   const loc=`${row.source.path}:${row.source.line} · h${row.source.level}`;
   const linked=(row.linked_item_ids??[]).map((id)=>`\`${itemById.get(id)?.mapping?.exercise_id??id}\``).join(', ')||'—';
   const esc=(v)=>String(v??'').replaceAll('|','\\|').replaceAll('\n',' ');
   lines.push(`| \`${esc(loc)}\` | ${esc(row.source.parent_heading??'—')} | ${esc(row.title)} | ${row.status} | ${linked} |`);
 }
 lines.push('');
}
lines.push('## Interpretation','','- `COVERED_BY_EXERCISE`: the nested heading is the detailed heading for an already current, indexed and mapped numbered exercise.','- `ALTERNATIVE_EXERCISE_VARIANT`: the source contains a real alternate implementation path for the same exercise number; it is retained instead of silently collapsing to the primary path.','- `REMOVED_SOURCE_TRACK`: the pinned file explicitly labels this material as removed from the current course; it remains visible as historical context but is not counted as current training parity.','- Nested concept/redundant/non-engineering statuses have the same intent as the primary section audit.','- Even 196/196 nested-heading disposition does **not** prove paragraph/example-level parity; unheaded warnings/code examples can still contain important teaching semantics.','');
const out=path.join(root,'docs/learning/curriculum/views/core-subheading-audit.md');fs.writeFileSync(out,lines.join('\n'));console.log(`wrote ${path.relative(root,out)}`);
