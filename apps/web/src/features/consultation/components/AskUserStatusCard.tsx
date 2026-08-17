import type { PendingInteraction } from "../types/consultation";

interface AskUserStatusCardProps {
  interaction: PendingInteraction;
}

function formatAnswer(answer: unknown): string | null {
  if (typeof answer === "string" && answer.trim().length > 0) {
    return answer.trim();
  }

  if (answer && typeof answer === "object") {
    const record = answer as Record<string, unknown>;
    if (typeof record.text === "string" && record.text.trim().length > 0) {
      return record.text.trim();
    }
    if (record.fields && typeof record.fields === "object") {
      const fields = record.fields as Record<string, unknown>;
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
  const answerText = formatAnswer(interaction.answer);
  const isAnswered = interaction.status === "answered";
  const isExpired = interaction.status === "expired";

  const title = isExpired
    ? "追问已过期"
    : isAnswered
      ? "问诊追问"
      : "待补充信息";
  const tone = isExpired
    ? "bg-[#F7F1EA] border-[#E6D5C4] text-[#6B4F3A]"
    : "bg-[#EEF4FF] border-[#D7E4FF] text-[#1F3558]";
  return (
    <div className="flex justify-start">
      <div className={`max-w-[80%] rounded-xl px-4 py-3 border ${tone}`}>
        <p
          className={`text-[11px] font-semibold tracking-wide ${isExpired ? "text-[#A67C52]" : "text-[#4D6FA3]"}`}
        >
          {title}
        </p>
        <p className="mt-1 text-sm font-medium leading-relaxed">
          {interaction.question.question}
        </p>
        {interaction.question.context && (
          <p className="mt-2 text-xs leading-relaxed text-[#58749C]">
            {interaction.question.context}
          </p>
        )}
        {isExpired ? (
          <p className="mt-3 text-xs font-medium text-[#8A5A2B]">
            该追问已过期，请在对话中重新说明相关信息以继续。
          </p>
        ) : isAnswered && answerText ? (
          <p className="mt-3 text-xs font-medium text-[#35527A]">
            你的回答：{answerText}
          </p>
        ) : (
          <p className="mt-3 text-xs font-medium text-[#35527A]">
            请直接在这张追问卡片中完成回答。
          </p>
        )}
      </div>
    </div>
  );
}
