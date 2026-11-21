import { Check, LoaderCircle, RefreshCw } from "lucide-react";
import { IconTip } from "@/components/ui/tooltip";

/**
 * In-place ledger status: a commit still waiting for its refetch, a read that
 * may be stale, pages that disagree on revision, and the saved receipt.
 */
export function BodyFactRecordsFeedback({
  snapshotBehindCommit,
  isError,
  stale,
  pagesDisagree,
  notice,
  lastSync,
  isFetching,
  onRefetch,
}: {
  snapshotBehindCommit: boolean;
  isError: boolean;
  stale: boolean;
  pagesDisagree: boolean;
  notice: string | null;
  lastSync: string | null;
  isFetching: boolean;
  onRefetch: () => void;
}) {
  return (
    <>
      {snapshotBehindCommit && !isError ? (
        <p role="status" className="bc-person-loading">
          <LoaderCircle size={16} className="animate-spin" />
          正在同步已保存的记录，暂不显示旧列表
        </p>
      ) : null}
      {stale ? (
        <div role="alert" className="bc-person-stale">
          <span className="bc-person-stale-chip">
            {notice ? "记录已保存，但列表刷新失败。" : "可能不是最新"}
          </span>
          {lastSync ? (
            <span className="bc-person-stale-time">上次同步 {lastSync}</span>
          ) : null}
          <IconTip label="重新读取">
            <button
              type="button"
              className="bc-icon-button"
              aria-label="重新读取"
              disabled={isFetching}
              onClick={onRefetch}
            >
              <RefreshCw
                size={16}
                className={isFetching ? "animate-spin" : ""}
              />
            </button>
          </IconTip>
        </div>
      ) : null}
      {pagesDisagree ? (
        <p role="alert" className="bc-error">
          翻页期间身体记录发生了变化，已暂时隐藏不同版本的历史页面。请刷新后继续查看或修改。
          <button type="button" className="bc-text-button" onClick={onRefetch}>
            重新读取
          </button>
        </p>
      ) : null}
      {notice ? (
        <p role="status" className="bc-record-saved">
          <Check size={15} />
          {notice}
        </p>
      ) : null}
    </>
  );
}
