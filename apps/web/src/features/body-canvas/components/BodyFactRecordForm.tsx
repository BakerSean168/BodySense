import { LoaderCircle } from "lucide-react";
import {
  BODY_FACT_KIND_LABELS as FACT_KINDS,
  type BodyFactKind as FactKind,
} from "../model/bodyFactPresentation";

/** Controlled draft; the ledger owner retains save requests and receipts. */
export function BodyFactRecordForm({
  habitsOnly,
  kind,
  statement,
  when,
  saving,
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
          <select
            value={kind}
            onChange={(event) => {
              onKindChange(event.target.value as FactKind);
            }}
            disabled={saving}
          >
            {Object.entries(FACT_KINDS).map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
        </label>
      ) : null}
      <label>
        你的描述
        <textarea
          value={statement}
          onChange={(event) => onStatementChange(event.target.value)}
          maxLength={2000}
          rows={3}
          disabled={saving}
          required
        />
      </label>
      <label>
        发生时间<span className="bc-optional">可选，保留你的原话</span>
        <input
          value={when}
          onChange={(event) => onWhenChange(event.target.value)}
          maxLength={500}
          placeholder="例如：昨天晚上；不清楚可以留空"
          disabled={saving}
        />
      </label>
      <div className="bc-record-actions">
        <button
          type="submit"
          className="bc-button bc-primary"
          disabled={saving || !statement.trim()}
        >
          {saving ? <LoaderCircle size={16} className="animate-spin" /> : null}
          {saving ? "正在保存" : "保存记录"}
        </button>
        <button
          type="button"
          className="bc-button"
          disabled={saving}
          onClick={onClose}
        >
          收起
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
