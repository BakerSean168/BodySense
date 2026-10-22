/**
 * Penpot onboarding controls: full-width choice rows (choose → advance), two-column
 * multi-select chips and the centred value box used for date / height / weight.
 */
import type { ChoiceOption } from "./onboardingSteps";

const focusRing =
  "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#2C694F]";

interface ChoiceListProps {
  options: ChoiceOption[];
  value: string;
  onChoose: (value: string) => void;
}

/** Single choice rows (620×66, r14). Choosing a row also advances, as on the boards. */
export function ChoiceList({ options, value, onChoose }: ChoiceListProps) {
  return (
    <div
      role="group"
      aria-labelledby="onboarding-question"
      aria-describedby="onboarding-question-hint"
      className="flex flex-col gap-3"
    >
      {options.map((option) => {
        const selected = option.value === value;
        return (
          <button
            key={option.value}
            type="button"
            aria-pressed={selected}
            onClick={() => onChoose(option.value)}
            className={`flex h-[60px] w-full items-center justify-between rounded-[14px] border-2 px-[26px] text-left text-base motion-safe:transition-colors lg:h-[66px] ${focusRing} ${
              selected
                ? "border-[#2C694F] bg-[#E8F0EA] font-bold text-[#2C694F]"
                : "border-transparent bg-[#F7F7F7] text-[#1A1A1A] hover:bg-[#F0F2F0]"
            }`}
          >
            <span>{option.label}</span>
            {selected && <CheckDot />}
          </button>
        );
      })}
    </div>
  );
}

interface ChipGridProps {
  options: ChoiceOption[];
  values: string[];
  onToggle: (value: string) => void;
}

/** Multi-select chips, two columns (304×60, r10, gap 12). */
export function ChipGrid({ options, values, onToggle }: ChipGridProps) {
  return (
    <div
      role="group"
      aria-labelledby="onboarding-question"
      aria-describedby="onboarding-question-hint"
      className="grid grid-cols-2 gap-2.5 lg:gap-3"
    >
      {options.map((option) => {
        const selected = values.includes(option.value);
        return (
          <button
            key={option.value}
            type="button"
            aria-pressed={selected}
            onClick={() => onToggle(option.value)}
            className={`flex h-[52px] items-center rounded-[10px] border-2 px-[14px] text-left text-[15px] motion-safe:transition-colors lg:h-[60px] lg:px-[18px] ${focusRing} ${
              selected
                ? "border-[#2C694F] bg-[#E8F0EA] font-bold text-[#2C694F]"
                : "border-transparent bg-[#F7F7F7] text-[#1A1A1A] hover:bg-[#F0F2F0]"
            }`}
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}

function CheckDot() {
  return (
    <span
      aria-hidden="true"
      className="flex h-[26px] w-[26px] shrink-0 items-center justify-center rounded-full bg-[#2C694F] text-white"
    >
      <svg
        viewBox="0 0 16 16"
        className="h-3.5 w-3.5"
        fill="none"
        stroke="currentColor"
        strokeWidth={2}
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="m3.5 8.5 3 3 6-7" />
      </svg>
    </span>
  );
}

interface ValueBoxProps {
  id: string;
  label: string;
  type: "date" | "number";
  inputMode?: "numeric" | "decimal";
  value: string;
  onChange: (value: string) => void;
  unit?: string;
  min?: number | string;
  max?: number | string;
  step?: number;
  placeholder?: string;
  error?: string;
  onEnter?: () => void;
}

/**
 * Centred value box (320×76, r16, #F7F7F7) with an optional unit, as on steps 02–04.
 * The date variant is a masked "YYYY / MM / DD" text field (the board shows
 * "1995 / 06 / 15"), so it reads the same in every browser locale.
 */
export function ValueBox({
  id,
  label,
  type,
  value,
  onChange,
  unit,
  min,
  max,
  step,
  placeholder,
  error,
  onEnter,
  inputMode,
}: ValueBoxProps) {
  const errorId = `${id}-error`;
  return (
    <div className="flex flex-col items-center">
      <div className="flex h-[76px] w-full max-w-[320px] items-center justify-center gap-3 rounded-2xl bg-[#F7F7F7] px-6 focus-within:outline-2 focus-within:outline-offset-2 focus-within:outline-[#2C694F]">
        <input
          id={id}
          type={type === "date" ? "text" : "number"}
          inputMode={inputMode ?? (type === "number" ? "decimal" : "numeric")}
          autoComplete={type === "date" ? "bday" : undefined}
          aria-label={label}
          aria-invalid={error ? true : undefined}
          aria-describedby={error ? errorId : "onboarding-question-hint"}
          value={value}
          min={min}
          max={max}
          step={step}
          placeholder={placeholder}
          onChange={(event) => onChange(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter" && onEnter) onEnter();
          }}
          className={`min-w-0 bg-transparent text-center text-[#1A1A1A] outline-none placeholder:text-[#9CA3AF] ${
            type === "date"
              ? "w-full text-2xl"
              : "w-[96px] text-[28px] [appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
          }`}
        />
        {unit && (
          <span aria-hidden="true" className="text-[15px] text-[#6B7280]">
            {unit}
          </span>
        )}
      </div>
      {error && (
        <p id={errorId} role="alert" className="mt-3 text-sm text-[#B42318]">
          {error}
        </p>
      )}
    </div>
  );
}
