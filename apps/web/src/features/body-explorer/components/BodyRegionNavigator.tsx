import type { BodyRecords } from "@/features/profile/services/bodyRecordsService";
import { selectBodyRegionVisualSummaries } from "../model/bodyExplorerSelectors";
import {
  bodyRegionDefinitions,
  getBodyRegionDefinition,
  parseBodyRegionId,
  type BodyRegionId,
} from "../model/bodyRegionOntology";
import { SelectField } from "@/components/ui/select";

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
        <SelectField
          value={selectedRegionId ?? ""}
          onValueChange={(next) =>
            onSelectRegion(next ? parseBodyRegionId(next) : null)
          }
          className="min-h-8 w-auto max-w-full text-xs"
          aria-label="选择身体区域"
          options={[
            { value: "", label: "全身" },
            ...bodyRegionDefinitions.map((region) => {
              const summary = summaryByRegion.get(region.id);
              const status = summary ? stateLabels[summary.visualState] : "";
              const count = summary?.statementCount ?? 0;
              return {
                value: region.id,
                label: `${region.labels["zh-CN"]}${status ? ` · ${status}${count > 0 ? ` ${count}` : ""}` : ""}`,
              };
            }),
          ]}
        />
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
