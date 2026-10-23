import { LoaderCircle } from "lucide-react";
import {
  BODY_FACT_KIND_LABELS as FACT_KINDS,
  type BodyFactKind as FactKind,
} from "../model/bodyFactPresentation";
import { Input } from "@/components/ui/Input";
import { SelectField } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";

/** Controlled draft; the ledger owner retains save requests and receipts. */
export function BodyFactRecordForm({
  habitsOnly,
  kind,
  statement,
  when,
  saving,
  blocked = false,
  saveError,
  onKindChange,
  onStatementChange,
  onWhenChange,
  onSubmit,
  onClose,
}: {
  habitsOnly: boolean;
  kind: FactKind;
  statement: string;
  when: string;
  saving: boolean;
  /** The ledger is mid-sync; the draft stays but cannot be saved yet. */
  blocked?: boolean;
  saveError: string | null;
  onKindChange: (kind: FactKind) => void;
  onStatementChange: (value: string) => void;
  onWhenChange: (value: string) => void;
  onSubmit: () => void;
  onClose: () => void;
}) {
  return (
    <form
      className="bc-equipment-form bc-person-record-form"
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit();
      }}
    >
      {!habitsOnly ? (
        <label>
          记录类别
          <SelectField
            aria-label="记录类别"
            value={kind}
            onValueChange={(next) => onKindChange(next as FactKind)}
            options={Object.entries(FACT_KINDS).map(([value, label]) => ({
              value,
              label,
            }))}
            disabled={saving}
          />
        </label>
      ) : null}
      <label>
        你的描述
        <Textarea
          value={statement}
          onChange={(event) => onStatementChange(event.target.value)}
          maxLength={2000}
          rows={3}
          disabled={saving}
          required
        />
      </label>
      <label>
        <span>
          发生时间<span className="bc-optional">可选</span>
        </span>
        <Input
          value={when}
          onChange={(event) => onWhenChange(event.target.value)}
          maxLength={500}
          placeholder="例如：昨天晚上"
          disabled={saving}
        />
      </label>
      <div className="bc-record-actions">
        <button
          type="submit"
          className="bc-button bc-primary"
          disabled={saving || blocked || !statement.trim()}
        >
          {saving ? <LoaderCircle size={16} className="animate-spin" /> : null}
          {saving ? "正在保存" : "保存"}
        </button>
        <button
          type="button"
          className="bc-button"
          disabled={saving}
          onClick={onClose}
        >
          取消
        </button>
      </div>
      {saveError ? (
        <p role="alert" className="bc-error">
          {saveError}
        </p>
      ) : null}
    </form>
  );
}
