import { useState } from "react";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ThreadMessageLike } from "@assistant-ui/react";
import { AssistantChatPanel } from "@/features/consultation/components/AssistantChatPanel";
import { ChatPanelSkeleton } from "@/features/consultation/components/ChatPanelSkeleton";
import { INITIAL_ACTIVE_TURN_STATE } from "@/features/consultation/runtime/activeTurnReducer";
import { BusinessWindowHost } from "./BusinessWindowHost";
import { ChatWindowHeader } from "../../consultation/components/chat/ChatWindowHeader";
import { windowCanvasDecorator } from "./windowStorySupport";

type ChatScenario = "loading" | "empty" | "messages" | "streaming" | "error";

const exampleMessages: ThreadMessageLike[] = [
  {
    role: "user",
    content: [{ type: "text", text: "最近训练之后，我感觉肩部有些紧张。" }],
  },
  {
    role: "assistant",
    content: [
      {
        type: "text",
        text: "我记录了你的描述。可以进一步说明不适发生的时间、位置和持续时间。",
      },
    ],
  },
  {
    role: "user",
    content: [{ type: "text", text: "主要是在俯卧撑之后，没有明显疼痛。" }],
  },
];

function ChatPreview({ scenario }: { scenario: ChatScenario }) {
  const mobile = useMediaQuery("(max-width: 760px)");
  const [closed, setClosed] = useState(false);

  if (closed)
    return (
      <button
        type="button"
        onClick={() => setClosed(false)}
        style={{
          position: "absolute",
          top: 110,
          left: 40,
          padding: 12,
          background: "white",
          borderRadius: 8,
        }}
      >
        重新打开窗口
      </button>
    );

  const active =
    scenario === "streaming"
      ? {
          ...INITIAL_ACTIVE_TURN_STATE,
          status: "streaming" as const,
          runId: "storybook-mock-run",
          text: "我正在根据你提供的描述整理相关信息，并核对仍需补充的细节。",
        }
      : scenario === "error"
        ? {
            ...INITIAL_ACTIVE_TURN_STATE,
            status: "failed" as const,
            runId: "storybook-mock-failed-run",
            error: "这条消息没有处理完成，请稍后重试。",
          }
        : null;

  return (
    <BusinessWindowHost
      windowId="chat"
      mobile={mobile}
      stackIndex={0}
      isActive
      hideTitle
      onClose={() => setClosed(true)}
      headerLeft={<ChatWindowHeader />}
    >
      <div className="bc-chat-content">
        {scenario === "loading" ? (
          <ChatPanelSkeleton />
        ) : (
          <AssistantChatPanel
            key={scenario}
            conversationId="storybook-preview"
            initialMessages={scenario === "empty" ? [] : exampleMessages}
            initialActiveTurn={active}
          />
        )}
      </div>
    </BusinessWindowHost>
  );
}

const meta = {
  title: "BodySense/Windows/04 Chat",
  component: ChatPreview,
  decorators: [windowCanvasDecorator],
  argTypes: {
    scenario: {
      control: "select",
      options: ["loading", "empty", "messages", "streaming", "error"],
    },
  },
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "真实 AssistantChatPanel、Skeleton、消息和 ActiveTurn 展示。使用本地消息及运行状态；API 请求被拦截，不发送诊断或对话数据。",
      },
    },
  },
} satisfies Meta<typeof ChatPreview>;

export default meta;
type Story = StoryObj<typeof meta>;
export const Loading: Story = { args: { scenario: "loading" } };
export const Empty: Story = { args: { scenario: "empty" } };
export const Messages: Story = { args: { scenario: "messages" } };
export const Streaming: Story = { args: { scenario: "streaming" } };
export const ErrorState: Story = { name: "Error", args: { scenario: "error" } };
