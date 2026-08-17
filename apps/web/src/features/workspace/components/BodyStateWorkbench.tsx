import { useMemo, useState } from "react";
import {
  AlertTriangle,
  Check,
  CirclePlus,
  History,
  Lightbulb,
  PencilLine,
  ShieldCheck,
  TrendingDown,
  TrendingUp,
  X,
} from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import type {
  BodyStateFact,
  BodyStateObservation,
  BodyStateSnapshot,
} from "@/features/consultation/types/consultation";
import { workspaceApi } from "../services/workspaceService";

interface BodyStateWorkbenchProps {
  snapshot: BodyStateSnapshot;
  onChanged: () => Promise<unknown> | unknown;
}

const factKindLabels: Record<string, string> = {
  discomfort: "不适 / 症状",
  limitation: "活动受限",
  lifestyle: "生活方式",
  negative_finding: "阴性发现",
  red_flags: "安全信号",
  safety_finding: "安全发现",
};

const trendLabels: Record<string, string> = {
  unknown: "趋势未知",
  stable: "基本稳定",
  improving: "正在改善",
  worsening: "正在加重",
};

function safetyState(snapshot: BodyStateSnapshot) {
  const raw = snapshot.safety_state || {};
  return {
    active:
      raw.has_red_flags === true &&
      (raw.status === "requires_review" || raw.status === "active"),
    status: typeof raw.status === "string" ? raw.status : "",
  };
}

export function BodyStateWorkbench({
  snapshot,
  onChanged,
}: BodyStateWorkbenchProps) {
  const [busyKey, setBusyKey] = useState<string | null>(null);
  const [showAdd, setShowAdd] = useState(false);
  const [kind, setKind] = useState("discomfort");
  const [bodyRegion, setBodyRegion] = useState("");
  const [value, setValue] = useState("");
  const [correction, setCorrection] = useState<{
    fact: BodyStateFact;
    value: string;
  } | null>(null);
  const [safetyNote, setSafetyNote] = useState("");
  const safety = useMemo(() => safetyState(snapshot), [snapshot]);

  const mutate = async (
    key: string,
    operation: () => Promise<unknown>,
    success: string,
  ) => {
    setBusyKey(key);
    try {
      await operation();
      await onChanged();
      toast.success(success);
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : "更新 BodyState 失败",
      );
    } finally {
      setBusyKey(null);
    }
  };

  const handleAddFact = async () => {
    if (!value.trim()) {
      toast.error("请填写事实内容");
      return;
    }
    await mutate(
      "add-fact",
      () =>
        workspaceApi.addFact(snapshot.current_revision, {
          kind,
          body_region: bodyRegion.trim(),
          concern_key: bodyRegion.trim()
            ? `region:${bodyRegion.trim()}`
            : "general",
          value: value.trim(),
          origin: "user_reported",
          review_state: "confirmed",
          lifecycle_state: "active",
          trend: "unknown",
          observed_at: new Date().toISOString(),
        }),
      "已作为新的当前事实记录",
    );
    setValue("");
    setBodyRegion("");
    setShowAdd(false);
  };

  const handleCorrection = async () => {
    if (!correction || !correction.value.trim()) return;
    const fact = correction.fact;
    await mutate(
      `correct:${fact.id}`,
      () =>
        workspaceApi.correctFact(fact.id, snapshot.current_revision, {
          concern_key: fact.concern_key || "general",
          kind: fact.kind,
          body_region: fact.body_region || "",
          value: correction.value.trim(),
          details: fact.details || {},
          origin: "user_edited",
          review_state: "confirmed",
          lifecycle_state: "active",
          trend: fact.trend || "unknown",
          observed_at: fact.observed_at || new Date().toISOString(),
        }),
      "已保留旧记录，并创建纠正后的事实",
    );
    setCorrection(null);
  };

  const hypotheses = snapshot.hypotheses ?? [];
  const pendingObservations = snapshot.pending_observations ?? [];
  const activeFacts = snapshot.facts.filter(
    (fact) => fact.lifecycle_state === "active",
  );
  const historicalFacts = snapshot.facts.filter(
    (fact) => fact.lifecycle_state !== "active",
  );

  return (
    <Card className="space-y-4 p-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <div className="flex items-center gap-2">
            <History className="h-4 w-4 text-primary" />
            <h3 className="font-semibold text-foreground">长期 BodyState</h3>
            <span className="rounded-full bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">
              R{snapshot.current_revision}
            </span>
          </div>
          <p className="mt-1 text-xs leading-5 text-muted-foreground">
            事实、观察和假设分开保存。确认、纠正记录、身体后来变化是三种不同操作。
          </p>
        </div>
        <Button
          size="sm"
          variant="outline"
          onClick={() => setShowAdd((open) => !open)}
        >
          <CirclePlus className="h-3.5 w-3.5" />
          新事实
        </Button>
      </div>

      {safety.active && (
        <div className="rounded-xl border border-red-200 bg-red-50 p-3 text-sm text-red-800 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-200">
          <div className="flex items-center gap-2 font-medium">
            <AlertTriangle className="h-4 w-4" />
            当前安全状态需要优先审核
          </div>
          <p className="mt-1 text-xs leading-5 opacity-90">
            普通 Diagnosis 和 Treatment
            已被确定性业务规则阻断。只有明确审核后才能解除。
          </p>
          <div className="mt-3 flex flex-col gap-2 sm:flex-row">
            <input
              value={safetyNote}
              onChange={(event) => setSafetyNote(event.target.value)}
              placeholder="记录审核依据（建议填写）"
              className="min-w-0 flex-1 rounded-lg border border-red-200 bg-background px-3 py-2 text-xs text-foreground outline-none focus:border-red-400"
            />
            <Button
              size="sm"
              variant="outline"
              isLoading={busyKey === "resolve-safety"}
              onClick={() =>
                mutate(
                  "resolve-safety",
                  () =>
                    workspaceApi.resolveSafety(
                      snapshot.current_revision,
                      "cleared_by_review",
                      safetyNote.trim(),
                    ),
                  "安全状态已通过明确审核更新",
                )
              }
            >
              <ShieldCheck className="h-3.5 w-3.5" />
              审核并解除
            </Button>
          </div>
        </div>
      )}

      {showAdd && (
        <div className="grid gap-2 rounded-xl border border-border bg-muted/30 p-3 sm:grid-cols-2">
          <select
            value={kind}
            onChange={(event) => setKind(event.target.value)}
            className="rounded-lg border border-border bg-background px-3 py-2 text-sm"
          >
            {Object.entries(factKindLabels).map(([key, label]) => (
              <option key={key} value={key}>
                {label}
              </option>
            ))}
          </select>
          <input
            value={bodyRegion}
            onChange={(event) => setBodyRegion(event.target.value)}
            placeholder="身体区域，例如：颈肩"
            className="rounded-lg border border-border bg-background px-3 py-2 text-sm"
          />
          <textarea
            value={value}
            onChange={(event) => setValue(event.target.value)}
            placeholder="当前事实内容"
            rows={2}
            className="rounded-lg border border-border bg-background px-3 py-2 text-sm sm:col-span-2"
          />
          <div className="flex justify-end gap-2 sm:col-span-2">
            <Button size="sm" variant="ghost" onClick={() => setShowAdd(false)}>
              取消
            </Button>
            <Button
              size="sm"
              isLoading={busyKey === "add-fact"}
              onClick={handleAddFact}
            >
              记录事实
            </Button>
          </div>
        </div>
      )}

      {pendingObservations.length > 0 && (
        <section className="space-y-2 rounded-xl border border-amber-200 bg-amber-50/60 p-3 dark:border-amber-900/50 dark:bg-amber-950/20">
          <div>
            <h4 className="text-xs font-semibold uppercase tracking-wide text-amber-800 dark:text-amber-200">
              待审核 Observation · {pendingObservations.length}
            </h4>
            <p className="mt-1 text-xs leading-5 text-amber-700 dark:text-amber-300">
              来自图片或 AI 评估的观察在确认前不会进入 Diagnosis reasoning。
            </p>
          </div>
          {pendingObservations.map((observation: BodyStateObservation) => {
            const label =
              typeof observation.value?.label === "string"
                ? observation.value.label
                : observation.kind;
            const description =
              typeof observation.value?.description === "string"
                ? observation.value.description
                : JSON.stringify(observation.value);
            return (
              <div
                key={observation.id}
                className="rounded-lg border border-amber-200 bg-background p-3 dark:border-amber-900/50"
              >
                <div className="text-xs text-muted-foreground">
                  {observation.body_region || "全身"} ·{" "}
                  {observation.method || "assessment"}
                </div>
                <p className="mt-1 text-sm font-medium text-foreground">
                  {label}
                </p>
                <p className="mt-1 text-xs leading-5 text-muted-foreground">
                  {description}
                </p>
                <div className="mt-3 flex flex-wrap gap-2">
                  <Button
                    size="xs"
                    variant="secondary"
                    isLoading={
                      busyKey === `confirm-observation:${observation.id}`
                    }
                    onClick={() =>
                      mutate(
                        `confirm-observation:${observation.id}`,
                        () =>
                          workspaceApi.reviewObservation(
                            observation.id,
                            snapshot.current_revision,
                            "confirmed",
                          ),
                        "Observation 已确认并进入长期 BodyState reasoning",
                      )
                    }
                  >
                    <Check className="h-3 w-3" />
                    确认观察
                  </Button>
                  <Button
                    size="xs"
                    variant="ghost"
                    isLoading={
                      busyKey === `reject-observation:${observation.id}`
                    }
                    onClick={() =>
                      mutate(
                        `reject-observation:${observation.id}`,
                        () =>
                          workspaceApi.reviewObservation(
                            observation.id,
                            snapshot.current_revision,
                            "rejected",
                          ),
                        "Observation 已拒绝，不会进入 reasoning",
                      )
                    }
                  >
                    <X className="h-3 w-3" />
                    不接受
                  </Button>
                </div>
              </div>
            );
          })}
        </section>
      )}

      <section className="space-y-2">
        <div className="flex items-center justify-between">
          <h4 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            当前事实 · {activeFacts.length}
          </h4>
        </div>
        {activeFacts.length === 0 ? (
          <p className="rounded-xl border border-dashed border-border p-4 text-center text-xs text-muted-foreground">
            还没有当前事实。可以继续对话，也可以手动添加。
          </p>
        ) : (
          activeFacts.map((fact) => (
            <div
              key={fact.id}
              className="rounded-xl border border-border bg-background/70 p-3"
            >
              <div className="flex flex-wrap items-start justify-between gap-2">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-1.5 text-[11px] text-muted-foreground">
                    <span>{factKindLabels[fact.kind] || fact.kind}</span>
                    {fact.body_region && <span>· {fact.body_region}</span>}
                    <span>· {trendLabels[fact.trend] || fact.trend}</span>
                    <span
                      className={`rounded-full px-1.5 py-0.5 ${
                        fact.review_state === "confirmed"
                          ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300"
                          : "bg-amber-100 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300"
                      }`}
                    >
                      {fact.review_state === "confirmed" ? "已确认" : "待确认"}
                    </span>
                  </div>
                  <p className="mt-1 break-words text-sm text-foreground">
                    {fact.value}
                  </p>
                </div>
              </div>

              {correction?.fact.id === fact.id ? (
                <div className="mt-3 rounded-lg border border-amber-200 bg-amber-50/70 p-2 dark:border-amber-900/50 dark:bg-amber-950/20">
                  <p className="mb-2 text-xs text-amber-800 dark:text-amber-200">
                    “记录纠正”表示旧记录本身有误；系统会保留旧版本并创建
                    replacement。
                  </p>
                  <textarea
                    rows={2}
                    value={correction.value}
                    onChange={(event) =>
                      setCorrection({ fact, value: event.target.value })
                    }
                    className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm"
                  />
                  <div className="mt-2 flex justify-end gap-2">
                    <Button
                      size="xs"
                      variant="ghost"
                      onClick={() => setCorrection(null)}
                    >
                      取消
                    </Button>
                    <Button
                      size="xs"
                      isLoading={busyKey === `correct:${fact.id}`}
                      onClick={handleCorrection}
                    >
                      保存纠正
                    </Button>
                  </div>
                </div>
              ) : (
                <div className="mt-3 flex flex-wrap gap-1.5">
                  {fact.review_state === "unverified" && (
                    <>
                      <Button
                        size="xs"
                        variant="secondary"
                        isLoading={busyKey === `confirm:${fact.id}`}
                        onClick={() =>
                          mutate(
                            `confirm:${fact.id}`,
                            () =>
                              workspaceApi.reviewFact(
                                fact.id,
                                snapshot.current_revision,
                                "confirmed",
                              ),
                            "已确认这条事实，没有改变其时间语义",
                          )
                        }
                      >
                        <Check className="h-3 w-3" />
                        确认
                      </Button>
                      <Button
                        size="xs"
                        variant="ghost"
                        isLoading={busyKey === `reject:${fact.id}`}
                        onClick={() =>
                          mutate(
                            `reject:${fact.id}`,
                            () =>
                              workspaceApi.reviewFact(
                                fact.id,
                                snapshot.current_revision,
                                "rejected",
                              ),
                            "已标记为不接受的提取结果",
                          )
                        }
                      >
                        <X className="h-3 w-3" />
                        不接受
                      </Button>
                    </>
                  )}
                  <Button
                    size="xs"
                    variant="outline"
                    onClick={() => setCorrection({ fact, value: fact.value })}
                  >
                    <PencilLine className="h-3 w-3" />
                    记录本身错了
                  </Button>
                  <Button
                    size="xs"
                    variant="ghost"
                    isLoading={busyKey === `improving:${fact.id}`}
                    onClick={() =>
                      mutate(
                        `improving:${fact.id}`,
                        () =>
                          workspaceApi.updateFactTemporal(
                            fact.id,
                            snapshot.current_revision,
                            {
                              trend: "improving",
                            },
                          ),
                        "已记录为后来正在改善",
                      )
                    }
                  >
                    <TrendingUp className="h-3 w-3" />
                    后来改善
                  </Button>
                  <Button
                    size="xs"
                    variant="ghost"
                    isLoading={busyKey === `worsening:${fact.id}`}
                    onClick={() =>
                      mutate(
                        `worsening:${fact.id}`,
                        () =>
                          workspaceApi.updateFactTemporal(
                            fact.id,
                            snapshot.current_revision,
                            {
                              trend: "worsening",
                            },
                          ),
                        "已记录为后来加重",
                      )
                    }
                  >
                    <TrendingDown className="h-3 w-3" />
                    后来加重
                  </Button>
                  <Button
                    size="xs"
                    variant="ghost"
                    isLoading={busyKey === `resolved:${fact.id}`}
                    onClick={() =>
                      mutate(
                        `resolved:${fact.id}`,
                        () =>
                          workspaceApi.updateFactTemporal(
                            fact.id,
                            snapshot.current_revision,
                            {
                              lifecycle_state: "resolved",
                              trend: "improving",
                              valid_until: new Date().toISOString(),
                            },
                          ),
                        "已记录为后来恢复；旧事实仍保留在历史中",
                      )
                    }
                  >
                    后来恢复
                  </Button>
                </div>
              )}
            </div>
          ))
        )}
      </section>

      {hypotheses.length > 0 && (
        <details className="rounded-xl border border-border p-3" open>
          <summary className="cursor-pointer list-none text-xs font-semibold text-muted-foreground">
            <span className="inline-flex items-center gap-2">
              <Lightbulb className="h-3.5 w-3.5" />
              推理假设 · {hypotheses.length}
            </span>
          </summary>
          <div className="mt-3 space-y-2">
            {hypotheses.map((hypothesis) => (
              <div
                key={hypothesis.id}
                className="rounded-lg bg-muted/40 p-2.5 text-xs"
              >
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <p className="font-medium text-foreground">
                    {hypothesis.statement}
                  </p>
                  <span className="rounded-full bg-background px-2 py-0.5 text-muted-foreground">
                    {hypothesis.lifecycle_state}
                  </span>
                </div>
                <p className="mt-1 text-muted-foreground">
                  这是可加强、削弱或退役的解释，不是用户事实。
                </p>
                <div className="mt-2 flex flex-wrap gap-1">
                  {(
                    [
                      "strengthened",
                      "weakened",
                      "unsupported",
                      "retired",
                    ] as const
                  ).map((state) => (
                    <Button
                      key={state}
                      size="xs"
                      variant="ghost"
                      isLoading={
                        busyKey === `hypothesis:${hypothesis.id}:${state}`
                      }
                      onClick={() =>
                        mutate(
                          `hypothesis:${hypothesis.id}:${state}`,
                          () =>
                            workspaceApi.updateHypothesisLifecycle(
                              hypothesis.id,
                              snapshot.current_revision,
                              state,
                            ),
                          `假设已更新为 ${state}`,
                        )
                      }
                    >
                      {state}
                    </Button>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </details>
      )}

      {((snapshot.observations?.length ?? 0) > 0 ||
        historicalFacts.length > 0) && (
        <details className="rounded-xl border border-border p-3">
          <summary className="cursor-pointer text-xs font-semibold text-muted-foreground">
            观察与历史记录 ·{" "}
            {(snapshot.observations?.length ?? 0) + historicalFacts.length}
          </summary>
          <div className="mt-3 space-y-2 text-xs text-muted-foreground">
            {snapshot.observations.map((observation) => (
              <div key={observation.id} className="rounded-lg bg-muted/30 p-2">
                <span className="font-medium text-foreground">
                  {observation.kind}
                </span>
                {observation.body_region ? ` · ${observation.body_region}` : ""}
                <pre className="mt-1 whitespace-pre-wrap font-sans">
                  {JSON.stringify(observation.value, null, 2)}
                </pre>
              </div>
            ))}
            {historicalFacts.map((fact) => (
              <div
                key={fact.id}
                className="rounded-lg bg-muted/30 p-2 opacity-75"
              >
                <span className="font-medium text-foreground">
                  {fact.value}
                </span>
                <span> · {fact.lifecycle_state}</span>
              </div>
            ))}
          </div>
        </details>
      )}
    </Card>
  );
}
