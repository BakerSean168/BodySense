import type { ReactNode } from "react";
import { Check, FileText, RefreshCw, Repeat2, X } from "lucide-react";
import { EmptyState } from "@/components/ui/EmptyState";
import { IconTip } from "@/components/ui/tooltip";

/** First ledger read: skeleton rows plus the status pill. */
export function BodyFactRecordsLoading() {
  return (
    <>
      <div className="bc-person-skeleton" aria-hidden="true">
        {[0, 1, 2, 3, 4].map((index) => (
          <span key={index} />
        ))}
      </div>
      <p role="status" className="bc-person-loading bc-person-loading-pill">
        正在读取记录
      </p>
    </>
  );
}

/** No page at all: the ledger read failed and there is nothing to show. */
export function BodyFactRecordsReadFailure({
  notice,
  isFetching,
  onRefetch,
}: {
  notice: string | null;
  isFetching: boolean;
  onRefetch: () => void;
}) {
  return (
    <div role="alert" className="bc-person-state" data-tone="danger">
      <span className="bc-person-state-icon" aria-hidden="true">
        <X size={18} />
      </span>
      <strong>记录没有读取出来</strong>
      <span>
        {notice
          ? "记录已保存，但列表刷新失败。"
          : "网络或服务暂时不可用，已有记录不会丢失"}
      </span>
      <IconTip label="重新读取">
        <button
          type="button"
          className="bc-icon-button"
          aria-label="重新读取"
          disabled={isFetching}
          onClick={onRefetch}
        >
          <RefreshCw size={16} />
        </button>
      </IconTip>
    </div>
  );
}

/** A genuine empty ledger; Episodes and hypotheses stay reachable here. */
export function BodyFactRecordsEmpty({
  habitsOnly,
  notice,
  related,
}: {
  habitsOnly: boolean;
  notice: string | null;
  related?: ReactNode;
}) {
  return (
    <EmptyState
      className="min-h-[300px]"
      icon={habitsOnly ? <Repeat2 size={24} /> : <FileText size={24} />}
      title={habitsOnly ? "还没有生活习惯" : "还没有身体记录"}
      description={
        habitsOnly
          ? "作息、久坐、饮食这些，在对话里说过就会整理到这里"
          : "在对话里说一句身体感受，就会出现在这里"
      }
    >
      {notice ? (
        <p role="status" className="bc-record-saved">
          <Check size={15} />
          {notice}
        </p>
      ) : null}
      {/* Episodes and the user's own hypotheses stay reachable with no
          statements yet (the Episode editor records its own symptom). */}
      {related ? (
        <div className="bc-person-state-related">{related}</div>
      ) : null}
    </EmptyState>
  );
}
