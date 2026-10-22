import { List } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { ChatHistory } from "@/features/body-canvas/model/chatHistory";

/**
 * Penpot Chat header list icon: the conversation's history by day, each with
 * how many messages the user wrote and the first of them. Choosing a day
 * scrolls the thread there; nothing here changes or deletes a message.
 */
export function ChatHistoryMenu({
  history,
  onSelectDay,
}: {
  history: ChatHistory;
  onSelectDay: (day: string) => void;
}) {
  if (history.days.length === 0) return null;
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className="bc-icon-button bc-header-chat-action"
        aria-label="对话历史"
        title="对话历史"
      >
        <List size={19} strokeWidth={1.8} aria-hidden="true" />
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        sideOffset={8}
        className="bc-chat-history-menu"
      >
        {history.days.map((day) => (
          <DropdownMenuItem
            key={day.day}
            className="bc-chat-history-item"
            onClick={() => onSelectDay(day.day)}
          >
            <span className="bc-chat-history-day">
              <strong>{day.label}</strong>
              <span>{day.count} 条</span>
            </span>
            <span className="bc-chat-history-preview">{day.preview}</span>
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
