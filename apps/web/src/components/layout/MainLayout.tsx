import type { ReactNode } from "react";
import { useState } from "react";
import { Menu, X } from "lucide-react";
import { NavLink, useLocation } from "react-router";
import {
  activeNavigationItem,
  appNavigation,
} from "@/app/navigation/appNavigation";
import { Button } from "@/components/ui/Button";
import { cn } from "@/lib/utils";
import { AppBrand } from "./AppBrand";
import { AppNavigationRail } from "./AppNavigationRail";
import { AppUserMenu } from "./AppUserMenu";

interface MainLayoutProps {
  children: ReactNode;
  fullHeight?: boolean;
  chrome?: "standard" | "immersive";
}

export function MainLayout({
  children,
  fullHeight = false,
  chrome = "standard",
}: MainLayoutProps) {
  const location = useLocation();
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const activeItem = activeNavigationItem(location.pathname);
  const immersive = chrome === "immersive";

  return (
    <div
      className={cn(
        "flex bg-background text-foreground",
        fullHeight ? "h-dvh overflow-hidden" : "min-h-dvh",
      )}
    >
      {!immersive && <AppNavigationRail />}

      {!immersive && isMobileMenuOpen && (
        <div className="fixed inset-0 z-50 md:hidden">
          <button
            type="button"
            aria-label="关闭导航"
            className="absolute inset-0 bg-foreground/20 backdrop-blur-sm"
            onClick={() => setIsMobileMenuOpen(false)}
          />
          <aside className="relative flex h-full w-[min(19rem,86vw)] flex-col border-r border-sidebar-border bg-sidebar shadow-2xl">
            <div className="flex h-16 items-center justify-between border-b border-sidebar-border px-4">
              <AppBrand />
              <Button
                type="button"
                size="icon"
                variant="ghost"
                aria-label="关闭导航"
                onClick={() => setIsMobileMenuOpen(false)}
              >
                <X />
              </Button>
            </div>
            <nav aria-label="移动端主导航" className="flex-1 space-y-1 p-3">
              {appNavigation.map((item) => {
                const Icon = item.icon;
                const active = item.match(location.pathname);
                return (
                  <NavLink
                    key={item.href}
                    to={item.href}
                    onClick={() => setIsMobileMenuOpen(false)}
                    className={cn(
                      "flex items-center gap-3 rounded-xl px-3 py-3 text-sm font-medium text-sidebar-foreground/75 outline-none transition-colors",
                      "hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:ring-2 focus-visible:ring-sidebar-ring",
                      active &&
                        "bg-sidebar-primary text-sidebar-primary-foreground",
                    )}
                  >
                    <Icon className="size-4.5" aria-hidden="true" />
                    {item.label}
                  </NavLink>
                );
              })}
            </nav>
            <div className="border-t border-sidebar-border p-3">
              <AppUserMenu />
            </div>
          </aside>
        </div>
      )}

      <div className="flex min-w-0 flex-1 flex-col overflow-hidden">
        {!immersive && (
          <header className="flex h-16 shrink-0 items-center justify-between border-b border-border bg-background/92 px-3 backdrop-blur-xl sm:px-5">
            <div className="flex min-w-0 items-center gap-3">
              <Button
                type="button"
                size="icon"
                variant="ghost"
                aria-label="打开导航"
                onClick={() => setIsMobileMenuOpen(true)}
                className="md:hidden"
              >
                <Menu />
              </Button>
              <div className="md:hidden">
                <AppBrand compact />
              </div>
              <div className="min-w-0">
                <p className="truncate text-sm font-semibold text-foreground">
                  {activeItem.label}
                </p>
                <p className="hidden truncate text-xs text-muted-foreground sm:block">
                  长期健康工作空间
                </p>
              </div>
            </div>
            <div className="flex items-center gap-1">
              <AppUserMenu compact />
            </div>
          </header>
        )}

        <main
          className={cn(
            "min-h-0 flex-1",
            fullHeight ? "flex flex-col overflow-hidden" : "overflow-y-auto",
          )}
        >
          <div
            className={cn(
              fullHeight
                ? "flex min-h-0 flex-1 flex-col"
                : "mx-auto w-full max-w-[1600px] px-4 py-6 sm:px-6 lg:px-8 lg:py-8",
            )}
          >
            {children}
          </div>
        </main>
      </div>
    </div>
  );
}
