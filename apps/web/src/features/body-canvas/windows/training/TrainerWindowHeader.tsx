import { TrainingIcon } from "../../components/TrainingIcon";

/** Penpot "Window / Title Trainer": 训练 / <the session's template>. */
export function TrainerWindowHeader({
  templateTitle,
}: {
  templateTitle: string | null;
}) {
  return (
    <div className="bc-window-context bc-training-context">
      <span className="bc-window-context-title">
        <TrainingIcon size={18} aria-hidden="true" />
        训练
      </span>
      {templateTitle ? (
        <>
          <span className="bc-window-context-sep" aria-hidden="true">
            /
          </span>
          <span
            className="bc-window-select-wrap bc-window-context-mode"
            data-testid="trainer-template"
          >
            {templateTitle}
          </span>
        </>
      ) : null}
    </div>
  );
}
