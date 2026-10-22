import { Switch as SwitchPrimitive } from "@base-ui/react/switch";

import { cn } from "@/lib/utils";

function Switch({ className, ...props }: SwitchPrimitive.Root.Props) {
  return (
    <SwitchPrimitive.Root
      data-slot="switch"
      className={cn(
        "peer group/switch relative inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full border border-transparent transition-colors outline-none after:absolute after:-inset-x-2 after:-inset-y-1.5 focus-visible:ring-2 focus-visible:ring-(--ui-primary) focus-visible:ring-offset-2 focus-visible:ring-offset-(--ui-paper) data-checked:bg-(--ui-primary) data-unchecked:bg-[color-mix(in_srgb,var(--ui-primary)_14%,var(--ui-line))] data-disabled:cursor-not-allowed data-disabled:opacity-50",
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
