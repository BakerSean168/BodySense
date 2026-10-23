import React, { useLayoutEffect, useRef, useState } from "react";
import {
  ArrowUp,
  ImagePlus,
  MapPin,
  Maximize2,
  Minimize2,
  Plus,
  X,
} from "lucide-react";
import type { ConsultationSpatialContext } from "../../types/consultation";
import { IconTip } from "@/components/ui/tooltip";
import { Textarea } from "@/components/ui/textarea";

export interface PendingChatImage {
  uploadId: string;
  previewUrl: string;
  mimeType: string;
  name: string;
}

interface ChatInputAreaProps {
  inputText: string;
  setInputText: (text: string) => void;
  isComposerLocked: boolean;
  handleKeyDown: (e: React.KeyboardEvent) => void;
  handleSend: () => void;
  textareaRef: React.RefObject<HTMLTextAreaElement | null>;
  pendingImages: PendingChatImage[];
  onAddImages: (files: FileList | null) => void;
  onRemoveImage: (uploadId: string) => void;
  isUploadingImage: boolean;
  spatialContext?: ConsultationSpatialContext | null;
  onClearSpatialContext?: () => void;
}

export function ChatInputArea({
  inputText,
  setInputText,
  isComposerLocked,
  handleKeyDown,
  handleSend,
  textareaRef,
  pendingImages,
  onAddImages,
  onRemoveImage,
  isUploadingImage,
  spatialContext,
  onClearSpatialContext,
}: ChatInputAreaProps) {
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const canSend = Boolean(inputText.trim() || pendingImages.length > 0);
  // ChatGPT-style: a single pill until the text wraps, then the field grows
  // upward (to a cap, then scrolls) with "+" and send on a bottom row.
  const [multiline, setMultiline] = useState(false);
  const [expanded, setExpanded] = useState(false);
  const wrapAtLength = useRef(0);

  useLayoutEffect(() => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = `${el.scrollHeight}px`;
    if (!inputText) {
      wrapAtLength.current = 0;
      setMultiline(false);
      setExpanded(false);
      return;
    }
    const wraps = el.scrollHeight > 44 || inputText.includes("\n");
    if (!multiline && wraps) {
      wrapAtLength.current = inputText.length;
      setMultiline(true);
    } else if (
      multiline &&
      !inputText.includes("\n") &&
      inputText.length < wrapAtLength.current * 0.75
    ) {
      setMultiline(false);
      setExpanded(false);
    }
  }, [inputText, multiline, textareaRef]);

  const hasExtras = Boolean(spatialContext) || pendingImages.length > 0;

  return (
    <div className="w-full">
      <input
        ref={fileInputRef}
        type="file"
        accept="image/jpeg,image/png,image/webp"
        multiple
        className="hidden"
        onChange={(event) => {
          onAddImages(event.target.files);
          event.target.value = "";
        }}
      />

      <div
        className="chat-composer"
        data-multiline={multiline || hasExtras ? "true" : "false"}
        data-expanded={expanded ? "true" : "false"}
      >
        {hasExtras ? (
          <div className="chat-composer-extras">
            {spatialContext ? (
              <div className="flex items-center gap-2 px-2 pb-1.5 pt-1">
                <div className="chat-context-chip inline-flex min-w-0 items-center gap-1.5 rounded-full border border-[#75d5a7]/20 bg-[#75d5a7]/[0.08] px-2.5 py-1 text-[11px] text-[#a9e7c8]">
                  <MapPin className="size-3 shrink-0" aria-hidden="true" />
                  <span className="truncate">
                    {spatialContext.body_region_label ||
                      (spatialContext.body_region_ids?.length
                        ? "当前身体选区"
                        : "当前动作情境")}
                    {(spatialContext.body_region_ids?.length ?? 0) > 1
                      ? ` +${(spatialContext.body_region_ids?.length ?? 1) - 1} 个区域`
                      : ""}
                    {spatialContext.anatomy_name
                      ? ` · ${spatialContext.anatomy_name}`
                      : ""}
                    {spatialContext.reference_motion
                      ? ` · ${spatialContext.reference_motion.label}${spatialContext.reference_motion.paused ? "（已暂停）" : ""}`
                      : ""}
                  </span>
                  {onClearSpatialContext ? (
                    <button
                      type="button"
                      onClick={onClearSpatialContext}
                      disabled={isComposerLocked}
                      className="chat-context-chip-clear -mr-1 inline-flex size-4 shrink-0 items-center justify-center rounded-full text-[#a9e7c8]/65 transition-colors hover:bg-white/10 hover:text-[#a9e7c8] disabled:opacity-40"
                      aria-label="移除身体区域上下文"
                    >
                      <X className="size-2.5" aria-hidden="true" />
                    </button>
                  ) : null}
                </div>
              </div>
            ) : null}

            {pendingImages.length > 0 ? (
              <div className="flex flex-wrap gap-2 px-1 pb-2 pt-1">
                {pendingImages.map((image) => (
                  <div
                    key={image.uploadId}
                    className="chat-pending-image relative size-16 overflow-hidden rounded-xl border border-white/10 bg-black/20"
                  >
                    <img
                      src={image.previewUrl}
                      alt={image.name}
                      className="h-full w-full object-cover"
                    />
                    <button
                      type="button"
                      onClick={() => onRemoveImage(image.uploadId)}
                      disabled={isComposerLocked}
                      className="absolute right-1 top-1 inline-flex size-5 items-center justify-center rounded-full bg-black/70 text-white/80 transition-colors hover:bg-black hover:text-white disabled:opacity-40"
                      aria-label="移除图片"
                    >
                      <X className="size-3" aria-hidden="true" />
                    </button>
                  </div>
                ))}
              </div>
            ) : null}
          </div>
        ) : null}

        <Textarea
          ref={textareaRef}
          variant="bare"
          value={inputText}
          onChange={(event) => setInputText(event.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="和 BodySense 说说你的身体感受…"
          disabled={isComposerLocked}
          rows={1}
          className="chat-composer-input font-normal"
        />

        {multiline ? (
          <IconTip label={expanded ? "收起" : "展开"}>
            <button
              type="button"
              onClick={() => setExpanded((value) => !value)}
              className="chat-composer-expand"
              aria-label={expanded ? "收起输入框" : "展开输入框"}
            >
              {expanded ? (
                <Minimize2 className="size-3.5" aria-hidden="true" />
              ) : (
                <Maximize2 className="size-3.5" aria-hidden="true" />
              )}
            </button>
          </IconTip>
        ) : null}

        <IconTip label="添加图片">
          <button
            type="button"
            onClick={() => fileInputRef.current?.click()}
            disabled={
              isComposerLocked || isUploadingImage || pendingImages.length >= 3
            }
            className="chat-composer-tool"
            aria-label="添加图片"
          >
            {isUploadingImage ? (
              <span className="size-3.5 animate-spin rounded-full border border-current border-t-transparent" />
            ) : pendingImages.length > 0 ? (
              <ImagePlus className="size-[18px]" aria-hidden="true" />
            ) : (
              <Plus className="size-[18px]" aria-hidden="true" />
            )}
          </button>
        </IconTip>

        <IconTip label="发送">
          <button
            type="button"
            onClick={handleSend}
            disabled={isComposerLocked || !canSend || isUploadingImage}
            className="chat-composer-send"
            aria-label="发送"
          >
            <ArrowUp
              className="size-[18px]"
              strokeWidth={2.3}
              aria-hidden="true"
            />
          </button>
        </IconTip>
      </div>
    </div>
  );
}
