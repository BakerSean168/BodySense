import { CircleHelp } from "lucide-react";
import type { PendingInteraction } from "../types/consultation";
import {
  BodyStateSuggestionStatus,
  isBodyStateSuggestion,
} from "./BodyStateSuggestionCard";

interface AskUserStatusCardProps {
  interaction: PendingInteraction;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function formatAnswer(answer: unknown): string | null {
  if (typeof answer === "string" && answer.trim().length > 0) {
    return answer.trim();
  }

  if (isRecord(answer)) {
    const record = answer;
    if (typeof record.text === "string" && record.text.trim().length > 0) {
      return record.text.trim();
    }
    if (isRecord(record.fields)) {
      const fields = record.fields;
      const parts = Object.entries(fields)
        .filter(
          ([, value]) =>
            value !== undefined && value !== null && String(value).length > 0,
        )
        .map(([key, value]) => `${key}: ${String(value)}`);
      if (parts.length > 0) return parts.join("；");
    }
    if (Array.isArray(record.selected) && record.selected.length > 0) {
      return record.selected.map((value) => String(value)).join("，");
    }
    if (record.value !== undefined && record.value !== null) {
      return String(record.value);
    }
  }

  return null;
}

export function AskUserStatusCard({ interaction }: AskUserStatusCardProps) {
  if (isBodyStateSuggestion(interaction.question)) {
    return <BodyStateSuggestionStatus interaction={interaction} />;
  }
  const answerText = formatAnswer(interaction.answer);
  const isAnswered = interaction.status === "answered";
  const isExpired = interaction.status === "expired";

  const isSymptomIntake = interaction.question.purpose === "symptom_intake";
  const title = isExpired
    ? isSymptomIntake
      ? "身体记录补全已过期"
      : "追问已过期"
    : isAnswered
      ? isSymptomIntake
        ? "身体记录已补全"
        : "问诊追问"
      : isSymptomIntake
        ? "补全身体记录"
        : "待补充信息";

  if (isAnswered && answerText) {
    // Penpot Chat / Running: the answered question stays in the thread as a
    // quiet card, the question beside a "?" disc and the answer as a chip.
    return (
      <div className="flex justify-start">
        <section
          data-tone="answered"
          aria-label={`${title}（已回答）`}
          className="chat-status-card chat-answered-card w-full max-w-[620px] rounded-xl border border-white/10 bg-white/[0.035] px-4 py-3.5"
        >
          <p className="chat-answered-question flex items-start gap-2.5 text-[13px] leading-5 text-white/62">
            <span
              className="chat-answered-icon mt-px grid size-5 shrink-0 place-items-center rounded-full bg-amber-300/10 text-amber-300/80"
              aria-hidden="true"
            >
              <CircleHelp size={13} strokeWidth={2.4} />
            </span>
            <span className="chat-status-question">
              {interaction.question.question}
            </span>
          </p>
          <p className="chat-answered-chip mt-2.5 inline-flex max-w-full items-center gap-1.5 rounded-full bg-[#75d5a7]/10 px-2.5 py-0.5 text-xs font-semibold leading-5 text-[#9edbbd]">
            <span
              className="chat-answered-dot size-1.5 shrink-0 rounded-full bg-current"
              aria-hidden="true"
            />
            <span className="sr-only">你的回答：</span>
            <span className="min-w-0 break-words">{answerText}</span>
          </p>
        </section>
      </div>
    );
  }

  return (
    <div className="flex justify-start">
      <div
        data-tone={isExpired ? "warning" : "info"}
        className={`chat-status-card w-full max-w-[620px] rounded-xl border px-4 py-3 ${
          isExpired
            ? "border-amber-300/12 bg-amber-300/[0.045] text-amber-100/80"
            : "border-sky-300/10 bg-sky-300/[0.04] text-white/80"
        }`}
      >
        <p
          className={`chat-status-title text-[11px] font-semibold tracking-wide ${
            isExpired ? "text-amber-200/65" : "text-sky-200/65"
          }`}
        >
          {title}
        </p>
        <p className="chat-status-question mt-1 text-sm font-medium leading-relaxed text-white/85">
          {interaction.question.question}
        </p>
        {interaction.question.context ? (
          <p className="chat-muted mt-2 text-xs leading-relaxed text-white/42">
            {interaction.question.context}
          </p>
        ) : null}
        {isExpired ? (
          <p className="chat-status-warning mt-3 text-xs font-medium text-amber-200/65">
            该追问已过期，请在对话中重新说明相关信息以继续。
          </p>
        ) : isAnswered && answerText ? (
          <p className="chat-status-answer mt-3 text-xs font-medium text-[#9edbbd]">
            你的回答：{answerText}
          </p>
        ) : (
          <p className="chat-muted mt-3 text-xs text-white/48">
            请直接在追问卡片中完成回答。
          </p>
        )}
      </div>
    </div>
  );
}
