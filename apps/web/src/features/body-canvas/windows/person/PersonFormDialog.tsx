import { useRef, type ReactNode } from "react";
import { LoaderCircle } from "lucide-react";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { useThemeClass } from "./useThemeClass";

/**
 * One add form in a modal: title, fields, error, 取消 + primary action.
 * Every Person category adds through this from the window header "+".
 */
export function PersonFormDialog({
  open,
  onOpenChange,
  title,
  submitLabel,
  submitting,
  canSubmit,
  error,
  onSubmit,
  children,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  submitLabel: string;
  submitting: boolean;
  canSubmit: boolean;
  error?: string | null;
  onSubmit: () => void;
  children: ReactNode;
}) {
  const anchor = useRef<HTMLSpanElement>(null);
  const themeClass = useThemeClass(anchor);
  return (
    <>
      <span ref={anchor} hidden />
      <Dialog
        open={open}
        onOpenChange={(next) => {
          if (!next && submitting) return;
          onOpenChange(next);
        }}
      >
        <DialogContent
          showCloseButton={false}
          className={`${themeClass()} bc-account-dialog bc-form-dialog`}
        >
          <DialogTitle className="bc-account-dialog-title">{title}</DialogTitle>
          <form
            className="bc-form-dialog-body"
            onSubmit={(event) => {
              event.preventDefault();
              if (canSubmit && !submitting) onSubmit();
            }}
          >
            {children}
            {error ? (
              <p role="alert" className="bc-error">
                {error}
              </p>
            ) : null}
            <div className="bc-form-dialog-actions">
              <button
                type="button"
                className="bc-button"
                disabled={submitting}
                onClick={() => onOpenChange(false)}
              >
                取消
              </button>
              <button
                type="submit"
                className="bc-button bc-primary"
                disabled={!canSubmit || submitting}
              >
                {submitting ? (
                  <LoaderCircle size={16} className="animate-spin" />
                ) : null}
                {submitLabel}
              </button>
            </div>
          </form>
        </DialogContent>
      </Dialog>
    </>
  );
}
