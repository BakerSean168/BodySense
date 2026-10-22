import * as React from "react";
import { CalendarIcon } from "lucide-react";

import { cn } from "@/lib/utils";
import {
  fieldVariants,
  type FieldVariant,
} from "@/components/ui/field-variants";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";

// DayPicker and its locale load only when a date picker first opens.
const Calendar = React.lazy(() =>
  import("@/components/ui/calendar").then((module) => ({
    default: module.Calendar,
  })),
);

/** "YYYY-MM-DD" -> local Date (no UTC shift); invalid -> undefined. */
export function parseIsoDate(value: string | null | undefined) {
  const match = value ? /^(\d{4})-(\d{2})-(\d{2})$/.exec(value) : null;
  if (!match) return undefined;
  const date = new Date(
    Number(match[1]),
    Number(match[2]) - 1,
    Number(match[3]),
  );
  return Number.isNaN(date.getTime()) ? undefined : date;
}

/** Local Date -> "YYYY-MM-DD". */
export function toIsoDate(date: Date) {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

export interface DatePickerProps {
  /** ISO calendar date "YYYY-MM-DD", or "" when empty. */
  value: string;
  onValueChange: (value: string) => void;
  /** Inclusive bounds as "YYYY-MM-DD". */
  min?: string;
  max?: string;
  placeholder?: string;
  /** Shows a 清除 action that sets the value back to "". */
  clearable?: boolean;
  variant?: FieldVariant;
  id?: string;
  disabled?: boolean;
  className?: string;
  /** Year/month dropdowns, for dates far from today such as a birthday. */
  yearNavigation?: boolean;
  "aria-label"?: string;
  "aria-labelledby"?: string;
  "aria-describedby"?: string;
  "aria-invalid"?: boolean;
  "data-testid"?: string;
}

/**
 * Date field: a themed trigger that opens a paper popover with the Calendar.
 * Replaces `<input type="date">`; the value contract stays "YYYY-MM-DD".
 */
export function DatePicker({
  value,
  onValueChange,
  min,
  max,
  placeholder = "选择日期",
  clearable = false,
  variant = "field",
  id,
  disabled,
  className,
  yearNavigation = false,
  ...aria
}: DatePickerProps) {
  const [open, setOpen] = React.useState(false);
  const selected = parseIsoDate(value);
  const minDate = parseIsoDate(min);
  const maxDate = parseIsoDate(max);
  const [month, setMonth] = React.useState<Date>(
    () => selected ?? maxDate ?? new Date(),
  );
  React.useEffect(() => {
    if (open) setMonth(selected ?? maxDate ?? new Date());
    // Re-centre on the value only when the popover opens.
  }, [open]);

  const disabledDays = [
    ...(minDate ? [{ before: minDate }] : []),
    ...(maxDate ? [{ after: maxDate }] : []),
  ];
  const label = selected
    ? `${selected.getFullYear()}/${String(selected.getMonth() + 1).padStart(2, "0")}/${String(selected.getDate()).padStart(2, "0")}`
    : placeholder;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        id={id}
        disabled={disabled}
        data-slot="date-picker-trigger"
        data-value={value}
        data-empty={selected ? "false" : "true"}
        className={cn(
          fieldVariants({ variant }),
          "flex cursor-pointer items-center justify-between gap-2 py-1.5 text-left tabular-nums select-none data-[empty=true]:text-(--ui-muted)",
          className,
        )}
        {...aria}
      >
        <span className="min-w-0 flex-1 truncate">{label}</span>
        <CalendarIcon
          aria-hidden="true"
          className="size-4 shrink-0 text-(--ui-secondary)"
        />
      </PopoverTrigger>
      <PopoverContent
        className="w-auto gap-1 p-2"
        aria-label={aria["aria-label"] ?? "选择日期"}
      >
        <React.Suspense
          fallback={<div className="h-[300px] w-[268px]" aria-busy="true" />}
        >
          <Calendar
            mode="single"
            selected={selected}
            month={month}
            onMonthChange={setMonth}
            captionLayout={yearNavigation ? "dropdown" : "label"}
            startMonth={
              minDate ?? (yearNavigation ? new Date(1900, 0) : undefined)
            }
            endMonth={
              maxDate ??
              (yearNavigation
                ? new Date(new Date().getFullYear() + 5, 11)
                : undefined)
            }
            disabled={disabledDays}
            onSelect={(date) => {
              if (!date) return;
              onValueChange(toIsoDate(date));
              setOpen(false);
            }}
          />
        </React.Suspense>
        {clearable && selected ? (
          <div className="flex justify-end border-t border-(--ui-paper-line) px-1 pt-2">
            <button
              type="button"
              className="rounded-lg px-2.5 py-1 text-xs font-semibold text-(--ui-secondary) outline-none hover:bg-(--ui-highlight) hover:text-(--ui-primary) focus-visible:ring-2 focus-visible:ring-(--ui-primary)"
              onClick={() => {
                onValueChange("");
                setOpen(false);
              }}
            >
              清除
            </button>
          </div>
        ) : null}
      </PopoverContent>
    </Popover>
  );
}
