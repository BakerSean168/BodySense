import { LogOut, UserRound } from "lucide-react";
import { useNavigate } from "react-router";
import { useAuthStore } from "@/stores/authStore";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

interface AppUserMenuProps {
  compact?: boolean;
  onOpenProfile?: () => void;
}

export function AppUserMenu({ compact = false, onOpenProfile }: AppUserMenuProps) {
  const { user, logout } = useAuthStore();
  const navigate = useNavigate();
  const initial = user?.email?.charAt(0).toUpperCase() || "U";

  const handleLogout = async () => {
    await logout();
    navigate("/login", { replace: true });
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label="打开用户菜单"
        title="用户菜单"
        className="inline-flex items-center gap-2 rounded-xl p-1.5 text-left outline-none transition-colors hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring"
      >
        <span className="flex size-8 shrink-0 items-center justify-center rounded-full border border-border bg-secondary text-xs font-semibold text-secondary-foreground">
          {initial}
        </span>
        {!compact && (
          <span className="hidden min-w-0 sm:block">
            <span className="block max-w-44 truncate text-xs font-medium text-foreground">
              {user?.email || "普通用户"}
            </span>
            <span className="block text-[10px] text-muted-foreground">
              BodySense member
            </span>
          </span>
        )}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" sideOffset={8} className="w-64">
        <DropdownMenuGroup>
          <DropdownMenuLabel>
            <span className="block truncate text-foreground">
              {user?.email || "普通用户"}
            </span>
            <span className="mt-0.5 block font-normal text-muted-foreground">
              登录账户
            </span>
          </DropdownMenuLabel>
        </DropdownMenuGroup>
        {onOpenProfile ? (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={onOpenProfile}>
              <UserRound />
              身体档案
            </DropdownMenuItem>
          </>
        ) : null}
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive" onClick={handleLogout}>
          <LogOut />
          退出登录
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
