import {
  Check,
  MessageCircle,
  Minus,
  ShieldCheck,
  Stethoscope,
} from "lucide-react";
import { WindowLoading } from "../../components/WindowLoading";
import type { DiagnosisRunView } from "../../model/diagnosisDecisionViewModel";
import { Banner, Chip, Mark, StateBlock } from "./DiagnosisPrimitives";

function formatReceiptTime(iso: string | null): string | null {
  if (!iso) return null;
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return null;
  const parts = new Intl.DateTimeFormat("zh-CN", {
    month: "numeric",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).formatToParts(date);
  const get = (type: string) =>
    parts.find((part) => part.type === type)?.value ?? "";
  return `${get("month")}月${get("day")}日 ${get("hour")}:${get("minute")}`;
}

/** Receipt / Update receipt: only from a committed settlement receipt. */
export function ReceiptScene({ run }: { run: DiagnosisRunView }) {
  const decided = run.proposals.filter(
    (row) => row.state === "committed" || row.state === "rejected",
  );
  const time = formatReceiptTime(run.committedAt);
  const receipt = run.committedReceiptId?.slice(0, 8).toUpperCase() ?? "";
  return (
    <div
      role="status"
      className="bc-dx-outcome"
      data-outcome={run.phase}
      data-testid="diagnosis-receipt"
    >
      <StateBlock
        tone="ok"
        icon={<Check size={16} strokeWidth={2.6} />}
        title="诊断已更新"
        description={
          run.phase === "updated"
            ? `${run.committedCount} 项变更已提交`
            : `${decided.length} 项决定已提交`
        }
      />
      <div className="bc-dx-receipt">
        <ul className="bc-dx-receipt-list">
          {decided.map((row) => {
            const committed = row.state === "committed";
            const label = committed
              ? row.kindLabel === "停用"
                ? "已停用"
                : "已采纳"
              : "未采纳";
            return (
              <li
                key={row.proposalId}
                className="bc-dx-receipt-row"
                data-proposal-id={row.proposalId}
                data-proposal-state={row.state}
              >
                <Mark
                  kind={
                    committed
                      ? label === "已停用"
                        ? "context"
                        : "support"
                      : "gap"
                  }
                />
                <span className="bc-dx-receipt-name">{row.title}</span>
                <Chip
                  tone={
                    committed ? (label === "已停用" ? "idle" : "ok") : "neutral"
                  }
                  testId="proposal-state"
                >
                  {label}
                </Chip>
              </li>
            );
          })}
        </ul>
        <p className="bc-dx-receipt-ref">
          回执 {receipt}
          {time ? ` · ${time}` : null}
        </p>
      </div>
    </div>
  );
}

export function NoChangeScene() {
  return (
    <div
      role="status"
      className="bc-dx-outcome"
      data-outcome="closed_without_change"
      data-testid="diagnosis-no-change"
    >
      <StateBlock
        tone="idle"
        icon={<Minus size={16} strokeWidth={2.6} />}
        title="诊断未更改"
        description="你的决定已记录"
      />
    </div>
  );
}

export function SafetyScene({ stopped }: { stopped: boolean }) {
  return (
    <div
      role="alert"
      className="bc-dx-outcome bc-diagnosis-safety"
      data-testid="diagnosis-safety"
      data-reason={stopped ? "safety_stopped" : "safety_review"}
    >
      <StateBlock
        tone="bad"
        icon={<ShieldCheck size={16} strokeWidth={2.4} />}
        title="分析已暂停"
        description="需要先处理一条安全提示"
      />
      <Banner tone="bad" mark="counter">
        {stopped
          ? "安全提示已停止本次分析，提案不会写入诊断"
          : "安全提示尚未处理，提案现在只能查看，不能决定"}
      </Banner>
    </div>
  );
}

export function EmptyScene({ onOpenChat }: { onOpenChat?: () => void }) {
  // Penpot Diagnosis / Empty: the chat mark asks for a formal analysis.
  return (
    <div
      className="bc-window-empty bc-dx-outcome"
      data-testid="diagnosis-empty"
    >
      <StateBlock
        tone="idle"
        icon={<Stethoscope size={18} strokeWidth={2.2} />}
        title="还没有诊断"
        description="记录足够后，可以在对话里请求一次正式分析"
      >
        {onOpenChat ? (
          <button
            type="button"
            className="bc-dx-icon-button"
            aria-label="在对话里请求正式分析"
            onClick={onOpenChat}
          >
            <MessageCircle size={16} aria-hidden="true" />
          </button>
        ) : null}
      </StateBlock>
    </div>
  );
}

/** Centered ink pulse while the current Diagnosis loads. */
export function LoadingScene() {
  return <WindowLoading label="正在读取当前诊断" />;
}
