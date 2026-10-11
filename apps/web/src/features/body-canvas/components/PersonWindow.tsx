import { useEffect, useRef, useState } from "react";
import {
  Backpack,
  Check,
  FileText,
  Images,
  LoaderCircle,
  Plus,
} from "lucide-react";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import type {
  BodyStateFact,
  BodyStateProjection,
} from "@/features/consultation/types/consultation";
import { useBodyStateCommand } from "@/features/workspace/hooks/useBodyStateCommand";
import { BodyStateWorkbench } from "@/features/workspace/components/BodyStateWorkbench";
import type { BodyRegionId } from "@/features/body-explorer/model/bodyRegionOntology";
import { errorMessage } from "@/lib/api-client";
import {
  formatFileSize,
  formatOcrStatus,
  getProfileCategoryMeta,
  PROFILE_CATEGORIES,
  AVAILABLE_EQUIPMENT_KIND,
  selectAvailableEquipment,
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
  /** Popover state is owned by the header that renders the identity trigger. */
  identityInfoOpen: boolean;
  onIdentityInfoOpenChange: (open: boolean) => void;
  /**
   * Records is the authoritative body-state surface, so the window embeds the
   * existing workbench instead of reimplementing record management. Selecting a
   * region only produces context; it never asserts that a problem exists.
   */
  selectedRegionId?: BodyRegionId | null;
  onSelectRegion?: (regionId: BodyRegionId | null) => void;
  onAskRegion?: (regionId: BodyRegionId) => void;
}

/** Avatar glyph is derived from the nickname only; never invented. */
function identityInitial(nickname: string): string {
  const trimmed = nickname.trim();
  return trimmed ? trimmed.charAt(0).toUpperCase() : "·";
}

/**
 * Avatar + nickname header trigger. Exported because the shared window host owns
 * the header chrome, and the design puts this trigger *instead of* a repeated
 * business title.
 */
export function PersonIdentityTrigger({
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

/** Stable empty snapshot so the workbench can render its own empty state. */
const EMPTY_BODY_STATE: BodyStateProjection = {
  current_revision: 0,
  safety_state: {},
  facts: [],
  observations: [],
  pending_facts: [],
  pending_observations: [],
  hypotheses: [],
  recent_revisions: [],
};

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

/**
 * Equipment is a real, user-owned inventory stored as confirmed BodyState facts
 * of kind `equipment.available`. The category has no fixed slot ceiling: cards
 * grow with the data (4 columns desktop / 2 columns mobile). The list never
 * asserts that an item is suitable for the user's current condition.
 */
function EquipmentCategory({
  bodyState,
  canEdit,
  onAddEquipment,
}: {
  bodyState: BodyStateProjection | null;
  canEdit: boolean;
  /** Opens the inline add form; there is no separate selector route yet. */
  onAddEquipment?: () => void;
}) {
  const equipment = selectAvailableEquipment(bodyState);
  const command = useBodyStateCommand();
  const [busyId, setBusyId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);
  const [name, setName] = useState("");
  const [notes, setNotes] = useState("");

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

  const add = async () => {
    const trimmed = name.trim();
    if (!bodyState || !canEdit || !trimmed || command.isPending) return;
    setError(null);
    if (
      equipment.some(
        (fact) => fact.value.trim().toLocaleLowerCase() === trimmed.toLocaleLowerCase(),
      )
    ) {
      setError("这件器材已经在当前清单中。");
      return;
    }
    try {
      await command.mutateAsync({
        type: "addFact",
        expectedRevision: bodyState.current_revision,
        fact: {
          concern_key: "resources:equipment",
          kind: AVAILABLE_EQUIPMENT_KIND,
          value: trimmed,
          details: { notes: notes.trim(), source: "user_inventory" },
          origin: "user_reported",
          // The user is stating their own inventory, so it is durable evidence
          // immediately rather than a pending candidate.
          review_state: "confirmed",
          lifecycle_state: "active",
          trend: "unknown",
          observed_at: new Date().toISOString(),
        },
      });
      setName("");
      setNotes("");
      setAdding(false);
    } catch (cause) {
      setError(errorMessage(cause, "器材没有保存，请重试。"));
    }
  };

  const addForm = (
    <form
      className="bc-equipment-form bc-person-equipment-form"
      onSubmit={(event) => {
        event.preventDefault();
        void add();
      }}
    >
      <label>
        设备名称
        <input
          value={name}
          onChange={(event) => setName(event.target.value)}
          maxLength={80}
          placeholder="例如：弹力带"
          disabled={!canEdit || command.isPending}
          autoFocus
        />
      </label>
      <label>
        规格或备注<span className="bc-optional">可选</span>
        <input
          value={notes}
          onChange={(event) => setNotes(event.target.value)}
          maxLength={240}
          placeholder="阻力、重量或使用限制"
          disabled={!canEdit || command.isPending}
        />
      </label>
      <button
        type="submit"
        className="bc-button bc-primary"
        disabled={!canEdit || command.isPending || !name.trim()}
      >
        {command.isPending ? (
          <LoaderCircle size={17} className="animate-spin" />
        ) : (
          <Plus size={17} />
        )}
        添加设备
      </button>
      <button
        type="button"
        className="bc-button"
        disabled={command.isPending}
        onClick={() => {
          setAdding(false);
          setName("");
          setNotes("");
        }}
      >
        取消
      </button>
    </form>
  );

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
      {equipment.length === 0 ? null : (
        <>
          <p className="bc-person-count">已确认 {equipment.length} 件器材</p>
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
        </>
      )}
      {adding ? (
        addForm
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

export function PersonWindow({
  identity,
  bodyState,
  canEdit,
  onEditProfile,
  identityInfoOpen: infoOpen,
  onIdentityInfoOpenChange,
  onSelectRegion,
  selectedRegionId,
  onAskRegion,
}: PersonWindowProps) {
  const mobile = useMediaQuery("(max-width: 760px)");
  const [category, setCategory] = useState<ProfileCategoryId>("records");
  const setInfoOpen = onIdentityInfoOpenChange;
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
          <BodyStateWorkbench
            hideHeading
            snapshot={{ ...(bodyState ?? EMPTY_BODY_STATE), hypotheses: bodyState?.hypotheses ?? [] }}
            selectedRegionId={selectedRegionId ?? null}
            onSelectRegion={onSelectRegion ?? (() => undefined)}
            onAskRegion={onAskRegion}
          />
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
