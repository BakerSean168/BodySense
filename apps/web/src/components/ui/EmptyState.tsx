import type { HTMLAttributes, ReactNode } from "react";

import { cn } from "@/lib/utils";

/**
 * Shared empty state: icon disc + title + description + optional action.
 *
 * The layout follows the existing Person / Diagnosis empty scenes. Colors are
 * inherited from the surface instead of a fixed palette so the same component
 * reads correctly on app paper, Body Canvas light/dark, and the chat surface.
 */
export type EmptyStateSize = "sm" | "md" | "lg";

interface EmptyStateProps extends Omit<
  HTMLAttributes<HTMLDivElement>,
  "title" | "children"
> {
  icon?: ReactNode;
  title: ReactNode;
  description?: ReactNode;
  action?: ReactNode;
  size?: EmptyStateSize;
  children?: ReactNode;
}

const shellSize: Record<EmptyStateSize, string> = {
  sm: "gap-1.5 px-3 py-4",
  md: "gap-2.5 px-4 py-7",
  lg: "gap-3 px-4 py-10",
};

const iconSize: Record<EmptyStateSize, string> = {
  sm: "size-9 [&_svg]:size-5",
  md: "size-14 [&_svg]:size-6",
  lg: "size-16 [&_svg]:size-6",
};

const titleSize: Record<EmptyStateSize, string> = {
  sm: "text-[13px]",
  md: "text-[15px]",
  lg: "text-xl",
};

const descriptionSize: Record<EmptyStateSize, string> = {
  sm: "max-w-[280px] text-xs",
  md: "max-w-[300px] text-[13px]",
  lg: "max-w-sm text-sm",
};

export function EmptyState({
  icon,
  title,
  description,
  action,
  size = "md",
  className,
  children,
  ...props
}: EmptyStateProps) {
  return (
    <div
      data-slot="empty-state"
      className={cn(
        "flex flex-col items-center justify-center text-center",
        shellSize[size],
        className,
      )}
      {...props}
    >
      {icon ? (
        <span
          aria-hidden="true"
          className={cn(
            "flex shrink-0 items-center justify-center rounded-full bg-[color-mix(in_srgb,currentColor_12%,transparent)]",
            iconSize[size],
          )}
        >
          {icon}
        </span>
      ) : null}
      <p className={cn("font-semibold leading-snug", titleSize[size])}>
        {title}
      </p>
      {description ? (
        <p className={cn("leading-[1.65] opacity-65", descriptionSize[size])}>
          {description}
        </p>
      ) : null}
      {children}
      {action ? <div className="pt-1.5">{action}</div> : null}
    </div>
  );
}
