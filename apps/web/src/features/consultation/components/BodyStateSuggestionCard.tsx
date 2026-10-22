import { useId } from "react";
import type {
  AskUserQuestion,
  BodyStateSuggestionReceipt,
  PendingInteraction,
} from "../types/consultation";

/**
 * Source-cited BodyState suggestion (BS-AGENT-001).
 *
 * Go bound the suggestion to the user's own words: `quote` is the excerpt Go
 * copied from the cited message and the question/options are Go's fixed
 * wording. The card never claims anything is saved: "saved" is shown only
 * from Go's commit receipt (see BodyStateSuggestionStatus).
 *
 * The pending card sits in the composer, which the canvas theme paints light;
 * the semantic ask-user-* / bss-* classes let body-canvas.css restyle it.
 */

export const BODY_STATE_SUGGESTION_PURPOSE = "body_state_suggestion";
const REJECT_OPTION = "不记录";

/** Go's fixed decline answer: nothing is written to the BodyState. */
export const BODY_STATE_SUGGESTION_DECLINE = {
  text: REJECT_OPTION,
  selected: [REJECT_OPTION],
} as const;

type SuggestionKind = "hypothesis" | "observation" | "episode";

const KIND_LABEL: Record<SuggestionKind, string> = {
  hypothesis: "你的猜测",
  observation: "身体观察",
  episode: "一次发作",
};

interface SuggestionBinding {
  kind: SuggestionKind;
  payload?: {
    observation_kind?: string;
    raw_value?: string;
    raw_unit?: string;
    symptom_type?: string;
  };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function isBodyStateSuggestion(question: AskUserQuestion): boolean {
  return (
    question.purpose === BODY_STATE_SUGGESTION_PURPOSE &&
    typeof question.quote === "string" &&
    readBinding(question) !== null
  );
}

function readBinding(question: AskUserQuestion): SuggestionBinding | null {
  const binding: unknown = question.state_binding;
  if (!isRecord(binding)) return null;
  const kind = binding.kind;
  if (kind !== "hypothesis" && kind !== "observation" && kind !== "episode") {
    return null;
  }
  return {
    kind,
    payload: isRecord(binding.payload) ? binding.payload : undefined,
  };
}

function confirmOption(question: AskUserQuestion): string | null {
  const option = question.options?.find((item) => item !== REJECT_OPTION);
  return option ?? null;
}

function SuggestionLabels({ binding }: { binding: SuggestionBinding }) {
  const payload = binding.payload;
  if (!payload) return null;
  const labels: Array<[string, string]> = [];
  if (payload.observation_kind) labels.push(["分类", payload.observation_kind]);
  if (payload.raw_value) {
    labels.push([
      "数值",
      [payload.raw_value, payload.raw_unit].filter(Boolean).join(" "),
    ]);
  }
  if (payload.symptom_type) labels.push(["症状", payload.symptom_type]);
  if (labels.length === 0) return null;
  return (
    <dl className="mb-3 flex flex-wrap gap-1.5 text-xs">
      {labels.map(([term, value]) => (
        <div
          key={term}
          className="bss-label flex gap-1 rounded-full border border-white/10 bg-white/[0.04] px-2.5 py-1"
        >
          <dt className="bss-muted text-white/45">{term}</dt>
          <dd className="bss-ink font-medium text-white/80">{value}</dd>
        </div>
      ))}
    </dl>
  );
}

interface BodyStateSuggestionCardProps {
  question: AskUserQuestion;
  onSubmit: (answer: unknown) => void;
  /** Decline, then let the user rewrite these words in the composer. */
  onEdit?: (quote: string) => void;
  isSubmitting?: boolean;
  error?: string | null;
  onRetry?: () => void;
}

export function BodyStateSuggestionCard({
  question,
  onSubmit,
  onEdit,
  isSubmitting = false,
  error,
  onRetry,
}: BodyStateSuggestionCardProps) {
  const headingId = useId();
  const binding = readBinding(question);
  const confirm = confirmOption(question);
  const quote = question.quote ?? "";
  if (!binding || !confirm) return null;

  const answer = (option: string) =>
    onSubmit({ text: option, selected: [option] });

  return (
    <section
      aria-labelledby={headingId}
      data-testid="body-state-suggestion-card"
      data-kind={binding.kind}
      className="ask-user-card bss-card my-2 rounded-xl border border-[#75d5a7]/25 bg-[#75d5a7]/[0.045] p-4 text-white/80"
    >
      <p className="ask-user-title mb-1 text-[11px] font-semibold tracking-wide text-[#9edbbd]/75">
        记录建议 · {KIND_LABEL[binding.kind]}
      </p>
      <p
        id={headingId}
        className="ask-user-question mb-3 text-sm font-medium text-white/90"
      >
        {question.question}
      </p>
      <blockquote
        data-testid="body-state-suggestion-quote"
        className="bss-quote mb-3 border-l-2 border-[#75d5a7]/45 pl-3 text-sm leading-relaxed text-white/85"
      >
        <span className="bss-muted mb-0.5 block text-[11px] text-white/40">
          你说过
        </span>
        “{quote}”
      </blockquote>
      <SuggestionLabels binding={binding} />
      <p className="ask-user-context mb-3 text-xs leading-relaxed text-white/42">
        只保存你的原话；是否已保存以系统回执为准。
      </p>

      {error && (
        <div
          role="alert"
          className="ask-user-error mb-3 rounded border border-red-300/15 bg-red-300/[0.055] p-3"
        >
          <p className="text-sm text-red-200/80">提交失败：{error}</p>
          {onRetry && (
            <button
              type="button"
              className="mt-2 text-sm text-red-300 underline hover:text-red-200"
              onClick={onRetry}
              disabled={isSubmitting}
            >
              重试
            </button>
          )}
        </div>
      )}

      <div className="flex flex-wrap items-center gap-2">
        <button
          type="button"
          className="ask-user-submit rounded-full bg-white/90 px-4 py-2 text-sm font-semibold text-[#171717] transition-colors hover:bg-white disabled:opacity-45"
          onClick={() => answer(confirm)}
          disabled={isSubmitting}
        >
          {isSubmitting ? "提交中..." : confirm}
        </button>
        {onEdit && (
          <button
            type="button"
            className="bss-secondary rounded-full border border-white/15 px-4 py-2 text-sm font-medium text-white/80 transition-colors hover:bg-white/[0.06] disabled:opacity-45"
            onClick={() => onEdit(quote)}
            disabled={isSubmitting}
          >
            改一改再说
          </button>
        )}
        <button
          type="button"
          className="bss-tertiary px-2 py-2 text-sm font-medium text-white/55 transition-colors hover:text-white/80 disabled:opacity-45"
          onClick={() => answer(REJECT_OPTION)}
          disabled={isSubmitting}
        >
          {REJECT_OPTION}
        </button>
      </div>
    </section>
  );
}

function readReceipt(
  interaction: PendingInteraction,
): BodyStateSuggestionReceipt | null | undefined {
  if (interaction.body_state_receipt !== undefined) {
    return interaction.body_state_receipt;
  }
  const stored = interaction.metadata?.body_state_receipt;
  if (isRecord(stored) && typeof stored.commit_id === "string") {
    return stored as BodyStateSuggestionReceipt;
  }
  return undefined;
}

function selectedOption(answer: unknown): string | null {
  if (!isRecord(answer) || !Array.isArray(answer.selected)) return null;
  const [first] = answer.selected;
  return typeof first === "string" ? first : null;
}

/** Answered/expired suggestion: "saved" only with Go's receipt. */
export function BodyStateSuggestionStatus({
  interaction,
}: {
  interaction: PendingInteraction;
}) {
  const binding = readBinding(interaction.question);
  const kindLabel = binding ? KIND_LABEL[binding.kind] : "记录";
  const receipt = readReceipt(interaction);
  const declined = selectedOption(interaction.answer) === REJECT_OPTION;
  let state: "saved" | "declined" | "expired" | "waiting" | "answered";
  if (interaction.status === "expired") state = "expired";
  else if (interaction.status !== "answered") state = "waiting";
  else if (receipt) state = "saved";
  else if (declined) state = "declined";
  else state = "answered";

  const title = {
    saved: `已保存为${kindLabel}`,
    declined: "未记录",
    expired: "记录建议已过期，未保存",
    waiting: "记录建议",
    answered: "记录建议",
  }[state];

  return (
    <div className="flex justify-start">
      <div
        data-testid={`body-state-suggestion-${state}`}
        data-tone={state === "saved" ? "saved" : "neutral"}
        className={`chat-status-card w-full max-w-[620px] rounded-xl border px-4 py-3 ${
          state === "saved"
            ? "border-[#75d5a7]/25 bg-[#75d5a7]/[0.05]"
            : "border-white/10 bg-white/[0.03]"
        }`}
      >
        <p
          className={`chat-status-title text-[11px] font-semibold tracking-wide ${
            state === "saved" ? "text-[#9edbbd]" : "text-white/50"
          }`}
        >
          {title}
        </p>
        {interaction.question.quote ? (
          <p className="chat-status-question mt-1 text-sm leading-relaxed text-white/80">
            “{interaction.question.quote}”
          </p>
        ) : null}
        {state === "declined" ? (
          <p className="chat-muted mt-2 text-xs text-white/45">
            这句话没有写入你的身体记录。
          </p>
        ) : null}
      </div>
    </div>
  );
}
