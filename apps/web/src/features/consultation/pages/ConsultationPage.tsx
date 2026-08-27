import {
  Suspense,
  lazy,
  useState,
  useEffect,
  useCallback,
  useRef,
  type ReactNode,
} from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { InfoPanelSkeleton } from "../components/InfoPanelSkeleton";
import { ChatPanelSkeleton } from "../components/ChatPanelSkeleton";
import { useConversationsQuery } from "../hooks/useConversationsQuery";
import { useConsultationThreadQuery } from "../hooks/useConsultationThreadQuery";
import { useThreadProjectionActions } from "../hooks/useThreadProjectionActions";
import { useDiagnosisActions } from "../hooks/useDiagnosisActions";
import type {
  ConsultationSpatialContext,
  InteractionHistoryItem,
} from "../types/consultation";
import {
  buildActiveTurnSeedFromRuntimeEvents,
  toInitialThreadTimeline,
} from "../runtime/threadMessageMapping";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { ConsultationWorkbenchShell } from "../components/workbench/ConsultationWorkbenchShell";
import { WorkspaceViewport } from "../components/workbench/WorkspaceViewport";
import { parseWorkspaceView, type WorkspaceView } from "../model/workbenchView";
import { useWorkbenchPreferencesStore } from "../model/workbenchPreferencesStore";
import { shouldPromoteProvisionalChatSession } from "../runtime/chatSessionIdentity";
import { ProfileDrawer } from "@/features/profile/components/profile/ProfileDrawer";
import {
  BodyStateWorkbench,
  OutcomeTrendsPanel,
  TreatmentPanel,
  useHealthWorkspaceQuery,
} from "@/features/workspace";
import {
  getBodyRegionDefinition,
  type BodyRegionId,
  useBodyExplorerWorkspace,
} from "@/features/body-explorer";

const loadAssistantChatPanel = () =>
  import("../components/AssistantChatPanel").then((module) => ({
    default: module.AssistantChatPanel,
  }));
const AssistantChatPanel = lazy(loadAssistantChatPanel);
const DiagnosisPanel = lazy(() =>
  import("../components/DiagnosisPanel").then((module) => ({
    default: module.DiagnosisPanel,
  })),
);
const DiagnosisHistoryPanel = lazy(() =>
  import("../components/DiagnosisHistoryPanel").then((module) => ({
    default: module.DiagnosisHistoryPanel,
  })),
);

export function ConsultationPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const workspaceQuery = useHealthWorkspaceQuery();
  const [chatSpatialContext, setChatSpatialContext] =
    useState<ConsultationSpatialContext | null>(null);
  const [composerFocusKey, setComposerFocusKey] = useState(0);
  const setChatOpen = useWorkbenchPreferencesStore(
    (state) => state.setChatOpen,
  );
  const setMobileSurface = useWorkbenchPreferencesStore(
    (state) => state.setMobileSurface,
  );

  // Chat is a persistent primary surface. Discover its lazy chunk immediately
  // instead of waiting for the thread query to settle before starting download.
  useEffect(() => {
    void loadAssistantChatPanel();
  }, []);

  // --- Derived identity: URL is the single source of truth for "which conversation" ---
  const routeConversationId = id && id !== "new" ? id : null;
  const consultationLocation = useCallback(
    (conversationId?: string | null) => {
      const pathname = conversationId
        ? `/consultation/${conversationId}`
        : "/consultation";
      const query = searchParams.toString();
      return query ? `${pathname}?${query}` : pathname;
    },
    [searchParams],
  );
  const navigateToConversation = useCallback(
    (conversationId?: string | null) => {
      navigate(consultationLocation(conversationId), { replace: true });
    },
    [consultationLocation, navigate],
  );

  const [chatSessionKey, setChatSessionKey] = useState<string>("new");
  const [pendingTerminalRemountId, setPendingTerminalRemountId] = useState<
    string | null
  >(null);
  const justCreatedRef = useRef<string | null>(null);
  // Tracks the active conversation ID synchronously, updated before navigate().
  // Needed because SSE callbacks in the same event batch fire before React re-renders
  // with the new routeConversationId.
  const activeConversationIdRef = useRef<string | null>(null);

  // --- Server state via TanStack Query ---
  const conversationsQuery = useConversationsQuery();
  const conversations = conversationsQuery.data ?? [];
  const threadQuery = useConsultationThreadQuery(routeConversationId);
  const threadData = threadQuery.data;
  const displayedConversationId = routeConversationId
    ? (threadData?.conversation.id ?? null)
    : null;
  const isThreadOutOfSync =
    Boolean(routeConversationId) &&
    Boolean(displayedConversationId) &&
    displayedConversationId !== routeConversationId;
  const isThreadSwitching = isThreadOutOfSync && threadQuery.isFetching;
  const hasThreadSwitchError = isThreadOutOfSync && threadQuery.isError;
  const displayedThread = routeConversationId ? (threadData ?? null) : null;
  const messages = displayedThread?.messages ?? [];
  const extractedInfo = displayedThread?.extracted_info ?? [];
  const workspace = workspaceQuery.data;
  const bodyState =
    workspace?.body_state ?? displayedThread?.body_state ?? null;
  const handleAskSpatialContext = useCallback(
    (context: ConsultationSpatialContext) => {
      setChatSpatialContext(context);
      setChatOpen(true);
      setMobileSurface("chat");
      setComposerFocusKey((key) => key + 1);
    },
    [setChatOpen, setMobileSurface],
  );
  const bodyExplorer = useBodyExplorerWorkspace(
    bodyState,
    handleAskSpatialContext,
  );
  const diagnosisAnalysis = workspace?.diagnosis?.analysis_id
    ? workspace.diagnosis
    : null;
  const candidates = diagnosisAnalysis?.candidates ?? [];
  const pendingInteractions = displayedThread?.pending_interactions ?? [];
  const interactionHistory = displayedThread?.interaction_history ?? [];

  // Resolve the old empty `/consultation` route to the user's existing long-lived
  // conversation. This prevents a reused backend conversation from looking like an
  // unaddressable "new" chat in the browser.
  useEffect(() => {
    if (
      !routeConversationId &&
      !conversationsQuery.isPending &&
      conversations.length > 0
    ) {
      navigateToConversation(conversations[0].id);
    }
  }, [
    routeConversationId,
    conversationsQuery.isPending,
    conversations,
    navigateToConversation,
  ]);

  // Keep rendering the previously resolved conversation until the target thread is ready.
  useEffect(() => {
    if (!displayedConversationId) {
      setChatSessionKey("new");
      if (!routeConversationId) {
        activeConversationIdRef.current = null;
      }
      return;
    }

    if (justCreatedRef.current === displayedConversationId) {
      justCreatedRef.current = null;
      return;
    }

    activeConversationIdRef.current = null;
    setChatSessionKey(`conversation:${displayedConversationId}`);
  }, [displayedConversationId, routeConversationId]);

  const latestAssistantStatus =
    [...messages].reverse().find((message) => message.role === "assistant")
      ?.status ?? null;

  useEffect(() => {
    if (
      !shouldPromoteProvisionalChatSession({
        chatSessionKey,
        pendingConversationId: pendingTerminalRemountId,
        displayedConversationId,
        activeTurnRunId: displayedThread?.active_turn_run_id,
        latestAssistantStatus,
      })
    ) {
      return;
    }

    setChatSessionKey(`conversation:${displayedConversationId}`);
    setPendingTerminalRemountId(null);
    activeConversationIdRef.current = null;
  }, [
    chatSessionKey,
    displayedConversationId,
    displayedThread?.active_turn_run_id,
    latestAssistantStatus,
    pendingTerminalRemountId,
  ]);

  // Client-only presentation state. The active workspace mode is URL-addressable.
  const workspaceView = parseWorkspaceView(searchParams.get("view"));
  const [isProfileOpen, setIsProfileOpen] = useState(false);

  const handleWorkspaceViewChange = useCallback(
    (view: WorkspaceView) => {
      const next = new URLSearchParams(searchParams);
      next.set("view", view);
      setSearchParams(next, { replace: true });
    },
    [searchParams, setSearchParams],
  );

  // Server-state actions are isolated in feature hooks; this page composes them.
  const threadActions = useThreadProjectionActions({
    routeConversationId,
    activeConversationIdRef,
  });
  const diagnosisActions = useDiagnosisActions({
    conversationId: routeConversationId,
    bodyState,
    analysis: diagnosisAnalysis,
  });
  const diagnosisHistory = diagnosisActions.history;

  const handleAnalyzeDiagnosis = useCallback(() => {
    if (
      !diagnosisActions.canAnalyze ||
      diagnosisActions.isAnalyzing ||
      workspace?.capabilities.can_request_diagnosis === false
    )
      return;
    diagnosisActions.analyze();
  }, [diagnosisActions, workspace?.capabilities.can_request_diagnosis]);

  // --- Loading / Error States ---
  const isThreadLoading =
    Boolean(routeConversationId) && threadQuery.isPending && !threadData;
  const hasThreadError =
    Boolean(routeConversationId) && threadQuery.isError && !isThreadOutOfSync;
  const chatConversationId = displayedConversationId ?? "new";

  // --- Render ---

  const replayedTurnSeed = buildActiveTurnSeedFromRuntimeEvents(
    displayedThread?.active_turn_events ?? [],
    pendingInteractions,
  );
  const initialTurnSeed = replayedTurnSeed;
  const historicalMessages = initialTurnSeed?.consumedMessageId
    ? messages.filter(
        (message) => message.id !== initialTurnSeed.consumedMessageId,
      )
    : messages;

  const chatPanel = (
    <div className="h-full min-h-0 bg-[#171717]">
      <div className="flex h-full min-h-0 flex-col overflow-hidden bg-[#171717]">
        {hasThreadError || hasThreadSwitchError ? (
          <div className="flex h-full items-center justify-center p-6">
            <Card className="max-w-md border border-white/[0.08] bg-white/[0.035] p-8 text-center text-[#ececec] shadow-none">
              <div className="mx-auto mb-4 flex size-14 items-center justify-center rounded-full border border-destructive/20 bg-destructive/10 text-destructive">
                <svg
                  className="size-7"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  aria-hidden="true"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                  />
                </svg>
              </div>
              <p className="mb-2 text-lg font-semibold text-white/90">
                暂时无法加载对话
              </p>
              <p className="mb-6 text-sm text-white/45">
                BodySense 的对话内容暂时无法同步，请稍后重试。
              </p>
              <Button onClick={threadActions.retryThread} className="w-full">
                重新加载
              </Button>
            </Card>
          </div>
        ) : isThreadLoading ? (
          <ChatPanelSkeleton />
        ) : (
          <div className="relative flex h-full min-h-0 flex-col">
            <Suspense fallback={<ChatPanelSkeleton />}>
              <AssistantChatPanel
                key={chatSessionKey}
                conversationId={chatConversationId}
                onConversationCreated={(newId) => {
                  console.debug(
                    "[SSE] ⑤ ConsultationPage.onConversationCreated 开始执行",
                    {
                      newId,
                      currentRouteId: id,
                      currentActiveRef: activeConversationIdRef.current,
                    },
                  );
                  justCreatedRef.current = newId;
                  activeConversationIdRef.current = newId;

                  threadActions.registerConversation(newId);

                  navigateToConversation(newId);
                }}
                initialMessages={toInitialThreadTimeline(
                  historicalMessages,
                  interactionHistory as InteractionHistoryItem[],
                )}
                initialActiveTurn={initialTurnSeed?.activeTurn ?? null}
                initialExtractedInfo={extractedInfo}
                onExtractedInfoUpdate={threadActions.updateExtractedInfo}
                onPhaseChange={threadActions.updatePhase}
                onTitleGenerated={threadActions.updateTitle}
                onMessagePersisted={threadActions.reconcileMessageId}
                spatialContext={chatSpatialContext}
                onClearSpatialContext={() => setChatSpatialContext(null)}
                composerFocusKey={composerFocusKey}
                onStreamFinished={() => {
                  const terminalConversationId =
                    activeConversationIdRef.current ??
                    routeConversationId ??
                    displayedConversationId;
                  if (chatSessionKey === "new" && terminalConversationId) {
                    setPendingTerminalRemountId(terminalConversationId);
                  }
                  threadActions.finishStream();
                }}
              />
            </Suspense>
            {isThreadSwitching ? (
              <PanelTransitionOverlay label="身体信息正在更新" />
            ) : null}
          </div>
        )}
      </div>
    </div>
  );

  // The health workspace is an independent read model. A slow or failed chat
  // thread must not hold the business surface behind a skeleton when the
  // workspace endpoint has already returned.
  const workspaceUnavailable = workspaceQuery.isError && !workspace;
  const bodyStateReady = Boolean(bodyState);
  const canRequestDiagnosis =
    workspace?.capabilities.can_request_diagnosis ??
    diagnosisActions.canAnalyze;

  const stateWorkspace = workspaceUnavailable ? (
    <WorkspaceError message="身体信息暂时无法同步，请稍后重试。" />
  ) : workspaceQuery.isPending && !bodyState ? (
    <InfoPanelSkeleton />
  ) : bodyStateReady && bodyState ? (
    <BodyStateWorkbench
      snapshot={{
        ...bodyState,
        hypotheses: bodyState.hypotheses ?? [],
      }}
      selectedRegionId={bodyExplorer.selectedRegionId}
      onSelectRegion={bodyExplorer.selectRegion}
      onAskRegion={(regionId: BodyRegionId) => {
        const region = getBodyRegionDefinition(regionId);
        handleAskSpatialContext({
          body_region_id: regionId,
          body_region_label: region.labels["zh-CN"],
        });
      }}
    />
  ) : (
    <WorkspaceEmptyState
      title="还没有身体记录"
      description="继续对话，确认后的身体信息会出现在这里。"
    />
  );

  const diagnosisWorkspace = workspaceUnavailable ? (
    <WorkspaceError message="当前分析暂时无法同步，请稍后重试。" />
  ) : workspaceQuery.isPending && !workspace ? (
    <InfoPanelSkeleton />
  ) : (
    <div className="space-y-5">
      {diagnosisActions.error ? (
        <p className="rounded-xl border border-destructive/20 bg-destructive/10 px-3 py-2 text-xs font-medium text-destructive">
          {diagnosisActions.error}
        </p>
      ) : null}
      <Suspense fallback={<InfoPanelSkeleton />}>
        {candidates.length === 0 &&
        diagnosisAnalysis?.status !== "insufficient_information" &&
        diagnosisAnalysis?.status !== "safety_blocked" ? (
          <WorkspaceEmptyState
            title="还没有分析结果"
            description="继续补充身体情况，信息足够后可以生成分析。"
            action={
              <Button
                onClick={handleAnalyzeDiagnosis}
                disabled={!canRequestDiagnosis || diagnosisActions.isAnalyzing}
              >
                {diagnosisActions.isAnalyzing ? "分析中..." : "生成分析"}
              </Button>
            }
          />
        ) : (
          <DiagnosisPanel
            analysisId={diagnosisAnalysis?.analysis_id}
            bodyStateRevision={diagnosisAnalysis?.body_state_revision}
            status={diagnosisAnalysis?.status}
            summary={diagnosisAnalysis?.summary}
            candidates={candidates}
            citations={diagnosisAnalysis?.citations}
            freshness={diagnosisAnalysis?.freshness}
            onSaveAssessments={diagnosisActions.saveAssessments}
            isSavingAssessments={diagnosisActions.isSavingAssessments}
          />
        )}
      </Suspense>
      <Suspense fallback={null}>
        <DiagnosisHistoryPanel
          analyses={diagnosisHistory}
          currentAnalysisId={diagnosisAnalysis?.analysis_id}
        />
      </Suspense>
    </div>
  );

  const treatmentWorkspace = workspaceUnavailable ? (
    <WorkspaceError message="当前方案暂时无法同步，请稍后重试。" />
  ) : workspaceQuery.isPending && !workspace ? (
    <InfoPanelSkeleton />
  ) : workspace ? (
    <TreatmentPanel workspace={workspace} />
  ) : (
    <WorkspaceEmptyState
      title="尚无当前方案"
      description="完成必要的分析与确认后，BodySense 会在这里整理下一步建议。"
    />
  );

  const progressWorkspace = workspaceUnavailable ? (
    <WorkspaceError message="进展信息暂时无法同步，请稍后重试。" />
  ) : workspaceQuery.isPending && !workspace ? (
    <InfoPanelSkeleton />
  ) : workspace &&
    (workspace.trends.length > 0 || workspace.recent_outcomes.length > 0) ? (
    <OutcomeTrendsPanel
      trends={workspace.trends}
      outcomes={workspace.recent_outcomes}
    />
  ) : (
    <WorkspaceEmptyState
      title="还没有进展记录"
      description="记录训练或身体变化后，趋势会出现在这里。"
    />
  );

  const workspacePanel = (
    <WorkspaceViewport
      view={workspaceView}
      bodyState={bodyState}
      state={stateWorkspace}
      diagnosis={diagnosisWorkspace}
      treatment={treatmentWorkspace}
      progress={progressWorkspace}
      overlay={
        isThreadSwitching ? (
          <PanelTransitionOverlay label="身体信息正在更新" />
        ) : null
      }
      bodyExplorerBridge={bodyExplorer.semanticBridge}
    />
  );

  return (
    <div className="h-dvh overflow-hidden bg-[#242624] text-foreground">
      <ConsultationWorkbenchShell
        title="BodySense"
        workspaceView={workspaceView}
        onWorkspaceViewChange={handleWorkspaceViewChange}
        onOpenProfile={() => setIsProfileOpen(true)}
        chat={chatPanel}
        workspace={workspacePanel}
      />
      <ProfileDrawer open={isProfileOpen} onOpenChange={setIsProfileOpen} />
    </div>
  );
}

function PanelTransitionOverlay({ label }: { label: string }) {
  return (
    <div className="absolute inset-0 z-20 flex items-center justify-center bg-background/72 backdrop-blur-[2px]">
      <div className="rounded-2xl border border-border bg-popover/95 px-5 py-4 text-center shadow-sm">
        <div className="mx-auto size-8 animate-spin rounded-full border-2 border-muted border-t-primary" />
        <p className="mt-3 text-sm font-semibold text-foreground">
          正在同步 BodySense
        </p>
        <p className="mt-1 max-w-[220px] text-xs text-muted-foreground">
          {label}
        </p>
      </div>
    </div>
  );
}

function WorkspaceEmptyState({
  title,
  description,
  action,
}: {
  title: string;
  description: string;
  action?: ReactNode;
}) {
  return (
    <div className="flex min-h-[280px] flex-col items-center justify-center px-6 py-12 text-center">
      <p className="text-sm font-semibold text-foreground">{title}</p>
      <p className="mx-auto mt-1.5 max-w-sm text-xs leading-5 text-muted-foreground">
        {description}
      </p>
      {action ? <div className="mt-5">{action}</div> : null}
    </div>
  );
}

function WorkspaceError({ message }: { message: string }) {
  return (
    <div className="rounded-2xl border border-destructive/20 bg-destructive/10 px-4 py-4 text-sm text-destructive">
      {message}
    </div>
  );
}
