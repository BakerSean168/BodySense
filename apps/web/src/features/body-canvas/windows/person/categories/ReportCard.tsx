import { FileText, MoreHorizontal, Trash2 } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  formatFileSize,
  formatOcrStatus,
  type ProfileUploadLike,
} from "../../../model/profileCategories";
import { formatMediaDate } from "../../../model/personMedia";

const EXTRACTION_LABELS: Record<string, { label: string; tone: string }> = {
  pending: { label: "等待", tone: "neutral" },
  processing: { label: "处理中", tone: "warning" },
  completed: { label: "完成", tone: "keep" },
  failed: { label: "失败", tone: "danger" },
};

function fileExtension(upload: ProfileUploadLike): string {
  const ext = upload.original_name.split(".").pop();
  if (ext && ext !== upload.original_name) return ext.toUpperCase();
  return upload.mime_type.split("/").pop()?.toUpperCase() ?? "文件";
}

/**
 * Penpot "Person / Reports" card: file glyph, name, "PDF · size · date", and a
 * 文件 / 提取 status row read from the manifest. The "…" menu opens the Penpot
 * Assets delete scope for this report.
 */
export function ReportCard({
  upload,
  onDelete,
}: {
  upload: ProfileUploadLike;
  onDelete: () => void;
}) {
  const sourceGone = upload.source_availability === "source_unavailable";
  const extraction = EXTRACTION_LABELS[upload.ocr_status] ?? {
    label: formatOcrStatus(upload.ocr_status),
    tone: "neutral",
  };
  return (
    <li className="bc-person-report" data-testid="person-report">
      <div className="bc-person-report-main">
        <span className="bc-asset-file-icon" aria-hidden="true">
          <FileText size={20} />
        </span>
        <span className="bc-person-report-text">
          <strong>{upload.original_name}</strong>
          <span>
            {fileExtension(upload)} ·{" "}
            {sourceGone
              ? "原件已移除"
              : `${formatFileSize(upload.file_size)} · ${formatMediaDate(upload.created_at)}上传`}
          </span>
        </span>
        <DropdownMenu>
          <DropdownMenuTrigger
            className="bc-icon-button bc-person-report-menu"
            aria-label={`管理“${upload.original_name}”`}
          >
            <MoreHorizontal size={18} aria-hidden="true" />
          </DropdownMenuTrigger>
          <DropdownMenuContent
            align="end"
            className="bc-account-menu bc-row-menu"
          >
            <DropdownMenuItem variant="destructive" onClick={onDelete}>
              <Trash2 />
              删除…
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      <dl className="bc-person-report-status">
        <div>
          <dt>文件</dt>
          <dd>
            <span
              className="bc-asset-chip"
              data-tone={sourceGone ? "warning" : "keep"}
            >
              {sourceGone ? "原件不可用" : "可用"}
            </span>
          </dd>
        </div>
        <div>
          <dt>提取</dt>
          <dd>
            <span className="bc-asset-chip" data-tone={extraction.tone}>
              {extraction.label}
            </span>
          </dd>
        </div>
      </dl>
    </li>
  );
}
