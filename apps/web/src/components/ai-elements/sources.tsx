import type { HTMLAttributes, PropsWithChildren } from "react";
import { BookOpenText, ChevronDown } from "lucide-react";
import { cn } from "@/lib/utils";

interface SourcesProps extends HTMLAttributes<HTMLDetailsElement> {
  count: number;
}

/** Lightweight, project-local AI Elements-style source disclosure. */
export function Sources({ count, children, className, ...props }: SourcesProps) {
  if (count <= 0) return null;

  return (
    <details
      className={cn(
        "group/sources rounded-xl border border-white/[0.07] bg-white/[0.025]",
        className,
      )}
      {...props}
    >
      <summary className="flex cursor-pointer list-none items-center gap-2 px-3 py-2 text-xs text-white/55 outline-none transition-colors hover:text-white/80 focus-visible:ring-2 focus-visible:ring-[#75d5a7]/45">
        <BookOpenText className="size-3.5" aria-hidden="true" />
        <span>参考知识 {count}</span>
        <ChevronDown
          className="ml-auto size-3.5 transition-transform duration-200 group-open/sources:rotate-180"
          aria-hidden="true"
        />
      </summary>
      <div className="border-t border-white/[0.06] px-3 py-2.5">
        {children}
      </div>
    </details>
  );
}

export function SourceList({
  children,
  className,
  ...props
}: PropsWithChildren<HTMLAttributes<HTMLDivElement>>) {
  return (
    <div className={cn("flex flex-col gap-1.5", className)} {...props}>
      {children}
    </div>
  );
}

export function Source({
  children,
  className,
  ...props
}: PropsWithChildren<HTMLAttributes<HTMLDivElement>>) {
  return (
    <div
      className={cn(
        "rounded-lg px-2.5 py-2 text-xs leading-5 text-white/65 transition-colors hover:bg-white/[0.04] hover:text-white/80",
        className,
      )}
      {...props}
    >
      {children}
    </div>
  );
}
