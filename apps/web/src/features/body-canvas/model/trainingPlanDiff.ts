import type {
  PlanContent,
  PlanExerciseRecord,
  ProposalChangeView,
} from "./trainingPlanViewTypes";
import { formatPrescription, formatRest } from "./trainingPlanPresentation";

/**
 * Exercise-level diff between two plan contents, used by the Proposal and
 * Updated boards. Exercises are matched by their stable `planExerciseId`.
 */

function flattenExercises(content: PlanContent | null | undefined) {
  const map = new Map<string, PlanExerciseRecord>();
  for (const template of content?.sessions ?? [])
    for (const stage of template.stages)
      for (const exercise of stage.exercises)
        if (!map.has(exercise.planExerciseId))
          map.set(exercise.planExerciseId, exercise);
  return map;
}

function describe(exercise: PlanExerciseRecord): string {
  return [formatPrescription(exercise), formatRest(exercise)]
    .filter(Boolean)
    .join(" · ");
}

function sameExercise(a: PlanExerciseRecord, b: PlanExerciseRecord): boolean {
  return (
    a.nameSnapshot === b.nameSnapshot &&
    a.exerciseId === b.exerciseId &&
    a.sets === b.sets &&
    a.restSeconds === b.restSeconds &&
    a.side === b.side &&
    a.loadOrIntensity === b.loadOrIntensity &&
    a.instructions === b.instructions &&
    JSON.stringify(a.target) === JSON.stringify(b.target)
  );
}

/**
 * Exercise-level diff of a proposal against the content it was proposed on.
 * Exercises are matched by their stable `planExerciseId`; a create proposal
 * compares against nothing, so every exercise is "added".
 */
export function diffPlanContent(
  base: PlanContent | null | undefined,
  next: PlanContent,
): ProposalChangeView[] {
  const before = flattenExercises(base);
  const after = flattenExercises(next);
  const changes: ProposalChangeView[] = [];
  for (const [id, exercise] of after) {
    const previous = before.get(id);
    if (!previous)
      changes.push({
        kind: "added",
        planExerciseId: id,
        name: exercise.nameSnapshot,
        before: null,
        after: describe(exercise),
      });
    else if (!sameExercise(previous, exercise))
      changes.push({
        kind: "modified",
        planExerciseId: id,
        name: exercise.nameSnapshot,
        before: describe(previous),
        after: describe(exercise),
      });
  }
  for (const [id, exercise] of before)
    if (!after.has(id))
      changes.push({
        kind: "removed",
        planExerciseId: id,
        name: exercise.nameSnapshot,
        before: describe(exercise),
        after: null,
      });
  return changes;
}
