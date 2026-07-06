/**
 * StreamingTurnToolCalls — pure display of tool call progress during the active turn.
 *
 * Receives a pre-normalized view model (sorted, deduped, ask_user filtered)
 * from the selector layer.
 */

import type { ToolCallInfo } from '../types/consultation';
import { ToolCallItem } from './ToolCallItem';

interface StreamingTurnToolCallsProps {
  toolCalls: ToolCallInfo[];
}

export function StreamingTurnToolCalls({ toolCalls }: StreamingTurnToolCallsProps) {
  if (toolCalls.length === 0) return null;

  return (
    <div className="flex justify-start">
      <div className="max-w-[80%] rounded-xl px-3 py-2 bg-[#F0F4F0] border border-[#D4DDD4]">
        <div className="flex flex-col gap-1.5">
          {toolCalls.map((tc) => (
            <ToolCallItem key={tc.id} toolCall={tc} />
          ))}
        </div>
      </div>
    </div>
  );
}
