import { MessageCircle } from "lucide-react";
import type { ChatHistory } from "@/features/body-canvas/model/chatHistory";
import { SelectField } from "@/components/ui/select";

/**
 * Penpot "Window / Title Chat": 💬 对话 / 今天 ▾. The picker lists the days the
 * user actually wrote something in their single Conversation; choosing one
 * scrolls the thread there. With no earlier day the menu holds only 今天.
 */
export function ChatWindowHeader({
  history,
  selectedDay,
  onSelectDay,
}: {
  history?: ChatHistory;
  selectedDay?: string;
  onSelectDay?: (day: string) => void;
}) {
  const todayKey = history?.todayKey ?? "";
  const earlier = (history?.days ?? []).filter((day) => day.day !== todayKey);
  const title = (
    <>
      <span className="bc-window-context-title bc-chat-context-title">
        <MessageCircle size={18} aria-hidden="true" />
        对话
      </span>
      <span className="bc-window-context-sep" aria-hidden="true">
        /
      </span>
    </>
  );
  if (!history || !onSelectDay)
    return (
      <div className="bc-window-context bc-chat-context">
        {title}
        <span
          className="bc-window-select-wrap bc-window-context-mode"
          data-testid="chat-day"
        >
          今天
        </span>
      </div>
    );
  return (
    <div className="bc-window-context bc-chat-context">
      {title}
      <span className="bc-window-select-wrap">
        <SelectField
          variant="ghost"
          className="bc-window-context-select"
          aria-label="跳到某天的对话"
          data-testid="chat-day"
          value={selectedDay ?? todayKey}
          options={[
            { value: todayKey, label: "今天" },
            ...earlier.map((day) => ({ value: day.day, label: day.label })),
          ]}
          onValueChange={onSelectDay}
        />
      </span>
    </div>
  );
}
