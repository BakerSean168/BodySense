import { useEffect, useMemo, useRef, useState } from "react";
import { Check, LoaderCircle, ShieldAlert, Trash2 } from "lucide-react";
import { bodyFactRecordsService } from "@/features/profile/services/bodyFactRecordsService";
import {
  statementErasureService,
  type StatementErasureReceipt,
} from "@/features/profile/services/statementErasureService";
import { errorMessage } from "@/lib/api-client";
import { projectStatementErasure } from "../model/statementErasureViewModel";

export interface StatementErasureTarget {
  recordId: string;
  statementId: string;
  ownerId: string;
}

/**
 * AS-002 erasure of one user statement and every copy of it. The server never
 * edits a record, Diagnosis or plan to finish: anything still resting on the
 * statement comes back as a review. Only withdrawing this very record is
 * offered here, as an explicit second confirmation.
 */
export function StatementErasure({
  target,
  revision,
  onClose,
  onChanged,
}: {
  target: StatementErasureTarget;
  revision: number | undefined;
  onClose: () => void;
  onChanged: (message: string, committedRevision?: number) => void;
}) {
  const [receipt, setReceipt] = useState<StatementErasureReceipt | null>(null);
  const [working, setWorking] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const key = useRef(crypto.randomUUID());
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  const view = useMemo(
    () => (receipt ? projectStatementErasure(receipt, target.recordId) : null),
    [receipt, target.recordId],
  );

  async function run(step: () => Promise<StatementErasureReceipt>) {
    setWorking(true);
    setError(null);
    try {
      const next = await step();
      if (!mounted.current) return;
      if (next.statement_id !== target.statementId)
        throw new Error("删除回执与这条陈述不一致。");
      setReceipt(next);
      if (next.status === "erased" || next.status === "erased_with_retention")
        onChanged("这条陈述已删除。");
    } catch (cause) {
      if (mounted.current)
        setError(
          errorMessage(cause, "尚未确认删除结果。再次检查会继续同一次删除。"),
        );
    } finally {
      if (mounted.current) setWorking(false);
    }
  }

  const request = () =>
    run(() => statementErasureService.request(target.statementId, key.current));

  const withdrawAndContinue = () =>
    run(async () => {
      if (revision === undefined)
        throw new Error("记录尚未读取完成，请稍后再试。");
      const withdrawn = await bodyFactRecordsService.withdraw(target.recordId, {
        idempotency_key: crypto.randomUUID(),
        expected_revision: revision,
      });
      if (withdrawn.user_id !== target.ownerId)
        throw new Error("回执的档案身份不一致。");
      onChanged("陈述已撤回，正在继续删除。", withdrawn.revision);
      return statementErasureService.request(target.statementId, key.current);
    });

  const terminal = view?.phase === "erased" || view?.phase === "retained";

  return (
    <section
      className="bc-equipment-form"
      aria-label="彻底删除这条陈述"
      data-testid="statement-erasure"
      data-phase={view?.phase ?? (working ? "in_progress" : "confirm")}
    >
      {!receipt ? (
        <>
          <p className="bc-record-statement">
            <Trash2 size={15} /> 彻底删除这条陈述？
          </p>
          <p className="bc-window-note">
            将删除陈述原文、由它整理出的记录文字、相关评估报告副本和对话运行缓存，并逐项验证。仍在使用它的记录、诊断条目或训练计划需要你先处理；已作出的诊断和计划所冻结的决策输入按政策保留，不会被改写。
          </p>
        </>
      ) : view ? (
        <>
          <p
            role="status"
            className={terminal ? "bc-record-saved" : "bc-window-note"}
          >
            {working || view.phase === "in_progress" ? (
              <LoaderCircle size={15} className="animate-spin" />
            ) : terminal ? (
              <Check size={15} />
            ) : (
              <ShieldAlert size={15} />
            )}
            <strong>{view.title}</strong>
          </p>
          <p className="bc-window-note">{view.summary}</p>
          {view.outstanding.length > 0 ? (
            <ul className="bc-record-list" aria-label="尚未完成的项目">
              {view.outstanding.map((line) => (
                <li key={line.target} className="bc-record-item">
                  <span className="bc-status">{line.label}</span>
                  <p className="bc-window-note">{line.reason}</p>
                </li>
              ))}
            </ul>
          ) : null}
          {view.reviews.length > 0 ? (
            <ul className="bc-record-list" aria-label="需要你处理的内容">
              {view.reviews.map((review) => (
                <li
                  key={`${review.subject}:${review.subjectId}`}
                  className="bc-record-item"
                >
                  <p className="bc-window-note">
                    {review.isThisRecord
                      ? "这条记录仍有效：请先撤回"
                      : review.label}
                  </p>
                </li>
              ))}
            </ul>
          ) : null}
          {view.retained.length > 0 ? (
            <ul className="bc-record-list" aria-label="按政策保留的内容">
              {view.retained.map((line) => (
                <li key={line.target} className="bc-record-item">
                  <span className="bc-status">按政策保留 · {line.label}</span>
                  <p className="bc-window-note">{line.reason}</p>
                </li>
              ))}
            </ul>
          ) : null}
        </>
      ) : null}
      <div className="bc-record-actions">
        {!receipt ? (
          <button
            type="button"
            className="bc-button bc-primary"
            disabled={working}
            onClick={() => void request()}
          >
            {working ? (
              <LoaderCircle size={14} className="animate-spin" />
            ) : null}
            {working ? "正在删除" : "确认彻底删除"}
          </button>
        ) : null}
        {view?.canWithdrawHere ? (
          <button
            type="button"
            className="bc-button bc-primary"
            disabled={working}
            onClick={() => void withdrawAndContinue()}
          >
            撤回此记录并继续删除
          </button>
        ) : null}
        {receipt && !terminal ? (
          <button
            type="button"
            className="bc-button"
            disabled={working}
            onClick={() => void request()}
          >
            再次检查
          </button>
        ) : null}
        <button
          type="button"
          className="bc-button"
          disabled={working}
          onClick={onClose}
        >
          {terminal ? "完成" : receipt ? "稍后再看" : "取消"}
        </button>
      </div>
      {error ? (
        <p role="alert" className="bc-error">
          {error}
        </p>
      ) : null}
    </section>
  );
}
