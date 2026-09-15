# Placement workflow

Placement answers a different question from curriculum coverage: **what can the learner already prove at the required mastery gate?**

The source ledgers remain canonical for curriculum items and current mastery. `ledger/learner-placement.json` is the machine-readable placement journal/configuration used to select the active track and record the latest assessed level for each ready item.

## Rules

- Assess only `EXERCISE_READY` / `LEARNER_VERIFIED` items.
- Do not infer mastery from the existence of production code or green tests.
- Record concrete evidence for every assessed level, including gaps.
- `L4`/`L5` only verifies an item when it meets that item's required mastery gate.
- A lower placement result updates `mastery.current` but leaves the item `EXERCISE_READY`.
- A later reassessment replaces the placement journal's latest row while canonical ledger evidence is retained append-only.

## Commands

Select a stable track and generate the active placement queue/status:

```bash
pnpm curriculum:select-track -- typescript-runtime-trust
pnpm curriculum:placement
```

Record one assessed item:

```bash
node scripts/learning/record-placement.mjs <ITEM_ID> <L1|L2|L3|L4|L5> \
  --method placement \
  --evidence "concrete learner observation" \
  --note "optional concise explanation"
```

Use `--dry-run` to preview the ledger/lifecycle effect without writing.

Then run:

```bash
pnpm curriculum:check
```

To change the active track, prefer `pnpm curriculum:select-track -- <TRACK_ID>` using one of the stable IDs shown in `views/placement-status.md`. Changing the active track does not alter mastery.
