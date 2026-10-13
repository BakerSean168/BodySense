import type { EpisodeSymptomDraft } from "../model/bodyEpisodeSymptomDraft";

interface BodyEpisodeSymptomFormProps {
  draft: EpisodeSymptomDraft;
  busy: boolean;
  unresolved: boolean;
  canSubmit: boolean;
  retry: boolean;
  error: string | null;
  onChange: (value: EpisodeSymptomDraft) => void;
  onSubmit: () => void;
  onCollapse: () => void;
}

/** Pure presentation of a self-report; storing it never authorizes an Episode. */
export function BodyEpisodeSymptomForm({
  draft,
  busy,
  unresolved,
  canSubmit,
  retry,
  error,
  onChange,
  onSubmit,
  onCollapse,
}: BodyEpisodeSymptomFormProps) {
  const disabled = busy || unresolved;
  return (
    <form
      className="bc-record-editor"
      aria-label="记录结构化本人症状"
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit();
      }}
    >
      <p className="bc-window-note">
        请按本人实际陈述填写；症状类型不是诊断，保存陈述后仍需单独确认关联。
      </p>
      <label>
        本人完整症状陈述
        <textarea
          value={draft.statement}
          rows={2}
          maxLength={1000}
          disabled={disabled}
          placeholder="例如：我今天右肩感觉疼痛"
          onChange={(event) =>
            onChange({ ...draft, statement: event.target.value })
          }
        />
      </label>
      <label>
        症状类型（本人描述）
        <input
          type="text"
          value={draft.symptomType}
          maxLength={120}
          disabled={disabled}
          placeholder="例如：肩部疼痛"
          onChange={(event) =>
            onChange({ ...draft, symptomType: event.target.value })
          }
        />
      </label>
      <label>
        当前陈述是否存在症状
        <select
          value={draft.presence}
          disabled={disabled}
          onChange={(event) =>
            onChange({
              ...draft,
              presence: event.target.value as EpisodeSymptomDraft["presence"],
            })
          }
        >
          <option value="present">有症状</option>
          <option value="absent">无症状</option>
        </select>
      </label>
      <label>
        侧别（不确定可以不指定）
        <select
          value={draft.laterality}
          disabled={disabled}
          onChange={(event) =>
            onChange({
              ...draft,
              laterality: event.target
                .value as EpisodeSymptomDraft["laterality"],
            })
          }
        >
          <option value="unknown">不确定</option>
          <option value="left">左侧</option>
          <option value="right">右侧</option>
          <option value="bilateral">双侧</option>
          <option value="not_applicable">不适用</option>
        </select>
      </label>
      {error ? (
        <p role="alert" className="bc-error">
          {error}
        </p>
      ) : null}
      <div className="bc-record-actions">
        <button
          type="submit"
          className="bc-button bc-primary"
          disabled={!canSubmit}
        >
          {busy ? "正在保存陈述" : retry ? "重试原症状记录" : "先保存症状陈述"}
        </button>
        <button
          type="button"
          className="bc-button"
          disabled={disabled}
          onClick={onCollapse}
        >
          收起症状录入
        </button>
      </div>
    </form>
  );
}
