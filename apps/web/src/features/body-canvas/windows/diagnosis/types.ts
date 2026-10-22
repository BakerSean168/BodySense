import type {
  NativeDiagnosisWindowView,
  ProposalChoice,
} from "../../model/diagnosisDecisionViewModel";

export interface DiagnosisDecisionControls {
  /** Local, not yet recorded choices (the serial stepper's state). */
  selection: Record<string, ProposalChoice>;
  onSelect: (proposalId: string, choice: ProposalChoice | null) => void;
  onReview: (proposalId: string) => void;
  /** Records the local choices (plus `final`) and settles the run. */
  onSubmit: (final?: { proposalId: string; choice: ProposalChoice }) => void;
  /** Receipt / No changes: back to the current Diagnosis. */
  onDismissOutcome: () => void;
  /** Empty state: ask for a formal analysis in Chat. */
  onOpenChat?: () => void;
  /** Review prompt / changed run: ask for a new formal analysis in Chat. */
  onRequestAnalysis?: () => void;
  /** §3.6 "暂不处理" on one Condition's review prompt (a UI record only). */
  onDismissReview?: (conditionId: string) => void;
  /** Safety stopped "查看限制": open the Safety / Restricted view. */
  onOpenSafety?: () => void;
  busy?: boolean;
  error?: string | null;
}

export interface DiagnosisWindowProps {
  view: NativeDiagnosisWindowView;
  /** The selector is in the shared Penpot V2 toolbar; the body follows that ID. */
  selectedConditionId?: string | null;
  loading?: boolean;
  error?: string | null;
  partialError?: string | null;
  onRetry?: () => void;
  /** Without controls the window is read-only (proposals are still never facts). */
  controls?: DiagnosisDecisionControls;
  /** Penpot Diagnosis / History instead of the current Diagnosis. */
  scene?: "current" | "history";
}
