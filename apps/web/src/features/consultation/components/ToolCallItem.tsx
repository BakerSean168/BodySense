/**
 * ToolCallItem — shared rendering primitive for a single tool call.
 *
 * Used by both ToolCallCard (standalone, with dedup defense) and
 * StreamingTurnToolCalls (within the active turn, pre-normalized by selectors).
 */

import type { ToolCallInfo } from "../types/consultation";

const TOOL_LABELS: Record<string, string> = {
  search_knowledge: "搜索知识库",
  extract_symptom_info: "提取症状信息",
};

/** Extract a human-readable summary from tool args. */
export function getToolSummary(tool: string, args: unknown): string {
  if (!args || typeof args !== "object") return "";
  const a = args as Record<string, unknown>;
  if (tool === "search_knowledge" && typeof a.query === "string") {
    return a.query;
  }
  if (tool === "extract_symptom_info" && typeof a.body_part === "string") {
    return a.body_part;
  }
  return "";
}

interface ToolCallItemProps {
  toolCall: ToolCallInfo;
}

export function ToolCallItem({ toolCall: tc }: ToolCallItemProps) {
  const label = TOOL_LABELS[tc.tool] || tc.tool;
  const summary = getToolSummary(tc.tool, tc.args);
  const isRunning = tc.status === "running";

  return (
    <div className="flex items-center gap-2 text-xs">
      {isRunning ? (
        <span className="size-3 shrink-0 animate-spin rounded-full border-2 border-[#83d4aa]/55 border-t-transparent" />
      ) : (
        <svg
          className="size-3 shrink-0 text-[#83d4aa]/75"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2.5}
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M5 13l4 4L19 7"
          />
        </svg>
      )}
      <span className="font-medium text-white/58">{label}</span>
      {summary && (
        <span className="max-w-[240px] truncate text-white/32" title={summary}>
          「{summary}」
        </span>
      )}
    </div>
  );
}
