import { useState } from "react";
import { Brain, LoaderCircle, Trash2 } from "lucide-react";
import { errorMessage } from "@/lib/api-client";
import type { EpisodicNote } from "@/features/profile/services/episodicNotesService";
import { useEpisodicNotes } from "../hooks/useEpisodicNotes";
import { PersonConfirmDialog } from "../PersonConfirmDialog";
import { PersonDataError } from "./PersonDataError";
import { PersonEmptyState } from "./PersonEmptyState";

const NOTE_STATUS_LABELS: Record<string, string> = {
  shadow: "仅保存，不参与对话",
};

const DELETE_DESCRIPTION =
  "删除后这条记忆会被永久移除，且不会被重新生成。这不会删除身体记录等临床事实，也不是账号数据清除。删除不可撤销。";

export function formatEpisodicNoteDate(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${date.getFullYear()}/${month}/${day}`;
}

export function episodicNoteStatusLabel(status: string): string {
  return NOTE_STATUS_LABELS[status] ?? status;
}

/**
 * S6-B "记忆" preview: the user's own non-authoritative conversational notes.
 * Copy keeps memory distinct from BodyState clinical facts and from account
 * data erasure; deletion is permanent and never re-generated from the same
 * source window. The whole category is flag-gated outside this component.
 */
export function EpisodicNotesCategory() {
  const {
    notes,
    isPending,
    isError,
    error,
    removingId,
    actionError,
    remove,
    retry,
  } = useEpisodicNotes();
  const [pendingDelete, setPendingDelete] = useState<EpisodicNote | null>(null);

  if (isPending) {
    return (
      <p role="status" className="bc-person-loading">
        <LoaderCircle size={16} className="animate-spin" />
        正在读取记忆
      </p>
    );
  }
  if (isError) {
    return (
      <PersonDataError
        title="记忆暂时无法读取"
        message={errorMessage(error, "请稍后重试。")}
        onRetry={retry}
      />
    );
  }
  if (notes.length === 0) {
    return (
      <PersonEmptyState
        icon={<Brain size={24} />}
        title="还没有记忆"
        message="对话摘要会出现在这里；它们是对话记忆，不是身体记录，也不会用于诊断。"
      />
    );
  }

  return (
    <div className="bc-person-category">
      {actionError && !pendingDelete ? (
        <p role="alert" className="bc-error">
          {actionError}
        </p>
      ) : null}
      <ul className="bc-person-equipment bc-person-notes">
        {notes.map((note) => {
          const date = formatEpisodicNoteDate(note.createdAt);
          return (
            <li key={note.id} data-testid="episodic-note">
              <div className="bc-person-equipment-head">
                <span className="bc-person-equipment-symbol" aria-hidden="true">
                  <Brain size={20} />
                </span>
                <button
                  type="button"
                  className="bc-icon-button bc-person-note-delete"
                  aria-label={`删除记忆：${date} · ${note.contentText.slice(0, 12)}`}
                  disabled={removingId !== null}
                  onClick={() => setPendingDelete(note)}
                >
                  <Trash2 size={16} aria-hidden="true" />
                </button>
              </div>
              <p className="bc-person-note-text">{note.contentText}</p>
              <p className="bc-person-note-meta">
                {date} · 对话第 {note.fromSeq}–{note.toSeq} 条
              </p>
              <span className="bc-person-note-status">
                {episodicNoteStatusLabel(note.status)}
              </span>
            </li>
          );
        })}
      </ul>
      {pendingDelete ? (
        <PersonConfirmDialog
          open
          onOpenChange={(open) => {
            if (!open) setPendingDelete(null);
          }}
          title="删除这条记忆？"
          description={DELETE_DESCRIPTION}
          confirmLabel="永久删除"
          working={removingId === pendingDelete.id}
          error={actionError}
          onConfirm={() => {
            void remove(pendingDelete).then((removed) => {
              if (removed) setPendingDelete(null);
            });
          }}
        />
      ) : null}
    </div>
  );
}
