import { useEffect, useRef, useState } from "react";
import {
  Check,
  Dumbbell,
  FileText,
  Image as ImageIcon,
  LoaderCircle,
  Plus,
  RotateCcw,
  Backpack,
  LogOut,
  MoreHorizontal,
  Settings,
  Trash2,
  Upload,
  UserRound,
  X,
} from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import personAvatar from "../assets/person-avatar.png";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import {
  availabilityChange,
  equipmentKeys,
  equipmentService,
  type EquipmentAsset,
} from "@/features/profile/services/equipmentService";
import { BodyFactRecords, bodyFactRecordsKey } from "./BodyFactRecords";
import { bodyFactRecordsService } from "@/features/profile/services/bodyFactRecordsService";
import { projectBodyFactSurface } from "../model/bodyFactViewModel";
import { useAuthStore } from "@/stores/authStore";
import { PersonRecordsView } from "./PersonRecordsView";
import type { BodyRegionId } from "@/features/body-explorer/model/bodyRegionOntology";
import { ApiRequestError, errorMessage } from "@/lib/api-client";
import {
  formatFileSize,
  formatOcrStatus,
  getProfileCategoryMeta,
  selectUploadsByCategory,
  type ProfileCategoryId,
  type ProfileUploadLike,
} from "../model/profileCategories";
import { usePersonWindowData } from "../hooks/usePersonWindowData";
import { PersonIdentityPopover } from "./PersonIdentityPopover";
import { PersonMediaGrid } from "./PersonMediaGrid";
import { PersonCategoryMenu } from "./PersonCategoryMenu";
import { PersonAssetDeletion } from "./PersonAssetDeletion";
import { formatMediaDate } from "../model/personMedia";
import { personDisplayName } from "../model/personIdentity";
import { useUploadStore } from "@/stores/uploadStore";

/**
 * Person/Profile business window.
 *
 * Layout contract (Penpot Desktop / Mobile Person boards):
 * - header: avatar + nickname, a static "/", then the category picker
 *   ("BakerSean / 身体记录"); Close on the right; no duplicate title, no
 *   body-status card and no tab strip;
 * - clicking avatar/nickname opens the Identity popover (minimal optional
 *   Profile: nickname, birth date + self-reported gender, display time zone +
 *   units); outside click closes it;
 * - exactly five categories; the picker label is the only place the category
 *   name appears — no duplicate H1 and no generic explanation beneath it;
 * - Equipment empty state is a single centred sentence + add action; added
 *   equipment grows naturally (Desktop 4 columns, Mobile 2 columns) with no
 *   fixed slot count.
 */

/**
 * Header identity: the Profile nickname, else the account email's local part,
 * else a placeholder (personDisplayName). Never invented.
 */
export interface PersonProfileIdentity {
  nickname: string;
  /** The signed-in account's email; only its local part is displayed. */
  accountEmail?: string;
}

export interface PersonWindowProps {
  canEdit: boolean;
  /** Opens the 身体档案 drawer from the Identity popover. */
  onOpenRecords?: () => void;
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
  /** The header picker owns the category; uncontrolled use keeps local state. */
  category?: ProfileCategoryId;
  onCategoryChange?: (category: ProfileCategoryId) => void;
}

/**
 * Avatar + nickname header trigger. Exported because the shared window host owns
 * the header chrome, and the design puts this trigger *instead of* a repeated
 * business title.
 */
/**
 * Penpot "Overlay / Person / Account menu": the avatar + nickname in the Person
 * header is the only account entry (Home has no app bar). It opens a menu with
 * 个人资料 (identity popover), 设置 (theme) and 退出登录 (confirm first).
 */
export function PersonIdentityTrigger({
  identity,
  open,
  onToggle,
  onOpenSettings,
  onRequestLogout,
}: {
  identity: PersonProfileIdentity;
  open: boolean;
  onToggle: () => void;
  onOpenSettings?: () => void;
  onRequestLogout?: () => void;
}) {
  const display = personDisplayName(identity);
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className="bc-person-identity"
        title={
          identity.accountEmail
            ? `账户菜单 · ${identity.accountEmail}`
            : "账户菜单"
        }
        data-identity-open={open ? "true" : undefined}
        data-name-source={display.source}
      >
        {/* Penpot Person header: the BodySense figure avatar (exported from the
            board) and the nickname with the gold dantian dot. */}
        <span className="bc-person-avatar" aria-hidden="true">
          <img src={personAvatar} alt="" width={32} height={32} />
        </span>
        <span className="bc-person-name">{display.name}</span>
        <span className="bc-person-dantian" aria-hidden="true" />
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="start"
        sideOffset={8}
        className="bc-account-menu"
      >
        <DropdownMenuItem
          onClick={() => {
            if (!open) onToggle();
          }}
        >
          <UserRound />
          个人资料
        </DropdownMenuItem>
        {onOpenSettings ? (
          <DropdownMenuItem onClick={onOpenSettings}>
            <Settings />
            设置
          </DropdownMenuItem>
        ) : null}
        {onRequestLogout ? (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" onClick={onRequestLogout}>
              <LogOut />
              退出登录
            </DropdownMenuItem>
          </>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/**
 * Counts for the category menu, only from data the app already read: the
 * upload list once fetched, the equipment inventory, and the body-fact ledger
 * (records = every fact, habits = lifestyle facts, the same split the two
 * categories render). A ledger count is shown only once every page is loaded
 * at one revision; a partial or stale read shows no number, never a guess.
 */
function usePersonCategoryCounts(): Partial<Record<ProfileCategoryId, string>> {
  const uploads = useUploadStore(
    (state) => state.uploads,
  ) as ProfileUploadLike[];
  const fetched = useUploadStore((state) => state.hasFetched);
  const owner = useAuthStore((state) => state.user?.id);
  const equipment = useQuery({
    queryKey: equipmentKeys.all,
    queryFn: equipmentService.list,
    enabled: Boolean(owner),
  });
  // Read-only view of the ledger the Records / Habits categories fetch.
  const ledger = useInfiniteQuery({
    queryKey: bodyFactRecordsKey(owner),
    initialPageParam: undefined as number | undefined,
    queryFn: async ({ pageParam }) => {
      const page = await bodyFactRecordsService.list(pageParam);
      if (page.user_id !== owner)
        throw new Error("档案身份已变化，请重新读取。");
      return page;
    },
    getNextPageParam: (page) =>
      page.has_more ? page.next_before_revision : undefined,
    enabled: false,
  });
  const counts: Partial<Record<ProfileCategoryId, string>> = {};
  if (fetched) {
    counts.media = `${selectUploadsByCategory(uploads, "media").length} 个`;
    counts.reports = `${selectUploadsByCategory(uploads, "reports").length} 份`;
  }
  if (equipment.data) counts.equipment = `${equipment.data.items.length} 件`;
  const pages = ledger.data?.pages;
  if (owner && pages?.length && pages.every((page) => page.user_id === owner)) {
    const surface = (kind: "records" | "habits") =>
      projectBodyFactSurface({
        pages,
        surface: kind,
        hasOwner: true,
        isPending: false,
        isError: false,
      });
    const records = surface("records");
    const habits = surface("habits");
    if (!records.hasMore && !records.pagesDisagree) {
      counts.records = `${records.records.length} 条`;
      counts.habits = `${habits.records.length} 项`;
    }
  }
  return counts;
}

/**
 * Penpot Person header: "BakerSean / 身体记录 ▾". The category picker replaces
 * the old tab strip; it opens Penpot's icon menu, built as an ARIA listbox.
 */
export function PersonWindowHeader({
  identity,
  open,
  onToggle,
  onOpenSettings,
  onRequestLogout,
  category,
  onCategoryChange,
}: {
  identity: PersonProfileIdentity;
  open: boolean;
  onToggle: () => void;
  onOpenSettings?: () => void;
  onRequestLogout?: () => void;
  category: ProfileCategoryId;
  onCategoryChange: (category: ProfileCategoryId) => void;
}) {
  const counts = usePersonCategoryCounts();
  return (
    <div className="bc-window-context bc-person-context">
      <PersonIdentityTrigger
        identity={identity}
        open={open}
        onToggle={onToggle}
        onOpenSettings={onOpenSettings}
        onRequestLogout={onRequestLogout}
      />
      <span className="bc-window-context-sep" aria-hidden="true">
        /
      </span>
      <PersonCategoryMenu
        value={category}
        onChange={onCategoryChange}
        counts={counts}
      />
    </div>
  );
}

/** Stable empty snapshot so the workbench can render its own empty state. */
function EmptyState({
  icon,
  title,
  message,
  actionLabel,
  onAction,
  disabled,
}: {
  icon: React.ReactNode;
  title: string;
  message: string;
  actionLabel?: string;
  onAction?: () => void;
  disabled?: boolean;
}) {
  // Penpot "<Category> · Empty": 64px mint icon disc, title, one quiet line
  // and a round 32px primary action (same anatomy as Records · Empty).
  return (
    <div className="bc-person-state bc-person-empty">
      <span className="bc-person-state-icon" aria-hidden="true">
        {icon}
      </span>
      <strong>{title}</strong>
      <span className="bc-person-empty-text">{message}</span>
      {actionLabel && onAction ? (
        <button
          type="button"
          className="bc-person-add bc-person-empty-action"
          aria-label={actionLabel}
          title={actionLabel}
          onClick={onAction}
          disabled={disabled}
        >
          <Plus size={17} />
        </button>
      ) : null}
    </div>
  );
}

/**
 * Equipment is the user's own EquipmentAsset inventory (PC-D2), read from
 * /api/v1/equipment — not BodyState. The category has no fixed slot ceiling:
 * cards grow with the data (4 columns desktop / 2 columns mobile). Marking an
 * item unavailable or removing it never edits an accepted plan, and the list
 * never asserts that an item is suitable for the user's current condition.
 */
function EquipmentCategory({
  canEdit,
  onAddEquipment,
}: {
  canEdit: boolean;
  /** Opens the inline add form; there is no separate selector route yet. */
  onAddEquipment?: () => void;
}) {
  const queryClient = useQueryClient();
  const inventory = useQuery({
    queryKey: equipmentKeys.all,
    queryFn: equipmentService.list,
  });
  const equipment = inventory.data?.items ?? [];
  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: equipmentKeys.all });
  const [busyId, setBusyId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);
  const [name, setName] = useState("");
  const [notes, setNotes] = useState("");
  // One key per add attempt, so a network retry replays instead of duplicating.
  const createKey = useRef<string | null>(null);
  const onConflict = (cause: unknown) => {
    if (cause instanceof ApiRequestError && cause.status === 409) {
      void refresh();
    }
  };
  const create = useMutation({
    mutationFn: equipmentService.create,
    onSuccess: refresh,
    onError: onConflict,
  });
  const update = useMutation({
    mutationFn: ({
      asset,
      availability,
    }: {
      asset: EquipmentAsset;
      availability: EquipmentAsset["availability"];
    }) =>
      equipmentService.update(
        asset.id,
        availabilityChange(asset, availability),
      ),
    onSuccess: refresh,
    onError: onConflict,
  });
  const pending = create.isPending || update.isPending;

  const changeAvailability = async (
    asset: EquipmentAsset,
    availability: EquipmentAsset["availability"],
  ) => {
    if (!canEdit || pending) return;
    setBusyId(asset.id);
    setError(null);
    try {
      await update.mutateAsync({ asset, availability });
    } catch (cause) {
      setError(errorMessage(cause, "器材状态没有保存，请重试。"));
    } finally {
      setBusyId(null);
    }
  };

  const resetForm = () => {
    setAdding(false);
    setName("");
    setNotes("");
    createKey.current = null;
  };

  const add = async () => {
    const trimmed = name.trim();
    if (!canEdit || !trimmed || pending) return;
    setError(null);
    if (
      equipment.some(
        (asset) =>
          asset.display_name.toLocaleLowerCase() ===
          trimmed.toLocaleLowerCase(),
      )
    ) {
      setError("这件器材已经在当前清单中。");
      return;
    }
    createKey.current ??= crypto.randomUUID();
    try {
      await create.mutateAsync({
        idempotency_key: createKey.current,
        display_name: trimmed,
        notes: notes.trim() || null,
        access_kind: "owned",
        availability: "available",
      });
      resetForm();
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
          onChange={(event) => {
            setName(event.target.value);
            createKey.current = null;
          }}
          maxLength={80}
          placeholder="例如：弹力带"
          disabled={!canEdit || pending}
          autoFocus
        />
      </label>
      <label>
        规格或备注<span className="bc-optional">可选</span>
        <input
          value={notes}
          onChange={(event) => {
            setNotes(event.target.value);
            createKey.current = null;
          }}
          maxLength={240}
          placeholder="阻力、重量或使用限制"
          disabled={!canEdit || pending}
        />
      </label>
      <button
        type="submit"
        className="bc-button bc-primary"
        disabled={!canEdit || pending || !name.trim()}
      >
        {create.isPending ? (
          <LoaderCircle size={17} className="animate-spin" />
        ) : (
          <Plus size={17} />
        )}
        添加设备
      </button>
      <button
        type="button"
        className="bc-button"
        disabled={pending}
        onClick={resetForm}
      >
        取消
      </button>
    </form>
  );

  if (inventory.isPending) {
    return (
      <p role="status" className="bc-person-loading">
        <LoaderCircle size={16} className="animate-spin" />
        正在读取器材
      </p>
    );
  }
  if (inventory.isError) {
    return (
      <p role="alert" className="bc-error">
        {errorMessage(inventory.error, "器材清单暂时无法读取。")}
      </p>
    );
  }

  if (equipment.length === 0 && !adding) {
    return (
      <>
        <EmptyState
          icon={<Dumbbell size={24} />}
          title="还没有器材"
          message="加上家里或健身房能用的器材，训练会按它来安排"
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
          <p className="bc-person-count">已登记 {equipment.length} 件器材</p>
          <ul className="bc-person-equipment">
            {equipment.map((asset) => {
              const available = asset.availability === "available";
              const busy = busyId === asset.id;
              return (
                <li key={asset.id}>
                  <span
                    className="bc-person-equipment-symbol"
                    aria-hidden="true"
                  >
                    <Backpack size={20} />
                  </span>
                  <strong>{asset.display_name}</strong>
                  {asset.notes ? <p>{asset.notes}</p> : null}
                  <span className="bc-status">
                    {available ? <Check size={13} /> : null}
                    {available ? "当前可用" : "暂不可用"}
                  </span>
                  <button
                    type="button"
                    className="bc-text-button"
                    disabled={!canEdit || pending || busy}
                    onClick={() =>
                      void changeAvailability(
                        asset,
                        available ? "unavailable" : "available",
                      )
                    }
                  >
                    {busy ? (
                      <LoaderCircle size={14} className="animate-spin" />
                    ) : null}
                    {available ? "标记暂不可用" : "恢复可用"}
                  </button>
                  <button
                    type="button"
                    className="bc-text-button"
                    disabled={!canEdit || pending || busy}
                    onClick={() => void changeAvailability(asset, "retired")}
                  >
                    移除
                  </button>
                </li>
              );
            })}
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

const EXTRACTION_LABELS: Record<string, { label: string; tone: string }> = {
  pending: { label: "等待", tone: "neutral" },
  processing: { label: "处理中", tone: "warning" },
  completed: { label: "完成", tone: "keep" },
  failed: { label: "失败", tone: "danger" },
};

function fileExtension(upload: ProfileUploadLike): string {
  const ext = upload.original_name.split(".").pop();
  if (ext && ext !== upload.original_name) return ext.toUpperCase();
  return upload.mime_type.split("/").pop()?.toUpperCase() ?? "文件";
}

/**
 * Penpot "Person / Reports" card: file glyph, name, "PDF · size · date", and a
 * 文件 / 提取 status row read from the manifest. The "…" menu opens the Penpot
 * Assets delete scope for this report.
 */
function ReportCard({
  upload,
  onDelete,
}: {
  upload: ProfileUploadLike;
  onDelete: () => void;
}) {
  const sourceGone = upload.source_availability === "source_unavailable";
  const extraction = EXTRACTION_LABELS[upload.ocr_status] ?? {
    label: formatOcrStatus(upload.ocr_status),
    tone: "neutral",
  };
  return (
    <li className="bc-person-report" data-testid="person-report">
      <div className="bc-person-report-main">
        <span className="bc-asset-file-icon" aria-hidden="true">
          <FileText size={20} />
        </span>
        <span className="bc-person-report-text">
          <strong>{upload.original_name}</strong>
          <span>
            {fileExtension(upload)} ·{" "}
            {sourceGone
              ? "原件已移除"
              : `${formatFileSize(upload.file_size)} · ${formatMediaDate(upload.created_at)}上传`}
          </span>
        </span>
        <DropdownMenu>
          <DropdownMenuTrigger
            className="bc-icon-button bc-person-report-menu"
            aria-label={`管理“${upload.original_name}”`}
          >
            <MoreHorizontal size={18} aria-hidden="true" />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="bc-account-menu">
            <DropdownMenuItem variant="destructive" onClick={onDelete}>
              <Trash2 />
              删除…
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      <dl className="bc-person-report-status">
        <div>
          <dt>文件</dt>
          <dd>
            <span
              className="bc-asset-chip"
              data-tone={sourceGone ? "warning" : "keep"}
            >
              {sourceGone ? "原件不可用" : "可用"}
            </span>
          </dd>
        </div>
        <div>
          <dt>提取</dt>
          <dd>
            <span className="bc-asset-chip" data-tone={extraction.tone}>
              {extraction.label}
            </span>
          </dd>
        </div>
      </dl>
    </li>
  );
}

function UploadsCategory({
  uploads,
  category,
  mobile = false,
}: {
  uploads: readonly ProfileUploadLike[];
  category: "media" | "reports";
  mobile?: boolean;
}) {
  const items = selectUploadsByCategory(uploads, category);
  const uploadFile = useUploadStore((state) => state.uploadFile);
  const forgetUpload = useUploadStore((state) => state.forgetUpload);
  // Penpot Assets scenes replace the list while one file is being deleted.
  const [deleting, setDeleting] = useState<ProfileUploadLike | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState(false);
  const [failed, setFailed] = useState<{
    name: string;
    size: number;
    at: Date;
  } | null>(null);
  const label = category === "reports" ? "上传报告" : "上传照片";
  // Penpot "Media · Upload failed" keeps the file name and offers a retry.
  const lastFile = useRef<File | null>(null);
  const send = async (file: File) => {
    lastFile.current = file;
    setUploading(true);
    setFailed(null);
    try {
      await uploadFile(
        file,
        category === "reports" ? "report" : "consultation_photo",
      );
      lastFile.current = null;
    } catch {
      setFailed({ name: file.name, size: file.size, at: new Date() });
    } finally {
      setUploading(false);
    }
  };
  const picker = (
    <input
      ref={inputRef}
      type="file"
      className="sr-only"
      tabIndex={-1}
      aria-hidden="true"
      accept={category === "reports" ? "image/*,application/pdf" : "image/*"}
      onChange={(event) => {
        const file = event.target.files?.[0];
        event.target.value = "";
        if (file) void send(file);
      }}
    />
  );
  const open = () => inputRef.current?.click();
  const status = uploading ? (
    <p role="status" className="bc-person-upload-status">
      <LoaderCircle size={14} className="animate-spin" aria-hidden="true" />
      正在上传…
    </p>
  ) : failed ? (
    // Penpot "Media · Upload failed": a pink row above the grid with the
    // file, 上传失败, retry and dismiss.
    <div role="alert" className="bc-person-upload-failed">
      <span className="bc-person-upload-failed-thumb" aria-hidden="true" />
      <span className="bc-person-upload-failed-text">
        <strong>{failed.name}</strong>
        <span>
          {formatFileSize(failed.size)} ·{" "}
          {failed.at.toLocaleString("zh-CN", {
            month: "long",
            day: "numeric",
            hour: "2-digit",
            minute: "2-digit",
          })}
        </span>
      </span>
      <span className="bc-person-upload-failed-chip">上传失败</span>
      <button
        type="button"
        className="bc-icon-button"
        aria-label="重试"
        title="重试"
        onClick={() => lastFile.current && void send(lastFile.current)}
      >
        <RotateCcw size={17} aria-hidden="true" />
      </button>
      <button
        type="button"
        className="bc-icon-button"
        aria-label="关闭失败提示"
        title="关闭"
        onClick={() => {
          setFailed(null);
          lastFile.current = null;
        }}
      >
        <X size={17} aria-hidden="true" />
      </button>
    </div>
  ) : null;
  if (deleting) {
    return (
      <PersonAssetDeletion
        upload={deleting}
        backLabel={category === "reports" ? "返回报告" : "返回媒体"}
        onBack={() => setDeleting(null)}
        onErased={(id) => {
          forgetUpload(id);
          setDeleting(null);
        }}
      />
    );
  }
  if (items.length === 0) {
    return (
      <>
        {picker}
        <EmptyState
          icon={
            category === "reports" ? (
              <FileText size={24} />
            ) : (
              <ImageIcon size={24} />
            )
          }
          title={category === "reports" ? "还没有报告" : "还没有照片或视频"}
          message={
            category === "reports"
              ? "上传体检、影像或化验报告，会关联到相关记录"
              : "上传动作视频或身体照片，方便之后对照"
          }
          actionLabel={label}
          onAction={open}
          disabled={uploading}
        />
        {status}
      </>
    );
  }
  return (
    <>
      {picker}
      <div className="bc-person-uploads-head">
        <span>
          {category === "media" ? `${items.length} 个` : `${items.length} 份`}
        </span>
        <button
          type="button"
          className="bc-person-upload"
          aria-label={label}
          title={label}
          onClick={open}
          disabled={uploading}
        >
          <Upload size={17} aria-hidden="true" />
          <span className="bc-person-upload-text" aria-hidden="true">
            上传
          </span>
        </button>
      </div>
      {status}
      {category === "media" ? (
        <PersonMediaGrid uploads={items} initialLimit={mobile ? 6 : 8} />
      ) : (
        <ul className="bc-person-reports">
          {items.map((upload) => (
            <ReportCard
              key={upload.id}
              upload={upload}
              onDelete={() => setDeleting(upload)}
            />
          ))}
        </ul>
      )}
    </>
  );
}

export function PersonWindow({
  canEdit,
  onOpenRecords,
  identityInfoOpen: infoOpen,
  onIdentityInfoOpenChange,
  category: controlledCategory,
  onCategoryChange,
}: PersonWindowProps) {
  const mobile = useMediaQuery("(max-width: 760px)");
  const [localCategory, setLocalCategory] =
    useState<ProfileCategoryId>("records");
  const category = controlledCategory ?? localCategory;
  const setCategory = (next: ProfileCategoryId) => {
    setLocalCategory(next);
    onCategoryChange?.(next);
  };
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
        <PersonIdentityPopover
          onClose={() => setInfoOpen(false)}
          onOpenRecords={onOpenRecords}
        />
      ) : null}

      <div
        className="bc-person-body"
        tabIndex={0}
        role="region"
        aria-label={meta.regionLabel}
        data-category={category}
      >
        {category === "records" ? (
          <PersonRecordsView canEdit={canEdit} />
        ) : category === "habits" ? (
          <BodyFactRecords habitsOnly canEdit={canEdit} />
        ) : data.error ? (
          <p role="alert" className="bc-error">
            {data.error}
          </p>
        ) : data.loading ? (
          <p role="status" className="bc-person-loading">
            <LoaderCircle size={16} className="animate-spin" />
            正在读取档案
          </p>
        ) : category === "equipment" ? (
          <EquipmentCategory
            canEdit={canEdit}
            onAddEquipment={() => setCategory("equipment")}
          />
        ) : category === "media" ? (
          <UploadsCategory
            uploads={data.uploads}
            category="media"
            mobile={mobile}
          />
        ) : category === "reports" ? (
          <UploadsCategory uploads={data.uploads} category="reports" />
        ) : null}
      </div>
    </section>
  );
}
