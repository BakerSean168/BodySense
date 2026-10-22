import { useRef } from "react";
import { LoaderCircle } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import { useThemeClass } from "./useThemeClass";

/**
 * The one confirmation for an action that cannot be taken back: a question,
 * one line of consequence, 取消 + the red action. Reversible actions use an
 * undo toast instead.
 */
export function PersonConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel,
  working,
  error,
  onConfirm,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: string;
  confirmLabel: string;
  working: boolean;
  error?: string | null;
  onConfirm: () => void;
}) {
  const anchor = useRef<HTMLSpanElement>(null);
  const themeClass = useThemeClass(anchor);
  return (
    <>
      <span ref={anchor} hidden />
      <Dialog
        open={open}
        onOpenChange={(next) => {
          if (!next && working) return;
          onOpenChange(next);
        }}
      >
        <DialogContent
          showCloseButton={false}
          className={`${themeClass()} bc-account-dialog bc-confirm-dialog`}
        >
          <DialogTitle className="bc-account-dialog-title">{title}</DialogTitle>
          {description ? (
            <DialogDescription className="bc-account-dialog-text">
              {description}
            </DialogDescription>
          ) : null}
          {error ? (
            <p role="alert" className="bc-error">
              {error}
            </p>
          ) : null}
          <div className="bc-account-dialog-actions">
            <button
              type="button"
              className="bc-button"
              disabled={working}
              onClick={() => onOpenChange(false)}
            >
              取消
            </button>
            <button
              type="button"
              className="bc-button bc-danger-button"
              disabled={working}
              onClick={onConfirm}
            >
              {working ? (
                <LoaderCircle size={15} className="animate-spin" />
              ) : null}
              {confirmLabel}
            </button>
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}
