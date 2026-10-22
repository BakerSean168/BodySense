import { useEffect, useMemo, useRef, useState } from "react";
import { LoaderCircle } from "lucide-react";
import {
  editableProfile,
  ProfileConflictError,
  useProfileStore,
  type ProfileEditable,
} from "@/stores/profileStore";
import {
  effectiveTimeZone,
  selectableTimeZones,
} from "@/features/profile/model/profilePreferences";
import { useAuthStore } from "@/stores/authStore";
import { DatePicker } from "@/components/ui/date-picker";
import { Input } from "@/components/ui/Input";
import { SelectField } from "@/components/ui/select";
import { personDisplayName } from "../../model/personIdentity";

const GENDER_OPTIONS = [
  { value: "", label: "未填写" },
  { value: "male", label: "男" },
  { value: "female", label: "女" },
];

const UNIT_OPTIONS = [
  { value: "", label: "默认（公制）" },
  { value: "metric", label: "公制" },
  { value: "imperial", label: "英制" },
];

/**
 * Identity popover (Penpot "Overlay / Person / Identity"): the minimal optional
 * Profile — nickname, birth date + self-reported gender, display time zone +
 * units. Every field can be left empty or cleared; an unset preference shows
 * the default it falls back to. Body measurements are BodyState records and are
 * deliberately not edited here, and the technical revision is never shown.
 * Profile editing is not a BodyState write, so BodyState capabilities do not
 * gate it. Body measurements, lifestyle, history, uploaded files and privacy
 * stay in the 身体档案 drawer, reached through the quiet link beside 关闭.
 */
export function PersonIdentityPopover({
  onClose,
  onOpenRecords,
}: {
  onClose: () => void;
  /** Opens the 身体档案 drawer (files, source removal and erasure live there). */
  onOpenRecords?: () => void;
}) {
  const profile = useProfileStore((state) => state.profile);
  const accountEmail = useAuthStore((state) => state.user?.email ?? "");
  // Same fallback as the Person header, so an empty nickname says what shows.
  const fallbackName = personDisplayName({ accountEmail }).name;
  const saveProfile = useProfileStore((state) => state.saveProfile);
  const panelRef = useRef<HTMLDivElement>(null);
  const [draft, setDraft] = useState<ProfileEditable>(() =>
    editableProfile(profile),
  );
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const stored = useMemo(() => editableProfile(profile), [profile]);
  const dirty = (Object.keys(stored) as Array<keyof ProfileEditable>).some(
    (key) => (draft[key] ?? null) !== (stored[key] ?? null),
  );
  const deviceZone = effectiveTimeZone(null).zone;
  const zones = useMemo(
    () => selectableTimeZones(stored.time_zone ?? undefined, deviceZone),
    [stored.time_zone, deviceZone],
  );
  const today = new Date().toISOString().slice(0, 10);
  const zoneOptions = useMemo(
    () => [
      { value: "", label: `跟随设备（${deviceZone}）` },
      ...zones.map((zone) => ({ value: zone, label: zone })),
    ],
    [zones, deviceZone],
  );

  // Outside click closes, matching the prototype behaviour. Escape is handled
  // by the window-level handler so the popover never traps focus.
  useEffect(() => {
    const handlePointerDown = (event: MouseEvent) => {
      if (!(event.target instanceof Node)) return;
      if (panelRef.current?.contains(event.target)) return;
      // Select / date popups are portaled outside the panel.
      if (
        event.target instanceof HTMLElement &&
        event.target.closest(
          '.bc-person-identity, [data-slot="select-content"], [data-slot="popover-content"]',
        )
      ) {
        return;
      }
      onClose();
    };
    document.addEventListener("mousedown", handlePointerDown);
    return () => document.removeEventListener("mousedown", handlePointerDown);
  }, [onClose]);

  const change = <K extends keyof ProfileEditable>(
    key: K,
    value: ProfileEditable[K],
  ) => {
    setDraft((current) => ({ ...current, [key]: value }));
    setNotice(null);
    setError(null);
  };

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!dirty || saving) return;
    if (draft.birth_date && draft.birth_date > today) {
      setError("出生日期不能晚于今天。");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const saved = await saveProfile(draft);
      setDraft(editableProfile(saved));
      setNotice("已保存");
    } catch (cause) {
      if (cause instanceof ProfileConflictError) {
        setDraft(editableProfile(useProfileStore.getState().profile));
        setError(cause.message);
      } else {
        setError("资料没有保存，请重试。");
      }
    } finally {
      setSaving(false);
    }
  };

  const disabled = saving;

  return (
    <div
      ref={panelRef}
      role="dialog"
      aria-label="个人资料"
      className="bc-person-popover bc-identity"
      data-testid="person-identity"
    >
      <div className="bc-identity-head">
        <h2>个人资料</h2>
      </div>
      <form className="bc-identity-form" onSubmit={submit} noValidate>
        <div className="bc-identity-row">
          <label htmlFor="identity-nickname">昵称</label>
          <Input
            id="identity-nickname"
            variant="bare"
            value={draft.display_name ?? ""}
            onChange={(event) =>
              change("display_name", event.target.value || null)
            }
            maxLength={40}
            placeholder={fallbackName}
            autoComplete="nickname"
            disabled={disabled}
          />
        </div>
        <fieldset className="bc-identity-row">
          <legend>生日 · 自报性别</legend>
          <div className="bc-identity-pair">
            <DatePicker
              variant="bare"
              aria-label="生日"
              value={draft.birth_date ?? ""}
              max={today}
              placeholder="未填写"
              clearable
              yearNavigation
              onValueChange={(next) => change("birth_date", next || null)}
              disabled={disabled}
            />
            <SelectField
              variant="bare"
              aria-label="自报性别"
              value={draft.gender ?? ""}
              options={GENDER_OPTIONS}
              onValueChange={(next) =>
                change("gender", (next || null) as ProfileEditable["gender"])
              }
              disabled={disabled}
            />
          </div>
        </fieldset>
        <fieldset className="bc-identity-row">
          <legend>显示时区 · 单位</legend>
          <div className="bc-identity-pair">
            <SelectField
              variant="bare"
              aria-label="显示时区"
              value={draft.time_zone ?? ""}
              options={zoneOptions}
              onValueChange={(next) => change("time_zone", next || null)}
              disabled={disabled}
            />
            <SelectField
              variant="bare"
              aria-label="单位"
              value={draft.unit_preference ?? ""}
              options={UNIT_OPTIONS}
              onValueChange={(next) =>
                change(
                  "unit_preference",
                  (next || null) as ProfileEditable["unit_preference"],
                )
              }
              disabled={disabled}
            />
          </div>
        </fieldset>
        {error ? (
          <p role="alert" className="bc-error bc-identity-message">
            {error}
          </p>
        ) : notice ? (
          <p role="status" className="bc-identity-message">
            {notice}
          </p>
        ) : null}
        <div className="bc-identity-actions">
          {onOpenRecords ? (
            <button
              type="button"
              className="bc-text-button bc-identity-records"
              aria-label="身体档案与文件"
              onClick={() => {
                onClose();
                onOpenRecords();
              }}
            >
              身体档案
            </button>
          ) : null}
          <button type="button" className="bc-text-button" onClick={onClose}>
            关闭
          </button>
          <button
            type="submit"
            className="bc-button bc-primary"
            disabled={!dirty || saving}
          >
            {saving ? (
              <LoaderCircle size={15} className="animate-spin" />
            ) : null}
            保存
          </button>
        </div>
      </form>
    </div>
  );
}
