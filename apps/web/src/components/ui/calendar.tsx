import * as React from "react";
import {
  DayPicker,
  getDefaultClassNames,
  type DayButton,
  type DropdownProps,
} from "react-day-picker";
import { zhCN } from "react-day-picker/locale";
import {
  ChevronDownIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
} from "lucide-react";

import { cn } from "@/lib/utils";
import { SelectField } from "@/components/ui/select";

/**
 * Month/year dropdowns use the themed Select instead of DayPicker's native
 * `<select>`, so no OS list ever opens inside the calendar.
 */
function CalendarDropdown({
  options = [],
  value,
  onChange,
  disabled,
  "aria-label": ariaLabel,
}: DropdownProps) {
  return (
    <SelectField
      variant="ghost"
      aria-label={ariaLabel}
      disabled={disabled}
      value={value === undefined ? "" : String(value)}
      options={options.map((option) => ({
        value: String(option.value),
        label: option.label,
        disabled: option.disabled,
      }))}
      className="w-auto min-w-0 gap-1 px-2 text-sm font-semibold"
      contentClassName="min-w-24"
      onValueChange={(next) =>
        onChange?.({
          target: { value: next },
        } as React.ChangeEvent<HTMLSelectElement>)
      }
    />
  );
}

function Calendar({
  className,
  classNames,
  showOutsideDays = true,
  captionLayout = "label",
  locale = zhCN,
  formatters,
  components,
  ...props
}: React.ComponentProps<typeof DayPicker>) {
  const defaultClassNames = getDefaultClassNames();

  return (
    <DayPicker
      showOutsideDays={showOutsideDays}
      className={cn(
        "group/calendar p-1 text-(--ui-ink) [--cell-radius:9px] [--cell-size:--spacing(9)]",
        className,
      )}
      captionLayout={captionLayout}
      locale={locale}
      formatters={{
        formatMonthDropdown: (date) =>
          date.toLocaleString(locale?.code, { month: "short" }),
        ...formatters,
      }}
      classNames={{
        root: cn("w-fit", defaultClassNames.root),
        months: cn("relative flex flex-col gap-4", defaultClassNames.months),
        month: cn("flex w-full flex-col gap-3", defaultClassNames.month),
        nav: cn(
          "absolute inset-x-0 top-0 flex w-full items-center justify-between gap-1",
          defaultClassNames.nav,
        ),
        button_previous: cn(
          "inline-flex size-(--cell-size) items-center justify-center rounded-(--cell-radius) text-(--ui-secondary) outline-none select-none hover:bg-(--ui-highlight) hover:text-(--ui-primary) focus-visible:ring-2 focus-visible:ring-(--ui-primary) aria-disabled:opacity-40",
          defaultClassNames.button_previous,
        ),
        button_next: cn(
          "inline-flex size-(--cell-size) items-center justify-center rounded-(--cell-radius) text-(--ui-secondary) outline-none select-none hover:bg-(--ui-highlight) hover:text-(--ui-primary) focus-visible:ring-2 focus-visible:ring-(--ui-primary) aria-disabled:opacity-40",
          defaultClassNames.button_next,
        ),
        month_caption: cn(
          "flex h-(--cell-size) w-full items-center justify-center px-(--cell-size)",
          defaultClassNames.month_caption,
        ),
        dropdowns: cn(
          "flex h-(--cell-size) w-full items-center justify-center gap-0.5 text-sm font-semibold",
          defaultClassNames.dropdowns,
        ),
        dropdown_root: cn("relative", defaultClassNames.dropdown_root),
        caption_label: cn(
          "text-sm font-semibold select-none",
          defaultClassNames.caption_label,
        ),
        month_grid: cn("w-full border-collapse", defaultClassNames.month_grid),
        weekdays: cn("flex", defaultClassNames.weekdays),
        weekday: cn(
          "flex-1 text-[0.75rem] font-medium text-(--ui-muted) select-none",
          defaultClassNames.weekday,
        ),
        week: cn("mt-1 flex w-full", defaultClassNames.week),
        day: cn(
          "group/day relative aspect-square h-full w-full p-0 text-center select-none",
          defaultClassNames.day,
        ),
        today: cn(defaultClassNames.today),
        outside: cn("text-(--ui-muted) opacity-60", defaultClassNames.outside),
        disabled: cn("opacity-35", defaultClassNames.disabled),
        hidden: cn("invisible", defaultClassNames.hidden),
        ...classNames,
      }}
      components={{
        Root: ({ className, rootRef, ...rootProps }) => (
          <div
            data-slot="calendar"
            ref={rootRef}
            className={cn(className)}
            {...rootProps}
          />
        ),
        Chevron: ({ className, orientation, ...chevronProps }) => {
          const Icon =
            orientation === "left"
              ? ChevronLeftIcon
              : orientation === "right"
                ? ChevronRightIcon
                : ChevronDownIcon;
          return <Icon className={cn("size-4", className)} {...chevronProps} />;
        },
        Dropdown: CalendarDropdown,
        DayButton: CalendarDayButton,
        ...components,
      }}
      {...props}
    />
  );
}

function CalendarDayButton({
  className,
  day,
  modifiers,
  ...props
}: React.ComponentProps<typeof DayButton>) {
  const defaultClassNames = getDefaultClassNames();
  const ref = React.useRef<HTMLButtonElement>(null);
  React.useEffect(() => {
    if (modifiers.focused) ref.current?.focus();
  }, [modifiers.focused]);

  return (
    <button
      ref={ref}
      type="button"
      data-day={day.isoDate}
      data-picked={modifiers.selected || undefined}
      data-today={modifiers.today || undefined}
      className={cn(
        "relative flex aspect-square w-full min-w-(--cell-size) items-center justify-center rounded-(--cell-radius) text-sm leading-none tabular-nums outline-none hover:bg-(--ui-highlight) hover:text-(--ui-primary) focus-visible:ring-2 focus-visible:ring-(--ui-primary) disabled:pointer-events-none data-today:font-semibold data-today:text-(--ui-primary) data-today:shadow-[inset_0_0_0_1px_var(--ui-primary)] data-picked:bg-(--ui-primary) data-picked:font-semibold data-picked:text-(--ui-primary-ink) data-picked:hover:bg-(--ui-primary) data-picked:hover:text-(--ui-primary-ink)",
        defaultClassNames.day,
        className,
      )}
      {...props}
    />
  );
}

export { Calendar, CalendarDayButton };
