import { NavLink, useLocation } from "react-router";
import { appNavigation } from "@/app/navigation/appNavigation";
import { cn } from "@/lib/utils";
import { AppBrand } from "./AppBrand";

export function AppNavigationRail() {
  const location = useLocation();

  return (
    <aside className="hidden h-dvh w-[72px] shrink-0 border-r border-sidebar-border bg-sidebar md:flex md:flex-col">
      <div className="flex h-16 items-center justify-center border-b border-sidebar-border">
        <AppBrand compact />
      </div>
      <nav
        aria-label="主导航"
        className="flex flex-1 flex-col items-center gap-2 px-2 py-4"
      >
        {appNavigation.map((item) => {
          const active = item.match(location.pathname);
          const Icon = item.icon;
          return (
            <NavLink
              key={item.href}
              to={item.href}
              aria-label={item.label}
              title={item.label}
              data-active={active || undefined}
              className={cn(
                "group relative flex size-11 items-center justify-center rounded-xl text-sidebar-foreground/65 outline-none transition-colors",
                "hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:ring-2 focus-visible:ring-sidebar-ring",
                active &&
                  "bg-sidebar-primary text-sidebar-primary-foreground shadow-sm",
              )}
            >
              <Icon className="size-[18px]" aria-hidden="true" />
              <span className="pointer-events-none absolute left-[calc(100%+10px)] z-50 hidden whitespace-nowrap rounded-md border border-border bg-popover px-2 py-1 text-xs font-medium text-popover-foreground shadow-md group-hover:block group-focus-visible:block">
                {item.label}
              </span>
            </NavLink>
          );
        })}
      </nav>
    </aside>
  );
}
