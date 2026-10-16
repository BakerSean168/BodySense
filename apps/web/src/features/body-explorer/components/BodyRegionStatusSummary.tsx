import type { BodyRecords } from "@/features/profile/services/bodyRecordsService";
import { recordBodyRegions } from "../model/bodyExplorerSelectors";
import { getBodyRegionDefinition, type BodyRegionId } from "../model/bodyRegionOntology";

export function BodyRegionStatusSummary({
  snapshot,
  regionId,
}: {
  snapshot: BodyRecords | null;
  regionId: BodyRegionId | null;
}) {
  if (!regionId) return null;

  const facts = (snapshot?.facts ?? []).filter(
    (fact) =>
      recordBodyRegions(fact).includes(regionId) &&
      fact.assertion_status === "asserted",
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
          {facts.length ? `${facts.length} 条` : snapshot ? "暂无记录" : "记录尚未读取"}
        </span>
      </div>
      {facts.length ? (
        <div className="mt-2 grid gap-2">
          {facts.slice(0, 3).map((fact) => (
            <article
              key={fact.id}
              className="rounded-lg border border-border/45 bg-background/45 px-3 py-2"
            >
              <p className="text-xs text-foreground">{fact.statement}</p>
              <p className="mt-1 text-[10px] text-muted-foreground">
                用户陈述 · {new Date(fact.recorded_at).toLocaleDateString("zh-CN")}
              </p>
            </article>
          ))}
        </div>
      ) : (
        <p className="mt-2 text-[11px] leading-relaxed text-muted-foreground">
          {snapshot ? "选择区域不会生成健康结论。这里没有记录时，可以继续提问或补充真实信息。" : "当前尚未取得区域记录，请等待或重试读取。"}
        </p>
      )}
    </section>
  );
}
