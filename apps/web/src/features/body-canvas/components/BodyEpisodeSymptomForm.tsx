import type { EpisodeSymptomDraft } from "../model/bodyEpisodeSymptomDraft";
import { Input } from "@/components/ui/Input";
import { SelectField } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";

const PRESENCE_OPTIONS = [
  { value: "present", label: "有症状" },
  { value: "absent", label: "无症状" },
];

const LATERALITY_OPTIONS = [
  { value: "unknown", label: "不确定" },
  { value: "left", label: "左侧" },
  { value: "right", label: "右侧" },
  { value: "bilateral", label: "双侧" },
  { value: "not_applicable", label: "不适用" },
];

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
      <p className="bc-window-note">症状类型不是诊断。</p>
      <label>
        本人完整症状陈述
        <Textarea
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
        <Input
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
        <SelectField
          aria-label="当前陈述是否存在症状"
          value={draft.presence}
          disabled={disabled}
          options={PRESENCE_OPTIONS}
          onValueChange={(next) =>
            onChange({
              ...draft,
              presence: next as EpisodeSymptomDraft["presence"],
            })
          }
        />
      </label>
      <label>
        侧别（不确定可以不指定）
        <SelectField
          aria-label="侧别（不确定可以不指定）"
          value={draft.laterality}
          disabled={disabled}
          options={LATERALITY_OPTIONS}
          onValueChange={(next) =>
            onChange({
              ...draft,
              laterality: next as EpisodeSymptomDraft["laterality"],
            })
          }
        />
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
