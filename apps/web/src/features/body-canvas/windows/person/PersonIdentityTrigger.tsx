import { LogOut, Settings, UserRound } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import personAvatar from "../../assets/person-avatar.png";
import { personDisplayName } from "../../model/personIdentity";
import type { PersonProfileIdentity } from "./types";

/**
 * Avatar + nickname header trigger. Exported because the shared window host owns
 * the header chrome, and the design puts this trigger *instead of* a repeated
 * business title.
 */
/**
 * Penpot "Overlay / Person / Account menu": the avatar + nickname in the Person
 * header is the only account entry (Home has no app bar). It opens a menu with
 * 个人资料 (identity popover), 设置 (theme) and 退出登录 (confirm first).
 */
export function PersonIdentityTrigger({
  identity,
  open,
  onToggle,
  onOpenSettings,
  onRequestLogout,
}: {
  identity: PersonProfileIdentity;
  open: boolean;
  onToggle: () => void;
  onOpenSettings?: () => void;
  onRequestLogout?: () => void;
}) {
  const display = personDisplayName(identity);
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className="bc-person-identity"
        title={
          identity.accountEmail
            ? `账户菜单 · ${identity.accountEmail}`
            : "账户菜单"
        }
        data-identity-open={open ? "true" : undefined}
        data-name-source={display.source}
      >
        {/* Penpot Person header: the BodySense figure avatar (exported from the
            board) and the nickname with the gold dantian dot. */}
        <span className="bc-person-avatar" aria-hidden="true">
          <img src={personAvatar} alt="" width={32} height={32} />
        </span>
        <span className="bc-person-name">{display.name}</span>
        <span className="bc-person-dantian" aria-hidden="true" />
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="start"
        sideOffset={8}
        className="bc-account-menu"
      >
        <DropdownMenuItem
          onClick={() => {
            if (!open) onToggle();
          }}
        >
          <UserRound />
          个人资料
        </DropdownMenuItem>
        {onOpenSettings ? (
          <DropdownMenuItem onClick={onOpenSettings}>
            <Settings />
            设置
          </DropdownMenuItem>
        ) : null}
        {onRequestLogout ? (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" onClick={onRequestLogout}>
              <LogOut />
              退出登录
            </DropdownMenuItem>
          </>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
