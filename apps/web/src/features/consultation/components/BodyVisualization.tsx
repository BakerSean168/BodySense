import { useState } from "react";
import { highlightedBodySvgIds } from "../model/bodyVisualization";
import { FrontViewSVG } from "./BodyFrontView";
import { SideViewSVG } from "./BodySideView";
import { BackViewSVG } from "./BodyBackView";

interface BodyVisualizationProps {
  highlightedParts: string[];
  onPartClick?: (part: string) => void;
}

type ViewType = "front" | "side" | "back";
const VIEW_TYPES: readonly ViewType[] = ["front", "side", "back"];

export function BodyVisualization({
  highlightedParts,
  onPartClick,
}: BodyVisualizationProps) {
  const [activeView, setActiveView] = useState<ViewType>("front");

  const highlightedIds = highlightedBodySvgIds(highlightedParts);

  const handlePartClick = (partName: string) => {
    onPartClick?.(partName);
  };

  return (
    <div className="flex flex-col items-center">
      {/* View selector */}
      <div className="flex gap-2.5 mb-5 bg-[#F7F5F0] p-1 rounded-full border border-[#E5E3DF]">
        {VIEW_TYPES.map((view) => (
          <button
            key={view}
            onClick={() => setActiveView(view)}
            className={`px-4.5 py-1.5 text-xs font-bold rounded-full transition-all duration-300 cursor-pointer ${
              activeView === view
                ? "bg-primary-700 text-[#FBFBFA] shadow-sm shadow-[#2a4b3a]/10"
                : "text-[#4A554E] hover:text-[#1A221E]"
            }`}
          >
            {view === "front" ? "正面" : view === "side" ? "侧面" : "背面"}
          </button>
        ))}
      </div>

      {/* SVG Body */}
      <div className="w-full max-w-[210px] py-2">
        {activeView === "front" && (
          <FrontViewSVG
            highlightedIds={highlightedIds}
            onPartClick={handlePartClick}
          />
        )}
        {activeView === "side" && (
          <SideViewSVG
            highlightedIds={highlightedIds}
            onPartClick={handlePartClick}
          />
        )}
        {activeView === "back" && (
          <BackViewSVG
            highlightedIds={highlightedIds}
            onPartClick={handlePartClick}
          />
        )}
      </div>

      {/* Legend */}
      {highlightedParts.length > 0 && (
        <div className="mt-5 px-4 py-2 bg-primary-50/50 border border-primary-200/30 rounded-2xl text-xs font-bold text-primary-800 flex items-center gap-1.5 shadow-sm">
          <span className="w-1.5 h-1.5 rounded-full bg-accent-terracotta animate-pulse" />
          高亮部位：{highlightedParts.join("、")}
        </div>
      )}
    </div>
  );
}
