import { useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  ArrowRight,
  LoaderCircle,
  ShieldCheck,
  Stethoscope,
  X,
} from "lucide-react";
import { nativeDiagnosisKeys } from "@/features/consultation/hooks/useDiagnosisActions";
import {
  nativeDiagnosisService as api,
  type NativeDiagnosisRun,
} from "@/features/consultation/services/nativeDiagnosisService";
import { ApiRequestError, errorMessage } from "@/lib/api-client";

export interface DiagnosisRequestMessage {
  /** Persisted id of a completed message the user wrote. */
  id: string;
  text: string;
}

export interface DiagnosisRequestEntryProps {
  message: DiagnosisRequestMessage | null;
  /** Current Diagnosis revision; null while it is still loading. */
  expectedRevision: number | null;
  onClose: () => void;
  onOpenDiagnosis: () => void;
  onOpenSafety: () => void;
}

function startError(error: unknown): string {
  if (error instanceof ApiRequestError) {
    if (error.status === 409 && error.code === "BODY_STATE_NOT_READY")
      return "记录还不够做正式分析。先在对话里多描述一些身体情况。";
    if (error.status === 409)
      return "诊断或记录刚刚有变化，已重新读取，请再试一次。";
    if (error.status === 400)
      return "这条消息不能作为分析请求（例如它是对提问的回答）。请直接写下想分析的内容再发送。";
    if (error.status === 404) return "没有找到这条消息，请稍后重试。";
    if (
      error.code === "AGENT_OUTPUT_REJECTED" ||
      error.code === "AI_SERVICE_ERROR"
    )
      return "分析服务暂时不可用，没有保存任何内容。";
  }
  return errorMessage(error, "暂时无法开始正式分析，请稍后重试。");
}

/**
 * Penpot Chat / Diagnosis request. The user picks their own latest message
 * as the request and starts a formal analysis explicitly (R-DX-06). The run's
 * proposals are AI output; they are decided in the Diagnosis window and
 * nothing is adopted until that settlement receipt exists.
 */
export default function DiagnosisRequestEntry({
  message,
  expectedRevision,
  onClose,
  onOpenDiagnosis,
  onOpenSafety,
}: DiagnosisRequestEntryProps) {
  const client = useQueryClient();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [run, setRun] = useState<NativeDiagnosisRun | null>(null);
  const keys = useRef(new Map<string, string>());

  const start = async () => {
    if (!message || expectedRevision === null || busy) return;
    setBusy(true);
    setError(null);
    try {
      const request = await api.bindRequest(message.id);
      if (request.messageId !== message.id || !request.excerpt)
        throw new Error("请求原话与所选消息不一致，未开始分析。");
      const attempt = `${request.requestEvidenceId}:${expectedRevision}`;
      const idempotencyKey = keys.current.get(attempt) ?? crypto.randomUUID();
      keys.current.set(attempt, idempotencyKey);
      const started = await api.start({
        requestEvidenceId: request.requestEvidenceId,
        trigger: "new_analysis",
        expectedRevision,
        idempotencyKey,
      });
      if (started.requestEvidenceId !== request.requestEvidenceId)
        throw new Error("分析回执与请求不一致，请刷新后重试。");
      setRun(started);
    } catch (startFailure) {
      setError(startError(startFailure));
    } finally {
      setBusy(false);
      await client.invalidateQueries({ queryKey: nativeDiagnosisKeys.all });
    }
  };

  const stopped = run?.outcome === "safety_blocked";
  const proposals = run?.proposals ?? [];
  const decided = proposals.filter((item) => item.decision).length;
  return (
    <section
      className="bc-dx bc-dx-request"
      aria-label="正式分析"
      data-testid="diagnosis-request"
      data-state={
        run ? (stopped ? "stopped" : "started") : busy ? "starting" : "idle"
      }
    >
      <header className="bc-dx-request-head">
        <span className="bc-dx-request-title">
          <Stethoscope size={17} aria-hidden="true" />
          正式分析
        </span>
        {run && !stopped && proposals.length > 0 ? (
          <span className="bc-dx-request-count">
            {decided} / {proposals.length} 已决定
          </span>
        ) : null}
        <button
          type="button"
          className="bc-dx-icon-button"
          aria-label="收起正式分析"
          onClick={onClose}
        >
          <X size={16} aria-hidden="true" />
        </button>
      </header>
      {run ? (
        stopped ? (
          <div className="bc-dx-banner" data-tone="bad" role="alert">
            <ShieldCheck size={15} aria-hidden="true" />
            <span className="bc-dx-banner-text">
              分析已暂停：需要先处理一条安全提示，提案不会写入诊断
            </span>
            <button
              type="button"
              className="bc-dx-text-button"
              onClick={onOpenSafety}
            >
              查看限制
            </button>
          </div>
        ) : (
          <>
            {proposals.length === 0 ? (
              <p className="bc-dx-note">
                这次分析没有提出新的病情提案，诊断保持不变。
              </p>
            ) : (
              <ul className="bc-dx-request-list">
                {proposals.map((item) => {
                  const entry = (
                    item.content as {
                      proposedEntry?: { medicalName?: unknown };
                    }
                  ).proposedEntry;
                  const name =
                    typeof entry?.medicalName === "string" &&
                    entry.medicalName.trim()
                      ? entry.medicalName
                      : "现有病情的调整";
                  return (
                    <li key={item.proposalId} className="bc-dx-request-row">
                      <span className="bc-dx-request-name">{name}</span>
                      <span className="bc-dx-chip" data-tone="ai">
                        AI 提案 · 待你决定
                      </span>
                    </li>
                  );
                })}
              </ul>
            )}
            {proposals.length > 0 ? (
              <div className="bc-dx-request-actions">
                <button
                  type="button"
                  className="bc-dx-primary"
                  onClick={onOpenDiagnosis}
                >
                  去决定
                  <ArrowRight size={15} aria-hidden="true" />
                </button>
              </div>
            ) : null}
          </>
        )
      ) : message ? (
        <>
          <blockquote
            className="bc-dx-request-quote"
            data-testid="diagnosis-request-quote"
          >
            “{message.text}”
          </blockquote>
          {error ? (
            <div className="bc-dx-banner" data-tone="bad" role="alert">
              <span className="bc-dx-banner-text">{error}</span>
            </div>
          ) : null}
          <div className="bc-dx-request-actions">
            <button
              type="button"
              className="bc-dx-primary"
              aria-label={busy ? undefined : "开始正式分析"}
              disabled={busy || expectedRevision === null}
              onClick={() => void start()}
            >
              {busy ? (
                <LoaderCircle
                  size={15}
                  className="animate-spin"
                  aria-hidden="true"
                />
              ) : null}
              {busy ? "正在分析" : "开始分析"}
            </button>
          </div>
        </>
      ) : (
        <p className="bc-dx-note" data-testid="diagnosis-request-empty">
          先在对话里说明想分析的问题。
        </p>
      )}
    </section>
  );
}
