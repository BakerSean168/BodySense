import { Stethoscope } from "lucide-react";
import { SelectField } from "@/components/ui/select";

export interface DiagnosisWindowHeaderProps {
  /** Only adopted ConditionEntries; proposals never appear in the picker. */
  view: { confirmed: readonly { conditionId: string; name: string }[] };
  selectedConditionId: string | null;
  onSelectCondition: (conditionId: string) => void;
  /**
   * Penpot Diagnosis boards: a formal run shows "正式分析" and its settled
   * outcome "提交回执" in the toolbar instead of the condition picker.
   */
  mode?: "analysis" | "receipt" | null;
  /** Penpot Diagnosis / History: the picker reads "历史". */
  history?: boolean;
  onOpenHistory?: () => void;
  onCloseHistory?: () => void;
}

const HISTORY_OPTION = "__history__";

const MODE_LABELS = { analysis: "正式分析", receipt: "提交回执" } as const;

/** Penpot "Window / Title Diagnosis": 🩺 诊断 / <selector>. */
function DiagnosisTitle({ separator = true }: { separator?: boolean }) {
  return (
    <>
      <span className="bc-window-context-title">
        <Stethoscope size={18} aria-hidden="true" />
        诊断
      </span>
      {separator ? (
        <span className="bc-window-context-sep" aria-hidden="true">
          /
        </span>
      ) : null}
    </>
  );
}

/** Penpot V2: condition picker belongs to the window toolbar, not the scroll body. */
export function DiagnosisWindowHeader({
  view,
  selectedConditionId,
  onSelectCondition,
  mode = null,
  history = false,
  onOpenHistory,
  onCloseHistory,
}: DiagnosisWindowHeaderProps) {
  if (mode && !history)
    return (
      <div className="bc-window-context bc-diagnosis-context">
        <DiagnosisTitle />
        <span
          className="bc-window-select-wrap bc-window-context-mode"
          data-testid="diagnosis-mode"
        >
          {MODE_LABELS[mode]}
        </span>
      </div>
    );

  const selected = view.confirmed.some(
    (item) => item.conditionId === selectedConditionId,
  )
    ? selectedConditionId!
    : (view.confirmed[0]?.conditionId ?? "");

  if (history && view.confirmed.length === 0)
    return (
      <div className="bc-window-context bc-diagnosis-context">
        <DiagnosisTitle />
        <span
          className="bc-window-select-wrap bc-window-context-mode"
          data-testid="diagnosis-mode"
        >
          历史
        </span>
      </div>
    );

  // Penpot Diagnosis / Empty: only the window title, no empty picker.
  if (view.confirmed.length === 0)
    return (
      <div className="bc-window-context bc-diagnosis-context">
        <DiagnosisTitle separator={false} />
      </div>
    );

  return (
    <div className="bc-window-context bc-diagnosis-context">
      <DiagnosisTitle />
      <span className="bc-window-select-wrap">
        <SelectField
          variant="ghost"
          className="bc-window-context-select"
          aria-label="选择已确认病情"
          value={history ? HISTORY_OPTION : selected}
          disabled={view.confirmed.length === 0}
          options={[
            ...view.confirmed.map((item) => ({
              value: item.conditionId,
              label: item.name,
            })),
            ...(onOpenHistory
              ? [{ value: HISTORY_OPTION, label: "历史" }]
              : []),
          ]}
          onValueChange={(next) => {
            if (next === HISTORY_OPTION) {
              onOpenHistory?.();
              return;
            }
            onSelectCondition(next);
            onCloseHistory?.();
          }}
        />
      </span>
    </div>
  );
}
