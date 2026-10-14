import { useMemo, useState } from "react";
import { AlertTriangle, Check, RefreshCcw, X } from "lucide-react";
import { toast } from "sonner";
import { errorMessage } from "@/lib/api-client";
import { selectProposedTreatmentRevisions } from "@/features/workspace/model/workspaceSelectors";
import {
  useTreatmentCommand,
  type TreatmentCommand,
} from "@/features/workspace/hooks/useTreatmentCommand";
import type { HealthWorkspace } from "@/features/workspace/types/workspace";

/**
 * Plan authorization section for the TrainingPlan window.
 *
 * Product contract (docs/product/domain.md §4, acceptance TP-03..TP-05):
 * an Agent suggestion is only a Proposal. Plan content changes exclusively when
 * the user accepts that exact proposal, and a rejected, closed or expired
 * proposal must leave every plan unchanged.
 *
 * This is deliberately the only place that mutates plan content, so the
 * authorization boundary is visible in one component instead of spread across
 * the window.
 */
export interface PlanProposalsProps {
  workspace: HealthWorkspace;
  /** Called after a successful acceptance so the window can select that plan. */
  onAccepted?: (planId: string) => void;
}

export function PlanProposals({ workspace, onAccepted }: PlanProposalsProps) {
  const command = useTreatmentCommand();
  const [busyKey, setBusyKey] = useState<string | null>(null);
  const proposals = useMemo(
    () => selectProposedTreatmentRevisions(workspace),
    [workspace],
  );

  const mutate = async (
    key: string,
    treatmentCommand: TreatmentCommand,
    success: string,
  ) => {
    setBusyKey(key);
    try {
      const result = await command.mutateAsync(treatmentCommand);
      toast.success(success);
      return result;
    } catch (error) {
      toast.error(errorMessage(error, "方案操作失败"));
      return null;
    } finally {
      setBusyKey(null);
    }
  };

  const reviewReasons = workspace.treatment?.status_reasons ?? [];
  const needsReview = workspace.capabilities.requires_treatment_review;

  return (
    <section className="bc-plan-proposals" aria-label="方案建议与授权">
      {needsReview ? (
        <div role="alert" className="bc-window-alert">
          <AlertTriangle size={16} />
          <span>
            <strong>当前方案需要审核</strong>
            {reviewReasons.map((reason, index) => (
              <span key={`${reason.code ?? "reason"}-${index}`} className="bc-block">
                {reason.message ?? reason.code}
              </span>
            ))}
          </span>
          <button
            type="button"
            className="bc-text-button"
            disabled={busyKey === "review"}
            onClick={() =>
              void mutate(
                "review",
                { type: "reviewCurrent" },
                "已重新计算当前方案审核状态",
              )
            }
          >
            <RefreshCcw size={13} />
            重新检查
          </button>
        </div>
      ) : null}

      {proposals.length === 0 ? null : (
        <>
          <h3 className="bc-stage-title">待确认方案 · {proposals.length}</h3>
          <p className="bc-window-note">
            只有你明确采用后才会写入计划；暂不采用、关闭或超时都不会改变任何计划内容。
          </p>
          {proposals.map((revision) => (
            <article key={revision.id} className="bc-proposal" data-proposal-id={revision.id}>
              <div className="bc-proposal-head">
                <strong>{revision.goal}</strong>
                <span className="bc-plan-option-status">
                  {revision.duration_weeks} 周
                </span>
              </div>
              {revision.plan?.summary ? (
                <p className="bc-exercise-note">{revision.plan.summary}</p>
              ) : null}
              {revision.interventions.length > 0 ? (
                <ul className="bc-proposal-actions">
                  {revision.interventions.map((intervention) => (
                    <li key={intervention.id}>{intervention.title}</li>
                  ))}
                </ul>
              ) : null}
              <div className="bc-proposal-buttons">
                <button
                  type="button"
                  className="bc-button bc-primary"
                  disabled={busyKey === `accept:${revision.id}`}
                  onClick={async () => {
                    const result = await mutate(
                      `accept:${revision.id}`,
                      {
                        type: "acceptRevision",
                        revisionId: revision.id,
                        consultationId: workspace.conversation_id,
                      },
                      "已采用这份方案",
                    );
                    if (
                      result &&
                      typeof result === "object" &&
                      "training_plan" in result &&
                      result.training_plan &&
                      typeof result.training_plan === "object" &&
                      "id" in result.training_plan &&
                      typeof result.training_plan.id === "string"
                    ) {
                      onAccepted?.(result.training_plan.id);
                    }
                  }}
                >
                  <Check size={15} />
                  采用此方案
                </button>
                <button
                  type="button"
                  className="bc-button"
                  disabled={busyKey === `reject:${revision.id}`}
                  onClick={() =>
                    void mutate(
                      `reject:${revision.id}`,
                      { type: "rejectRevision", revisionId: revision.id },
                      "已暂不采用，计划内容保持不变",
                    )
                  }
                >
                  <X size={15} />
                  暂不采用
                </button>
              </div>
            </article>
          ))}
        </>
      )}
    </section>
  );
}
