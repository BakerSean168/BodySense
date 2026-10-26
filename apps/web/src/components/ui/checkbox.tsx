import { Checkbox as CheckboxPrimitive } from "@base-ui/react/checkbox";
import { CheckIcon } from "lucide-react";

import { cn } from "@/lib/utils";
import { toggleControl } from "@/components/ui/field-variants";

function Checkbox({ className, ...props }: CheckboxPrimitive.Root.Props) {
  return (
    <CheckboxPrimitive.Root
      data-slot="checkbox"
      className={cn(
        toggleControl,
        "peer inline-flex size-[18px] items-center justify-center rounded-[5px] border border-[color-mix(in_srgb,var(--ui-primary)_35%,var(--ui-line))] bg-(--ui-field) text-(--ui-primary-ink) transition-colors hover:border-(--ui-primary) aria-invalid:border-(--ui-danger) data-checked:border-(--ui-primary) data-checked:bg-(--ui-primary)",
        className,
      )}
      {...props}
    >
      <CheckboxPrimitive.Indicator
        data-slot="checkbox-indicator"
        className="grid place-content-center text-current transition-none [&>svg]:size-3.5"
      >
        <CheckIcon strokeWidth={3} />
      </CheckboxPrimitive.Indicator>
    </CheckboxPrimitive.Root>
  );
}

export { Checkbox };
