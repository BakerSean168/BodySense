/**
 * Diagnosis window view model on the V2 decision API.
 *
 * Sources of truth:
 * - `GET /diagnosis` is the only list of conditions. Only `adopted` entries are
 *   shown as conditions; Agent proposals never are.
 * - The latest formal run carries the Agent proposals. They are AI output, so
 *   they stay proposals until the user decides and Go settles the whole run.
 * - A decision receipt only records a choice. A proposal is `committed` only
 *   when a settlement receipt with outcome `committed` exists for its run.
 *
 * Implementation is split by seam: types/constants, condition and proposal
 * data transforms, run/window decision projection, the confirm stepper, and
 * the session reducers. This file stays the stable import surface.
 */

export {
  EMPTY_DECISION_SESSION,
  PROPOSAL_STATE_LABELS,
} from "./diagnosisDecisionTypes";
export type {
  DiagnosisDecisionSession,
  DiagnosisRunPhase,
  DiagnosisRunView,
  DiagnosisStepDecision,
  DiagnosisStepper,
  NativeDiagnosisWindowView,
  ProjectNativeDiagnosisInput,
  ProposalChoice,
  ProposalRow,
  ProposalUiState,
} from "./diagnosisDecisionTypes";
export { projectAdoptedCondition } from "./diagnosisProposalProjection";
export {
  diagnosisHeaderMode,
  projectNativeDiagnosisWindow,
} from "./diagnosisRunProjection";
export { projectDiagnosisStepper } from "./diagnosisStepper";
export {
  decisionAttemptContent,
  settlementAttemptContent,
  withCommandError,
  withDecisionReceipt,
  withProposalReviewed,
  withSettlementReceipt,
} from "./diagnosisDecisionSession";
