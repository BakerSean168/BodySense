import {
  Suspense,
  lazy,
  useState,
  useEffect,
  useCallback,
  useRef,
} from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { SessionHistorySidebar } from "../components/SessionHistorySidebar";
import { SessionHistorySidebarSkeleton } from "../components/SessionHistorySidebarSkeleton";
import { InfoPanelSkeleton } from "../components/InfoPanelSkeleton";
import { ChatPanelSkeleton } from "../components/ChatPanelSkeleton";
import { InteractionMetricsPanel } from "../components/InteractionMetricsPanel";
import { useConversationsQuery } from "../hooks/useConversationsQuery";
import { useConsultationThreadQuery } from "../hooks/useConsultationThreadQuery";
import { useConversationActions } from "../hooks/useConversationActions";
import { useThreadProjectionActions } from "../hooks/useThreadProjectionActions";
import { useDiagnosisActions } from "../hooks/useDiagnosisActions";
import type {
  InteractionHistoryItem,
  ConsultationPhase,
} from "../types/consultation";
import {
  buildActiveTurnSeedFromRuntimeEvents,
  toInitialThreadTimeline,
} from "../runtime/threadMessageMapping";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { MainLayout } from "@/components/layout/MainLayout";
import { ConsultationWorkbenchShell } from "../components/workbench/ConsultationWorkbenchShell";
import { ConversationHistoryDrawer } from "../components/workbench/ConversationHistoryDrawer";
import { WorkspaceViewport } from "../components/workbench/WorkspaceViewport";
import { parseWorkspaceView, type WorkspaceView } from "../model/workbenchView";
import {
  BodyStateWorkbench,
  OutcomeTrendsPanel,
  TreatmentPanel,
  useHealthWorkspaceQuery,
} from "@/features/workspace";

const AssistantChatPanel = lazy(() =>
  import("../components/AssistantChatPanel").then((module) => ({
    default: module.AssistantChatPanel,
  })),
);
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

const PHASE_LABELS: Record<ConsultationPhase, string> = {
  collecting: "问诊收集中",
  ready_for_analysis: "可生成分析",
  analysis_ready: "分析已生成",
};

export function ConsultationPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const workspaceQuery = useHealthWorkspaceQuery();

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
  const selectedConversationSummary =
    conversations.find(
      (conversation) => conversation.id === routeConversationId,
    ) ?? null;
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
  const currentConversation = displayedThread?.conversation ?? null;
  const messages = displayedThread?.messages ?? [];
  const extractedInfo = displayedThread?.extracted_info ?? [];
  const phase = isThreadSwitching
    ? null
    : (displayedThread?.phase ?? "collecting");
  const workspace = workspaceQuery.data;
  const bodyState =
    workspace?.body_state ?? displayedThread?.body_state ?? null;
  const diagnosisAnalysis = workspace?.diagnosis?.analysis_id
    ? workspace.diagnosis
    : null;
  const candidates = diagnosisAnalysis?.candidates ?? [];
  const bodyStateItemCount =
    (bodyState?.facts?.length ?? 0) +
    (bodyState?.observations?.length ?? 0) +
    (bodyState?.hypotheses?.length ?? 0);
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

  // Client-only presentation state. The active workspace mode is URL-addressable.
  const workspaceView = parseWorkspaceView(searchParams.get("view"));
  const [isHistoryOpen, setIsHistoryOpen] = useState(false);

  const handleWorkspaceViewChange = useCallback(
    (view: WorkspaceView) => {
      const next = new URLSearchParams(searchParams);
      next.set("view", view);
      setSearchParams(next, { replace: true });
    },
    [searchParams, setSearchParams],
  );

  // Server-state actions are isolated in feature hooks; this page composes them.
  const conversationActions = useConversationActions({
    conversations,
    routeConversationId,
    navigateTo: navigateToConversation,
  });
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

  const handleNewConsultation = useCallback(() => {
    const existingConversationId = conversations[0]?.id;
    if (existingConversationId) {
      navigateToConversation(existingConversationId);
      return;
    }
    if (routeConversationId) navigateToConversation();
  }, [conversations, navigateToConversation, routeConversationId]);

  const handleSelectConversation = useCallback(
    (conversationId: string) => {
      setIsHistoryOpen(false);
      navigateToConversation(conversationId);
    },
    [navigateToConversation],
  );

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
  const isConversationListLoading =
    conversationsQuery.isPending && conversations.length === 0;
  const isThreadLoading =
    Boolean(routeConversationId) && threadQuery.isPending && !threadData;
  const hasThreadError =
    Boolean(routeConversationId) && threadQuery.isError && !isThreadOutOfSync;
  const displayedPhaseLabel = phase ? PHASE_LABELS[phase] : "正在切换会话";
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

  const historyPanel = isConversationListLoading ? (
    <SessionHistorySidebarSkeleton />
  ) : (
    <SessionHistorySidebar
      conversations={conversations}
      activeId={routeConversationId}
      onPrefetch={conversationActions.prefetchConversation}
      onSelect={handleSelectConversation}
      onNew={handleNewConsultation}
      onDelete={conversationActions.deleteConversation}
      onDeleteAll={conversationActions.deleteAllConversations}
      onPin={conversationActions.pinConversation}
      onRename={conversationActions.renameConversation}
      onShare={conversationActions.shareConversation}
      onUnshare={conversationActions.unshareConversation}
    />
  );

  const chatPanel = (
    <div className="h-full min-h-0 bg-muted/20 p-3 sm:p-4">
      <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border border-border bg-card shadow-sm">
        {hasThreadError || hasThreadSwitchError ? (
          <div className="flex h-full items-center justify-center p-6">
            <Card className="max-w-md border border-border bg-card p-8 text-center shadow-none">
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
              <p className="mb-2 text-lg font-semibold text-foreground">
                加载会话失败
              </p>
              <p className="mb-6 text-sm text-muted-foreground">
                当前会话内容暂时无法加载，请稍后重试。
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
            <div className="shrink-0 px-4 pt-3">
              <InteractionMetricsPanel
                conversationId={displayedConversationId}
              />
            </div>
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
                onStreamFinished={threadActions.finishStream}
              />
            </Suspense>
            {isThreadSwitching ? (
              <PanelTransitionOverlay
                label={selectedConversationSummary?.title || "正在切换会话"}
              />
            ) : null}
          </div>
        )}
      </div>
    </div>
  );

  const workspaceUnavailable = hasThreadError || hasThreadSwitchError;
  const bodyStateReady = Boolean(bodyState);
  const canRequestDiagnosis =
    workspace?.capabilities.can_request_diagnosis ??
    diagnosisActions.canAnalyze;

  const stateWorkspace = workspaceUnavailable ? (
    <WorkspaceError message="会话详情加载失败，暂时无法展示长期身体状态。" />
  ) : isThreadLoading || (workspaceQuery.isPending && !bodyState) ? (
    <InfoPanelSkeleton />
  ) : bodyStateReady && bodyState ? (
    <div className="space-y-4">
      <WorkspaceHeading
        eyebrow="Durable state"
        title="当前身体状态"
        description="事实、已确认观察与活动中的假设共同组成可追溯的长期 BodyState。"
      />
      <BodyStateWorkbench
        snapshot={{
          ...bodyState,
          hypotheses: bodyState.hypotheses ?? [],
        }}
      />
    </div>
  ) : (
    <WorkspaceEmptyState
      title="BodyState 尚未建立"
      description="继续对话或记录一项事实后，长期身体状态会在这里形成。"
    />
  );

  const diagnosisWorkspace = workspaceUnavailable ? (
    <WorkspaceError message="会话详情加载失败，暂时无法展示可能性分析。" />
  ) : isThreadLoading ? (
    <InfoPanelSkeleton />
  ) : (
    <div className="space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <WorkspaceHeading
          eyebrow="Reasoning snapshot"
          title="可能性分析"
          description={`${displayedPhaseLabel} · 每次分析固定到明确的 BodyState revision，并保留不确定性。`}
        />
        {candidates.length === 0 ? (
          <Button
            onClick={handleAnalyzeDiagnosis}
            disabled={!canRequestDiagnosis || diagnosisActions.isAnalyzing}
            className="shrink-0"
          >
            {diagnosisActions.isAnalyzing ? "分析中..." : "生成当前分析"}
          </Button>
        ) : null}
      </div>
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
            title="尚无当前分析"
            description="BodyState 中形成至少一项事实或已确认观察后，可以生成可能性分析。"
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
    <WorkspaceError message="会话详情加载失败，暂时无法展示当前方案。" />
  ) : workspace ? (
    <div className="space-y-4">
      <WorkspaceHeading
        eyebrow="Reviewable strategy"
        title="当前方案"
        description="AI 只创建可审核 proposal；接受、暂停与执行均由明确业务边界控制。"
      />
      <TreatmentPanel workspace={workspace} />
    </div>
  ) : (
    <WorkspaceEmptyState
      title="尚无方案"
      description="完成并审核可能性分析后，才能生成需要明确接受的方案 proposal。"
    />
  );

  const progressWorkspace = workspaceUnavailable ? (
    <WorkspaceError message="会话详情加载失败，暂时无法展示长期进展。" />
  ) : workspace ? (
    <div className="space-y-4">
      <WorkspaceHeading
        eyebrow="Outcome loop"
        title="变化与进展"
        description="记录干预后的变化，并保持“时间关联”与“已证明因果”之间的边界。"
      />
      <OutcomeTrendsPanel
        trends={workspace.trends}
        outcomes={workspace.recent_outcomes}
      />
    </div>
  ) : (
    <WorkspaceEmptyState
      title="尚无进展记录"
      description="接受方案并记录训练或身体变化后，趋势会在这里出现。"
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
          <PanelTransitionOverlay
            label={selectedConversationSummary?.title || "正在切换会话"}
          />
        ) : null
      }
    />
  );

  return (
    <MainLayout fullHeight chrome="immersive">
      <ConsultationWorkbenchShell
        title="智能问诊工作台"
        phaseLabel={`${currentConversation?.title || selectedConversationSummary?.title || (routeConversationId ? "会话已激活" : "准备咨询")} · ${displayedPhaseLabel}`}
        bodyStateRevision={bodyState?.current_revision ?? 0}
        bodyStateItemCount={bodyStateItemCount}
        workspaceView={workspaceView}
        onWorkspaceViewChange={handleWorkspaceViewChange}
        onOpenHistory={() => setIsHistoryOpen(true)}
        chat={chatPanel}
        workspace={workspacePanel}
      />
      <ConversationHistoryDrawer
        open={isHistoryOpen}
        onOpenChange={setIsHistoryOpen}
      >
        {historyPanel}
      </ConversationHistoryDrawer>
    </MainLayout>
  );
}

function PanelTransitionOverlay({ label }: { label: string }) {
  return (
    <div className="absolute inset-0 z-20 flex items-center justify-center bg-background/72 backdrop-blur-[2px]">
      <div className="rounded-2xl border border-border bg-popover/95 px-5 py-4 text-center shadow-sm">
        <div className="mx-auto size-8 animate-spin rounded-full border-2 border-muted border-t-primary" />
        <p className="mt-3 text-sm font-semibold text-foreground">
          正在切换会话
        </p>
        <p className="mt-1 max-w-[220px] text-xs text-muted-foreground">
          {label}
        </p>
      </div>
    </div>
  );
}

function WorkspaceHeading({
  eyebrow,
  title,
  description,
}: {
  eyebrow: string;
  title: string;
  description: string;
}) {
  return (
    <header>
      <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-primary">
        {eyebrow}
      </p>
      <h2 className="mt-1 text-xl font-semibold text-foreground">{title}</h2>
      <p className="mt-1.5 max-w-2xl text-sm leading-6 text-muted-foreground">
        {description}
      </p>
    </header>
  );
}

function WorkspaceEmptyState({
  title,
  description,
}: {
  title: string;
  description: string;
}) {
  return (
    <div className="rounded-2xl border border-dashed border-border bg-muted/25 px-5 py-10 text-center">
      <p className="text-sm font-semibold text-foreground">{title}</p>
      <p className="mx-auto mt-2 max-w-md text-xs leading-5 text-muted-foreground">
        {description}
      </p>
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
