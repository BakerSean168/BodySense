import {
  Suspense,
  lazy,
  useState,
  useEffect,
  useCallback,
  useMemo,
  useRef,
} from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { ChatPanelSkeleton } from "../components/ChatPanelSkeleton";
import { useConversationsQuery } from "../hooks/useConversationsQuery";
import { useConsultationThreadQuery } from "../hooks/useConsultationThreadQuery";
import { useThreadProjectionActions } from "../hooks/useThreadProjectionActions";
import type { ConsultationSpatialContext } from "../types/consultation";
import {
  buildActiveTurnSeedFromRuntimeEvents,
  toInitialThreadTimeline,
} from "../runtime/threadMessageMapping";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { BodyCanvasWorkspace } from "@/features/body-canvas/components/BodyCanvasWorkspace";
import type { MotionId } from "@/features/body-canvas/model/bodyCanvas";
import { retryDynamicImport } from "@/lib/retryDynamicImport";
import { shouldPromoteProvisionalChatSession } from "../runtime/chatSessionIdentity";
import { ProfileDrawer } from "@/features/profile/components/profile/ProfileDrawer";
import { useHealthWorkspaceQuery } from "@/features/workspace/hooks/useHealthWorkspaceQuery";
import { useBodyExplorerWorkspace } from "@/features/body-explorer";
import {
  resolveWindowFromView,
  type BusinessWindowId,
} from "@/features/body-canvas/model/businessWindows";
import { useProfileStore } from "@/stores/profileStore";
import { useAuthStore } from "@/stores/authStore";

const loadAssistantChatPanel = () =>
  retryDynamicImport(() => import("../components/AssistantChatPanel")).then(
    (module) => ({
      default: module.AssistantChatPanel,
    }),
  );
const AssistantChatPanel = lazy(loadAssistantChatPanel);

export function ConsultationPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const workspaceQuery = useHealthWorkspaceQuery();
  const [chatSpatialContext, setChatSpatialContext] =
    useState<ConsultationSpatialContext | null>(null);
  const [composerFocusKey, setComposerFocusKey] = useState(0);
  // Asking a question from any canvas surface must surface the Chat window; the
  // revision counter keeps repeat requests observable.
  const [windowRequest, setWindowRequest] = useState<{
    window: BusinessWindowId;
    revision: number;
  } | null>(null);

  // Chat is a persistent contextual surface even though V3 is body-first.
  // Discover its lazy chunk early so opening the assistant does not wait on the
  // thread query before beginning the download.
  useEffect(() => {
    void loadAssistantChatPanel().catch(() => undefined);
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
  const bodyState = workspace?.body_state ?? null;
  const handleAskSpatialContext = useCallback(
    (context: ConsultationSpatialContext) => {
      setChatSpatialContext(context);
      setComposerFocusKey((key) => key + 1);
      setWindowRequest((current) => ({
        window: "chat",
        revision: (current?.revision ?? 0) + 1,
      }));
    },
    [],
  );
  const bodyExplorer = useBodyExplorerWorkspace(
    bodyState,
    handleAskSpatialContext,
  );
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

  // Client-only presentation state. A bare consultation route stays body-first.
  // An explicit ?view= deep link opens the matching business window once; the
  // route mirror afterwards is a consequence of the user's window choice, not a
  // second source of truth for which window is open.
  const requestedWindow = searchParams.get("view");
  const initialWindow: BusinessWindowId | null =
    resolveWindowFromView(requestedWindow);
  const demonstrationRequest: { motion: MotionId; revision: number } | null =
    null;
  const [isProfileOpen, setIsProfileOpen] = useState(false);
  const profile = useProfileStore((state) => state.profile);
  const accountName = useAuthStore((state) => state.user?.email ?? "");
  // The identity header shows the real account name so it is never a fabricated
  // label; missing values stay blank rather than being invented.
  const personIdentity = useMemo(
    () => ({
      nickname: accountName,
      birth_date: profile?.birth_date,
      gender: profile?.gender,
    }),
    [accountName, profile?.birth_date, profile?.gender],
  );
  const handleWindowChange = useCallback(
    (window: BusinessWindowId | null) => {
      // Functional update avoids mirroring a stale query string when the route
      // changes in the same batch as the window action.
      setSearchParams(
        (current) => {
          const next = new URLSearchParams(current);
          if (window) next.set("view", window);
          else next.delete("view");
          return next;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

  // Server-state actions are isolated in feature hooks; this page composes them.
  const threadActions = useThreadProjectionActions({
    routeConversationId,
    activeConversationIdRef,
  });
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
                  interactionHistory,
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

  return (
    <div className="h-dvh overflow-hidden text-foreground">
      <BodyCanvasWorkspace
        bodyState={bodyState}
        workspace={workspace}
        bodyExplorer={bodyExplorer}
        chat={chatPanel}
        onAskContext={handleAskSpatialContext}
        onOpenProfile={() => setIsProfileOpen(true)}
        identity={personIdentity}
        initialWindow={initialWindow}
        onWindowChange={handleWindowChange}
        windowRequest={windowRequest}
        demonstrationRequest={demonstrationRequest}
        loading={workspaceQuery.isFetching || isThreadSwitching}
        error={workspaceUnavailable || hasThreadSwitchError}
        onRetry={() => {
          void workspaceQuery.refetch();
          threadActions.retryThread();
        }}
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
