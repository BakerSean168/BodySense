import * as React from "react";
import { Popover as PopoverPrimitive } from "@base-ui/react/popover";

import { cn } from "@/lib/utils";
import { popupSurface } from "@/components/ui/field-variants";
import {
  PortalScopeContext,
  usePortalContainer,
  usePortalScopeRef,
  useScopeTriggerRef,
} from "@/components/ui/portal-scope";

function Popover({ ...props }: PopoverPrimitive.Root.Props) {
  const scope = usePortalScopeRef();
  return (
    <PortalScopeContext.Provider value={scope}>
      <PopoverPrimitive.Root data-slot="popover" {...props} />
    </PortalScopeContext.Provider>
  );
}

function PopoverTrigger({ ref, ...props }: PopoverPrimitive.Trigger.Props) {
  const triggerRef = useScopeTriggerRef<HTMLButtonElement>(ref);
  return (
    <PopoverPrimitive.Trigger
      ref={triggerRef}
      data-slot="popover-trigger"
      {...props}
    />
  );
}

function PopoverContent({
  className,
  align = "start",
  alignOffset = 0,
  side = "bottom",
  sideOffset = 6,
  ...props
}: PopoverPrimitive.Popup.Props &
  Pick<
    PopoverPrimitive.Positioner.Props,
    "align" | "alignOffset" | "side" | "sideOffset"
  >) {
  const container = usePortalContainer();
  return (
    <PopoverPrimitive.Portal container={container}>
      <PopoverPrimitive.Positioner
        align={align}
        alignOffset={alignOffset}
        side={side}
        sideOffset={sideOffset}
        className="isolate z-[1000]"
      >
        <PopoverPrimitive.Popup
          data-slot="popover-content"
          className={cn(
            popupSurface,
            "flex w-72 origin-(--transform-origin) flex-col gap-2.5 p-3 text-sm duration-100 data-closed:animate-out data-closed:fade-out-0 data-closed:zoom-out-95 data-open:animate-in data-open:fade-in-0 data-open:zoom-in-95",
            className,
          )}
          {...props}
        />
      </PopoverPrimitive.Positioner>
    </PopoverPrimitive.Portal>
  );
}

function PopoverHeader({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="popover-header"
      className={cn("flex flex-col gap-0.5 text-sm", className)}
      {...props}
    />
  );
}

function PopoverTitle({ className, ...props }: PopoverPrimitive.Title.Props) {
  return (
    <PopoverPrimitive.Title
      data-slot="popover-title"
      className={cn("font-semibold text-(--ui-ink)", className)}
      {...props}
    />
  );
}

function PopoverDescription({
  className,
  ...props
}: PopoverPrimitive.Description.Props) {
  return (
    <PopoverPrimitive.Description
      data-slot="popover-description"
      className={cn("text-(--ui-secondary)", className)}
      {...props}
    />
  );
}

export {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
};
