import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { ApiRequestError } from "@/lib/api-client";
import {
  formatPlanTarget,
  trainingApi,
  type TrainingPlanRecord,
  type TrainingSessionRecord,
} from "../services/trainingService";

interface TrainingExecutionPanelProps {
  planId: string;
}

/**
 * Minimal Trainer bound to the Go runtime (`/training-sessions`, TP-003).
 * Every set is an explicit user action with a receipt; nothing is inferred
 * from navigation. The full Trainer design (player, pause/resume, corrections)
 * is UI-TP-001.
 */
export function TrainingExecutionPanel({
  planId,
}: TrainingExecutionPanelProps) {
  const [plan, setPlan] = useState<TrainingPlanRecord | null>(null);
  const [session, setSession] = useState<TrainingSessionRecord | null>(null);
  const [templateId, setTemplateId] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [feedback, setFeedback] = useState("");

  const refresh = useCallback(async () => {
    const [planRecord, sessions] = await Promise.all([
      trainingApi.getPlanRecord(planId),
      trainingApi.listSessions(),
    ]);
    setPlan(planRecord);
    const live = sessions.find(
      (item) => item.planId === planId && item.state !== "ended",
    );
    setSession(
      (current) => live ?? (current?.planId === planId ? current : null),
    );
    setTemplateId(
      (current) =>
        current ?? planRecord.currentContent.sessions[0]?.templateId ?? null,
    );
  }, [planId]);

  useEffect(() => {
    let cancelled = false;
    void refresh()
      .catch(() => {
        if (!cancelled) setPlan(null);
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [refresh]);

  const run = async (action: () => Promise<void>) => {
    setBusy(true);
    try {
      await action();
    } catch (error) {
      const code = error instanceof ApiRequestError ? error.code : undefined;
      if (code === "SAFETY_STALE" || code === "TRAINER_GATE_BLOCKED") {
        toast.error("安全状态已变化或不允许继续，请查看最新提醒后再确认。");
      } else {
        toast.error("操作没有被记录，请刷新后重试。");
      }
      await refresh().catch(() => undefined);
    } finally {
      setBusy(false);
    }
  };

  if (isLoading) {
    return (
      <div className="flex min-h-64 items-center justify-center">
        <p className="text-sm text-muted-foreground" role="status">
          正在加载训练计划…
        </p>
      </div>
    );
  }

  if (!plan) {
    return (
      <Card className="border-dashed p-6 text-center">
        <p className="text-sm font-semibold text-foreground">
          训练计划暂时不可用
        </p>
        <p className="mt-1 text-xs text-muted-foreground">
          请稍后重试或重新审核当前方案。
        </p>
      </Card>
    );
  }

  const templates = plan.currentContent.sessions;
  const activeTemplateId = session?.templateId ?? templateId;
  const template = templates.find(
    (item) => item.templateId === activeTemplateId,
  );
  const gate = plan.executionGate;
  const effective = new Map<string, string>();
  if (session) {
    const retracted = new Set(
      session.actions.flatMap((a) =>
        a.correctsActionId ? [a.correctsActionId] : [],
      ),
    );
    for (const action of session.actions) {
      if (action.kind === "corrected" || retracted.has(action.actionId))
        continue;
      effective.set(
        `${action.planExerciseId}:${action.setNumber ?? "all"}`,
        action.kind,
      );
    }
  }
  const live = session !== null && session.state !== "ended";

  const start = () =>
    run(async () => {
      if (!activeTemplateId) return;
      setSession(await trainingApi.startSession(plan, activeTemplateId));
    });
  const recordSet = (
    planExerciseId: string,
    setNumber: number,
    kind: "performed" | "skipped",
  ) =>
    run(async () => {
      if (!session) return;
      await trainingApi.recordSet(
        session,
        planExerciseId,
        setNumber,
        kind,
        gate.safetyRevision,
      );
      await refresh();
    });
  const finish = (reason: "finished" | "stopped_early") =>
    run(async () => {
      if (!session) return;
      setSession(await trainingApi.finishSession(session, reason));
    });
  const submitFeedback = () =>
    run(async () => {
      if (!session || !feedback.trim()) return;
      setSession(await trainingApi.recordFeedback(session.sessionId, feedback));
      toast.success("反馈已记录，仅用于计划复核提示，不会自动修改计划。");
    });

  return (
    <div className="w-full space-y-6" data-testid="trainer-session-panel">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">
          训练计划 (Training Plan)
        </h1>
        <p className="text-sm text-muted-foreground">
          {plan.title || plan.goal}
        </p>
      </div>

      {!gate.permitsExecution && (
        <Card className="p-4 text-sm" role="alert">
          当前安全状态不允许开始或继续训练（{gate.result}
          ）。已记录的内容会保留。
        </Card>
      )}

      {!session && (
        <Card className="space-y-3 p-4">
          <label
            className="block text-sm font-semibold"
            htmlFor="trainer-template"
          >
            选择本次训练
          </label>
          <select
            id="trainer-template"
            className="w-full rounded-md border p-2 text-sm"
            value={activeTemplateId ?? ""}
            onChange={(event) => setTemplateId(event.target.value)}
          >
            {templates.map((item) => (
              <option key={item.templateId} value={item.templateId}>
                {item.title}
              </option>
            ))}
          </select>
          <Button
            onClick={start}
            disabled={busy || !gate.permitsExecution || !activeTemplateId}
          >
            开始本次训练
          </Button>
        </Card>
      )}

      {template && (
        <Card className="space-y-4 p-4">
          <h2 className="text-lg font-semibold">{template.title}</h2>
          {template.stages.map((stage, stageIndex) => (
            <div key={`${stage.phase}-${stageIndex}`} className="space-y-3">
              {stage.exercises.map((exercise) => (
                <div
                  key={exercise.planExerciseId}
                  className="rounded-md border p-3"
                >
                  <p className="font-medium">{exercise.nameSnapshot}</p>
                  <p className="text-xs text-muted-foreground">
                    {exercise.sets} 组 ·{" "}
                    {formatPlanTarget(exercise) || "目标未提供"}
                  </p>
                  {session && (
                    <ul className="mt-2 space-y-1">
                      {Array.from(
                        { length: exercise.sets },
                        (_, index) => index + 1,
                      ).map((setNumber) => {
                        const recorded =
                          effective.get(
                            `${exercise.planExerciseId}:${setNumber}`,
                          ) ?? effective.get(`${exercise.planExerciseId}:all`);
                        return (
                          <li
                            key={setNumber}
                            className="flex items-center gap-2 text-sm"
                          >
                            <span>第 {setNumber} 组</span>
                            {recorded ? (
                              <span>
                                {recorded === "performed" ? "已完成" : "已跳过"}
                              </span>
                            ) : live ? (
                              <>
                                <Button
                                  size="sm"
                                  disabled={
                                    busy || session.state !== "in_progress"
                                  }
                                  onClick={() =>
                                    recordSet(
                                      exercise.planExerciseId,
                                      setNumber,
                                      "performed",
                                    )
                                  }
                                >
                                  完成
                                </Button>
                                <Button
                                  size="sm"
                                  variant="outline"
                                  disabled={
                                    busy || session.state !== "in_progress"
                                  }
                                  onClick={() =>
                                    recordSet(
                                      exercise.planExerciseId,
                                      setNumber,
                                      "skipped",
                                    )
                                  }
                                >
                                  跳过
                                </Button>
                              </>
                            ) : (
                              <span className="text-muted-foreground">
                                未记录
                              </span>
                            )}
                          </li>
                        );
                      })}
                    </ul>
                  )}
                </div>
              ))}
            </div>
          ))}
          {live && (
            <div className="flex gap-2">
              <Button disabled={busy} onClick={() => finish("finished")}>
                结束训练
              </Button>
              <Button
                variant="outline"
                disabled={busy}
                onClick={() => finish("stopped_early")}
              >
                提前结束
              </Button>
            </div>
          )}
        </Card>
      )}

      {session?.state === "ended" && (
        <Card className="space-y-3 p-4">
          <p className="text-sm font-semibold">
            本次训练已{session.endedReason === "finished" ? "结束" : "提前结束"}
          </p>
          {session.feedback ? (
            <p
              className="whitespace-pre-wrap text-sm"
              data-testid="trainer-session-feedback"
            >
              {session.feedback.text}
            </p>
          ) : (
            <>
              <label className="block text-sm" htmlFor="trainer-feedback">
                本次感受（原话保存，只用于计划复核提示；身体不适请在对话中告诉我们）
              </label>
              <textarea
                id="trainer-feedback"
                className="w-full rounded-md border p-2 text-sm"
                maxLength={2000}
                rows={3}
                value={feedback}
                onChange={(event) => setFeedback(event.target.value)}
              />
              <Button
                disabled={busy || !feedback.trim()}
                onClick={submitFeedback}
              >
                提交反馈
              </Button>
            </>
          )}
          <Button
            variant="outline"
            disabled={busy}
            onClick={() => setSession(null)}
          >
            返回
          </Button>
        </Card>
      )}
    </div>
  );
}
