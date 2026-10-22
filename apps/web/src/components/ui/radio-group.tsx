import { Radio as RadioPrimitive } from "@base-ui/react/radio";
import { RadioGroup as RadioGroupPrimitive } from "@base-ui/react/radio-group";

import { cn } from "@/lib/utils";

function RadioGroup({ className, ...props }: RadioGroupPrimitive.Props) {
  return (
    <RadioGroupPrimitive
      data-slot="radio-group"
      className={cn("grid w-full gap-2", className)}
      {...props}
    />
  );
}

function RadioGroupItem({ className, ...props }: RadioPrimitive.Root.Props) {
  return (
    <RadioPrimitive.Root
      data-slot="radio-group-item"
      className={cn(
        "group/radio-group-item peer relative inline-flex aspect-square size-[18px] shrink-0 cursor-pointer items-center justify-center rounded-full border border-[color-mix(in_srgb,var(--ui-primary)_35%,var(--ui-line))] bg-(--ui-field) outline-none after:absolute after:-inset-x-2 after:-inset-y-1.5 hover:border-(--ui-primary) focus-visible:ring-2 focus-visible:ring-(--ui-primary) focus-visible:ring-offset-2 focus-visible:ring-offset-(--ui-paper) aria-invalid:border-(--ui-danger) data-checked:border-(--ui-primary) data-disabled:cursor-not-allowed data-disabled:opacity-50",
        className,
      )}
      {...props}
    >
      <RadioPrimitive.Indicator
        data-slot="radio-group-indicator"
        className="flex size-full items-center justify-center"
      >
        <span className="size-2.5 rounded-full bg-(--ui-primary)" />
      </RadioPrimitive.Indicator>
    </RadioPrimitive.Root>
  );
}

export { RadioGroup, RadioGroupItem };
