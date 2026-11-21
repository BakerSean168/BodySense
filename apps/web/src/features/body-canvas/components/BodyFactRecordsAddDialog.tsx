import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import type { BodyFactKind as FactKind } from "../model/bodyFactPresentation";
import { BodyFactRecordForm } from "./BodyFactRecordForm";

/** The one add-statement dialog; the Person window header may own its open state. */
export function BodyFactRecordsAddDialog({
  open,
  habitsOnly,
  kind,
  statement,
  when,
  saving,
  blocked,
  saveError,
  dialogClassName,
  onKindChange,
  onStatementChange,
  onWhenChange,
  onSubmit,
  onDismiss,
  onRequestClose,
}: {
  open: boolean;
  habitsOnly: boolean;
  kind: FactKind;
  statement: string;
  when: string;
  saving: boolean;
  blocked: boolean;
  saveError: string | null;
  dialogClassName: string;
  onKindChange: (kind: FactKind) => void;
  onStatementChange: (statement: string) => void;
  onWhenChange: (when: string) => void;
  onSubmit: () => void;
  /** Dismissal while a write is in flight is ignored. */
  onDismiss: () => void;
  onRequestClose: () => void;
}) {
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) onDismiss();
      }}
    >
      <DialogContent showCloseButton={false} className={dialogClassName}>
        <DialogTitle className="bc-account-dialog-title">
          {habitsOnly ? "补充生活习惯" : "补充记录"}
        </DialogTitle>
        <BodyFactRecordForm
          habitsOnly={habitsOnly}
          kind={kind}
          statement={statement}
          when={when}
          saving={saving}
          blocked={blocked}
          saveError={saveError}
          onKindChange={onKindChange}
          onStatementChange={onStatementChange}
          onWhenChange={onWhenChange}
          onSubmit={onSubmit}
          onClose={onRequestClose}
        />
      </DialogContent>
    </Dialog>
  );
}
