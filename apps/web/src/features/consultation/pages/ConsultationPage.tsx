import { useBodyRecordsQuery } from "@/features/profile/hooks/useBodyRecordsQuery";
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
import { useConversationQuery } from "../hooks/useConversationQuery";
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
import { useHealthWorkspaceQuery } from "@/features/workspace/hooks/useHealthWorkspaceQuery";
import { useBodyExplorerWorkspace } from "@/features/body-explorer";
import {
  resolveWindowFromView,
  type BusinessWindowId,
} from "@/features/body-canvas/model/businessWindows";
import { useProfileStore } from "@/stores/profileStore";
import { useAuthStore } from "@/stores/authStore";
import { effectiveTimeZone } from "@/features/profile/model/profilePreferences";
import { groupChatHistory } from "@/features/body-canvas/model/chatHistory";

const UUID_PATTERN =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

const loadAssistantChatPanel = () =>
  retryDynamicImport(() => import("../components/AssistantChatPanel")).then(
    (module) => ({
      default: module.AssistantChatPanel,
    }),
  );
const AssistantChatPanel = lazy(loadAssistantChatPanel);

// The 身体档案 drawer (files, source removal, erasure) opens on demand, so it
// is fetched on first open and stays out of the consultation chunk.
const ProfileDrawer = lazy(() =>
  retryDynamicImport(
    () => import("@/features/profile/components/profile/ProfileDrawer"),
  ).then((module) => ({ default: module.ProfileDrawer })),
);

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
  const conversationQuery = useConversationQuery();
  const singletonConversationId =
    conversationQuery.data?.conversationId ?? null;
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
  const bodyRecordsQuery = useBodyRecordsQuery();
  const bodyState = bodyRecordsQuery.isError
    ? null
    : (bodyRecordsQuery.data ?? null);
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

  // Every user has exactly one Conversation (PC-D4). `/consultation`, `/new`
  // and any stale or foreign id all resolve to that single thread, so a second
  // tab or device always lands on the same conversation.
  useEffect(() => {
    if (
      singletonConversationId &&
      routeConversationId !== singletonConversationId
    ) {
      navigateToConversation(singletonConversationId);
    }
  }, [routeConversationId, singletonConversationId, navigateToConversation]);

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
  // Mounted after the first open and kept, so closing still animates out.
  const [profileDrawerMounted, setProfileDrawerMounted] = useState(false);
  const profile = useProfileStore((state) => state.profile);
  const accountName = useAuthStore((state) => state.user?.email ?? "");
  // Penpot Chat header 今天 picker + history list: days the user wrote in,
  // on their display clock, straight from the thread's own messages.
  const chatTimeZone = effectiveTimeZone(profile?.preferences).zone;
  const chatHistory = useMemo(
    () => groupChatHistory(messages, chatTimeZone),
    [messages, chatTimeZone],
  );
  // The identity header shows the Profile nickname, else the account email's
  // local part (personDisplayName), so it is never a fabricated label.
  const personIdentity = useMemo(
    () => ({
      nickname: profile?.display_name ?? "",
      accountEmail: accountName,
    }),
    [accountName, profile?.display_name],
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
  // Penpot Chat / Diagnosis request: only the user's own latest completed,
  // persisted message can become a formal analysis request (never AI text,
  // never a structured Ask User answer).
  const latestUserMessage = useMemo(() => {
    for (let index = messages.length - 1; index >= 0; index -= 1) {
      const message = messages[index]!;
      if (message.role !== "user") continue;
      const metadata = message.metadata as
        Record<string, unknown> | null | undefined;
      if (metadata?.is_interaction_answer === true) continue;
      if (message.status !== "completed") return null;
      const text = (message.parts as Array<{ type?: string; text?: unknown }>)
        .flatMap((part) =>
          part.type === "text" &&
          typeof part.text === "string" &&
          part.text.trim()
            ? [part.text.trim()]
            : [],
        )
        .join("\n");
      return UUID_PATTERN.test(message.id) && text
        ? { id: message.id, text }
        : null;
    }
    return null;
  }, [messages]);
  const historicalMessages = initialTurnSeed?.consumedMessageId
    ? messages.filter(
        (message) => message.id !== initialTurnSeed.consumedMessageId,
      )
    : messages;

  const chatPanel = (
    <div className="chat-thread h-full min-h-0 bg-[#171717]">
      <div className="chat-thread flex h-full min-h-0 flex-col overflow-hidden bg-[#171717]">
        {hasThreadError || hasThreadSwitchError ? (
          <div className="flex h-full items-center justify-center p-6">
            <Card className="chat-status-card max-w-md border border-white/[0.08] bg-white/[0.035] p-8 text-center text-[#ececec] shadow-none">
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
              <p className="chat-status-question mb-2 text-lg font-semibold text-white/90">
                暂时无法加载对话
              </p>
              <p className="chat-muted mb-6 text-sm text-white/45">
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
        latestUserMessage={latestUserMessage}
        chatHistory={chatHistory}
        onAskContext={handleAskSpatialContext}
        onOpenProfile={() => {
          setProfileDrawerMounted(true);
          setIsProfileOpen(true);
        }}
        identity={personIdentity}
        initialWindow={initialWindow}
        onWindowChange={handleWindowChange}
        windowRequest={windowRequest}
        demonstrationRequest={demonstrationRequest}
        loading={
          workspaceQuery.isFetching ||
          bodyRecordsQuery.isFetching ||
          isThreadSwitching
        }
        error={
          workspaceUnavailable ||
          bodyRecordsQuery.isError ||
          hasThreadSwitchError
        }
        onRetry={() => {
          void workspaceQuery.refetch();
          void bodyRecordsQuery.refetch();
          threadActions.retryThread();
        }}
      />
      {profileDrawerMounted ? (
        <Suspense fallback={null}>
          <ProfileDrawer open={isProfileOpen} onOpenChange={setIsProfileOpen} />
        </Suspense>
      ) : null}
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
