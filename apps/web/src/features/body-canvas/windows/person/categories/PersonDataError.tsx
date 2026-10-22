import { AlertTriangle, RotateCcw } from "lucide-react";

export function PersonDataError({
  title = "资料暂时无法读取",
  message,
  onRetry,
}: {
  title?: string;
  message: string;
  onRetry: () => void;
}) {
  return (
    <div role="alert" className="bc-person-state" data-tone="danger">
      <span className="bc-person-state-icon" aria-hidden="true">
        <AlertTriangle size={24} />
      </span>
      <strong>{title}</strong>
      <span>{message}</span>
      <button type="button" className="bc-button mt-4" onClick={onRetry}>
        <RotateCcw size={16} aria-hidden="true" />
        重新读取
      </button>
    </div>
  );
}
