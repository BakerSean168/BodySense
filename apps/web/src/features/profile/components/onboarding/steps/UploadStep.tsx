/**
 * Step 16 · 上传体检报告或体态照片 (Penpot: one 620×200 drop zone).
 *
 * The upload API needs a file type per file (report, or posture photo front / side /
 * back), which a single Penpot drop zone cannot express. PDFs go straight in as
 * reports; for an image we ask one small follow-up (体检报告 / 正面 / 侧面 / 背面).
 */
import { useEffect, useRef, useState, type DragEvent } from "react";
import { useUploadStore } from "@/stores/uploadStore";
import type { FileType, PostureAnalysis, UserUpload } from "../../../types/upload.types";
import { FILE_TYPE_LABELS } from "../../../types/upload.types";
import { PostureAnalysisView } from "../../uploads/PostureAnalysisView";

type UploadKind = Extract<FileType, "photo_front" | "photo_side" | "photo_back" | "report">;

const PHOTO_KINDS = ["photo_front", "photo_side", "photo_back"] as const;
const KIND_CHOICES: { value: UploadKind; label: string }[] = [
  { value: "report", label: "体检报告" },
  { value: "photo_front", label: "正面照" },
  { value: "photo_side", label: "侧面照" },
  { value: "photo_back", label: "背面照" },
];

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
  const { uploads: allUploads, fetchUploads, uploadFile, deleteUpload } = useUploadStore();
  // Chat photos live in the same store; onboarding only lists reports and posture photos.
  const uploads = allUploads.filter((u) => u.file_type !== "consultation_photo");
  const inputRef = useRef<HTMLInputElement>(null);
  const [pendingImage, setPendingImage] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [dragging, setDragging] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchUploads();
  }, [fetchUploads]);

  // Posture analysis and OCR run as async jobs; poll while any is still in flight.
  const inFlight = uploads.some((u) =>
    u.file_type === "report"
      ? u.ocr_status === "processing" || u.ocr_status === "pending"
      : u.analysis_status === "pending" || u.analysis_status === "processing",
  );
  useEffect(() => {
    if (!inFlight) return;
    const timer = setInterval(() => fetchUploads(), 4000);
    return () => clearInterval(timer);
  }, [inFlight, fetchUploads]);

  const usedPhotoKinds = new Set(
    uploads.filter((u) => u.file_type.startsWith("photo_")).map((u) => u.file_type),
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
    try {
      await deleteUpload(id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "文件删除失败");
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
        className={`flex h-[168px] w-full flex-col items-center justify-center gap-2 rounded-[14px] border-2 border-dashed px-6 text-center focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#2C694F] lg:h-[200px] ${
          dragging ? "border-[#2C694F] bg-[#E8F0EA]" : "border-[#E5E7EB] bg-[#F6F8F7] hover:border-[#C9D3CC]"
        }`}
      >
        <span className="text-[15px] text-[#6B7280]">
          {uploading ? "上传中…" : "点击上传或拖拽文件到这里"}
        </span>
        <span id="onboarding-upload-hint" className="text-[13px] text-[#9CA3AF]">
          支持体检报告 PDF、体态照片（选填）
        </span>
      </button>

      {pendingImage && (
        <div
          role="group"
          aria-label={`这张图片是？${pendingImage.name}`}
          className="rounded-[14px] bg-[#F7F7F7] p-4"
        >
          <p className="truncate text-sm text-[#1A1A1A]">
            这张图片是？<span className="text-[#6B7280]">{pendingImage.name}</span>
          </p>
          <div className="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-4">
            {KIND_CHOICES.map((choice) => {
              const taken = choice.value !== "report" && usedPhotoKinds.has(choice.value);
              return (
                <button
                  key={choice.value}
                  type="button"
                  disabled={taken || uploading}
                  onClick={() => void send(pendingImage, choice.value)}
                  className="h-10 rounded-[10px] border-2 border-transparent bg-white text-sm text-[#1A1A1A] hover:border-[#2C694F] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#2C694F] disabled:cursor-not-allowed disabled:opacity-40"
                >
                  {choice.label}
                </button>
              );
            })}
          </div>
          <button
            type="button"
            onClick={() => setPendingImage(null)}
            className="mt-2 text-[13px] text-[#6B7280] hover:text-[#1A1A1A]"
          >
            取消
          </button>
        </div>
      )}

      {error && (
        <p role="alert" className="text-sm text-[#B42318]">
          {error}
        </p>
      )}

      {uploads.length > 0 && (
        <ul aria-label="已上传资料" className="flex flex-col gap-2">
          {uploads.map((file) => (
            <li
              key={file.id}
              className="flex items-center justify-between gap-3 rounded-[10px] bg-[#F7F7F7] px-4 py-3 text-sm"
            >
              <span className="min-w-0 truncate">
                <span className="text-[#6B7280]">{FILE_TYPE_LABELS[file.file_type] ?? "资料"} · </span>
                {file.original_name}
              </span>
              <span className="flex shrink-0 items-center gap-2">
                <span className="text-[13px] text-[#2C694F]">{statusText(file)}</span>
                <button
                  type="button"
                  onClick={() => void remove(file.id)}
                  aria-label={`删除 ${file.original_name}`}
                  className="flex h-7 w-7 items-center justify-center rounded-full text-[#9CA3AF] hover:bg-white hover:text-[#B42318] focus-visible:outline-2 focus-visible:outline-[#2C694F]"
                >
                  <svg aria-hidden="true" viewBox="0 0 16 16" className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={1.8} strokeLinecap="round">
                    <path d="m4 4 8 8M12 4l-8 8" />
                  </svg>
                </button>
              </span>
            </li>
          ))}
        </ul>
      )}

      {postureAnalyses.length > 0 && <PostureAnalysisView analyses={postureAnalyses} />}
    </div>
  );
}
