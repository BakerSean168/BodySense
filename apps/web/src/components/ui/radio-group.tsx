import { Radio as RadioPrimitive } from "@base-ui/react/radio";
import { RadioGroup as RadioGroupPrimitive } from "@base-ui/react/radio-group";

import { cn } from "@/lib/utils";
import { toggleControl } from "@/components/ui/field-variants";

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
        toggleControl,
        "group/radio-group-item peer inline-flex aspect-square size-[18px] items-center justify-center rounded-full border border-[color-mix(in_srgb,var(--ui-primary)_35%,var(--ui-line))] bg-(--ui-field) hover:border-(--ui-primary) aria-invalid:border-(--ui-danger) data-checked:border-(--ui-primary)",
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
