import { useEffect, useRef, useState } from "react";
import { LoaderCircle, Pencil, Undo2 } from "lucide-react";
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

type Pending =
  | { mode: "correct"; text: string; command: BodyFactCorrectionInput }
  | { mode: "withdraw"; command: BodyFactWithdrawalInput };

/** Exact-target user amendment. Withdrawal is deliberately not a Delete action. */
export function BodyFactAmendment({
  record,
  revision,
  onCommitted,
  onRefresh,
}: {
  record: BodyFactRecord;
  revision: number;
  onCommitted: (message: string, committedRevision: number) => void;
  onRefresh: () => void;
}) {
  const [mode, setMode] = useState<"correct" | "withdraw" | null>(null);
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
        attempt.mode === "correct"
          ? "更正已保存，原陈述保留在历史中。"
          : "陈述已撤回，历史保留；这不表示隐私数据已删除。",
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

  if (!mode)
    return (
      <div className="bc-record-actions">
        <button
          type="button"
          className="bc-text-button"
          onClick={() => {
            setMode("correct");
            setText(record.statement);
            setKind(record.kind);
            setError(null);
            pending.current = null;
          }}
        >
          <Pencil size={13} />
          更正
        </button>
        <button
          type="button"
          className="bc-text-button"
          onClick={() => {
            setMode("withdraw");
            setError(null);
            pending.current = null;
          }}
        >
          <Undo2 size={13} />
          撤回陈述
        </button>
      </div>
    );
  return (
    <form
      className="bc-equipment-form"
      aria-label={mode === "correct" ? "更正此记录" : "撤回此记录"}
      onSubmit={(event) => {
        event.preventDefault();
        void submit();
      }}
    >
      {mode === "correct" ? (
        <>
          <label htmlFor={`correct-kind-${record.id}`}>更正后的类别</label>
          <select
            id={`correct-kind-${record.id}`}
            value={kind}
            onChange={(event) => setKind(event.target.value as BodyFactKind)}
            disabled={saving}
          >
            {Object.entries(BODY_FACT_KIND_LABELS).map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
          <label htmlFor={`correct-${record.id}`}>更正后的完整陈述</label>
          <textarea
            id={`correct-${record.id}`}
            value={text}
            onChange={(event) => setText(event.target.value)}
            rows={3}
            maxLength={2000}
            disabled={saving}
            required
          />
          <p className="bc-window-note">用于纠正原来说错的内容。</p>
        </>
      ) : (
        <p className="bc-window-note">
          确认撤回这条陈述？它将不再作为当前有效记录；原始内容及更正历史仍保留。这不是隐私数据删除，也不表示症状已经结束。
        </p>
      )}
      <div className="bc-record-actions">
        <button
          type="submit"
          className="bc-button"
          disabled={
            saving ||
            (mode === "correct" &&
              (!text.trim() ||
                (text.trim() === record.statement && kind === record.kind)))
          }
        >
          {saving ? <LoaderCircle size={14} className="animate-spin" /> : null}
          {saving ? "正在提交" : mode === "correct" ? "保存更正" : "确认撤回"}
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
