import { useEffect, useRef, useState } from "react";
import {
  LoaderCircle,
  MoreHorizontal,
  Pencil,
  Trash2,
  Undo2,
} from "lucide-react";
import {
  bodyFactRecordsService,
  type BodyFactRecord,
  type BodyFactCorrectionInput,
  type BodyFactWithdrawalInput,
} from "@/features/profile/services/bodyFactRecordsService";
import { ApiRequestError, errorMessage } from "@/lib/api-client";
import {
  BODY_FACT_KIND_LABELS,
  type BodyFactKind,
} from "../model/bodyFactPresentation";
import { SelectField } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { PersonConfirmDialog } from "../windows/person/PersonConfirmDialog";

type Pending =
  | { mode: "correct"; text: string; command: BodyFactCorrectionInput }
  | { mode: "withdraw"; command: BodyFactWithdrawalInput };

export type BodyFactAmendMode = "correct" | "withdraw" | null;

/**
 * The statement's ⋯ menu: 更正, then the red 撤回陈述 and (when the statement
 * has a private source) 彻底删除.
 */
export function BodyFactActionsMenu({
  onCorrect,
  onWithdraw,
  onErase,
  disabled,
}: {
  onCorrect: () => void;
  onWithdraw: () => void;
  onErase?: () => void;
  disabled?: boolean;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className="bc-icon-button bc-record-menu"
        aria-label="更多操作"
        title="更多"
        disabled={disabled}
      >
        <MoreHorizontal size={17} aria-hidden="true" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="bc-account-menu">
        <DropdownMenuItem onClick={onCorrect}>
          <Pencil />
          更正
        </DropdownMenuItem>
        <DropdownMenuItem variant="destructive" onClick={onWithdraw}>
          <Undo2 />
          撤回陈述
        </DropdownMenuItem>
        {onErase ? (
          <DropdownMenuItem variant="destructive" onClick={onErase}>
            <Trash2 />
            彻底删除
          </DropdownMenuItem>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/**
 * Exact-target user amendment, opened from BodyFactActionsMenu. Correction is
 * an inline form; withdrawal cannot be taken back, so it asks once in a
 * Dialog. Withdrawal is deliberately not a Delete action.
 */
export function BodyFactAmendment(props: AmendmentProps) {
  // Every opening starts fresh from the record as it is now.
  return props.mode ? <Amendment key={props.mode} {...props} /> : null;
}

interface AmendmentProps {
  record: BodyFactRecord;
  revision: number;
  mode: BodyFactAmendMode;
  onModeChange: (mode: BodyFactAmendMode) => void;
  onCommitted: (message: string, committedRevision: number) => void;
  onRefresh: () => void;
}

function Amendment({
  record,
  revision,
  mode,
  onModeChange: setMode,
  onCommitted,
  onRefresh,
}: AmendmentProps) {
  const [text, setText] = useState(record.statement);
  const [kind, setKind] = useState<BodyFactKind>(record.kind);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const pending = useRef<Pending | null>(null);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);

  async function submit() {
    if (!mode || saving) return;
    if (
      mode === "correct" &&
      (!text.trim() ||
        (text.trim() === record.statement && kind === record.kind))
    )
      return;
    if (
      pending.current?.mode !== mode ||
      (pending.current.mode === "correct" &&
        (pending.current.text !== text.trim() ||
          pending.current.command.replacement.kind !== kind))
    ) {
      const identity = {
        idempotency_key: crypto.randomUUID(),
        expected_revision: revision,
      };
      pending.current =
        mode === "withdraw"
          ? { mode, command: identity }
          : {
              mode,
              text: text.trim(),
              command: {
                ...identity,
                replacement: {
                  kind,
                  statement: text.trim(),
                  body_region_ids: [],
                  temporal: { precision: "unknown" },
                  details: {},
                },
              },
            };
    }
    const attempt = pending.current;
    setSaving(true);
    setError(null);
    try {
      const receipt =
        attempt.mode === "correct"
          ? await bodyFactRecordsService.correct(record.id, attempt.command)
          : await bodyFactRecordsService.withdraw(record.id, attempt.command);
      if (!mounted.current) return;
      if (receipt.user_id !== record.user_id)
        throw new Error("回执的档案身份不一致。");
      if (attempt.mode === "withdraw" && !receipt.fact_ids.includes(record.id))
        throw new Error("回执未确认目标记录已撤回。");
      pending.current = null;
      setMode(null);
      onCommitted(
        attempt.mode === "correct" ? "已更正" : "已撤回",
        receipt.revision,
      );
    } catch (cause) {
      if (!mounted.current) return;
      if (
        cause instanceof ApiRequestError &&
        (cause.status === 409 || cause.status === 404)
      ) {
        pending.current = null;
        setError("这条记录或档案已发生变化。已重新读取，请核对后操作。");
        onRefresh();
      } else {
        setError(errorMessage(cause, "尚未确认操作结果。重试使用同一条请求。"));
      }
    } finally {
      if (mounted.current) setSaving(false);
    }
  }

  if (mode === "withdraw")
    return (
      <PersonConfirmDialog
        open
        onOpenChange={(open) => {
          if (!open) setMode(null);
        }}
        title="撤回这条陈述？"
        description="它不再作为当前记录，原内容和更正历史会保留。"
        confirmLabel="撤回"
        working={saving}
        error={error}
        onConfirm={() => void submit()}
      />
    );
  return (
    <form
      className="bc-equipment-form"
      aria-label="更正此记录"
      onSubmit={(event) => {
        event.preventDefault();
        void submit();
      }}
    >
      <label htmlFor={`correct-kind-${record.id}`}>更正后的类别</label>
      <SelectField
        id={`correct-kind-${record.id}`}
        value={kind}
        onValueChange={(next) => setKind(next as BodyFactKind)}
        options={Object.entries(BODY_FACT_KIND_LABELS).map(
          ([value, label]) => ({ value, label }),
        )}
        disabled={saving}
      />
      <label htmlFor={`correct-${record.id}`}>更正后的完整陈述</label>
      <Textarea
        id={`correct-${record.id}`}
        value={text}
        onChange={(event) => setText(event.target.value)}
        rows={3}
        maxLength={2000}
        disabled={saving}
        required
      />
      <div className="bc-record-actions">
        <button
          type="submit"
          className="bc-button"
          disabled={
            saving ||
            !text.trim() ||
            (text.trim() === record.statement && kind === record.kind)
          }
        >
          {saving ? <LoaderCircle size={14} className="animate-spin" /> : null}
          {saving ? "正在提交" : "保存更正"}
        </button>
        <button
          type="button"
          className="bc-button"
          disabled={saving}
          onClick={() => {
            setMode(null);
            setError(null);
          }}
        >
          取消
        </button>
      </div>
      {error ? (
        <p role="alert" className="bc-error">
          {error}
        </p>
      ) : null}
    </form>
  );
}
