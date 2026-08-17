import { useMemo } from "react";
import type { ToolCallInfo } from "../types/consultation";
import { ToolCallItem } from "./ToolCallItem";

interface ToolCallCardProps {
  toolCalls: ToolCallInfo[];
}

/**
 * Displays tool call activity (search knowledge, extract symptoms, etc.)
 * as a compact inline indicator above the assistant's reply.
 */
export function ToolCallCard({ toolCalls }: ToolCallCardProps) {
  // Deduplicate by tool_call_id (latest wins) and sort: running first, completed last.
  // This is a UI defense layer — the reducer already handles idempotency.
  const normalizedCalls = useMemo(() => {
    const seen = new Map<string, ToolCallInfo>();
    for (const tc of toolCalls) {
      seen.set(tc.id, tc);
    }
    const deduped = Array.from(seen.values());
    deduped.sort((a, b) => {
      if (a.status === b.status) return 0;
      return a.status === "running" ? -1 : 1;
    });
    return deduped;
  }, [toolCalls]);

  if (normalizedCalls.length === 0) return null;

  // Filter out ask_user — those are rendered as AskUserCard separately
  const visibleCalls = normalizedCalls.filter((tc) => tc.tool !== "ask_user");
  if (visibleCalls.length === 0) return null;

  return (
    <div className="flex justify-start">
      <div className="max-w-[80%] rounded-xl px-3 py-2 bg-[#F0F4F0] border border-[#D4DDD4]">
        <div className="flex flex-col gap-1.5">
          {visibleCalls.map((tc) => (
            <ToolCallItem key={tc.id} toolCall={tc} />
          ))}
        </div>
      </div>
    </div>
  );
}
