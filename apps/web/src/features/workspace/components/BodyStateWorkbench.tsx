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
import {
  getBodyRegionDefinition,
  resolveBodyRegionInput,
  resolveRecordBodyRegion,
  type BodyRegionId,
} from "@/features/body-explorer";
import type {
  BodyStateFact,
  BodyStateObservation,
  BodyStateSnapshot,
} from "@/features/consultation/types/consultation";
import { errorMessage } from "@/lib/api-client";
import {
  useBodyStateCommand,
  type BodyStateCommand,
} from "../hooks/useBodyStateCommand";

interface BodyStateWorkbenchProps {
  snapshot: BodyStateSnapshot;
  selectedRegionId?: BodyRegionId | null;
  onSelectRegion?: (regionId: BodyRegionId | null) => void;
  onAskRegion?: (regionId: BodyRegionId) => void;
}

const factKindLabels: Record<string, string> = {
  discomfort: "不适 / 症状",
  limitation: "活动受限",
  lifestyle: "生活方式",
  "lifestyle.activity": "生活方式 · 日常活动",
  "lifestyle.sleep": "生活方式 · 睡眠作息",
  "lifestyle.exercise": "生活方式 · 运动",
  "lifestyle.nutrition": "生活方式 · 饮食节律",
  "lifestyle.substances": "生活方式 · 相关摄入",
  "lifestyle.recovery": "生活方式 · 恢复压力",
  "history.injury_summary": "既往伤病摘要",
  negative_finding: "阴性发现",
  red_flags: "安全信号",
  safety_finding: "安全发现",
};

const observationKindLabels: Record<string, string> = {
  "anthropometry.height": "身体测量 · 身高",
  "anthropometry.weight": "身体测量 · 体重",
};

const trendLabels: Record<string, string> = {
  unknown: "待观察",
  stable: "基本稳定",
  improving: "正在改善",
  worsening: "正在加重",
};

const hypothesisStateLabels: Record<string, string> = {
  active: "待验证",
  strengthened: "更有可能",
  weakened: "可能性降低",
  unsupported: "证据不足",
  retired: "不再考虑",
};

function observationValueText(value: unknown): string {
  if (typeof value === "string") return value;
  if (!value || typeof value !== "object") return String(value ?? "");
  const record = value as Record<string, unknown>;
  const label = typeof record.label === "string" ? record.label : "";
  const description =
    typeof record.description === "string" ? record.description : "";
  if (label && description) return `${label}：${description}`;
  if (description) return description;
  if (label) return label;
  return Object.values(record)
    .filter((item) => ["string", "number", "boolean"].includes(typeof item))
    .map(String)
    .join(" · ");
}

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
  selectedRegionId = null,
  onSelectRegion,
  onAskRegion,
}: BodyStateWorkbenchProps) {
  const bodyStateCommand = useBodyStateCommand();
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
  const selectedRegionLabel = selectedRegionId
    ? getBodyRegionDefinition(selectedRegionId).labels["zh-CN"]
    : null;

  const mutate = async (
    key: string,
    command: BodyStateCommand,
    success: string,
  ) => {
    setBusyKey(key);
    try {
      await bodyStateCommand.mutateAsync(command);
      toast.success(success);
    } catch (error) {
      toast.error(errorMessage(error, "身体记录更新失败"));
    } finally {
      setBusyKey(null);
    }
  };

  const handleAddFact = async () => {
    if (!value.trim()) {
      toast.error("请填写记录内容");
      return;
    }
    const effectiveBodyRegion = bodyRegion.trim() || selectedRegionLabel || "";
    const regionResolution = effectiveBodyRegion
      ? resolveBodyRegionInput(effectiveBodyRegion)
      : null;
    const bodyRegionId =
      selectedRegionId ??
      (regionResolution?.status === "resolved" ? regionResolution.id : null);
    await mutate(
      "add-fact",
      {
        type: "addFact",
        expectedRevision: snapshot.current_revision,
        fact: {
          kind,
          body_region: effectiveBodyRegion,
          body_region_id: bodyRegionId,
          concern_key: effectiveBodyRegion
            ? `region:${bodyRegionId ?? effectiveBodyRegion}`
            : "general",
          value: value.trim(),
          origin: "user_reported",
          review_state: "confirmed",
          lifecycle_state: "active",
          trend: "unknown",
          observed_at: new Date().toISOString(),
        },
      },
      "身体记录已添加",
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
      {
        type: "correctFact",
        factId: fact.id,
        expectedRevision: snapshot.current_revision,
        replacement: {
          concern_key: fact.concern_key || "general",
          kind: fact.kind,
          body_region: fact.body_region || "",
          body_region_id: fact.body_region_id ?? null,
          value: correction.value.trim(),
          details: fact.details || {},
          origin: "user_edited",
          review_state: "confirmed",
          lifecycle_state: "active",
          trend: fact.trend || "unknown",
          observed_at: fact.observed_at || new Date().toISOString(),
        },
      },
      "已保留原记录，并保存纠正后的内容",
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
  const inSelectedRegion = (record: {
    body_region_id?: string | null;
    body_region?: string;
  }) =>
    !selectedRegionId || resolveRecordBodyRegion(record) === selectedRegionId;
  const visibleActiveFacts = activeFacts.filter(inSelectedRegion);
  const visiblePendingObservations =
    pendingObservations.filter(inSelectedRegion);
  const visibleObservations = snapshot.observations.filter(inSelectedRegion);
  const visibleHistoricalFacts = historicalFacts.filter(inSelectedRegion);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <History className="h-3.5 w-3.5 text-primary" />
          <h3 className="text-[15px] font-semibold tracking-[-0.01em] text-foreground">
            身体记录
          </h3>
        </div>
        <div className="flex flex-wrap items-center justify-end gap-1.5">
          {selectedRegionId && onAskRegion ? (
            <Button
              size="xs"
              variant="ghost"
              onClick={() => onAskRegion(selectedRegionId)}
            >
              询问 BodySense
            </Button>
          ) : null}
          <Button
            size="xs"
            variant="outline"
            onClick={() => setShowAdd((open) => !open)}
          >
            <CirclePlus className="h-3.5 w-3.5" />
            添加记录
          </Button>
        </div>
      </div>

      {selectedRegionId ? (
        <div className="flex items-center justify-between gap-3 border-b border-border/45 pb-2 text-xs">
          <span className="font-medium text-foreground">
            {selectedRegionLabel}
          </span>
          {onSelectRegion ? (
            <button
              type="button"
              onClick={() => onSelectRegion(null)}
              className="text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/35"
            >
              返回全部
            </button>
          ) : null}
        </div>
      ) : null}

      {safety.active && (
        <div className="rounded-xl border border-destructive/25 bg-destructive/10 p-3 text-sm text-destructive">
          <div className="flex items-center gap-2 font-medium">
            <AlertTriangle className="h-4 w-4" />
            有些信息需要优先确认
          </div>
          <p className="mt-1 text-xs leading-5 opacity-90">
            在这些安全信息确认前，BodySense 会暂停生成新的分析和方案。
          </p>
          <div className="mt-3 flex flex-col gap-2 sm:flex-row">
            <input
              value={safetyNote}
              onChange={(event) => setSafetyNote(event.target.value)}
              placeholder="补充确认依据（可选）"
              className="min-w-0 flex-1 rounded-lg border border-destructive/25 bg-background px-3 py-2 text-xs text-foreground outline-none focus:border-destructive"
            />
            <Button
              size="sm"
              variant="outline"
              isLoading={busyKey === "resolve-safety"}
              onClick={() =>
                mutate(
                  "resolve-safety",
                  {
                    type: "resolveSafety",
                    expectedRevision: snapshot.current_revision,
                    resolution: "cleared_by_review",
                    note: safetyNote.trim(),
                  },
                  "安全信息已确认",
                )
              }
            >
              <ShieldCheck className="h-3.5 w-3.5" />
              完成确认
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
            placeholder="记录内容"
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
              保存记录
            </Button>
          </div>
        </div>
      )}

      {visiblePendingObservations.length > 0 && (
        <section className="space-y-2 rounded-xl border border-warning/35 bg-warning/10 p-3">
          <div>
            <h4 className="text-xs font-semibold uppercase tracking-wide text-warning-foreground">
              待确认观察 · {visiblePendingObservations.length}
            </h4>
            <p className="mt-1 text-xs leading-5 text-warning-foreground/85">
              来自图片或 BodySense 分析的观察，需要你确认后才会用于后续分析。
            </p>
          </div>
          {visiblePendingObservations.map(
            (observation: BodyStateObservation) => {
              const label =
                typeof observation.value?.label === "string"
                  ? observation.value.label
                  : observation.kind;
              const description = observationValueText(observation.value);
              return (
                <div
                  key={observation.id}
                  className="rounded-lg border border-warning/35 bg-background p-3"
                >
                  <div className="text-xs text-muted-foreground">
                    {observation.body_region || "全身"} · BodySense 观察
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
                          {
                            type: "reviewObservation",
                            observationId: observation.id,
                            expectedRevision: snapshot.current_revision,
                            reviewState: "confirmed",
                          },
                          "观察已确认，会用于后续分析",
                        )
                      }
                    >
                      <Check className="h-3 w-3" />
                      确认
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
                          {
                            type: "reviewObservation",
                            observationId: observation.id,
                            expectedRevision: snapshot.current_revision,
                            reviewState: "rejected",
                          },
                          "已忽略这条观察",
                        )
                      }
                    >
                      <X className="h-3 w-3" />
                      不接受
                    </Button>
                  </div>
                </div>
              );
            },
          )}
        </section>
      )}

      <section className="space-y-2">
        {visibleActiveFacts.length === 0 ? (
          <div className="mx-auto max-w-sm py-5 text-center">
            <p className="text-[13px] font-medium text-foreground/82">
              {selectedRegionId ? "这个区域还没有记录" : "还没有记录"}
            </p>
            <p className="mt-1 text-xs leading-[1.55] text-muted-foreground">
              继续对话，或手动添加一条身体记录。
            </p>
          </div>
        ) : (
          <>
            <h4 className="text-xs font-semibold tracking-wide text-muted-foreground">
              当前记录 · {visibleActiveFacts.length}
            </h4>
            {visibleActiveFacts.map((fact) => (
              <div
                key={fact.id}
                className="border-b border-border/50 py-3 last:border-b-0"
              >
                <div className="flex flex-wrap items-start justify-between gap-2">
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-1.5 text-[11px] text-muted-foreground">
                      <span>{factKindLabels[fact.kind] || fact.kind}</span>
                      {fact.body_region ? (
                        resolveRecordBodyRegion(fact) && onSelectRegion ? (
                          <button
                            type="button"
                            onClick={() =>
                              onSelectRegion(resolveRecordBodyRegion(fact))
                            }
                            className="rounded px-0.5 text-primary/80 transition-colors hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/30"
                          >
                            · {fact.body_region}
                          </button>
                        ) : (
                          <span>· {fact.body_region}</span>
                        )
                      ) : null}
                      <span>· {trendLabels[fact.trend] || fact.trend}</span>
                      <span
                        className={`rounded-full px-1.5 py-0.5 ${
                          fact.review_state === "confirmed"
                            ? "bg-success/20 text-success-foreground"
                            : "bg-warning/20 text-warning-foreground"
                        }`}
                      >
                        {fact.review_state === "confirmed"
                          ? "已确认"
                          : "待确认"}
                      </span>
                    </div>
                    <p className="mt-1 break-words text-sm text-foreground">
                      {fact.value}
                    </p>
                  </div>
                </div>

                {correction?.fact.id === fact.id ? (
                  <div className="mt-3 rounded-lg border border-warning/35 bg-warning/10 p-2">
                    <p className="mb-2 text-xs text-warning-foreground">
                      如果原记录本身有误，可以在保留历史的同时保存一条纠正后的记录。
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
                  <div className="mt-2.5 flex flex-wrap gap-1.5">
                    {fact.review_state === "unverified" && (
                      <>
                        <Button
                          size="xs"
                          variant="secondary"
                          isLoading={busyKey === `confirm:${fact.id}`}
                          onClick={() =>
                            mutate(
                              `confirm:${fact.id}`,
                              {
                                type: "reviewFact",
                                factId: fact.id,
                                expectedRevision: snapshot.current_revision,
                                reviewState: "confirmed",
                              },
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
                              {
                                type: "reviewFact",
                                factId: fact.id,
                                expectedRevision: snapshot.current_revision,
                                reviewState: "rejected",
                              },
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
                      纠正记录
                    </Button>
                    <Button
                      size="xs"
                      variant="ghost"
                      isLoading={busyKey === `improving:${fact.id}`}
                      onClick={() =>
                        mutate(
                          `improving:${fact.id}`,
                          {
                            type: "updateFactTemporal",
                            factId: fact.id,
                            expectedRevision: snapshot.current_revision,
                            input: { trend: "improving" },
                          },
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
                          {
                            type: "updateFactTemporal",
                            factId: fact.id,
                            expectedRevision: snapshot.current_revision,
                            input: { trend: "worsening" },
                          },
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
                          {
                            type: "updateFactTemporal",
                            factId: fact.id,
                            expectedRevision: snapshot.current_revision,
                            input: {
                              lifecycle_state: "resolved",
                              trend: "improving",
                              valid_until: new Date().toISOString(),
                            },
                          },
                          "已记录为后来恢复；旧事实仍保留在历史中",
                        )
                      }
                    >
                      后来恢复
                    </Button>
                  </div>
                )}
              </div>
            ))}
          </>
        )}
      </section>

      {hypotheses.length > 0 && (
        <details className="border-t border-border/55 pt-4" open>
          <summary className="cursor-pointer list-none text-xs font-semibold text-muted-foreground">
            <span className="inline-flex items-center gap-2">
              <Lightbulb className="h-3.5 w-3.5" />
              可能解释 · {hypotheses.length}
            </span>
          </summary>
          <div className="mt-3 space-y-2">
            {hypotheses.map((hypothesis) => (
              <div
                key={hypothesis.id}
                className="border-b border-border/45 py-2.5 text-xs last:border-b-0"
              >
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <p className="font-medium text-foreground">
                    {hypothesis.statement}
                  </p>
                  <span className="rounded-full bg-background px-2 py-0.5 text-muted-foreground">
                    {hypothesisStateLabels[hypothesis.lifecycle_state] ||
                      "待验证"}
                  </span>
                </div>
                <p className="mt-1 text-muted-foreground">
                  这只是当前的可能解释，会随着新信息增加或降低可信度。
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
                          {
                            type: "updateHypothesisLifecycle",
                            hypothesisId: hypothesis.id,
                            expectedRevision: snapshot.current_revision,
                            lifecycleState: state,
                          },
                          `可能解释已更新为${hypothesisStateLabels[state] || state}`,
                        )
                      }
                    >
                      {hypothesisStateLabels[state] || state}
                    </Button>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </details>
      )}

      {(visibleObservations.length > 0 ||
        visibleHistoricalFacts.length > 0) && (
        <details className="border-t border-border/55 pt-4">
          <summary className="cursor-pointer list-none text-xs font-semibold text-muted-foreground">
            观察与历史 ·{" "}
            {visibleObservations.length + visibleHistoricalFacts.length}
          </summary>
          <div className="mt-3 space-y-2 text-xs text-muted-foreground">
            {visibleObservations.map((observation) => (
              <div
                key={observation.id}
                className="border-b border-border/40 py-2 last:border-b-0"
              >
                <span className="font-medium text-foreground">
                  {observationKindLabels[observation.kind] || observation.kind}
                </span>
                {observation.body_region ? ` · ${observation.body_region}` : ""}
                <p className="mt-1 leading-5">
                  {observationValueText(observation.value)}
                </p>
              </div>
            ))}
            {visibleHistoricalFacts.map((fact) => (
              <div
                key={fact.id}
                className="border-b border-border/40 py-2 opacity-75 last:border-b-0"
              >
                <span className="font-medium text-foreground">
                  {fact.value}
                </span>
                <span> · 历史记录</span>
              </div>
            ))}
          </div>
        </details>
      )}
    </div>
  );
}
