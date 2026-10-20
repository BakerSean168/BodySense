import { ChevronDown, List, MessageCircle, Stethoscope } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { ChatHistory } from "../model/chatHistory";
import { TrainingIcon } from "./TrainingIcon";
import type { TrainingSelection } from "../model/trainingPlanSelection";

export interface DiagnosisWindowHeaderProps {
  /** Only adopted ConditionEntries; proposals never appear in the picker. */
  view: { confirmed: readonly { conditionId: string; name: string }[] };
  selectedConditionId: string | null;
  onSelectCondition: (conditionId: string) => void;
  /**
   * Penpot Diagnosis boards: a formal run shows "正式分析" and its settled
   * outcome "提交回执" in the toolbar instead of the condition picker.
   */
  mode?: "analysis" | "receipt" | null;
  /** Penpot Diagnosis / History: the picker reads "历史". */
  history?: boolean;
  onOpenHistory?: () => void;
  onCloseHistory?: () => void;
}

const HISTORY_OPTION = "__history__";

const MODE_LABELS = { analysis: "正式分析", receipt: "提交回执" } as const;

/** Penpot "Window / Title Diagnosis": 🩺 诊断 / <selector>. */
function DiagnosisTitle({ separator = true }: { separator?: boolean }) {
  return (
    <>
      <span className="bc-window-context-title">
        <Stethoscope size={18} aria-hidden="true" />
        诊断
      </span>
      {separator ? (
        <span className="bc-window-context-sep" aria-hidden="true">
          /
        </span>
      ) : null}
    </>
  );
}

/** Penpot V2: condition picker belongs to the window toolbar, not the scroll body. */
export function DiagnosisWindowHeader({
  view,
  selectedConditionId,
  onSelectCondition,
  mode = null,
  history = false,
  onOpenHistory,
  onCloseHistory,
}: DiagnosisWindowHeaderProps) {
  if (mode && !history)
    return (
      <div className="bc-window-context bc-diagnosis-context">
        <DiagnosisTitle />
        <span
          className="bc-window-select-wrap bc-window-context-mode"
          data-testid="diagnosis-mode"
        >
          {MODE_LABELS[mode]}
        </span>
      </div>
    );

  const selected = view.confirmed.some(
    (item) => item.conditionId === selectedConditionId,
  )
    ? selectedConditionId!
    : (view.confirmed[0]?.conditionId ?? "");

  if (history && view.confirmed.length === 0)
    return (
      <div className="bc-window-context bc-diagnosis-context">
        <DiagnosisTitle />
        <span
          className="bc-window-select-wrap bc-window-context-mode"
          data-testid="diagnosis-mode"
        >
          历史
        </span>
      </div>
    );

  // Penpot Diagnosis / Empty: only the window title, no empty picker.
  if (view.confirmed.length === 0)
    return (
      <div className="bc-window-context bc-diagnosis-context">
        <DiagnosisTitle separator={false} />
      </div>
    );

  return (
    <div className="bc-window-context bc-diagnosis-context">
      <DiagnosisTitle />
      <span className="bc-window-select-wrap">
        <select
          className="bc-window-context-select"
          aria-label="选择已确认病情"
          value={history ? HISTORY_OPTION : selected}
          disabled={view.confirmed.length === 0}
          onChange={(event) => {
            if (event.target.value === HISTORY_OPTION) {
              onOpenHistory?.();
              return;
            }
            onSelectCondition(event.target.value);
            onCloseHistory?.();
          }}
        >
          {view.confirmed.length === 0 ? (
            <option value="">暂无已确认病情</option>
          ) : (
            view.confirmed.map((item) => (
              <option key={item.conditionId} value={item.conditionId}>
                {item.name}
              </option>
            ))
          )}
          {onOpenHistory ? <option value={HISTORY_OPTION}>历史</option> : null}
        </select>
        <ChevronDown size={15} aria-hidden="true" />
      </span>
    </div>
  );
}

export interface TrainingWindowHeaderProps {
  view: TrainingSelection;
  onSelectPlan: (planId: string) => void;
}

/**
 * Penpot "Window / Title Training": 训练 / <plan picker>. Every plan record is
 * a user-accepted revision, so all of the user's plans are offered; an empty
 * list shows the title only, like Diagnosis / Empty.
 */
export function TrainingWindowHeader({
  view,
  onSelectPlan,
}: TrainingWindowHeaderProps) {
  const title = (separator: boolean) => (
    <>
      <span className="bc-window-context-title">
        <TrainingIcon size={18} aria-hidden="true" />
        训练
      </span>
      {separator ? (
        <span className="bc-window-context-sep" aria-hidden="true">
          /
        </span>
      ) : null}
    </>
  );
  if (view.plans.length === 0)
    return (
      <div className="bc-window-context bc-training-context">
        {title(false)}
      </div>
    );
  return (
    <div className="bc-window-context bc-training-context">
      {title(true)}
      <span className="bc-window-select-wrap">
        <select
          className="bc-window-context-select"
          aria-label="选择训练计划"
          value={view.selected?.planId ?? ""}
          onChange={(event) => onSelectPlan(event.target.value)}
        >
          {view.selected === null ? <option value="">选择计划</option> : null}
          {view.plans.map((item) => (
            <option key={item.planId} value={item.planId}>
              {item.title}
            </option>
          ))}
        </select>
        <ChevronDown size={15} aria-hidden="true" />
      </span>
    </div>
  );
}

/** Penpot Safety / Restricted: 训练 / 安全限制. */
export function TrainingSafetyHeader() {
  return (
    <div className="bc-window-context bc-training-context">
      <span className="bc-window-context-title">
        <TrainingIcon size={18} aria-hidden="true" />
        训练
      </span>
      <span className="bc-window-context-sep" aria-hidden="true">
        /
      </span>
      <span
        className="bc-window-select-wrap bc-window-context-mode"
        data-testid="training-mode"
      >
        安全限制
      </span>
    </div>
  );
}

/** Penpot "Window / Title Trainer": 训练 / <the session's template>. */
export function TrainerWindowHeader({
  templateTitle,
}: {
  templateTitle: string | null;
}) {
  return (
    <div className="bc-window-context bc-training-context">
      <span className="bc-window-context-title">
        <TrainingIcon size={18} aria-hidden="true" />
        训练
      </span>
      {templateTitle ? (
        <>
          <span className="bc-window-context-sep" aria-hidden="true">
            /
          </span>
          <span
            className="bc-window-select-wrap bc-window-context-mode"
            data-testid="trainer-template"
          >
            {templateTitle}
          </span>
        </>
      ) : null}
    </div>
  );
}

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
        <select
          className="bc-window-context-select"
          aria-label="跳到某天的对话"
          data-testid="chat-day"
          value={selectedDay ?? todayKey}
          onChange={(event) => onSelectDay(event.target.value)}
        >
          <option value={todayKey}>今天</option>
          {earlier.map((day) => (
            <option key={day.day} value={day.day}>
              {day.label}
            </option>
          ))}
        </select>
        <ChevronDown size={15} aria-hidden="true" />
      </span>
    </div>
  );
}

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
