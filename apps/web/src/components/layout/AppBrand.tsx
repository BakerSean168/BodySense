import { Link } from "react-router";
import { Activity } from "lucide-react";
import { cn } from "@/lib/utils";

interface AppBrandProps {
  compact?: boolean;
  className?: string;
}

export function AppBrand({ compact = false, className }: AppBrandProps) {
  return (
    <Link
      to="/dashboard"
      aria-label="体悟首页"
      className={cn(
        "inline-flex items-center gap-2 rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-ring",
        className,
      )}
    >
      <span className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-primary text-primary-foreground shadow-sm">
        <Activity className="size-4.5" aria-hidden="true" />
      </span>
      {!compact && (
        <span className="text-base font-semibold tracking-tight text-foreground">
          体悟
        </span>
      )}
    </Link>
  );
}
