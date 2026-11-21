import {
  lazy,
  Suspense,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import { useThemeClass } from "../windows/person/useThemeClass";
import { LoaderCircle } from "lucide-react";
import { useAuthStore } from "@/stores/authStore";
import { useProfileStore } from "@/stores/profileStore";
import { effectiveTimeZone } from "@/features/profile/model/profilePreferences";
import { BodyFactRecordList } from "./BodyFactRecordList";
import { type BodyFactAmendMode } from "./BodyFactAmendment";
import type { StatementErasureTarget } from "./StatementErasure";
import { BodyFactRecordsAddDialog } from "./BodyFactRecordsAddDialog";
import {
  BodyFactRecordsDetail,
  type BodyFactRelatedLink,
} from "./BodyFactRecordsDetail";
import { BodyFactRecordsFeedback } from "./BodyFactRecordsFeedback";
import {
  BodyFactRecordsEmpty,
  BodyFactRecordsLoading,
  BodyFactRecordsReadFailure,
} from "./BodyFactRecordsStates";
import { useBodyFactRecordSubmission } from "../hooks/useBodyFactRecordSubmission";
import { type BodyFactKind as FactKind } from "../model/bodyFactPresentation";
import { projectBodyFactSurface } from "../model/bodyFactViewModel";
import { projectBoundedBodyFactVM } from "../model/bodyFactV2Adapter";
import { recordedClock } from "../model/personRecordsTimeline";
import {
  bodyFactRecordsKey,
  fetchBodyFactRecordsPage,
} from "../model/bodyFactRecordsQuery";

export { bodyFactRecordsKey } from "../model/bodyFactRecordsQuery";
export type { BodyFactRelatedLink } from "./BodyFactRecordsDetail";

// Rarely opened; kept out of the ConsultationPage chunk budget.
const StatementErasure = lazy(() =>
  import("./StatementErasure").then((module) => ({
    default: module.StatementErasure,
  })),
);

/** One live, user-scoped ledger surface; Habits is a filtered view, not a new owner. */
export function BodyFactRecords({
  habitsOnly = false,
  canEdit,
  related,
  relatedLinks,
  onOpenRelated,
  now,
  addOpen,
  onAddOpenChange,
}: {
  habitsOnly?: boolean;
  /** Controlled by the Person window header "+"; uncontrolled when omitted. */
  addOpen?: boolean;
  onAddOpenChange?: (open: boolean) => void;
  canEdit: boolean;
  /** Penpot Body / Region "相关" chips, rendered under the loaded count. */
  related?: ReactNode;
  /** Organising links (Episodes) of one fact, for its detail view. */
  relatedLinks?: (factId: string) => readonly BodyFactRelatedLink[];
  onOpenRelated?: (link: BodyFactRelatedLink) => void;
  /** Clock for "今天 / 昨天" day labels; tests pin it. */
  now?: Date;
}) {
  const owner = useAuthStore((state) => state.user?.id);
  const displayZone = effectiveTimeZone(
    useProfileStore((state) => state.profile?.preferences),
  ).zone;
  const ownerRef = useRef(owner);
  // Commit identity before asynchronous receipts can update this surface.
  useLayoutEffect(() => {
    ownerRef.current = owner;
  }, [owner]);
  const queryClient = useQueryClient();
  const query = useInfiniteQuery({
    queryKey: bodyFactRecordsKey(owner),
    initialPageParam: undefined as number | undefined,
    queryFn: ({ pageParam }) => fetchBodyFactRecordsPage(owner, pageParam),
    getNextPageParam: (page) =>
      page.has_more ? page.next_before_revision : undefined,
    enabled: !!owner,
    retry: false,
    staleTime: 0,
    refetchOnWindowFocus: true,
  });
  const [localAdding, setLocalAdding] = useState(false);
  const adding = onAddOpenChange ? addOpen === true : localAdding;
  const setAdding = (open: boolean) => {
    if (onAddOpenChange) onAddOpenChange(open);
    else setLocalAdding(open);
  };
  const themeAnchor = useRef<HTMLDivElement>(null);
  const themeClass = useThemeClass(themeAnchor);
  const [kind, setKind] = useState<FactKind>(
    habitsOnly ? "lifestyle" : "symptom",
  );
  const [statement, setStatement] = useState("");
  const [when, setWhen] = useState("");
  // The erasure panel outlives the record: a withdrawn record leaves the list.
  const [erasing, setErasing] = useState<StatementErasureTarget | null>(null);
  // Penpot Records rows open one statement's detail inside the same window.
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [amendMode, setAmendMode] = useState<BodyFactAmendMode>(null);

  const page = query.data?.pages[0];
  const view = useMemo(
    () =>
      projectBodyFactSurface({
        pages: query.data?.pages,
        surface: habitsOnly ? "habits" : "records",
        hasOwner: Boolean(owner),
        isPending: query.isPending,
        isError: query.isError,
      }),
    [query.data?.pages, habitsOnly, owner, query.isPending, query.isError],
  );

  // A bounded transport projection is not a complete V2 health read-model.
  // Keep ledger parsing stable while the user types into the local draft form.
  const boundedVM = useMemo(
    () => projectBoundedBodyFactVM(view, habitsOnly ? "habits" : "records"),
    [view, habitsOnly],
  );

  const {
    submit,
    saving,
    saveError,
    setSaveError,
    notice,
    setNotice,
    setSavedRevision,
    snapshotBehindCommit,
    reset: resetSubmission,
    resetPending,
  } = useBodyFactRecordSubmission({
    owner,
    ownerRef,
    canEdit,
    page,
    pagesDisagree: view.pagesDisagree,
    habitsOnly,
    kind,
    statement,
    when,
    onSaved: () => {
      setStatement("");
      setWhen("");
      setAdding(false);
    },
    refetch: () => void query.refetch(),
  });

  useEffect(() => {
    resetSubmission();
    setStatement("");
    setWhen("");
    setAdding(false);
    setErasing(null);
    setSelectedId(null);
    setAmendMode(null);
  }, [owner, resetSubmission]);

  if (!owner) return <p className="bc-window-note">请登录后查看个人记录。</p>;

  const clock = now ?? new Date();
  const rows = snapshotBehindCommit ? [] : view.records;
  const selected = selectedId
    ? (rows.find((row) => row.id === selectedId) ?? null)
    : null;
  const readFailed = query.isError && !page;
  const stale = query.isError && Boolean(page);
  const lastSync = query.dataUpdatedAt
    ? recordedClock(new Date(query.dataUpdatedAt).toISOString(), displayZone)
    : null;
  const addDisabled = !page || view.pagesDisagree || snapshotBehindCommit;
  const emptyState =
    !query.isError &&
    !query.isPending &&
    !snapshotBehindCommit &&
    boundedVM.state === "empty";

  const closeAdd = () => {
    setAdding(false);
    setSaveError(null);
  };
  const addForm = (
    <BodyFactRecordsAddDialog
      open={adding && canEdit}
      habitsOnly={habitsOnly}
      kind={kind}
      statement={statement}
      when={when}
      saving={saving}
      blocked={addDisabled}
      saveError={saveError}
      dialogClassName={`${themeClass()} bc-account-dialog bc-form-dialog`}
      onKindChange={(value) => {
        setKind(value);
        resetPending();
      }}
      onStatementChange={setStatement}
      onWhenChange={setWhen}
      onSubmit={() => void submit()}
      onDismiss={() => {
        if (!saving) closeAdd();
      }}
      onRequestClose={closeAdd}
    />
  );

  const erasure =
    erasing && canEdit ? (
      <Suspense
        fallback={
          <p role="status" className="bc-person-loading">
            <LoaderCircle size={16} className="animate-spin" />
            正在打开删除
          </p>
        }
      >
        <StatementErasure
          key={erasing.statementId}
          target={erasing}
          revision={page?.current_revision}
          onClose={() => setErasing(null)}
          onChanged={(message, committedRevision) => {
            if (ownerRef.current !== erasing.ownerId) return;
            if (committedRevision !== undefined)
              setSavedRevision((prev) =>
                Math.max(prev ?? 0, committedRevision),
              );
            setNotice(message);
            void queryClient.invalidateQueries({ queryKey: ["workspace"] });
          }}
        />
      </Suspense>
    ) : null;

  const feedback = (
    <BodyFactRecordsFeedback
      snapshotBehindCommit={snapshotBehindCommit}
      isError={query.isError}
      stale={stale}
      pagesDisagree={view.pagesDisagree}
      notice={notice}
      lastSync={lastSync}
      isFetching={query.isFetching}
      onRefetch={() => void query.refetch()}
    />
  );

  if (selected && !erasing) {
    return (
      <BodyFactRecordsDetail
        selected={selected}
        habitsOnly={habitsOnly}
        canEdit={canEdit}
        page={page}
        pagesDisagree={view.pagesDisagree}
        amendMode={amendMode}
        displayZone={displayZone}
        contract={boundedVM.contract}
        state={boundedVM.state}
        relatedLinks={relatedLinks}
        onOpenRelated={onOpenRelated}
        onCorrect={() => {
          setNotice(null);
          setAmendMode("correct");
        }}
        onWithdraw={() => {
          setNotice(null);
          setAmendMode("withdraw");
        }}
        onErase={(target) => {
          setNotice(null);
          setErasing(target);
        }}
        onAmendModeChange={setAmendMode}
        onRefresh={() => void query.refetch()}
        onCommitted={(message, committedRevision) => {
          if (ownerRef.current !== selected.record.user_id) return;
          // A correction/withdrawal also invalidates the old live
          // ledger snapshot; never display a revoked assertion while
          // its authoritative revision is still being fetched.
          setSavedRevision((prev) => Math.max(prev ?? 0, committedRevision));
          setNotice(message);
          setSelectedId(null);
          void queryClient.invalidateQueries({ queryKey: ["workspace"] });
        }}
        onBack={() => {
          setAmendMode(null);
          setSelectedId(null);
        }}
        feedback={feedback}
      />
    );
  }

  return (
    <div
      ref={themeAnchor}
      className="bc-person-category bc-person-records"
      data-testid="body-fact-records"
      data-vm-contract={boundedVM.contract}
      data-vm-state={boundedVM.state}
    >
      {query.isPending ? <BodyFactRecordsLoading /> : null}
      {readFailed ? (
        <BodyFactRecordsReadFailure
          notice={notice}
          isFetching={query.isFetching}
          onRefetch={() => void query.refetch()}
        />
      ) : null}
      {emptyState ? (
        <BodyFactRecordsEmpty
          habitsOnly={habitsOnly}
          notice={notice}
          related={related}
        />
      ) : null}
      {!query.isPending && !readFailed && !emptyState ? (
        <>
          <div className="bc-person-listbar">
            <span className="bc-person-count">
              {rows.length}
              {view.hasMore ? "+" : ""} {habitsOnly ? "项" : "条"}
            </span>
          </div>
          {related}
          {feedback}
        </>
      ) : null}
      {!query.isError &&
      !snapshotBehindCommit &&
      boundedVM.state === "partial_empty" ? (
        <p className="bc-window-empty-text">
          {habitsOnly
            ? "当前加载的记录中没有生活习惯，可以继续读取更早的记录。"
            : "当前加载的记录为空，可以继续读取更早的记录。"}
        </p>
      ) : null}
      {addForm}
      {erasure}
      <BodyFactRecordList
        rows={rows}
        habitsOnly={habitsOnly}
        displayZone={displayZone}
        now={clock}
        onSelect={(id) => {
          setAmendMode(null);
          setSelectedId(id);
          setNotice(null);
          setAdding(false);
        }}
      />
      {query.hasNextPage ? (
        <button
          type="button"
          className="bc-person-more"
          disabled={query.isFetchingNextPage}
          onClick={() => void query.fetchNextPage()}
        >
          {query.isFetchingNextPage ? "正在读取" : "读取更早记录"}
        </button>
      ) : null}
    </div>
  );
}
