import { useId, useState } from "react";
import type { AskUserField, AskUserQuestion } from "../types/consultation";
import {
  BodyStateSuggestionCard,
  isBodyStateSuggestion,
} from "./BodyStateSuggestionCard";
import { Checkbox } from "@/components/ui/checkbox";
import { DatePicker } from "@/components/ui/date-picker";
import { Input } from "@/components/ui/Input";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";

const CUSTOM_CHOICE = "__custom__";

interface AskUserCardProps {
  question: AskUserQuestion;
  onSubmit: (answer: unknown) => void;
  isSubmitting?: boolean;
  error?: string | null;
  onRetry?: () => void;
  title?: string;
  /** BodyState suggestion only: decline, then rewrite the words in the composer. */
  onEdit?: (quote: string) => void;
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
  onEdit,
}: AskUserCardProps) {
  const questionId = useId();
  const [textAnswer, setTextAnswer] = useState("");
  const [selectedSingle, setSelectedSingle] = useState<string>("");
  const [selectedMulti, setSelectedMulti] = useState<string[]>([]);
  const [customSingleEnabled, setCustomSingleEnabled] = useState(false);
  const [customMultiEnabled, setCustomMultiEnabled] = useState(false);
  const [customAnswer, setCustomAnswer] = useState("");
  const [fieldValues, setFieldValues] = useState<Record<string, string>>({});

  if (isBodyStateSuggestion(question)) {
    return (
      <BodyStateSuggestionCard
        question={question}
        onSubmit={onSubmit}
        onEdit={onEdit}
        isSubmitting={isSubmitting}
        error={error}
        onRetry={onRetry}
      />
    );
  }

  const allowCustomInput = question.allow_custom_input === true;
  const multiFields = question.fields ?? [];
  const isMultiField = multiFields.length > 0;

  const buildCustomAnswerPayload = (values: string[]) => {
    const normalized = values.map((value) => value.trim()).filter(Boolean);
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
    const fields: Record<string, string | string[]> = {};
    const missing: string[] = [];
    for (const field of multiFields) {
      const raw = (fieldValues[field.key] ?? "").trim();
      if (!raw && field.required !== false) {
        missing.push(field.label);
        continue;
      }
      if (raw) {
        fields[field.key] =
          field.answer_type === "multi_choice"
            ? raw.split("，").filter(Boolean)
            : raw;
      }
    }
    if (missing.length > 0) {
      return;
    }
    const text = multiFields
      .filter((f) => fields[f.key])
      .map((f) => {
        const value = fields[f.key];
        return `${f.label}: ${Array.isArray(value) ? value.join("，") : value}`;
      })
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
        <RadioGroup
          className="gap-1.5"
          name={`${questionId}-${field.key}`}
          aria-label={field.label}
          value={value}
          onValueChange={(next) => setValue(String(next))}
          disabled={isSubmitting}
        >
          {field.options.map((opt) => (
            <label key={opt} className="flex items-center gap-2 text-sm">
              <RadioGroupItem value={opt} />
              {opt}
            </label>
          ))}
        </RadioGroup>
      );
    }

    if (field.answer_type === "multi_choice" && field.options?.length) {
      const selected = value ? value.split("，").filter(Boolean) : [];
      return (
        <div className="space-y-1.5">
          {field.options.map((opt) => (
            <label key={opt} className="flex items-center gap-2 text-sm">
              <Checkbox
                checked={selected.includes(opt)}
                onCheckedChange={(checked) => {
                  const next = checked
                    ? field.exclusive_options?.includes(opt)
                      ? [opt]
                      : [
                          ...selected.filter(
                            (o) => !field.exclusive_options?.includes(o),
                          ),
                          opt,
                        ]
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

    if (inputType === "date")
      return (
        <DatePicker
          aria-label={field.label}
          placeholder={field.label}
          value={value}
          onValueChange={setValue}
          disabled={isSubmitting}
          yearNavigation
        />
      );

    return (
      <Input
        type={inputType}
        placeholder={field.label}
        aria-label={field.label}
        value={value}
        onChange={(e) => setValue(e.target.value)}
        disabled={isSubmitting}
      />
    );
  };

  return (
    <div className="ask-user-card my-2 rounded-xl border border-sky-300/10 bg-sky-300/[0.04] p-4 text-white/80">
      <p className="ask-user-title mb-1 text-[11px] font-semibold tracking-wide text-sky-200/65">
        {title}
      </p>
      <p
        id={questionId}
        className="ask-user-question mb-3 text-sm font-medium text-white/90"
      >
        {question.question}
      </p>

      {question.context && (
        <p className="ask-user-context mb-3 text-xs leading-relaxed text-white/45">
          {question.context}
        </p>
      )}

      {error && (
        <div
          role="alert"
          className="ask-user-error mb-3 rounded border border-red-300/15 bg-red-300/[0.055] p-3"
        >
          <p className="text-sm text-red-200/80">提交失败：{error}</p>
          {onRetry && (
            <button
              className="mt-2 text-sm text-red-300 underline hover:text-red-200"
              onClick={onRetry}
              disabled={isSubmitting}
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
            question.answer_type === "number") && (
            <Input
              type={question.answer_type === "number" ? "number" : "text"}
              className="mb-3"
              aria-labelledby={questionId}
              placeholder={
                question.answer_type === "text"
                  ? "输入你的回答..."
                  : "输入数字..."
              }
              value={textAnswer}
              onChange={(e) => setTextAnswer(e.target.value)}
              disabled={isSubmitting}
            />
          )}
          {question.answer_type === "date" && (
            <DatePicker
              className="mb-3"
              aria-labelledby={questionId}
              placeholder="选择日期..."
              value={textAnswer}
              onValueChange={setTextAnswer}
              disabled={isSubmitting}
              yearNavigation
            />
          )}

          {question.answer_type === "single_choice" && question.options && (
            <div className="mb-3 space-y-2">
              <RadioGroup
                className="gap-2"
                name={questionId}
                aria-labelledby={questionId}
                value={customSingleEnabled ? CUSTOM_CHOICE : selectedSingle}
                onValueChange={(next) => {
                  if (next === CUSTOM_CHOICE) {
                    setSelectedSingle("");
                    setCustomSingleEnabled(true);
                    return;
                  }
                  setSelectedSingle(String(next));
                  setCustomSingleEnabled(false);
                }}
                disabled={isSubmitting}
              >
                {question.options.map((opt) => (
                  <label key={opt} className="flex items-center gap-2 text-sm">
                    <RadioGroupItem value={opt} />
                    {opt}
                  </label>
                ))}
                {allowCustomInput && (
                  <label className="flex items-center gap-2 text-sm">
                    <RadioGroupItem value={CUSTOM_CHOICE} />
                    自定义输入
                  </label>
                )}
              </RadioGroup>
              {allowCustomInput && customSingleEnabled && (
                <Input
                  type="text"
                  aria-label="自定义回答"
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
                  <Checkbox
                    checked={selectedMulti.includes(opt)}
                    onCheckedChange={(checked) => {
                      if (checked) {
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
                  <Checkbox
                    checked={customMultiEnabled}
                    onCheckedChange={(checked) =>
                      setCustomMultiEnabled(checked)
                    }
                    disabled={isSubmitting}
                  />
                  自定义输入
                </label>
              )}
              {allowCustomInput && customMultiEnabled && (
                <Input
                  type="text"
                  aria-label="自定义回答"
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
        className="ask-user-submit rounded-full bg-white/90 px-4 py-2 text-sm font-semibold text-[#171717] transition-colors hover:bg-white disabled:opacity-45"
        onClick={handleSubmit}
        disabled={isSubmitting}
      >
        {isSubmitting ? "提交中..." : "提交"}
      </button>
    </div>
  );
}
