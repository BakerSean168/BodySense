import type { LucideIcon } from "lucide-react";
import {
  ClipboardList,
  History,
  House,
  MessageCircleMore,
  UserRound,
} from "lucide-react";

export interface AppNavigationItem {
  label: string;
  href: string;
  icon: LucideIcon;
  match: (pathname: string) => boolean;
}

export const appNavigation: AppNavigationItem[] = [
  {
    label: "首页",
    href: "/dashboard",
    icon: House,
    match: (pathname) => pathname.startsWith("/dashboard"),
  },
  {
    label: "身体档案",
    href: "/profile",
    icon: UserRound,
    match: (pathname) =>
      pathname.startsWith("/profile") || pathname.startsWith("/onboarding"),
  },
  {
    label: "智能问诊",
    href: "/consultation",
    icon: MessageCircleMore,
    match: (pathname) => pathname.startsWith("/consultation"),
  },
  {
    label: "健康评估",
    href: "/assessment",
    icon: ClipboardList,
    match: (pathname) => pathname.startsWith("/assessment"),
  },
  {
    label: "历史记录",
    href: "/history",
    icon: History,
    match: (pathname) => pathname.startsWith("/history"),
  },
];

export function activeNavigationItem(pathname: string): AppNavigationItem {
  const matched = appNavigation.find((item) => item.match(pathname));
  if (matched) return matched;
  if (pathname.startsWith("/training")) {
    return {
      label: "训练执行",
      href: pathname,
      icon: ClipboardList,
      match: (candidate) => candidate.startsWith("/training"),
    };
  }
  return appNavigation[0];
}
