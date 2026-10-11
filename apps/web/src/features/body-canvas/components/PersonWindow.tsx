import { useEffect, useMemo, useRef, useState } from "react";
import {
  Backpack,
  Check,
  FileText,
  Images,
  LoaderCircle,
  Plus,
  Sparkles,
} from "lucide-react";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import type {
  BodyStateFact,
  BodyStateProjection,
} from "@/features/consultation/types/consultation";
import { useBodyStateCommand } from "@/features/workspace/hooks/useBodyStateCommand";
import { errorMessage } from "@/lib/api-client";
import {
  formatFileSize,
  formatOcrStatus,
  getProfileCategoryMeta,
  PROFILE_CATEGORIES,
  selectAvailableEquipment,
  selectProfileRecords,
  selectUploadsByCategory,
  type ProfileCategoryId,
  type ProfileUploadLike,
} from "../model/profileCategories";
import { usePersonWindowData, type PersonHabitRow } from "../hooks/usePersonWindowData";
import { computeBmi, formatCompactDate, formatGender, formatMetricValue } from "../model/businessWindows";

/**
 * Person/Profile business window.
 *
 * Layout contract (Penpot V5.5, `PROFILE_UNIFIED_CENTERING` /
 * `PROFILE_TAB_CONTENT_SIMPLIFICATION`):
 * - slim header: avatar + nickname on the left, Close on the right; no caret,
 *   no header edit action, no duplicate title, no body-status card;
 * - clicking avatar/nickname opens a compact value-only info popover
 *   (Desktop 348x184, Mobile 340x184) with no repeated avatar/name and no
 *   field labels; outside click closes it;
 * - exactly five category tabs; the tab label is the only place the category
 *   name appears — no duplicate H1 and no generic explanation beneath it;
 * - Equipment empty state is a single centred sentence + add action; added
 *   equipment grows naturally (Desktop 4 columns, Mobile 2 columns) with no
 *   fixed slot count.
 */

/** Stable identity, supplied by the profile store. Never invented. */
export interface PersonProfileIdentity {
  nickname: string;
  birth_date?: string;
  gender?: string;
}

export interface PersonWindowProps {
  identity: PersonProfileIdentity;
  bodyState: BodyStateProjection | null;
  canEdit: boolean;
  onEditProfile: () => void;
  onClose: () => void;
}

/** Avatar glyph is derived from the nickname only; never invented. */
function identityInitial(nickname: string): string {
  const trimmed = nickname.trim();
  return trimmed ? trimmed.charAt(0).toUpperCase() : "·";
}

function ProfileIdentityTrigger({
  identity,
  open,
  onToggle,
}: {
  identity: PersonProfileIdentity;
  open: boolean;
  onToggle: () => void;
}) {
  const displayName = identity.nickname.trim() || "未设置昵称";
  return (
    <button
      type="button"
      className="bc-person-identity"
      aria-expanded={open}
      aria-haspopup="dialog"
      onClick={onToggle}
    >
      <span className="bc-person-avatar" aria-hidden="true">
        {identityInitial(identity.nickname)}
      </span>
      <span className="bc-person-name">{displayName}</span>
    </button>
  );
}

function CompactIdentityPopover({
  identity,
  metrics,
  onEditProfile,
  onClose,
}: {
  identity: PersonProfileIdentity;
  metrics: { height?: { value: number; unit: string } | null; weight?: { value: number; unit: string } | null; bmi?: number | null } | null;
  onEditProfile: () => void;
  onClose: () => void;
}) {
  const panelRef = useRef<HTMLDivElement>(null);
  const bmi = computeBmi({
    height: metrics?.height ?? undefined,
    weight: metrics?.weight ?? undefined,
    bmi: metrics?.bmi ?? undefined,
  });

  // Outside click closes, matching the prototype behaviour. Escape is handled by
  // the window-level handler so the popover never traps focus.
  useEffect(() => {
    const handlePointerDown = (event: MouseEvent) => {
      if (!(event.target instanceof Node)) return;
      if (panelRef.current?.contains(event.target)) return;
      if (
        event.target instanceof HTMLElement &&
        event.target.closest(".bc-person-identity")
      ) {
        return;
      }
      onClose();
    };
    document.addEventListener("mousedown", handlePointerDown);
    return () => document.removeEventListener("mousedown", handlePointerDown);
  }, [onClose]);

  const height = metrics?.height ?? null;
  const weight = metrics?.weight ?? null;
  const heightText = height
    ? formatMetricValue(height.value, height.unit || "cm", {
        digits: height.unit === "m" ? 2 : 0,
      })
    : "— cm";
  const weightText = weight
    ? formatMetricValue(weight.value, weight.unit || "kg", { digits: 1 })
    : "— kg";

  return (
    <div
      ref={panelRef}
      role="dialog"
      aria-label="基础资料"
      className="bc-person-popover"
    >
      <p className="bc-person-popover-value">
        {formatCompactDate(identity.birth_date)} · {formatGender(identity.gender)}
      </p>
      <p className="bc-person-popover-value">
        {heightText} · {weightText} · BMI {bmi === null ? "—" : bmi.toFixed(1)}
      </p>
      <p className="bc-person-popover-hint">
        {identity.birth_date || height || weight
          ? "以上为已记录的基础资料"
          : "还没有基础资料"}
      </p>
      <button
        type="button"
        className="bc-person-popover-action"
        onClick={onEditProfile}
      >
        编辑基础资料
      </button>
    </div>
  );
}

function EmptyState({
  icon,
  message,
  actionLabel,
  onAction,
  disabled,
}: {
  icon: React.ReactNode;
  message: string;
  actionLabel?: string;
  onAction?: () => void;
  disabled?: boolean;
}) {
  return (
    <div className="bc-person-empty">
      <span className="bc-person-empty-icon" aria-hidden="true">
        {icon}
      </span>
      <p className="bc-person-empty-text">{message}</p>
      {actionLabel && onAction ? (
        <button
          type="button"
          className="bc-button bc-primary bc-person-empty-action"
          onClick={onAction}
          disabled={disabled}
        >
          <Plus size={16} />
          {actionLabel}
        </button>
      ) : null}
    </div>
  );
}

function EquipmentCategory({
  bodyState,
  canEdit,
  onAddEquipment,
}: {
  bodyState: BodyStateProjection | null;
  canEdit: boolean;
  /** Opens the inline equipment form; there is no separate selector route yet. */
  onAddEquipment?: () => void;
}) {
  const equipment = selectAvailableEquipment(bodyState);
  const command = useBodyStateCommand();
  const [busyId, setBusyId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);

  const remove = async (fact: BodyStateFact) => {
    if (!bodyState || !canEdit || command.isPending) return;
    setBusyId(fact.id);
    setError(null);
    try {
      await command.mutateAsync({
        type: "updateFactTemporal",
        factId: fact.id,
        expectedRevision: bodyState.current_revision,
        input: {
          lifecycle_state: "inactive",
          valid_until: new Date().toISOString(),
        },
      });
    } catch (cause) {
      setError(errorMessage(cause, "器材状态没有保存，请重试。"));
    } finally {
      setBusyId(null);
    }
  };

  if (equipment.length === 0 && !adding) {
    return (
      <>
        <EmptyState
          icon={<Backpack size={30} />}
          message="添加个人器材，帮助 AI 更好规划训练项目"
          actionLabel="添加个人器材"
          onAction={() => {
            setAdding(true);
            onAddEquipment?.();
          }}
          disabled={!canEdit}
        />
        {error ? (
          <p role="alert" className="bc-error">
            {error}
          </p>
        ) : null}
      </>
    );
  }

  return (
    <div className="bc-person-category">
      <p className="bc-person-count">已确认 {equipment.length} 件器材</p>
      {equipment.length === 0 ? null : (
      <ul className="bc-person-equipment">
        {equipment.map((fact) => (
          <li key={fact.id}>
            <span className="bc-person-equipment-symbol" aria-hidden="true">
              <Backpack size={20} />
            </span>
            <strong>{fact.value}</strong>
            {typeof fact.details?.notes === "string" && fact.details.notes ? (
              <p>{fact.details.notes}</p>
            ) : null}
            <span className="bc-status">
              <Check size={13} />
              当前可用
            </span>
            <button
              type="button"
              className="bc-text-button"
              disabled={!canEdit || command.isPending || busyId === fact.id}
              onClick={() => void remove(fact)}
            >
              {busyId === fact.id ? (
                <LoaderCircle size={14} className="animate-spin" />
              ) : null}
              暂不可用
            </button>
          </li>
        ))}
      </ul>
      )}
      {adding ? (
        <form
          className="bc-person-equipment-form"
          onSubmit={(event) => {
            event.preventDefault();
            setAdding(false);
          }}
        >
          <label>
            器材名称
            <input
              name="equipmentName"
              maxLength={80}
              placeholder="例如：弹力带"
              disabled={!canEdit || command.isPending}
              autoFocus
            />
          </label>
          <p className="bc-person-hint">
            器材清单会随数量自然变化，不设固定槽位；它不会自动改变已接受的训练计划。
          </p>
          <div className="bc-person-equipment-actions">
            <button
              type="submit"
              className="bc-button"
              disabled={command.isPending}
            >
              取消
            </button>
          </div>
        </form>
      ) : (
        <button
          type="button"
          className="bc-button"
          onClick={() => {
            setAdding(true);
            onAddEquipment?.();
          }}
          disabled={!canEdit}
        >
          <Plus size={16} />
          添加个人器材
        </button>
      )}
      {error ? (
        <p role="alert" className="bc-error">
          {error}
        </p>
      ) : null}
    </div>
  );
}

function UploadsCategory({
  uploads,
  category,
}: {
  uploads: readonly ProfileUploadLike[];
  category: "media" | "reports";
}) {
  const items = selectUploadsByCategory(uploads, category);
  if (items.length === 0) {
    return (
      <EmptyState
        icon={category === "reports" ? <FileText size={30} /> : <Images size={30} />}
        message={
          category === "reports"
            ? "还没有医学报告。上传报告后会显示来源与处理状态。"
            : "还没有影像资料。上传照片或视频后可以在这里核对。"
        }
      />
    );
  }
  return (
    <ul className="bc-person-uploads">
      {items.map((upload) => (
        <li key={upload.id}>
          <strong>{upload.original_name}</strong>
          <span>
            {formatFileSize(upload.file_size)} · {formatOcrStatus(upload.ocr_status)}
          </span>
        </li>
      ))}
    </ul>
  );
}

function HabitsCategory({ habits }: { habits: PersonHabitRow[] }) {
  return (
    <dl className="bc-person-habits">
      {habits.map((habit) => {
        const recorded = habit.summary.trim().length > 0;
        return (
          <div key={habit.key}>
            <dt>{habit.label}</dt>
            <dd className={recorded ? undefined : "bc-person-habit-empty"}>
              {recorded ? habit.summary : "未记录"}
            </dd>
          </div>
        );
      })}
    </dl>
  );
}

function RecordsCategory({ bodyState }: { bodyState: BodyStateProjection | null }) {
  const records = selectProfileRecords(bodyState);
  if (records.length === 0) {
    return (
      <EmptyState
        icon={<Sparkles size={30} />}
        message="还没有身体记录。继续对话后，确认过的信息会出现在这里。"
      />
    );
  }
  /** Records group by their concern key; the key is data, not a second title. */
  const groups = new Map<string, BodyStateFact[]>();
  for (const record of records) {
    const key = record.concern_key || "general";
    groups.set(key, [...(groups.get(key) ?? []), record]);
  }
  return (
    <div className="bc-person-category">
      <p className="bc-person-count">当前 {records.length} 条身体记录</p>
      {[...groups.entries()].map(([key, items]) => (
        <section key={key} className="bc-person-record-group">
          {groups.size > 1 ? (
            <h3 className="bc-person-group-title">{key === "general" ? "综合" : key}</h3>
          ) : null}
          <ul>
            {items.map((record) => (
              <li key={record.id}>
                <strong>{record.value}</strong>
                <span>
                  {record.review_state === "confirmed" ? "已确认" : "待核实"}
                  {record.observed_at
                    ? ` · ${new Date(record.observed_at).toLocaleDateString("zh-CN")}`
                    : ""}
                </span>
              </li>
            ))}
          </ul>
        </section>
      ))}
    </div>
  );
}

export function PersonWindow({
  identity,
  bodyState,
  canEdit,
  onEditProfile,
  onClose,
}: PersonWindowProps) {
  const mobile = useMediaQuery("(max-width: 760px)");
  const [category, setCategory] = useState<ProfileCategoryId>("records");
  const [infoOpen, setInfoOpen] = useState(false);
  // The window owns its own reads: nothing loads until the window is open, and
  // a failure is surfaced instead of substituted with placeholder values.
  const data = usePersonWindowData();
  const meta = getProfileCategoryMeta(category);

  // Category switching must never leave the identity popover floating over a
  // different category.
  useEffect(() => {
    setInfoOpen(false);
  }, [category]);

  return (
    <section
      className="bc-person"
      aria-label="个人档案"
      data-testid="person-window"
      data-mobile={mobile ? "true" : "false"}
    >
      <header className="bc-person-header">
        <ProfileIdentityTrigger
          identity={identity}
          open={infoOpen}
          onToggle={() => setInfoOpen((open) => !open)}
        />
        <button
          type="button"
          className="bc-icon-button"
          aria-label="关闭个人档案"
          onClick={onClose}
        >
          ✕
        </button>
      </header>

      {infoOpen ? (
        <CompactIdentityPopover
          identity={identity}
          metrics={data.metrics}
          onEditProfile={onEditProfile}
          onClose={() => setInfoOpen(false)}
        />
      ) : null}

      <nav className="bc-person-tabs" aria-label="档案分类">
        {PROFILE_CATEGORIES.map((item) => (
          <button
            key={item.id}
            type="button"
            aria-pressed={category === item.id}
            onClick={() => setCategory(item.id)}
          >
            {item.label}
          </button>
        ))}
      </nav>

      <div
        className="bc-person-body"
        role="region"
        aria-label={meta.regionLabel}
        data-category={category}
      >
        {data.error ? (
          <p role="alert" className="bc-error">
            {data.error}
          </p>
        ) : data.loading ? (
          <p role="status" className="bc-person-loading">
            <LoaderCircle size={16} className="animate-spin" />
            正在读取档案
          </p>
        ) : category === "records" ? (
          <RecordsCategory bodyState={bodyState} />
        ) : category === "equipment" ? (
          <EquipmentCategory
            bodyState={bodyState}
            canEdit={canEdit}
            onAddEquipment={() => setCategory("equipment")}
          />
        ) : category === "media" ? (
          <UploadsCategory uploads={data.uploads} category="media" />
        ) : category === "reports" ? (
          <UploadsCategory uploads={data.uploads} category="reports" />
        ) : (
          <HabitsCategory habits={data.habits} />
        )}
      </div>
    </section>
  );
}
