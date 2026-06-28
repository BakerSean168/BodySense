import { useState, useRef, useEffect } from 'react';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import type { Conversation } from '../types/consultation';

interface SessionCardProps {
  conversation: Conversation;
  isActive: boolean;
  onSelect: () => void;
  onDelete: () => void;
  onPin: (pinned: boolean) => void;
  onRename: (title: string) => void;
  onShare: () => void;
  onUnshare: () => void;
}

export function SessionCard({
  conversation,
  isActive,
  onSelect,
  onDelete,
  onPin,
  onRename,
  onShare,
  onUnshare,
}: SessionCardProps) {
  const [isEditing, setIsEditing] = useState(false);
  const [editTitle, setEditTitle] = useState('');
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (isEditing && inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  }, [isEditing]);

  const handleRename = () => {
    setIsEditing(true);
    setEditTitle(conversation.title || '新咨询');
  };

  const handleSaveRename = () => {
    if (editTitle.trim()) {
      onRename(editTitle.trim());
    }
    setIsEditing(false);
  };

  const handleCancelRename = () => {
    setIsEditing(false);
  };

  const displayTitle = conversation.title || '新咨询';

  return (
    <div
      className={`group flex items-center gap-2 px-3 py-2 rounded-lg cursor-pointer transition-colors ${
        isActive
          ? 'bg-primary-50 text-primary-900'
          : 'bg-gray-50 hover:bg-gray-100'
      }`}
      onClick={onSelect}
    >
      <div className="flex-1 min-w-0">
        {isEditing ? (
          <input
            ref={inputRef}
            type="text"
            value={editTitle}
            onChange={e => setEditTitle(e.target.value)}
            onBlur={handleSaveRename}
            onKeyDown={e => {
              if (e.key === 'Enter') handleSaveRename();
              if (e.key === 'Escape') handleCancelRename();
            }}
            className="w-full bg-white border rounded px-2 py-1 text-sm"
            onClick={e => e.stopPropagation()}
          />
        ) : (
          <span className="text-sm truncate block">{displayTitle}</span>
        )}
      </div>

      {/* 操作按钮 — hover 时显示 */}
      <div className="opacity-0 group-hover:opacity-100 transition-opacity flex items-center gap-1">
        <button
          onClick={e => { e.stopPropagation(); onPin(!conversation.pinned); }}
          className="p-1 hover:bg-gray-200 rounded text-xs"
          title={conversation.pinned ? '取消置顶' : '置顶'}
        >
          📌
        </button>

        <DropdownMenu>
          <DropdownMenuTrigger
            onClick={e => e.stopPropagation()}
            className="p-1 hover:bg-gray-200 rounded text-xs"
          >
            ⋯
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={handleRename}>重命名</DropdownMenuItem>
            <DropdownMenuItem onClick={onShare}>复制链接</DropdownMenuItem>
            <DropdownMenuItem onClick={() => onUnshare()}>取消分享</DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onClick={() => setShowDeleteDialog(true)}
              className="text-red-600"
            >
              删除
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      {/* 删除确认对话框 */}
      <Dialog open={showDeleteDialog} onOpenChange={setShowDeleteDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>确认删除</DialogTitle>
            <DialogDescription>
              确定要删除会话 &quot;{displayTitle}&quot; 吗？此操作不可恢复。
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowDeleteDialog(false)}>
              取消
            </Button>
            <Button variant="destructive" onClick={() => { onDelete(); setShowDeleteDialog(false); }}>
              删除
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
