import type {
  DiagnosisRunView,
  DiagnosisStepper,
  DiagnosisStepDecision,
  ProposalChoice,
  ProposalRow,
} from "./diagnosisDecisionTypes";

export function projectDiagnosisStepper(
  run: DiagnosisRunView,
  selection: Readonly<Record<string, ProposalChoice>>,
): DiagnosisStepper {
  const decided: DiagnosisStepDecision[] = [];
  const open: ProposalRow[] = [];
  for (const row of run.proposals) {
    const local = row.selectable ? selection[row.proposalId] : undefined;
    if (local) decided.push({ row, choice: local, changeable: true });
    else if (row.state === "pending" || row.state === "committed")
      decided.push({ row, choice: "accept", changeable: false });
    else if (row.state === "rejected")
      decided.push({ row, choice: "reject", changeable: false });
    else open.push(row);
  }
  const current = open[0] ?? null;
  const total = run.proposals.length;
  return {
    step: Math.min(decided.length + 1, total),
    total,
    decided,
    current,
    next: open[1] ?? null,
    isLast: open.length <= 1,
    anyAccepted: decided.some((item) => item.choice === "accept"),
    canSubmitEarly: decided.length > 0 && open.length > 1,
  };
}
