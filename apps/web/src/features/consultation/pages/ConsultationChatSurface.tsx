import { lazy, Suspense, type ReactNode } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { retryDynamicImport } from "@/lib/retryDynamicImport";
import { ChatPanelSkeleton } from "../components/ChatPanelSkeleton";

export const loadAssistantChatPanel = () =>
  retryDynamicImport(() => import("../components/AssistantChatPanel")).then(
    (module) => ({
      default: module.AssistantChatPanel,
    }),
  );
export const AssistantChatPanel = lazy(loadAssistantChatPanel);

export function ConsultationChatSurface({
  error,
  loading,
  switching,
  panel,
  onRetry,
}: {
  error: boolean;
  loading: boolean;
  switching: boolean;
  panel: ReactNode;
  onRetry: () => void;
}) {
  return (
    <div className="chat-thread h-full min-h-0 bg-[#171717]">
      <div className="chat-thread flex h-full min-h-0 flex-col overflow-hidden bg-[#171717]">
        {error ? (
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
              <Button onClick={onRetry} className="w-full">
                重新加载
              </Button>
            </Card>
          </div>
        ) : loading ? (
          <ChatPanelSkeleton />
        ) : (
          <div className="relative flex h-full min-h-0 flex-col">
            <Suspense fallback={<ChatPanelSkeleton />}>{panel}</Suspense>
            {switching ? (
              <PanelTransitionOverlay label="身体信息正在更新" />
            ) : null}
          </div>
        )}
      </div>
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
