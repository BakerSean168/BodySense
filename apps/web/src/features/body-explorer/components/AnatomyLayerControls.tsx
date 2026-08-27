import { useState } from "react";
import { ChevronDown } from "lucide-react";
import { Button } from "@/components/ui/Button";
import type { AnatomyDisplayMode } from "../adapters/anatomyViewerPort";

const prioritySystems = [
  "regional-anatomy",
  "muscular",
  "skeletal",
  "nervous",
] as const;

const systemLabels: Record<string, string> = {
  "regional-anatomy": "区域",
  muscular: "肌肉",
  skeletal: "骨骼",
  nervous: "神经",
};

const displayModeLabels: Record<AnatomyDisplayMode, string> = {
  normal: "正常",
  xray: "X-Ray",
  ghost: "Ghost",
};

export interface AnatomySystemOption {
  id: string;
  name: string;
}

export function AnatomyLayerControls({
  systems,
  visibleSystems,
  displayMode,
  onSelectSystem,
  onDisplayModeChange,
}: {
  systems: readonly AnatomySystemOption[];
  visibleSystems: readonly string[];
  displayMode: AnatomyDisplayMode;
  onSelectSystem: (id: string) => void;
  onDisplayModeChange: (mode: AnatomyDisplayMode) => void;
}) {
  const [moreOpen, setMoreOpen] = useState(false);
  const available = new Map(systems.map((system) => [system.id, system]));
  const primary = prioritySystems
    .map((id) => available.get(id))
    .filter((system): system is AnatomySystemOption => Boolean(system));
  const more = systems.filter(
    (system) => !prioritySystems.includes(system.id as (typeof prioritySystems)[number]),
  );

  return (
    <div className="space-y-2" aria-label="解剖显示控制">
      <div className="flex flex-wrap items-center gap-1.5">
        {primary.map((system) => (
          <Button
            key={system.id}
            size="xs"
            variant={visibleSystems.includes(system.id) ? "secondary" : "ghost"}
            aria-pressed={visibleSystems.includes(system.id)}
            onClick={() => onSelectSystem(system.id)}
          >
            {systemLabels[system.id] ?? system.name}
          </Button>
        ))}
        {more.length > 0 ? (
          <Button
            size="xs"
            variant="ghost"
            aria-expanded={moreOpen}
            onClick={() => setMoreOpen((open) => !open)}
          >
            更多
            <ChevronDown
              className={`size-3 transition-transform motion-reduce:transition-none ${moreOpen ? "rotate-180" : ""}`}
              aria-hidden="true"
            />
          </Button>
        ) : null}
      </div>

      {moreOpen ? (
        <div className="flex flex-wrap gap-1.5 border-l border-border/60 pl-2">
          {more.map((system) => (
            <Button
              key={system.id}
              size="xs"
              variant={visibleSystems.includes(system.id) ? "secondary" : "ghost"}
              aria-pressed={visibleSystems.includes(system.id)}
              onClick={() => onSelectSystem(system.id)}
            >
              {system.name}
            </Button>
          ))}
        </div>
      ) : null}

      <div className="flex flex-wrap items-center gap-1.5 text-[11px] text-muted-foreground">
        <span className="mr-1">显示</span>
        {(Object.keys(displayModeLabels) as AnatomyDisplayMode[]).map((mode) => (
          <button
            key={mode}
            type="button"
            aria-pressed={displayMode === mode}
            onClick={() => onDisplayModeChange(mode)}
            className={`rounded-md px-2 py-1 outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring ${
              displayMode === mode
                ? "bg-secondary text-secondary-foreground"
                : "hover:bg-muted/70 hover:text-foreground"
            }`}
          >
            {displayModeLabels[mode]}
          </button>
        ))}
      </div>
    </div>
  );
}
