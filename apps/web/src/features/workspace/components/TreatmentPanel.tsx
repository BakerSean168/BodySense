import { useMemo, useState } from "react";
import {
  Activity,
  AlertTriangle,
  Check,
  ClipboardList,
  RefreshCcw,
  Sparkles,
  X,
} from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/Button";
import type { HealthWorkspace, TreatmentRevision } from "../types/workspace";
import { errorMessage } from "@/lib/api-client";
import { TrainingExecutionPanel } from "@/features/training/components/TrainingExecutionPanel";
import {
  useTreatmentCommand,
  type TreatmentCommand,
} from "../hooks/useTreatmentCommand";
import { selectProposedTreatmentRevisions } from "../model/workspaceSelectors";

interface TreatmentPanelProps {
  workspace: HealthWorkspace;
}

const statusLabels: Record<string, string> = {
  active: "执行中",
  review_recommended: "建议审核",
  paused: "已暂停",
  superseded: "已被新版本替代",
  completed: "已结束",
};

function prescriptionText(value: Record<string, unknown>) {
  return Object.entries(value)
    .filter(([, item]) => item !== "" && item !== null && item !== undefined)
    .map(
      ([key, item]) =>
        `${key}: ${Array.isArray(item) ? item.join("、") : String(item)}`,
    )
    .join(" · ");
}

export function TreatmentPanel({ workspace }: TreatmentPanelProps) {
  const treatmentCommand = useTreatmentCommand();
  const [busyKey, setBusyKey] = useState<string | null>(null);
  const [acceptedTrainingPlanId, setAcceptedTrainingPlanId] = useState<string | null>(null);
  const [showTrainingExecution, setShowTrainingExecution] = useState(false);
  const [showOutcome, setShowOutcome] = useState(false);
  const [outcomeDescription, setOutcomeDescription] = useState("");
  const [outcomeTrend, setOutcomeTrend] = useState("stable");
  const [outcomeRegion, setOutcomeRegion] = useState("");
  const [interventionId, setInterventionId] = useState("");

  const proposals = useMemo(
    () => selectProposedTreatmentRevisions(workspace),
    [workspace.treatment_revisions],
  );
  const current = workspace.treatment?.current || null;
  const interventions = current?.interventions || [];
  const trainingPlanId = workspace.training_plan?.id ?? acceptedTrainingPlanId;

  const mutate = async (
    key: string,
    command: TreatmentCommand,
    success: string,
  ): Promise<unknown | null> => {
    setBusyKey(key);
    try {
      const result = await treatmentCommand.mutateAsync(command);
      toast.success(success);
      return result;
    } catch (error) {
      toast.error(errorMessage(error, "方案操作失败"));
      return null;
    } finally {
      setBusyKey(null);
    }
  };

  const generateProposal = async () => {
    const analysisId = workspace.diagnosis?.analysis_id;
    if (!analysisId) {
      toast.error("当前还没有可用于生成方案的分析结果");
      return;
    }
    await mutate(
      "generate",
      { type: "generateProposal", diagnosisAnalysisId: analysisId },
      "新的方案建议已生成，确认采用后才会开始执行",
    );
  };

  const accept = async (revision: TreatmentRevision) => {
    const result = await mutate(
      `accept:${revision.id}`,
      {
        type: "acceptRevision",
        revisionId: revision.id,
        consultationId: workspace.conversation_id,
      },
      "已采用当前方案",
    );
    const accepted = result as {
      training_plan?: { id?: string } | null;
    } | null;
    if (accepted?.training_plan?.id) {
      setAcceptedTrainingPlanId(accepted.training_plan.id);
    }
  };

  const recordOutcome = async () => {
    if (!current || !outcomeDescription.trim()) {
      toast.error("请填写方案执行后的身体变化");
      return;
    }
    const acceptedRevision = current;
    const selected = interventions.find((item) => item.id === interventionId);
    const result = await mutate(
      "outcome",
      {
        type: "recordOutcome",
        input: {
          treatment_id: acceptedRevision.treatment_id,
          treatment_revision_id: acceptedRevision.id,
          intervention_id: selected?.id,
          source_type: "web_checkin",
          source_key: `web-checkin:${crypto.randomUUID()}`,
          kind: "symptom_change",
          concern_key:
            workspace.diagnosis?.candidates?.[0]?.concern_key || "general",
          body_region: outcomeRegion.trim(),
          value: {
            description: outcomeDescription.trim(),
            trend: outcomeTrend,
          },
          notes: outcomeDescription.trim(),
        },
      },
      "身体变化已记录",
    );
    if (result) {
      setOutcomeDescription("");
      setShowOutcome(false);
    }
  };

  return (
    <div className="space-y-5">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <ClipboardList className="h-4 w-4 text-primary" />
          <h3 className="font-semibold text-foreground">当前方案</h3>
        </div>
        {workspace.capabilities.can_generate_treatment && (
          <Button
            size="sm"
            isLoading={busyKey === "generate"}
            onClick={generateProposal}
          >
            <Sparkles className="h-3.5 w-3.5" />生成方案建议
          </Button>
        )}
      </div>

      {workspace.capabilities.requires_treatment_review && (
        <div className="rounded-xl border border-warning/35 bg-warning/10 p-3 text-xs text-warning-foreground">
          <div className="flex items-center gap-2 font-medium">
            <AlertTriangle className="h-4 w-4" />
            当前方案需要审核
          </div>
          {(workspace.treatment?.status_reasons || []).map((reason, index) => (
            <p key={`${reason.code || "reason"}-${index}`} className="mt-1">
              {reason.message || reason.code}
            </p>
          ))}
          <Button
            size="xs"
            variant="outline"
            className="mt-2"
            isLoading={busyKey === "review"}
            onClick={() =>
              mutate(
                "review",
                { type: "reviewCurrent" },
                "已重新计算当前方案审核状态",
              )
            }
          >
            <RefreshCcw className="h-3 w-3" />
            重新检查
          </Button>
        </div>
      )}

      {current ? (
        <div className="rounded-xl border border-border bg-background/70 p-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div>
              <p className="text-sm font-semibold text-foreground">
                {current.goal}
              </p>
              <p className="mt-1 text-xs text-muted-foreground">
                预计 {current.duration_weeks} 周
              </p>
            </div>
            <span className="rounded-full bg-primary/10 px-2 py-1 text-xs text-primary">
              {statusLabels[workspace.treatment?.status || ""] ||
                workspace.treatment?.status}
            </span>
          </div>
          {current.plan?.summary && (
            <p className="mt-3 text-xs leading-5 text-muted-foreground">
              {current.plan.summary}
            </p>
          )}
          <div className="mt-3 space-y-2">
            {interventions.map((intervention) => (
              <div
                key={intervention.id}
                className="rounded-lg bg-muted/40 p-2.5 text-xs"
              >
                <div className="flex items-center gap-2 font-medium text-foreground">
                  <Activity className="h-3.5 w-3.5" />
                  {intervention.title}
                </div>
                <p className="mt-1 text-muted-foreground">
                  {intervention.description}
                </p>
                {prescriptionText(intervention.prescription) && (
                  <p className="mt-1 text-muted-foreground">
                    {prescriptionText(intervention.prescription)}
                  </p>
                )}
              </div>
            ))}
          </div>
          <div className="mt-3 flex flex-wrap gap-2">
            {workspace.capabilities.can_record_outcome && (
              <Button
                size="sm"
                variant="outline"
                onClick={() => setShowOutcome((open) => !open)}
              >
                记录变化
              </Button>
            )}
            {trainingPlanId && (
              <Button
                size="sm"
                variant="secondary"
                onClick={() => setShowTrainingExecution((open) => !open)}
              >
                {showTrainingExecution ? "收起训练执行" : "展开训练执行"}
              </Button>
            )}
          </div>
        </div>
      ) : (
        <div className="py-10 text-center">
          <p className="text-sm font-medium text-foreground/85">还没有当前方案</p>
          <p className="mt-1 text-xs text-muted-foreground">
            生成方案建议后，你可以先查看再决定是否采用。
          </p>
        </div>
      )}

      {showOutcome && current && (
        <div className="grid gap-2 rounded-xl border border-border bg-muted/30 p-3 sm:grid-cols-2">
          <select
            value={interventionId}
            onChange={(event) => setInterventionId(event.target.value)}
            className="rounded-lg border border-border bg-background px-3 py-2 text-sm"
          >
            <option value="">整个方案</option>
            {interventions.map((item) => (
              <option key={item.id} value={item.id}>
                {item.title}
              </option>
            ))}
          </select>
          <input
            value={outcomeRegion}
            onChange={(event) => setOutcomeRegion(event.target.value)}
            placeholder="身体区域"
            className="rounded-lg border border-border bg-background px-3 py-2 text-sm"
          />
          <select
            value={outcomeTrend}
            onChange={(event) => setOutcomeTrend(event.target.value)}
            className="rounded-lg border border-border bg-background px-3 py-2 text-sm"
          >
            <option value="improving">改善</option>
            <option value="stable">稳定</option>
            <option value="worsening">加重</option>
            <option value="unknown">不确定</option>
          </select>
          <textarea
            value={outcomeDescription}
            onChange={(event) => setOutcomeDescription(event.target.value)}
            placeholder="执行方案后，身体有什么变化？"
            rows={2}
            className="rounded-lg border border-border bg-background px-3 py-2 text-sm sm:col-span-2"
          />
          <p className="text-[11px] leading-4 text-muted-foreground sm:col-span-2">
            系统默认只记录时间关联，不会把“训练后发生”自动表述成“由训练导致”。
          </p>
          <div className="flex justify-end gap-2 sm:col-span-2">
            <Button
              size="sm"
              variant="ghost"
              onClick={() => setShowOutcome(false)}
            >
              取消
            </Button>
            <Button
              size="sm"
              isLoading={busyKey === "outcome"}
              onClick={recordOutcome}
            >
              保存变化
            </Button>
          </div>
        </div>
      )}

      {trainingPlanId && showTrainingExecution ? (
        <section className="space-y-3 border-t border-border pt-4">
          <div>
            <h4 className="text-sm font-semibold text-foreground">训练执行</h4>
            <p className="mt-1 text-xs text-muted-foreground">
              训练、打卡、日志与复评都在“方案”工作区内完成，不再跳转到独立页面。
            </p>
          </div>
          <TrainingExecutionPanel planId={trainingPlanId} />
        </section>
      ) : null}

      {proposals.length > 0 && (
        <section className="space-y-2">
          <h4 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            待确认方案 · {proposals.length}
          </h4>
          {proposals.map((revision) => (
            <div
              key={revision.id}
              className="rounded-xl border border-dashed border-primary/40 p-3"
            >
              <p className="text-sm font-medium text-foreground">
                {revision.goal}
              </p>
              <div className="mt-3 flex flex-wrap gap-2">
                <Button
                  size="sm"
                  isLoading={busyKey === `accept:${revision.id}`}
                  onClick={() => accept(revision)}
                >
                  <Check className="h-3.5 w-3.5" />
                  采用此方案
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  isLoading={busyKey === `reject:${revision.id}`}
                  onClick={() =>
                    mutate(
                      `reject:${revision.id}`,
                      { type: "rejectRevision", revisionId: revision.id },
                      "已暂不采用这份方案，历史仍会保留",
                    )
                  }
                >
                  <X className="h-3.5 w-3.5" />
                  暂不采用
                </Button>
              </div>
            </div>
          ))}
        </section>
      )}
    </div>
  );
}
