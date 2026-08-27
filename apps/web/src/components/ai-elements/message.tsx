import type { HTMLAttributes, PropsWithChildren } from "react";
import { cn } from "@/lib/utils";

type MessageRole = "user" | "assistant";

interface MessageProps extends HTMLAttributes<HTMLDivElement> {
  from: MessageRole;
}

/**
 * Project-local AI Elements composition layer.
 *
 * assistant-ui remains the behavioral/runtime owner. These components only
 * provide the flat AI Elements-style message surface so BodySense can evolve
 * presentation without introducing a second conversation runtime.
 */
export function Message({ from, className, ...props }: MessageProps) {
  return (
    <div
      data-role={from}
      className={cn(
        "group/message flex w-full flex-col",
        from === "user" ? "items-end" : "items-stretch",
        className,
      )}
      {...props}
    />
  );
}

export function MessageContent({
  children,
  className,
  ...props
}: PropsWithChildren<HTMLAttributes<HTMLDivElement>>) {
  return (
    <div
      className={cn(
        "min-w-0 text-sm leading-7 text-[#ececec]",
        "group-data-[role=user]/message:max-w-[86%] group-data-[role=user]/message:rounded-[20px] group-data-[role=user]/message:bg-[#2b2b2b] group-data-[role=user]/message:px-4 group-data-[role=user]/message:py-2.5",
        "group-data-[role=assistant]/message:w-full",
        className,
      )}
      {...props}
    >
      {children}
    </div>
  );
}

export function MessageResponse({
  children,
  className,
  ...props
}: PropsWithChildren<HTMLAttributes<HTMLDivElement>>) {
  return (
    <div
      className={cn(
        "min-w-0 break-words text-[14px] leading-7 text-[#ececec]",
        className,
      )}
      {...props}
    >
      {children}
    </div>
  );
}
