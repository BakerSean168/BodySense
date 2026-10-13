import type { BodyStateProjection } from "@/features/consultation/types/consultation";
import { resolveRecordBodyRegion } from "../model/bodyExplorerSelectors";
import { getBodyRegionDefinition, type BodyRegionId } from "../model/bodyRegionOntology";

export function BodyRegionStatusSummary({
  snapshot,
  regionId,
}: {
  snapshot: BodyStateProjection | null;
  regionId: BodyRegionId | null;
}) {
  if (!regionId) return null;

  const facts = (snapshot?.facts ?? []).filter(
    (fact) =>
      resolveRecordBodyRegion(fact) === regionId &&
      fact.lifecycle_state === "active" &&
      fact.review_state !== "rejected" &&
      !fact.excluded_from_reasoning,
  );
  const label = getBodyRegionDefinition(regionId).labels["zh-CN"];

  return (
    <section
      aria-label={`${label}当前记录`}
      className="mt-3 rounded-xl border border-border/55 bg-background/35 p-3"
    >
      <div className="flex items-center justify-between gap-3">
        <h3 className="text-xs font-semibold text-foreground">{label} · 当前记录</h3>
        <span className="text-[10px] text-muted-foreground">
          {facts.length ? `${facts.length} 条` : "暂无记录"}
        </span>
      </div>
      {facts.length ? (
        <div className="mt-2 grid gap-2">
          {facts.slice(0, 3).map((fact) => (
            <article
              key={fact.id}
              className="rounded-lg border border-border/45 bg-background/45 px-3 py-2"
            >
              <p className="text-xs text-foreground">{fact.value}</p>
              <p className="mt-1 text-[10px] text-muted-foreground">
                {fact.review_state === "confirmed" ? "已确认记录" : "待核实记录"}
                {fact.observed_at
                  ? ` · ${new Date(fact.observed_at).toLocaleDateString("zh-CN")}`
                  : ""}
              </p>
            </article>
          ))}
        </div>
      ) : (
        <p className="mt-2 text-[11px] leading-relaxed text-muted-foreground">
          选择区域不会生成健康结论。这里没有记录时，可以继续提问或补充真实信息。
        </p>
      )}
    </section>
  );
}
