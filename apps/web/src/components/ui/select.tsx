import * as React from "react";
import { Select as SelectPrimitive } from "@base-ui/react/select";
import { CheckIcon, ChevronDownIcon, ChevronUpIcon } from "lucide-react";

import { cn } from "@/lib/utils";
import {
  fieldVariants,
  popupSurface,
  type FieldVariant,
} from "@/components/ui/field-variants";
import {
  PortalScopeContext,
  usePortalContainer,
  usePortalScopeRef,
  useScopeTriggerRef,
} from "@/components/ui/portal-scope";

function Select<Value, Multiple extends boolean | undefined = false>(
  props: SelectPrimitive.Root.Props<Value, Multiple>,
) {
  const scope = usePortalScopeRef();
  return (
    <PortalScopeContext.Provider value={scope}>
      <SelectPrimitive.Root {...props} />
    </PortalScopeContext.Provider>
  );
}

function SelectGroup({ className, ...props }: SelectPrimitive.Group.Props) {
  return (
    <SelectPrimitive.Group
      data-slot="select-group"
      className={cn("scroll-my-1 p-1", className)}
      {...props}
    />
  );
}

function SelectValue({ className, ...props }: SelectPrimitive.Value.Props) {
  return (
    <SelectPrimitive.Value
      data-slot="select-value"
      className={cn(
        "min-w-0 flex-1 text-left leading-snug break-words whitespace-normal",
        className,
      )}
      {...props}
    />
  );
}

function SelectTrigger({
  className,
  variant = "field",
  children,
  ref,
  ...props
}: SelectPrimitive.Trigger.Props & { variant?: FieldVariant }) {
  const triggerRef = useScopeTriggerRef<HTMLButtonElement>(ref);
  return (
    <SelectPrimitive.Trigger
      ref={triggerRef}
      data-slot="select-trigger"
      data-variant={variant}
      className={cn(
        fieldVariants({ variant }),
        "flex cursor-pointer items-center justify-between gap-2 py-1.5 text-left select-none data-placeholder:text-(--ui-muted) [&_svg]:pointer-events-none [&_svg]:shrink-0",
        className,
      )}
      {...props}
    >
      {children}
      <SelectPrimitive.Icon
        render={
          <ChevronDownIcon className="size-4 text-(--ui-secondary) transition-transform duration-150 in-data-popup-open:rotate-180" />
        }
      />
    </SelectPrimitive.Trigger>
  );
}

function SelectContent({
  className,
  children,
  side = "bottom",
  sideOffset = 6,
  align = "start",
  alignOffset = 0,
  alignItemWithTrigger = false,
  ...props
}: SelectPrimitive.Popup.Props &
  Pick<
    SelectPrimitive.Positioner.Props,
    "align" | "alignOffset" | "side" | "sideOffset" | "alignItemWithTrigger"
  >) {
  const container = usePortalContainer();
  return (
    <SelectPrimitive.Portal container={container}>
      <SelectPrimitive.Positioner
        side={side}
        sideOffset={sideOffset}
        align={align}
        alignOffset={alignOffset}
        alignItemWithTrigger={alignItemWithTrigger}
        className="isolate z-[1000]"
      >
        <SelectPrimitive.Popup
          data-slot="select-content"
          className={cn(
            popupSurface,
            "relative max-h-[min(320px,var(--available-height))] w-max max-w-[min(360px,calc(100vw-24px))] min-w-(--anchor-width) origin-(--transform-origin) overflow-x-hidden overflow-y-auto p-1.5 duration-100 data-closed:animate-out data-closed:fade-out-0 data-closed:zoom-out-95 data-open:animate-in data-open:fade-in-0 data-open:zoom-in-95",
            className,
          )}
          {...props}
        >
          <SelectScrollUpButton />
          <SelectPrimitive.List>{children}</SelectPrimitive.List>
          <SelectScrollDownButton />
        </SelectPrimitive.Popup>
      </SelectPrimitive.Positioner>
    </SelectPrimitive.Portal>
  );
}

function SelectLabel({
  className,
  ...props
}: SelectPrimitive.GroupLabel.Props) {
  return (
    <SelectPrimitive.GroupLabel
      data-slot="select-label"
      className={cn(
        "px-2.5 py-1.5 text-xs font-semibold text-(--ui-muted)",
        className,
      )}
      {...props}
    />
  );
}

function SelectItem({
  className,
  children,
  ...props
}: SelectPrimitive.Item.Props) {
  return (
    <SelectPrimitive.Item
      data-slot="select-item"
      data-value={
        typeof props.value === "string" || typeof props.value === "number"
          ? String(props.value)
          : undefined
      }
      className={cn(
        "relative flex min-h-9 w-full cursor-pointer items-center gap-2 rounded-lg py-2 pr-8 pl-2.5 text-sm leading-snug text-(--ui-ink) outline-hidden select-none data-disabled:pointer-events-none data-disabled:opacity-50 data-highlighted:bg-(--ui-highlight) data-highlighted:text-(--ui-primary) data-[selected]:font-semibold [&_svg]:pointer-events-none [&_svg]:shrink-0",
        className,
      )}
      {...props}
    >
      <SelectPrimitive.ItemText className="min-w-0 flex-1 break-words whitespace-normal">
        {children}
      </SelectPrimitive.ItemText>
      <SelectPrimitive.ItemIndicator
        render={
          <span className="pointer-events-none absolute right-2.5 flex size-4 items-center justify-center text-(--ui-primary)" />
        }
      >
        <CheckIcon className="size-4" />
      </SelectPrimitive.ItemIndicator>
    </SelectPrimitive.Item>
  );
}

function SelectSeparator({
  className,
  ...props
}: SelectPrimitive.Separator.Props) {
  return (
    <SelectPrimitive.Separator
      data-slot="select-separator"
      className={cn(
        "pointer-events-none -mx-1.5 my-1 h-px bg-(--ui-paper-line)",
        className,
      )}
      {...props}
    />
  );
}

function SelectScrollUpButton({
  className,
  ...props
}: React.ComponentProps<typeof SelectPrimitive.ScrollUpArrow>) {
  return (
    <SelectPrimitive.ScrollUpArrow
      data-slot="select-scroll-up-button"
      className={cn(
        "top-0 z-10 flex w-full cursor-default items-center justify-center bg-(--ui-paper) py-1 text-(--ui-secondary) [&_svg:not([class*='size-'])]:size-4",
        className,
      )}
      {...props}
    >
      <ChevronUpIcon />
    </SelectPrimitive.ScrollUpArrow>
  );
}

function SelectScrollDownButton({
  className,
  ...props
}: React.ComponentProps<typeof SelectPrimitive.ScrollDownArrow>) {
  return (
    <SelectPrimitive.ScrollDownArrow
      data-slot="select-scroll-down-button"
      className={cn(
        "bottom-0 z-10 flex w-full cursor-default items-center justify-center bg-(--ui-paper) py-1 text-(--ui-secondary) [&_svg:not([class*='size-'])]:size-4",
        className,
      )}
      {...props}
    >
      <ChevronDownIcon />
    </SelectPrimitive.ScrollDownArrow>
  );
}

export interface SelectOption<V extends string = string> {
  value: V;
  label: React.ReactNode;
  disabled?: boolean;
}

export interface SelectFieldProps<V extends string = string> {
  value: V;
  onValueChange: (value: V) => void;
  options: ReadonlyArray<SelectOption<V>>;
  variant?: FieldVariant;
  id?: string;
  name?: string;
  disabled?: boolean;
  placeholder?: React.ReactNode;
  className?: string;
  contentClassName?: string;
  "aria-label"?: string;
  "aria-labelledby"?: string;
  "aria-describedby"?: string;
  "aria-invalid"?: boolean;
  "data-testid"?: string;
  triggerRef?: React.Ref<HTMLButtonElement>;
}

/**
 * The one-line replacement for a native `<select>`: string values, a flat
 * option list, and the selected option's label shown in the trigger. The
 * trigger carries `data-value` so tests can read the current value.
 */
function SelectField<V extends string = string>({
  value,
  onValueChange,
  options,
  variant = "field",
  id,
  name,
  disabled,
  placeholder,
  className,
  contentClassName,
  triggerRef,
  ...aria
}: SelectFieldProps<V>) {
  const items = React.useMemo(
    () =>
      options.map((option) => ({ value: option.value, label: option.label })),
    [options],
  );
  const selected = options.find((option) => option.value === value);
  return (
    <Select<V>
      value={value}
      onValueChange={(next) => {
        if (next !== null && next !== undefined) onValueChange(next);
      }}
      items={items}
      name={name}
      disabled={disabled}
    >
      <SelectTrigger
        id={id}
        ref={triggerRef}
        variant={variant}
        className={className}
        data-value={value}
        {...aria}
      >
        <SelectValue>
          {selected ? selected.label : (placeholder ?? null)}
        </SelectValue>
      </SelectTrigger>
      <SelectContent className={contentClassName}>
        {options.map((option) => (
          <SelectItem
            key={option.value}
            value={option.value}
            disabled={option.disabled}
          >
            {option.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

export {
  Select,
  SelectContent,
  SelectField,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectScrollDownButton,
  SelectScrollUpButton,
  SelectSeparator,
  SelectTrigger,
  SelectValue,
};
