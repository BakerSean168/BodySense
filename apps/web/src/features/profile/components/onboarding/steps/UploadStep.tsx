/**
 * Step 16 · 上传体检报告或体态照片 (Penpot: one 620×200 drop zone).
 *
 * The upload API needs a file type per file (report, or posture photo front / side /
 * back), which a single Penpot drop zone cannot express. PDFs go straight in as
 * reports; for an image we ask one small follow-up (体检报告 / 正面 / 侧面 / 背面).
 */
import { useEffect, useRef, useState, type DragEvent } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogTitle,
} from "@/components/ui/dialog";
import { useUploadStore } from "@/stores/uploadStore";
import type {
  FileType,
  PostureAnalysis,
  UserUpload,
} from "../../../types/upload.types";
import { FILE_TYPE_LABELS } from "../../../types/upload.types";
import { PostureAnalysisView } from "../../uploads/PostureAnalysisView";

type UploadKind = Extract<
  FileType,
  "photo_front" | "photo_side" | "photo_back" | "report"
>;

const PHOTO_KINDS = ["photo_front", "photo_side", "photo_back"] as const;
const KIND_CHOICES: { value: UploadKind; label: string }[] = [
  { value: "report", label: "体检报告" },
  { value: "photo_front", label: "正面照" },
  { value: "photo_side", label: "侧面照" },
  { value: "photo_back", label: "背面照" },
];

/** Removing the original cannot be undone, so it asks once before it runs. */
const REMOVAL_NOTE =
  "原始文件会被永久移除，不能再查看或核验；由它整理出的记录和指标会保留，并标注来源不可用。移除不可撤销。";

function statusText(file: UserUpload): string {
  if (file.file_type === "report") {
    switch (file.ocr_status) {
      case "completed":
        return "识别成功";
      case "processing":
        return "识别中";
      case "failed":
        return "识别失败";
      default:
        return "等待识别";
    }
  }
  switch (file.analysis_status) {
    case "completed":
      return "分析完成";
    case "processing":
      return "分析中";
    case "failed":
      return "分析失败";
    case "pending":
      return "等待分析";
    default:
      return "已上传";
  }
}

export function UploadStep() {
  const {
    uploads: allUploads,
    fetchUploads,
    uploadFile,
    deleteUpload,
  } = useUploadStore();
  // Chat photos live in the same store; onboarding only lists reports and posture photos.
  const uploads = allUploads.filter(
    (u) => u.file_type !== "consultation_photo",
  );
  const inputRef = useRef<HTMLInputElement>(null);
  const [pendingImage, setPendingImage] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [dragging, setDragging] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [pendingRemoval, setPendingRemoval] = useState<UserUpload | null>(null);
  const [removingId, setRemovingId] = useState<string | null>(null);
  const [removalError, setRemovalError] = useState<string | null>(null);

  useEffect(() => {
    void fetchUploads();
  }, [fetchUploads]);

  // Posture analysis and OCR run as async jobs; poll while any is still in flight.
  const inFlight = uploads.some((u) =>
    u.file_type === "report"
      ? u.ocr_status === "processing" || u.ocr_status === "pending"
      : u.analysis_status === "pending" || u.analysis_status === "processing",
  );
  useEffect(() => {
    if (!inFlight) return;
    const timer = setInterval(() => void fetchUploads(), 4000);
    return () => clearInterval(timer);
  }, [inFlight, fetchUploads]);

  const usedPhotoKinds = new Set(
    uploads
      .filter((u) => u.file_type.startsWith("photo_"))
      .map((u) => u.file_type),
  );

  const send = async (file: File, kind: UploadKind) => {
    setError(null);
    setUploading(true);
    try {
      await uploadFile(file, kind);
    } catch (err) {
      setError(err instanceof Error ? err.message : "文件上传失败");
    } finally {
      setUploading(false);
      setPendingImage(null);
    }
  };

  const accept = (file: File | undefined) => {
    if (!file) return;
    setError(null);
    if (file.type === "application/pdf") {
      void send(file, "report");
    } else if (file.type.startsWith("image/")) {
      setPendingImage(file);
    } else {
      setError("仅支持 PDF 或图片文件");
    }
  };

  const onDrop = (event: DragEvent<HTMLButtonElement>) => {
    event.preventDefault();
    setDragging(false);
    accept(event.dataTransfer.files?.[0]);
  };

  const remove = async (id: string) => {
    setRemovingId(id);
    setRemovalError(null);
    try {
      await deleteUpload(id);
      setPendingRemoval(null);
    } catch (err) {
      setRemovalError(err instanceof Error ? err.message : "文件删除失败");
    } finally {
      setRemovingId(null);
    }
  };

  const postureAnalyses: PostureAnalysis[] = PHOTO_KINDS.flatMap((kind) => {
    const upload = uploads.find((u) => u.file_type === kind);
    return upload?.analysis_status === "completed" && upload.analysis_result
      ? [upload.analysis_result]
      : [];
  });

  return (
    <div className="flex flex-col gap-4">
      {/* U5 豁免：上传必须走系统文件对话框，只能用原生 <input type="file">；
          控件隐藏，由下方上传按钮触发。 */}
      <input
        ref={inputRef}
        type="file"
        accept="application/pdf,image/*"
        className="hidden"
        data-testid="onboarding-upload-input"
        onChange={(event) => {
          accept(event.target.files?.[0]);
          event.target.value = "";
        }}
      />
      <button
        type="button"
        disabled={uploading}
        onClick={() => inputRef.current?.click()}
        onDragOver={(event) => {
          event.preventDefault();
          setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={onDrop}
        aria-describedby="onboarding-upload-hint"
        className={`flex h-[168px] w-full flex-col items-center justify-center gap-2 rounded-[14px] border-2 border-dashed px-6 text-center focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--ui-primary)] lg:h-[200px] ${
          dragging
            ? "border-[var(--ui-primary)] bg-[var(--ui-highlight)]"
            : "border-border bg-muted/60 hover:border-ring/40"
        }`}
      >
        <span className="text-[15px] text-muted-foreground">
          {uploading ? "上传中…" : "点击上传或拖拽文件到这里"}
        </span>
        <span
          id="onboarding-upload-hint"
          className="text-[13px] text-muted-foreground/70"
        >
          支持体检报告 PDF、体态照片（选填）
        </span>
      </button>

      {pendingImage && (
        <div
          role="group"
          aria-label={`这张图片是？${pendingImage.name}`}
          className="rounded-[14px] bg-muted/60 p-4"
        >
          <p className="truncate text-sm text-foreground">
            这张图片是？
            <span className="text-muted-foreground">{pendingImage.name}</span>
          </p>
          <div className="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-4">
            {KIND_CHOICES.map((choice) => {
              const taken =
                choice.value !== "report" && usedPhotoKinds.has(choice.value);
              return (
                <button
                  key={choice.value}
                  type="button"
                  disabled={taken || uploading}
                  onClick={() => void send(pendingImage, choice.value)}
                  className="h-10 rounded-[10px] border-2 border-transparent bg-card text-sm text-foreground hover:border-[var(--ui-primary)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--ui-primary)] disabled:cursor-not-allowed disabled:opacity-40"
                >
                  {choice.label}
                </button>
              );
            })}
          </div>
          <button
            type="button"
            onClick={() => setPendingImage(null)}
            className="mt-2 text-[13px] text-muted-foreground hover:text-foreground"
          >
            取消
          </button>
        </div>
      )}

      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}

      {uploads.length > 0 && (
        <ul aria-label="已上传资料" className="flex flex-col gap-2">
          {uploads.map((file) => (
            <li
              key={file.id}
              className="flex items-center justify-between gap-3 rounded-[10px] bg-muted/60 px-4 py-3 text-sm"
            >
              <span className="min-w-0 truncate">
                <span className="text-muted-foreground">
                  {FILE_TYPE_LABELS[file.file_type] ?? "资料"} ·{" "}
                </span>
                {file.original_name}
                {file.source_availability === "source_unavailable" ? (
                  <span className="block text-[13px] text-muted-foreground/70">
                    原件已移除
                  </span>
                ) : null}
              </span>
              <span className="flex shrink-0 items-center gap-2">
                <span className="text-[13px] text-[var(--ui-primary)]">
                  {statusText(file)}
                </span>
                {file.source_availability === "source_unavailable" ? null : (
                  <button
                    type="button"
                    onClick={() => {
                      setRemovalError(null);
                      setPendingRemoval(file);
                    }}
                    aria-label={`删除 ${file.original_name}`}
                    className="flex h-7 w-7 items-center justify-center rounded-full text-muted-foreground/70 hover:bg-card hover:text-destructive focus-visible:outline-2 focus-visible:outline-[var(--ui-primary)]"
                  >
                    <svg
                      aria-hidden="true"
                      viewBox="0 0 16 16"
                      className="h-3.5 w-3.5"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth={1.8}
                      strokeLinecap="round"
                    >
                      <path d="m4 4 8 8M12 4l-8 8" />
                    </svg>
                  </button>
                )}
              </span>
            </li>
          ))}
        </ul>
      )}

      <Dialog
        open={pendingRemoval !== null}
        onOpenChange={(open) => {
          if (!open && removingId === null) setPendingRemoval(null);
        }}
      >
        <DialogContent showCloseButton={false}>
          <DialogTitle>移除这个文件？</DialogTitle>
          <DialogDescription>{REMOVAL_NOTE}</DialogDescription>
          {removalError ? (
            <p role="alert" className="text-sm text-destructive">
              {removalError}
            </p>
          ) : null}
          <DialogFooter>
            <button
              type="button"
              disabled={removingId !== null}
              onClick={() => setPendingRemoval(null)}
              className="rounded-md px-3 py-1.5 text-sm text-foreground/80 hover:bg-muted disabled:opacity-50"
            >
              取消
            </button>
            <button
              type="button"
              disabled={removingId !== null}
              onClick={() => pendingRemoval && void remove(pendingRemoval.id)}
              className="rounded-md bg-destructive px-3 py-1.5 text-sm text-white hover:bg-destructive/90 disabled:opacity-50"
            >
              {removingId !== null ? "正在移除" : "移除原件"}
            </button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {postureAnalyses.length > 0 && (
        <PostureAnalysisView analyses={postureAnalyses} />
      )}
    </div>
  );
}
