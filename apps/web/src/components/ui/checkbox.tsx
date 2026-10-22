import { Checkbox as CheckboxPrimitive } from "@base-ui/react/checkbox";
import { CheckIcon } from "lucide-react";

import { cn } from "@/lib/utils";

function Checkbox({ className, ...props }: CheckboxPrimitive.Root.Props) {
  return (
    <CheckboxPrimitive.Root
      data-slot="checkbox"
      className={cn(
        "peer relative inline-flex size-[18px] shrink-0 cursor-pointer items-center justify-center rounded-[5px] border border-[color-mix(in_srgb,var(--ui-primary)_35%,var(--ui-line))] bg-(--ui-field) text-(--ui-primary-ink) transition-colors outline-none after:absolute after:-inset-x-2 after:-inset-y-1.5 hover:border-(--ui-primary) focus-visible:ring-2 focus-visible:ring-(--ui-primary) focus-visible:ring-offset-2 focus-visible:ring-offset-(--ui-paper) aria-invalid:border-(--ui-danger) data-checked:border-(--ui-primary) data-checked:bg-(--ui-primary) data-disabled:cursor-not-allowed data-disabled:opacity-50",
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
