import { useState } from "react";
import type { AskUserField, AskUserQuestion } from "../types/consultation";

interface AskUserCardProps {
  question: AskUserQuestion;
  onSubmit: (answer: unknown) => void;
  isSubmitting?: boolean;
  error?: string | null;
  onRetry?: () => void;
  title?: string;
}

/**
 * Renders a pending ask_user interaction card.
 * Supports text, single_choice, multi_choice, number, and date answer types,
 * plus optional multi-field forms (T0-1).
 */
export function AskUserCard({
  question,
  onSubmit,
  isSubmitting = false,
  error,
  onRetry,
  title = "问诊追问",
}: AskUserCardProps) {
  const [textAnswer, setTextAnswer] = useState("");
  const [selectedSingle, setSelectedSingle] = useState<string>("");
  const [selectedMulti, setSelectedMulti] = useState<string[]>([]);
  const [customSingleEnabled, setCustomSingleEnabled] = useState(false);
  const [customMultiEnabled, setCustomMultiEnabled] = useState(false);
  const [customAnswer, setCustomAnswer] = useState("");
  const [fieldValues, setFieldValues] = useState<Record<string, string>>({});

  const allowCustomInput =
    question.allow_custom_input ?? Boolean(question.options?.length);
  const multiFields = question.fields?.slice(0, 3) ?? [];
  const isMultiField = multiFields.length > 0;

  const buildCustomAnswerPayload = (values: string[]) => {
    const normalized = values.filter((value) => value.trim().length > 0);
    if (normalized.length === 0) {
      return null;
    }
    return {
      text: normalized.join("，"),
      selected: normalized,
      is_custom: true,
    };
  };

  const handleMultiFieldSubmit = () => {
    const fields: Record<string, string> = {};
    const missing: string[] = [];
    for (const field of multiFields) {
      const raw = (fieldValues[field.key] ?? "").trim();
      if (!raw && field.required !== false) {
        missing.push(field.label);
        continue;
      }
      if (raw) {
        fields[field.key] = raw;
      }
    }
    if (missing.length > 0) {
      return;
    }
    const text = multiFields
      .filter((f) => fields[f.key])
      .map((f) => `${f.label}: ${fields[f.key]}`)
      .join("；");
    onSubmit({ text, fields });
  };

  const handleSubmit = () => {
    if (isMultiField) {
      handleMultiFieldSubmit();
      return;
    }
    switch (question.answer_type) {
      case "text":
        if (textAnswer.trim()) onSubmit({ text: textAnswer.trim() });
        break;
      case "single_choice":
        if (customSingleEnabled) {
          const payload = buildCustomAnswerPayload([customAnswer]);
          if (payload) onSubmit(payload);
          break;
        }
        if (selectedSingle)
          onSubmit({ text: selectedSingle, selected: [selectedSingle] });
        break;
      case "multi_choice":
        {
          if (!customMultiEnabled && selectedMulti.length > 0) {
            onSubmit({
              text: selectedMulti.join("，"),
              selected: selectedMulti,
            });
            break;
          }
          const payload = buildCustomAnswerPayload([
            ...selectedMulti,
            customAnswer.trim(),
          ]);
          if (payload) onSubmit(payload);
        }
        break;
      case "number":
        if (textAnswer.trim())
          onSubmit({
            text: textAnswer.trim(),
            value: Number(textAnswer.trim()),
          });
        break;
      case "date":
        if (textAnswer.trim()) onSubmit({ text: textAnswer.trim() });
        break;
    }
  };

  const renderFieldInput = (field: AskUserField) => {
    const value = fieldValues[field.key] ?? "";
    const setValue = (next: string) =>
      setFieldValues((prev) => ({ ...prev, [field.key]: next }));

    if (field.answer_type === "single_choice" && field.options?.length) {
      return (
        <div className="space-y-1.5">
          {field.options.map((opt) => (
            <label key={opt} className="flex items-center gap-2 text-sm">
              <input
                type="radio"
                name={`field_${field.key}`}
                checked={value === opt}
                onChange={() => setValue(opt)}
                disabled={isSubmitting}
              />
              {opt}
            </label>
          ))}
        </div>
      );
    }

    if (field.answer_type === "multi_choice" && field.options?.length) {
      const selected = value ? value.split("，").filter(Boolean) : [];
      return (
        <div className="space-y-1.5">
          {field.options.map((opt) => (
            <label key={opt} className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={selected.includes(opt)}
                onChange={(e) => {
                  const next = e.target.checked
                    ? [...selected, opt]
                    : selected.filter((o) => o !== opt);
                  setValue(next.join("，"));
                }}
                disabled={isSubmitting}
              />
              {opt}
            </label>
          ))}
        </div>
      );
    }

    const inputType =
      field.answer_type === "number" || field.answer_type === "scale"
        ? "number"
        : field.answer_type === "date"
          ? "date"
          : "text";

    return (
      <input
        type={inputType}
        className="w-full rounded-lg border border-white/10 bg-white/[0.045] px-3 py-2 text-sm text-white/85 outline-none placeholder:text-white/30 focus:border-[#75d5a7]/45"
        placeholder={field.label}
        value={value}
        onChange={(e) => setValue(e.target.value)}
        disabled={isSubmitting}
      />
    );
  };

  return (
    <div className="my-2 rounded-xl border border-sky-300/10 bg-sky-300/[0.04] p-4 text-white/80">
      <p className="mb-1 text-[11px] font-semibold tracking-wide text-sky-200/65">
        {title}
      </p>
      <p className="mb-3 text-sm font-medium text-white/90">
        {question.question}
      </p>

      {question.context && (
        <p className="mb-3 text-xs leading-relaxed text-white/45">
          {question.context}
        </p>
      )}

      {error && (
        <div className="mb-3 rounded border border-red-300/15 bg-red-300/[0.055] p-3">
          <p className="text-sm text-red-200/80">提交失败：{error}</p>
          {onRetry && (
            <button
              className="mt-2 text-sm text-red-300 underline hover:text-red-200"
              onClick={onRetry}
            >
              重试
            </button>
          )}
        </div>
      )}

      {isMultiField ? (
        <div className="space-y-4 mb-3">
          {multiFields.map((field) => (
            <div key={field.key}>
              <label className="mb-1 block text-xs font-semibold text-white/60">
                {field.label}
                {field.required !== false ? " *" : ""}
              </label>
              {renderFieldInput(field)}
            </div>
          ))}
        </div>
      ) : (
        <>
          {(question.answer_type === "text" ||
            question.answer_type === "number" ||
            question.answer_type === "date") && (
            <input
              type={
                question.answer_type === "number"
                  ? "number"
                  : question.answer_type === "date"
                    ? "date"
                    : "text"
              }
              className="mb-3 w-full rounded-lg border border-white/10 bg-white/[0.045] px-3 py-2 text-sm text-white/85 outline-none placeholder:text-white/30 focus:border-[#75d5a7]/45"
              placeholder={
                question.answer_type === "text"
                  ? "输入你的回答..."
                  : question.answer_type === "number"
                    ? "输入数字..."
                    : "选择日期..."
              }
              value={textAnswer}
              onChange={(e) => setTextAnswer(e.target.value)}
              disabled={isSubmitting}
            />
          )}

          {question.answer_type === "single_choice" && question.options && (
            <div className="space-y-2 mb-3">
              {question.options.map((opt) => (
                <label key={opt} className="flex items-center gap-2 text-sm">
                  <input
                    type="radio"
                    name="ask_user_single"
                    value={opt}
                    checked={selectedSingle === opt}
                    onChange={() => {
                      setSelectedSingle(opt);
                      setCustomSingleEnabled(false);
                    }}
                    disabled={isSubmitting}
                  />
                  {opt}
                </label>
              ))}
              {allowCustomInput && (
                <label className="flex items-center gap-2 text-sm">
                  <input
                    type="radio"
                    name="ask_user_single"
                    checked={customSingleEnabled}
                    onChange={() => {
                      setSelectedSingle("");
                      setCustomSingleEnabled(true);
                    }}
                    disabled={isSubmitting}
                  />
                  自定义输入
                </label>
              )}
              {allowCustomInput && customSingleEnabled && (
                <input
                  type="text"
                  className="w-full rounded-lg border border-white/10 bg-white/[0.045] px-3 py-2 text-sm text-white/85 outline-none placeholder:text-white/30 focus:border-[#75d5a7]/45"
                  placeholder="请输入你的补充回答..."
                  value={customAnswer}
                  onChange={(e) => setCustomAnswer(e.target.value)}
                  disabled={isSubmitting}
                />
              )}
            </div>
          )}

          {question.answer_type === "multi_choice" && question.options && (
            <div className="space-y-2 mb-3">
              {question.options.map((opt) => (
                <label key={opt} className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={selectedMulti.includes(opt)}
                    onChange={(e) => {
                      if (e.target.checked) {
                        setSelectedMulti([...selectedMulti, opt]);
                      } else {
                        setSelectedMulti(
                          selectedMulti.filter((o) => o !== opt),
                        );
                      }
                    }}
                    disabled={isSubmitting}
                  />
                  {opt}
                </label>
              ))}
              {allowCustomInput && (
                <label className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={customMultiEnabled}
                    onChange={(e) => setCustomMultiEnabled(e.target.checked)}
                    disabled={isSubmitting}
                  />
                  自定义输入
                </label>
              )}
              {allowCustomInput && customMultiEnabled && (
                <input
                  type="text"
                  className="w-full rounded-lg border border-white/10 bg-white/[0.045] px-3 py-2 text-sm text-white/85 outline-none placeholder:text-white/30 focus:border-[#75d5a7]/45"
                  placeholder="请输入你的补充回答..."
                  value={customAnswer}
                  onChange={(e) => setCustomAnswer(e.target.value)}
                  disabled={isSubmitting}
                />
              )}
            </div>
          )}
        </>
      )}

      <button
        className="rounded-full bg-white/90 px-4 py-2 text-sm font-semibold text-[#171717] transition-colors hover:bg-white disabled:opacity-45"
        onClick={handleSubmit}
        disabled={isSubmitting}
      >
        {isSubmitting ? "提交中..." : "提交"}
      </button>
    </div>
  );
}
