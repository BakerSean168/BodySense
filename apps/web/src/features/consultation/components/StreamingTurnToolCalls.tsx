/**
 * StreamingTurnToolCalls — pure display of tool call progress during the active turn.
 *
 * Receives a pre-normalized view model (sorted, deduped, ask_user filtered)
 * from the selector layer.
 */

import type { ToolCallInfo } from "../types/consultation";
import { ToolCallItem } from "./ToolCallItem";

interface StreamingTurnToolCallsProps {
  toolCalls: ToolCallInfo[];
}

export function StreamingTurnToolCalls({
  toolCalls,
}: StreamingTurnToolCallsProps) {
  if (toolCalls.length === 0) return null;

  return (
    <div className="w-full rounded-xl border border-white/[0.07] bg-white/[0.025] px-3 py-2.5">
      <div className="flex flex-col gap-1.5">
        {toolCalls.map((toolCall) => (
          <ToolCallItem key={toolCall.id} toolCall={toolCall} />
        ))}
      </div>
    </div>
  );
}
