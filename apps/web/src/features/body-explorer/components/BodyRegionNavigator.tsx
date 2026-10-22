import type { BodyRecords } from "@/features/profile/services/bodyRecordsService";
import { selectBodyRegionVisualSummaries } from "../model/bodyExplorerSelectors";
import {
  bodyRegionDefinitions,
  getBodyRegionDefinition,
  parseBodyRegionId,
  type BodyRegionId,
} from "../model/bodyRegionOntology";

const stateLabels = {
  none: "",
  observed: "有记录",
} as const;

export function BodyRegionNavigator({
  snapshot,
  selectedRegionId,
  onSelectRegion,
}: {
  snapshot: BodyRecords | null;
  selectedRegionId: BodyRegionId | null;
  onSelectRegion: (regionId: BodyRegionId | null) => void;
}) {
  const summaries = selectBodyRegionVisualSummaries(snapshot);
  const summaryByRegion = new Map(
    summaries.map((summary) => [summary.regionId, summary]),
  );
  const activeRegionCount = summaries.filter(
    (summary) => summary.visualState !== "none",
  ).length;

  return (
    <div className="flex flex-wrap items-center justify-between gap-2 border-t border-border/45 pt-3">
      <label className="min-w-0 flex-1">
        <span className="sr-only">选择身体区域</span>
        <select
          value={selectedRegionId ?? ""}
          onChange={(event) =>
            onSelectRegion(
              event.target.value ? parseBodyRegionId(event.target.value) : null,
            )
          }
          className="h-8 max-w-full rounded-lg border border-border/70 bg-background/45 px-2.5 text-xs text-foreground outline-none transition-colors focus:border-primary/55 focus:ring-2 focus:ring-primary/15"
          aria-label="选择身体区域"
        >
          <option value="">全身</option>
          {bodyRegionDefinitions.map((region) => {
            const summary = summaryByRegion.get(region.id);
            const status = summary ? stateLabels[summary.visualState] : "";
            const count = summary?.statementCount ?? 0;
            return (
              <option key={region.id} value={region.id}>
                {region.labels["zh-CN"]}
                {status ? ` · ${status}${count > 0 ? ` ${count}` : ""}` : ""}
              </option>
            );
          })}
        </select>
      </label>

      <p className="shrink-0 text-[11px] text-muted-foreground">
        {selectedRegionId
          ? getBodyRegionDefinition(selectedRegionId).labels["zh-CN"]
          : activeRegionCount > 0
            ? `${activeRegionCount} 个区域有记录`
            : snapshot
              ? "暂无区域记录"
              : "区域记录尚未读取"}
      </p>
    </div>
  );
}
