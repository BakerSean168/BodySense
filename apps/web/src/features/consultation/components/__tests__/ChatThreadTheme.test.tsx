import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { CancelledRunStatusCard } from "../CancelledRunStatusCard";
import { FailedRunStatusCard } from "../FailedRunStatusCard";
import { ToolCallItem } from "../ToolCallItem";
import { Message, MessageContent } from "@/components/ai-elements/message";

// UI-001: the Body Canvas restyles the chat thread through semantic chat-*
// hooks (light Penpot thread). Losing a hook silently falls back to the dark
// standalone utilities, so the hooks themselves are part of the contract.
describe("chat thread theme hooks", () => {
  it("marks message bubbles and status cards with their semantic tone", () => {
    render(
      <>
        <Message from="user">
          <MessageContent>我每晚睡七小时。</MessageContent>
        </Message>
        <CancelledRunStatusCard />
        <FailedRunStatusCard />
      </>,
    );
    const bubble = screen.getByText("我每晚睡七小时。");
    expect(bubble).toHaveClass("chat-msg-content");
    expect(bubble.parentElement).toHaveAttribute("data-role", "user");
    expect(
      screen.getByText("本次处理已停止").closest(".chat-status-card"),
    ).toHaveAttribute("data-tone", "neutral");
    expect(
      screen.getByText("本次处理已安全停止").closest(".chat-status-card"),
    ).toHaveAttribute("data-tone", "warning");
  });

  it("only marks the BodyFact receipt as saved when a receipt exists", () => {
    render(
      <ToolCallItem
        toolCall={{
          id: "t1",
          tool: "record_body_state",
          args: {},
          status: "completed",
          result: { status: "rejected" },
        }}
      />,
    );
    expect(screen.getByTestId("body-fact-tool-status")).toHaveAttribute(
      "data-saved",
      "false",
    );
  });
});
