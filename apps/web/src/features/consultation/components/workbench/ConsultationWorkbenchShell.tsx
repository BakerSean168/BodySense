import type { ReactNode } from "react";
import { useCallback, useEffect } from "react";
import {
  Activity,
  ChartNoAxesCombined,
  Clock3,
  MessagesSquare,
  PanelLeftClose,
  PanelLeftOpen,
  Stethoscope,
  Target,
} from "lucide-react";
import {
  Group,
  Panel,
  Separator,
  usePanelRef,
  type Layout,
  type LayoutChangedMeta,
} from "react-resizable-panels";
import { AppBrand } from "@/components/layout/AppBrand";
import { AppUserMenu } from "@/components/layout/AppUserMenu";
import { Button } from "@/components/ui/Button";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { cn } from "@/lib/utils";
import type { WorkspaceView } from "../../model/workbenchView";
import { useWorkbenchPreferencesStore } from "../../model/workbenchPreferencesStore";

interface ConsultationWorkbenchShellProps {
  title: string;
  phaseLabel: string;
  bodyStateRevision: number;
  bodyStateItemCount: number;
  workspaceView: WorkspaceView;
  onWorkspaceViewChange: (view: WorkspaceView) => void;
  onOpenHistory: () => void;
  chat: ReactNode;
  workspace: ReactNode;
}

const workspaceTabs: Array<{
  view: WorkspaceView;
  label: string;
  icon: typeof Activity;
}> = [
  { view: "state", label: "状态", icon: Activity },
  { view: "diagnosis", label: "分析", icon: Stethoscope },
  { view: "treatment", label: "方案", icon: Target },
  { view: "progress", label: "进展", icon: ChartNoAxesCombined },
];

export function ConsultationWorkbenchShell({
  title,
  phaseLabel,
  bodyStateRevision,
  bodyStateItemCount,
  workspaceView,
  onWorkspaceViewChange,
  onOpenHistory,
  chat,
  workspace,
}: ConsultationWorkbenchShellProps) {
  const isDesktop = useMediaQuery("(min-width: 768px)", true);
  const chatPanelRef = usePanelRef();
  const chatOpen = useWorkbenchPreferencesStore((state) => state.chatOpen);
  const chatSize = useWorkbenchPreferencesStore((state) => state.chatSize);
  const mobileSurface = useWorkbenchPreferencesStore(
    (state) => state.mobileSurface,
  );
  const setChatOpen = useWorkbenchPreferencesStore(
    (state) => state.setChatOpen,
  );
  const toggleChat = useWorkbenchPreferencesStore((state) => state.toggleChat);
  const setChatSize = useWorkbenchPreferencesStore(
    (state) => state.setChatSize,
  );
  const setMobileSurface = useWorkbenchPreferencesStore(
    (state) => state.setMobileSurface,
  );

  useEffect(() => {
    if (!isDesktop) return;
    if (chatOpen) chatPanelRef.current?.expand();
    else chatPanelRef.current?.collapse();
  }, [chatOpen, chatPanelRef, isDesktop]);

  const handleChatToggle = useCallback(() => {
    if (isDesktop) {
      toggleChat();
      return;
    }
    setMobileSurface(mobileSurface === "chat" ? "workspace" : "chat");
  }, [isDesktop, mobileSurface, setMobileSurface, toggleChat]);

  useEffect(() => {
    const handleKeyboard = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "b") {
        event.preventDefault();
        handleChatToggle();
      }
    };
    window.addEventListener("keydown", handleKeyboard);
    return () => window.removeEventListener("keydown", handleKeyboard);
  }, [handleChatToggle]);

  const handleLayoutChanged = useCallback(
    (layout: Layout, meta: LayoutChangedMeta) => {
      if (!meta.isUserInteraction) return;
      const nextSize = layout.chat ?? 0;
      const nextOpen = nextSize > 0.5;
      setChatOpen(nextOpen);
      if (nextOpen) setChatSize(nextSize);
    },
    [setChatOpen, setChatSize],
  );

  const handleWorkspaceViewChange = (view: WorkspaceView) => {
    onWorkspaceViewChange(view);
    if (!isDesktop) setMobileSurface("workspace");
  };

  const chatVisible = isDesktop ? chatOpen : mobileSurface === "chat";

  return (
    <div className="flex h-full min-h-0 flex-col bg-background">
      <header className="flex h-14 shrink-0 items-center gap-2 border-b border-border bg-background/95 px-2 backdrop-blur-xl sm:px-3">
        <div className="flex shrink-0 items-center gap-1">
          <AppBrand compact className="mr-1 hidden sm:inline-flex" />
          <Button
            type="button"
            size="icon"
            variant="ghost"
            aria-label={chatVisible ? "收起对话区" : "展开对话区"}
            title={`${chatVisible ? "收起" : "展开"}对话区（⌘/Ctrl+B）`}
            onClick={handleChatToggle}
          >
            {chatVisible ? <PanelLeftClose /> : <PanelLeftOpen />}
          </Button>
          <Button
            type="button"
            size="icon"
            variant="ghost"
            aria-label="打开咨询历史"
            title="咨询历史"
            onClick={onOpenHistory}
          >
            <Clock3 />
          </Button>
        </div>

        <div className="min-w-0 flex-1">
          <h1 className="sr-only truncate text-xs font-semibold text-foreground lg:not-sr-only lg:block">
            {title}
          </h1>
          <p className="hidden truncate text-[10px] text-muted-foreground lg:block">
            {phaseLabel}
          </p>
        </div>

        <nav
          aria-label="健康工作区"
          role="tablist"
          className="mx-auto flex min-w-0 items-center gap-1 overflow-x-auto rounded-xl border border-border bg-muted/55 p-1"
        >
          {workspaceTabs.map((tab) => {
            const active = workspaceView === tab.view;
            const Icon = tab.icon;
            return (
              <button
                key={tab.view}
                id={`workspace-tab-${tab.view}`}
                type="button"
                role="tab"
                aria-selected={active}
                aria-controls={`workspace-panel-${tab.view}`}
                onClick={() => handleWorkspaceViewChange(tab.view)}
                className={cn(
                  "inline-flex h-8 shrink-0 items-center gap-1.5 rounded-lg px-2.5 text-xs font-medium outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring",
                  active
                    ? "bg-background text-foreground shadow-sm"
                    : "text-muted-foreground hover:bg-background/70 hover:text-foreground",
                )}
              >
                <Icon className="size-3.5" aria-hidden="true" />
                <span>{tab.label}</span>
                {tab.view === "state" && bodyStateItemCount > 0 ? (
                  <span className="rounded-full bg-primary/10 px-1.5 text-[10px] text-primary">
                    {bodyStateItemCount}
                  </span>
                ) : null}
              </button>
            );
          })}
        </nav>

        <div className="ml-auto flex shrink-0 items-center gap-1">
          <span className="hidden rounded-full border border-border bg-muted/45 px-2.5 py-1 text-[10px] font-medium text-muted-foreground sm:inline-flex">
            R{bodyStateRevision}
          </span>
          <AppUserMenu compact />
        </div>
      </header>

      {!isDesktop ? (
        <div className="flex min-h-0 flex-1 flex-col">
          <div className="grid grid-cols-2 border-b border-border bg-background p-1.5">
            <button
              type="button"
              onClick={() => setMobileSurface("chat")}
              className={cn(
                "inline-flex h-9 items-center justify-center gap-2 rounded-lg text-xs font-medium",
                mobileSurface === "chat"
                  ? "bg-muted text-foreground"
                  : "text-muted-foreground",
              )}
            >
              <MessagesSquare className="size-4" aria-hidden="true" />
              对话
            </button>
            <button
              type="button"
              onClick={() => setMobileSurface("workspace")}
              className={cn(
                "inline-flex h-9 items-center justify-center gap-2 rounded-lg text-xs font-medium",
                mobileSurface === "workspace"
                  ? "bg-muted text-foreground"
                  : "text-muted-foreground",
              )}
            >
              <Activity className="size-4" aria-hidden="true" />
              工作区
            </button>
          </div>
          <div className="relative min-h-0 flex-1">
            <div
              className="absolute inset-0 min-h-0"
              hidden={mobileSurface !== "chat"}
            >
              {chat}
            </div>
            <div
              className="absolute inset-0 min-h-0"
              hidden={mobileSurface !== "workspace"}
            >
              {workspace}
            </div>
          </div>
        </div>
      ) : (
        <Group
          id="consultation-workbench"
          orientation="horizontal"
          defaultLayout={{
            chat: chatOpen ? chatSize : 0,
            workspace: chatOpen ? 100 - chatSize : 100,
          }}
          onLayoutChanged={handleLayoutChanged}
          className="min-h-0 flex-1"
        >
          <Panel
            id="chat"
            panelRef={chatPanelRef}
            collapsible
            collapsedSize="0%"
            defaultSize={`${chatSize}%`}
            minSize="360px"
            maxSize="52%"
            groupResizeBehavior="preserve-relative-size"
            className="min-h-0 bg-background"
          >
            <div
              className="h-full min-h-0"
              aria-hidden={!chatOpen}
              inert={!chatOpen ? true : undefined}
            >
              {chat}
            </div>
          </Panel>
          <Separator
            id="chat-workspace-separator"
            disabled={!chatOpen}
            aria-label="调整对话区与健康工作区宽度"
            className={cn(
              "group relative z-20 w-3 bg-background outline-none transition-[width] focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset",
              "after:absolute after:inset-y-0 after:left-1/2 after:w-px after:-translate-x-1/2 after:bg-border after:transition-colors hover:after:bg-primary data-[separator=active]:after:bg-primary",
              !chatOpen && "w-0 overflow-hidden",
            )}
          />
          <Panel
            id="workspace"
            minSize="48%"
            className="min-h-0 min-w-0 bg-background"
          >
            {workspace}
          </Panel>
        </Group>
      )}
    </div>
  );
}
