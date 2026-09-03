import { useState } from "react";
import { useUploadStore } from "@/stores/uploadStore";
import type { UserUpload } from "../../types/upload.types";
import { FILE_TYPE_LABELS, OCR_STATUS_LABELS } from "../../types/upload.types";
import { OCRResultView } from "./OCRResultView";

export function UploadList() {
  const { uploads, deleteUpload } = useUploadStore();
  const [deletingId, setDeletingId] = useState<string | null>(null);

  if (uploads.length === 0) {
    return (
      <div className="text-center py-8 text-gray-500">
        <svg
          className="mx-auto h-12 w-12 text-gray-400 mb-3"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z"
          />
        </svg>
        <p className="text-sm">暂无上传文件</p>
      </div>
    );
  }

  const handleDelete = async (id: string) => {
    if (!confirm("确定要删除此文件吗？")) return;

    setDeletingId(id);
    try {
      await deleteUpload(id);
    } catch {
      // Error handled by store
    } finally {
      setDeletingId(null);
    }
  };

  return (
    <div className="space-y-4">
      {uploads.map((upload) => (
        <UploadItem
          key={upload.id}
          upload={upload}
          onDelete={handleDelete}
          isDeleting={deletingId === upload.id}
        />
      ))}
    </div>
  );
}

function UploadItem({
  upload,
  onDelete,
  isDeleting,
}: {
  upload: UserUpload;
  onDelete: (id: string) => void;
  isDeleting: boolean;
}) {
  const [showOCR, setShowOCR] = useState(false);

  const isImage = upload.mime_type.startsWith("image/");
  const isReport = upload.file_type === "report";

  return (
    <div className="border border-gray-200 rounded-lg overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between p-4 bg-gray-50">
        <div className="flex items-center gap-3">
          {/* Thumbnail or icon */}
          {isImage ? (
            <div className="w-12 h-12 rounded bg-gray-200 flex items-center justify-center">
              <svg
                className="w-6 h-6 text-gray-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
                />
              </svg>
            </div>
          ) : (
            <div className="w-12 h-12 rounded bg-gray-200 flex items-center justify-center">
              <svg
                className="w-6 h-6 text-gray-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                />
              </svg>
            </div>
          )}

          <div>
            <p className="text-sm font-medium text-gray-900 truncate max-w-[200px]">
              {upload.original_name}
            </p>
            <div className="flex items-center gap-2 mt-1">
              <span className="text-xs text-gray-500">
                {FILE_TYPE_LABELS[upload.file_type]}
              </span>
              <span className="text-xs text-gray-400">•</span>
              <span className="text-xs text-gray-500">
                {formatFileSize(upload.file_size)}
              </span>
              <span className="text-xs text-gray-400">•</span>
              <span className="text-xs text-gray-500">
                {formatDate(upload.created_at)}
              </span>
            </div>
          </div>
        </div>

        <div className="flex items-center gap-2">
          {/* OCR status */}
          {isReport && <OCRStatusBadge status={upload.ocr_status} />}

          {/* Delete button */}
          <button
            type="button"
            onClick={() => onDelete(upload.id)}
            disabled={isDeleting}
            className="p-1.5 text-gray-400 hover:text-red-500 rounded-md hover:bg-red-50 transition-colors disabled:opacity-50"
            title="删除"
          >
            {isDeleting ? (
              <svg
                className="w-4 h-4 animate-spin"
                fill="none"
                viewBox="0 0 24 24"
              >
                <circle
                  className="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  strokeWidth="4"
                />
                <path
                  className="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                />
              </svg>
            ) : (
              <svg
                className="w-4 h-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                />
              </svg>
            )}
          </button>
        </div>
      </div>

      {/* OCR Result */}
      {isReport && upload.ocr_status === "completed" && upload.ocr_result && (
        <div className="p-4 border-t border-gray-200">
          <button
            type="button"
            onClick={() => setShowOCR(!showOCR)}
            className="text-sm text-blue-600 hover:text-blue-500 flex items-center gap-1 mb-3"
          >
            <svg
              className={`w-4 h-4 transition-transform ${showOCR ? "rotate-90" : ""}`}
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M9 5l7 7-7 7"
              />
            </svg>
            {showOCR ? "隐藏识别结果" : "查看识别结果"}
          </button>

          {showOCR && <OCRResultView result={upload.ocr_result} uploadId={upload.id} />}
        </div>
      )}

      {/* OCR processing indicator */}
      {isReport && upload.ocr_status === "processing" && (
        <div className="p-4 border-t border-gray-200 bg-blue-50">
          <div className="flex items-center gap-2 text-sm text-blue-700">
            <svg
              className="w-4 h-4 animate-spin"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle
                className="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                strokeWidth="4"
              />
              <path
                className="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
              />
            </svg>
            正在识别体检报告...
          </div>
        </div>
      )}

      {/* OCR failed indicator */}
      {isReport && upload.ocr_status === "failed" && (
        <div className="p-4 border-t border-gray-200 bg-red-50">
          <p className="text-sm text-red-700">识别失败，请重新上传或联系支持</p>
        </div>
      )}
    </div>
  );
}

function OCRStatusBadge({ status }: { status: UserUpload["ocr_status"] }) {
  const styles = {
    pending: "bg-gray-100 text-gray-600",
    processing: "bg-blue-100 text-blue-700",
    completed: "bg-green-100 text-green-700",
    failed: "bg-red-100 text-red-700",
  };

  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${styles[status]}`}
    >
      {OCR_STATUS_LABELS[status]}
    </span>
  );
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function formatDate(dateStr: string): string {
  const date = new Date(dateStr);
  return date.toLocaleDateString("zh-CN", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
