import { Switch as SwitchPrimitive } from "@base-ui/react/switch";

import { cn } from "@/lib/utils";
import { toggleControl } from "@/components/ui/field-variants";

function Switch({ className, ...props }: SwitchPrimitive.Root.Props) {
  return (
    <SwitchPrimitive.Root
      data-slot="switch"
      className={cn(
        toggleControl,
        "peer group/switch inline-flex h-5 w-9 items-center rounded-full border border-transparent transition-colors data-checked:bg-(--ui-primary) data-unchecked:bg-[color-mix(in_srgb,var(--ui-primary)_14%,var(--ui-line))]",
        className,
      )}
      {...props}
    >
      <SwitchPrimitive.Thumb
        data-slot="switch-thumb"
        className="pointer-events-none block size-4 translate-x-0.5 rounded-full bg-(--ui-paper) shadow-sm transition-transform data-checked:translate-x-[18px]"
      />
    </SwitchPrimitive.Root>
  );
}

export { Switch };
