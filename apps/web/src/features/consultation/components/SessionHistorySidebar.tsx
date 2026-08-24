import { useState } from "react";
import { Button } from "@/components/ui/Button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { SessionCard } from "./SessionCard";
import type { Conversation } from "../types/consultation";

interface SessionHistorySidebarProps {
  conversations: Conversation[];
  activeId: string | null;
  onPrefetch?: (id: string) => void;
  onSelect: (id: string) => void;
  onNew: () => void;
  onDelete: (id: string) => void;
  onDeleteAll: () => void;
  onPin: (id: string, pinned: boolean) => void;
  onRename: (id: string, title: string) => void;
  onShare: (id: string) => void;
  onUnshare: (id: string) => void;
}

export function SessionHistorySidebar({
  conversations,
  activeId,
  onPrefetch,
  onSelect,
  onNew,
  onDelete,
  onDeleteAll,
  onPin,
  onRename,
  onShare,
  onUnshare,
}: SessionHistorySidebarProps) {
  const [showDeleteAllDialog, setShowDeleteAllDialog] = useState(false);

  const pinnedConversations = conversations.filter((c) => c.pinned);
  const unpinnedConversations = conversations.filter((c) => !c.pinned);

  return (
    <div className="flex flex-col h-full">
      {/* 按钮区 */}
      <div className="p-3 space-y-2">
        <Button onClick={onNew} className="w-full rounded-full">
          + 开始新咨询
        </Button>
        <button
          onClick={() => setShowDeleteAllDialog(true)}
          className="w-full text-sm text-gray-500 hover:text-red-500 transition-colors"
        >
          清空全部历史
        </button>
      </div>

      <div className="border-t" />

      {/* 置顶区 */}
      {pinnedConversations.length > 0 && (
        <div className="px-3 py-2">
          <h3 className="text-xs font-medium text-gray-500 mb-2">📌 已置顶</h3>
          <div className="space-y-1">
            {pinnedConversations.map((conv) => (
              <SessionCard
                key={conv.id}
                conversation={conv}
                isActive={conv.id === activeId}
                onPrefetch={() => onPrefetch?.(conv.id)}
                onSelect={() => onSelect(conv.id)}
                onDelete={() => onDelete(conv.id)}
                onPin={(pinned) => onPin(conv.id, pinned)}
                onRename={(title) => onRename(conv.id, title)}
                onShare={() => onShare(conv.id)}
                onUnshare={() => onUnshare(conv.id)}
              />
            ))}
          </div>
        </div>
      )}

      {pinnedConversations.length > 0 && <div className="border-t" />}

      {/* 全部会话区 */}
      <div className="flex-1 overflow-y-auto px-3 py-2">
        <h3 className="text-xs font-medium text-gray-500 mb-2">全部会话</h3>
        <div className="space-y-1">
          {unpinnedConversations.map((conv) => (
            <SessionCard
              key={conv.id}
              conversation={conv}
              isActive={conv.id === activeId}
              onPrefetch={() => onPrefetch?.(conv.id)}
              onSelect={() => onSelect(conv.id)}
              onDelete={() => onDelete(conv.id)}
              onPin={(pinned) => onPin(conv.id, pinned)}
              onRename={(title) => onRename(conv.id, title)}
              onShare={() => onShare(conv.id)}
              onUnshare={() => onUnshare(conv.id)}
            />
          ))}
        </div>
      </div>

      {/* 删除全部确认对话框 */}
      <Dialog open={showDeleteAllDialog} onOpenChange={setShowDeleteAllDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>确认清空</DialogTitle>
            <DialogDescription>
              此操作只删除全部会话历史和对应分享，且不可恢复；不会删除 BodyState、诊断、治疗、训练、结果或上传文件。完整数据删除请前往“身体档案 → 数据与隐私”。
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setShowDeleteAllDialog(false)}
            >
              取消
            </Button>
            <Button
              variant="destructive"
              onClick={() => {
                onDeleteAll();
                setShowDeleteAllDialog(false);
              }}
            >
              确认清空
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
