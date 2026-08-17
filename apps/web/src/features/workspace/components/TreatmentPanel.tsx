import { useMemo, useState } from "react";
import { Link } from "react-router";
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
import { Card } from "@/components/ui/Card";
import type { HealthWorkspace, TreatmentRevision } from "../types/workspace";
import { workspaceApi } from "../services/workspaceService";

interface TreatmentPanelProps {
  workspace: HealthWorkspace;
  onChanged: () => Promise<unknown> | unknown;
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

export function TreatmentPanel({ workspace, onChanged }: TreatmentPanelProps) {
  const [busyKey, setBusyKey] = useState<string | null>(null);
  const [acceptedTrainingPlanId, setAcceptedTrainingPlanId] = useState<
    string | null
  >(null);
  const [showOutcome, setShowOutcome] = useState(false);
  const [outcomeDescription, setOutcomeDescription] = useState("");
  const [outcomeTrend, setOutcomeTrend] = useState("stable");
  const [outcomeRegion, setOutcomeRegion] = useState("");
  const [interventionId, setInterventionId] = useState("");

  const proposals = useMemo(
    () =>
      workspace.treatment_revisions.filter(
        (revision) => revision.acceptance_state === "proposed",
      ),
    [workspace.treatment_revisions],
  );
  const current = workspace.treatment?.current || null;
  const interventions = current?.interventions || [];
  const trainingPlanId = workspace.training_plan?.id ?? acceptedTrainingPlanId;

  const mutate = async (
    key: string,
    operation: () => Promise<unknown>,
    success: string,
  ): Promise<unknown | null> => {
    setBusyKey(key);
    try {
      const result = await operation();
      await onChanged();
      toast.success(success);
      return result;
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : "Treatment 操作失败",
      );
      return null;
    } finally {
      setBusyKey(null);
    }
  };

  const generateProposal = async () => {
    const analysisId = workspace.diagnosis?.analysis_id;
    if (!analysisId) {
      toast.error("当前没有可固定的 DiagnosisAnalysis");
      return;
    }
    await mutate(
      "generate",
      () => workspaceApi.generateTreatmentProposal(analysisId),
      "已创建方案 proposal；需要明确接受后才会执行",
    );
  };

  const accept = async (revision: TreatmentRevision) => {
    const result = await mutate(
      `accept:${revision.id}`,
      () =>
        workspaceApi.acceptTreatmentRevision(
          revision.id,
          workspace.conversation_id,
        ),
      `已接受 Treatment R${revision.revision}`,
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
      toast.error("请填写干预后的变化");
      return;
    }
    const acceptedRevision = current;
    const selected = interventions.find((item) => item.id === interventionId);
    const result = await mutate(
      "outcome",
      () =>
        workspaceApi.recordOutcome({
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
        }),
      "Outcome 已记录，并进入 BodyState / review policy",
    );
    if (result) {
      setOutcomeDescription("");
      setShowOutcome(false);
    }
  };

  return (
    <Card className="space-y-4 p-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <div className="flex items-center gap-2">
            <ClipboardList className="h-4 w-4 text-primary" />
            <h3 className="font-semibold text-foreground">
              Treatment revisions
            </h3>
          </div>
          <p className="mt-1 text-xs leading-5 text-muted-foreground">
            AI 只能生成 proposal。只有你明确接受的 revision 才会成为当前方案。
          </p>
        </div>
        {workspace.capabilities.can_generate_treatment && (
          <Button
            size="sm"
            isLoading={busyKey === "generate"}
            onClick={generateProposal}
          >
            <Sparkles className="h-3.5 w-3.5" />新 proposal
          </Button>
        )}
      </div>

      {workspace.capabilities.requires_treatment_review && (
        <div className="rounded-xl border border-amber-200 bg-amber-50 p-3 text-xs text-amber-800 dark:border-amber-900/50 dark:bg-amber-950/20 dark:text-amber-200">
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
                workspaceApi.reviewCurrentTreatment,
                "已重新计算当前方案审核状态",
              )
            }
          >
            <RefreshCcw className="h-3 w-3" />
            重新评估状态
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
                Treatment R{current.revision} · BodyState R
                {current.source_body_state_revision} · {current.duration_weeks}{" "}
                周
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
                记录 Outcome
              </Button>
            )}
            {trainingPlanId && (
              <Button
                size="sm"
                variant="secondary"
                render={<Link to={`/training/${trainingPlanId}`} />}
              >
                打开训练执行页
              </Button>
            )}
          </div>
        </div>
      ) : (
        <p className="rounded-xl border border-dashed border-border p-4 text-center text-xs text-muted-foreground">
          还没有被用户接受的当前方案。
        </p>
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
            placeholder="干预后发生了什么？"
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
              保存 Outcome
            </Button>
          </div>
        </div>
      )}

      {proposals.length > 0 && (
        <section className="space-y-2">
          <h4 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            待审核 proposal · {proposals.length}
          </h4>
          {proposals.map((revision) => (
            <div
              key={revision.id}
              className="rounded-xl border border-dashed border-primary/40 p-3"
            >
              <p className="text-sm font-medium text-foreground">
                {revision.goal}
              </p>
              <p className="mt-1 text-xs text-muted-foreground">
                Proposal R{revision.revision} · 基于 BodyState R
                {revision.source_body_state_revision}
              </p>
              <div className="mt-3 flex flex-wrap gap-2">
                <Button
                  size="sm"
                  isLoading={busyKey === `accept:${revision.id}`}
                  onClick={() => accept(revision)}
                >
                  <Check className="h-3.5 w-3.5" />
                  明确接受
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  isLoading={busyKey === `reject:${revision.id}`}
                  onClick={() =>
                    mutate(
                      `reject:${revision.id}`,
                      () => workspaceApi.rejectTreatmentRevision(revision.id),
                      "已拒绝该 proposal，历史仍保留",
                    )
                  }
                >
                  <X className="h-3.5 w-3.5" />
                  拒绝
                </Button>
              </div>
            </div>
          ))}
        </section>
      )}
    </Card>
  );
}
