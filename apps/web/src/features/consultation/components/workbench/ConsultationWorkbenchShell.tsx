import type { ReactNode } from "react";
import { useCallback, useEffect } from "react";
import {
  Activity,
  ChartNoAxesCombined,
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
import { AppUserMenu } from "@/components/layout/AppUserMenu";
import { Button } from "@/components/ui/Button";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { cn } from "@/lib/utils";
import type { WorkspaceView } from "../../model/workbenchView";
import { useWorkbenchPreferencesStore } from "../../model/workbenchPreferencesStore";

interface ConsultationWorkbenchShellProps {
  title: string;
  workspaceView: WorkspaceView;
  onWorkspaceViewChange: (view: WorkspaceView) => void;
  onOpenProfile: () => void;
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
  workspaceView,
  onWorkspaceViewChange,
  onOpenProfile,
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
    <div className="bodysense-workbench-theme flex h-full min-h-0 flex-col bg-[#242624] text-foreground">
      <header className="relative flex h-12 shrink-0 items-center gap-2 bg-[#242624] px-2 sm:px-3">
        <div className="flex shrink-0 items-center gap-1">
          <Button
            type="button"
            size="icon"
            variant="ghost"
            aria-label={chatVisible ? "收起对话区" : "展开对话区"}
            title={`${chatVisible ? "收起" : "展开"}对话区（⌘/Ctrl+B）`}
            onClick={handleChatToggle}
            className="text-muted-foreground hover:text-foreground"
          >
            {chatVisible ? <PanelLeftClose /> : <PanelLeftOpen />}
          </Button>
          <h1 className="sr-only">{title}</h1>
        </div>

        <nav
          aria-label="健康工作区"
          role="tablist"
          className="absolute left-1/2 flex -translate-x-1/2 items-stretch gap-1"
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
                  "relative inline-flex h-10 shrink-0 items-center gap-1.5 px-3 text-[13px] font-medium outline-none transition-colors duration-150 after:absolute after:inset-x-2.5 after:bottom-0 after:h-0.5 after:origin-center after:rounded-full after:bg-primary after:transition-transform after:duration-200 focus-visible:ring-2 focus-visible:ring-ring",
                  active
                    ? "text-foreground after:scale-x-100"
                    : "text-muted-foreground after:scale-x-0 hover:text-foreground",
                )}
              >
                <Icon className="size-3.5" aria-hidden="true" />
                <span>{tab.label}</span>
              </button>
            );
          })}
        </nav>

        <div className="ml-auto flex shrink-0 items-center gap-1">
          <AppUserMenu compact onOpenProfile={onOpenProfile} />
        </div>
      </header>

      {!isDesktop ? (
        <div className="flex min-h-0 flex-1 flex-col px-2 pb-2">
          <div className="mb-2 grid grid-cols-2 rounded-xl bg-muted/45 p-1">
            <button
              type="button"
              onClick={() => setMobileSurface("chat")}
              className={cn(
                "inline-flex h-8 items-center justify-center gap-2 rounded-lg text-xs font-medium transition-colors",
                mobileSurface === "chat"
                  ? "bg-card text-foreground shadow-sm"
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
                "inline-flex h-8 items-center justify-center gap-2 rounded-lg text-xs font-medium transition-colors",
                mobileSurface === "workspace"
                  ? "bg-card text-foreground shadow-sm"
                  : "text-muted-foreground",
              )}
            >
              <Activity className="size-4" aria-hidden="true" />
              工作区
            </button>
          </div>
          <div className="relative min-h-0 flex-1 overflow-hidden rounded-[22px] border border-border/70 bg-card shadow-[0_14px_36px_rgba(0,0,0,0.16)]">
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
            className="min-h-0 transition-[flex-grow] duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] motion-reduce:transition-none"
          >
            <div
              className={cn(
                "h-full min-h-0 overflow-hidden rounded-none rounded-tr-[26px] bg-[#171717] transition-[opacity,transform] duration-200 ease-out motion-reduce:transition-none",
                chatOpen ? "opacity-100" : "translate-x-[-6px] opacity-0",
              )}
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
              "group relative z-20 w-1.5 outline-none transition-[width] duration-200 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset",
              "after:absolute after:inset-y-5 after:left-1/2 after:w-px after:-translate-x-1/2 after:bg-white/10 after:opacity-0 after:transition-opacity hover:after:opacity-100 data-[separator=active]:after:bg-primary data-[separator=active]:after:opacity-100",
              !chatOpen && "w-0 overflow-hidden",
            )}
          />

          <Panel
            id="workspace"
            minSize="48%"
            className="min-h-0 min-w-0 transition-[flex-grow] duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] motion-reduce:transition-none"
          >
            <div className="bodysense-workspace-surface h-full min-h-0 overflow-hidden bg-background">
              {workspace}
            </div>
          </Panel>
        </Group>
      )}
    </div>
  );
}
