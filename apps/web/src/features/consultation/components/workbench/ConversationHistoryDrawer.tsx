import type { ReactNode } from "react";
import { Clock3, X } from "lucide-react";
import { Button } from "@/components/ui/Button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";

interface ConversationHistoryDrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  children: ReactNode;
}

export function ConversationHistoryDrawer({
  open,
  onOpenChange,
  children,
}: ConversationHistoryDrawerProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        showCloseButton={false}
        className="top-0 left-0 flex h-dvh w-[min(22rem,92vw)] max-w-none translate-x-0 translate-y-0 flex-col gap-0 rounded-none border-r border-border bg-background p-0 sm:max-w-none data-open:slide-in-from-left data-closed:slide-out-to-left"
      >
        <div className="flex h-14 shrink-0 items-center justify-between border-b border-border px-4">
          <div className="flex min-w-0 items-center gap-2">
            <Clock3
              className="size-4 text-muted-foreground"
              aria-hidden="true"
            />
            <DialogTitle className="truncate">咨询历史</DialogTitle>
          </div>
          <Button
            type="button"
            size="icon-sm"
            variant="ghost"
            aria-label="关闭咨询历史"
            onClick={() => onOpenChange(false)}
          >
            <X />
          </Button>
        </div>
        <DialogDescription className="sr-only">
          选择、重命名、分享或管理长期健康对话。
        </DialogDescription>
        <div className="min-h-0 flex-1 overflow-hidden">{children}</div>
      </DialogContent>
    </Dialog>
  );
}
