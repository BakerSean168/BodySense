/**
 * Tests for StreamingAssistantTurn — streaming UI component rendering.
 */

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import React, { useEffect } from "react";
import { StreamingAssistantTurn } from "../StreamingAssistantTurn";
import {
  ActiveTurnProvider,
  useActiveTurnActions,
} from "../../context/ActiveTurnContext";
import { INITIAL_ACTIVE_TURN_STATE } from "../../runtime/activeTurnReducer";
import type { ActiveTurnState } from "../../runtime/activeTurnReducer";

// ---------------------------------------------------------------------------
// Wrapper
// ---------------------------------------------------------------------------

/** Hydrate the ActiveTurnContext with a given state on mount. */
function Hydrator({
  state,
  children,
}: {
  state: ActiveTurnState;
  children: React.ReactNode;
}) {
  const { hydrateTurn } = useActiveTurnActions();
  useEffect(() => {
    hydrateTurn(state);
  }, []);
  return <>{children}</>;
}

function renderWithState(state: ActiveTurnState) {
  return render(
    <ActiveTurnProvider>
      <Hydrator state={state}>
        <StreamingAssistantTurn />
      </Hydrator>
    </ActiveTurnProvider>,
  );
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function stateWith(overrides: Partial<ActiveTurnState> = {}): ActiveTurnState {
  return { ...INITIAL_ACTIVE_TURN_STATE, ...overrides };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("StreamingAssistantTurn", () => {
  it("renders nothing when state is empty", () => {
    const { container } = renderWithState(INITIAL_ACTIVE_TURN_STATE);
    expect(container.firstChild).toBeNull();
  });

  it("renders streaming markdown text", () => {
    renderWithState(
      stateWith({ text: "**Hello** _world_", status: "streaming" }),
    );
    expect(screen.getByText("Hello")).toBeDefined();
    expect(screen.getByText("world")).toBeDefined();
  });

  it("renders tool calls", () => {
    renderWithState(
      stateWith({
        status: "streaming",
        toolCallsById: {
          "tc-1": {
            id: "tc-1",
            tool: "search_knowledge",
            args: { query: "back pain" },
            status: "running",
          },
        },
      }),
    );
    expect(screen.getByText("搜索知识库")).toBeDefined();
    expect(screen.getByText(/back pain/)).toBeDefined();
  });

  it("renders citations when present", () => {
    renderWithState(
      stateWith({
        status: "streaming",
        citationsByKey: {
          "Posture Guide": { title: "Posture Guide", summary: "Good habits" },
        },
      }),
    );
    expect(screen.getByText(/参考知识/)).toBeDefined();
    expect(screen.getByText("Posture Guide")).toBeDefined();
  });

  it("shows a subtle thinking indicator when running but no content", () => {
    renderWithState(
      stateWith({ status: "streaming", text: "", toolCallsById: {} }),
    );
    expect(screen.getByLabelText("BodySense 正在思考")).toBeInTheDocument();
  });

  it("renders ask_user status card while interrupted", () => {
    renderWithState(
      stateWith({
        status: "interrupted",
        pendingInteraction: {
          id: "ia-1",
          run_id: "run-1",
          conversation_id: "conv-1",
          tool_call_id: "tc-1",
          tool_name: "ask_user",
          question: {
            question: "是否感觉到颈部或肩部不适？",
            context:
              "为了判断更偏向姿态问题还是已经伴随代偿，需要先确认这一点。",
            answer_type: "single_choice",
            options: ["有", "无", "不确定"],
          },
          status: "pending",
          created_at: "2026-07-06T00:00:00Z",
        },
      }),
    );
    expect(screen.getByText("待补充信息")).toBeDefined();
    expect(screen.getByText("是否感觉到颈部或肩部不适？")).toBeDefined();
  });

  it("renders answered ask_user summary while resume is running", () => {
    renderWithState(
      stateWith({
        status: "streaming",
        pendingInteraction: {
          id: "ia-1",
          run_id: "run-1",
          conversation_id: "conv-1",
          tool_call_id: "tc-1",
          tool_name: "ask_user",
          question: {
            question: "是否感觉到颈部或肩部不适？",
            answer_type: "single_choice",
            options: ["有", "无", "不确定"],
          },
          status: "answered",
          answer: { text: "无", selected: ["无"] },
          created_at: "2026-07-06T00:00:00Z",
        },
      }),
    );
    const card = screen.getByRole("region", { name: "问诊追问（已回答）" });
    expect(card.textContent).toContain("是否感觉到颈部或肩部不适？");
    expect(card.textContent).toContain("你的回答：无");
  });

  it("keeps the answered open question above 继续分析 after the Run resumes", () => {
    renderWithState(
      stateWith({
        status: "streaming",
        pendingInteraction: null,
        answeredInteractions: [
          {
            id: "ia-1",
            run_id: "run-1",
            conversation_id: "conv-1",
            tool_call_id: "tc-1",
            tool_name: "ask_user",
            question: {
              question: "这次和上次是一直持续，还是结束后又出现？",
              answer_type: "single_choice",
              options: ["一直持续到现在", "曾经结束，之后再次出现", "暂不确定"],
            },
            status: "answered",
            answer: {
              text: "曾经结束，之后再次出现",
              selected: ["曾经结束，之后再次出现"],
            },
            created_at: "Mon 2026-10-20 9:00 AM CST (UTC+08:00)",
          },
        ],
      }),
    );
    const card = screen.getByRole("region", { name: "问诊追问（已回答）" });
    const pill = screen.getByRole("status", { name: "BodySense 正在思考" });
    expect(card.textContent).toContain("曾经结束，之后再次出现");
    expect(pill.textContent).toContain("继续分析");
    expect(
      card.compareDocumentPosition(pill) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("keeps an answered BodyState suggestion visible after the Run resumes", () => {
    renderWithState(
      stateWith({
        status: "streaming",
        text: "好的，继续。",
        pendingInteraction: null,
        answeredInteractions: [
          {
            id: "int-s",
            run_id: "run-1",
            conversation_id: "conv-1",
            tool_call_id: "tc-1",
            tool_name: "ask_user",
            status: "answered",
            answer: { text: "记录为我的猜测", selected: ["记录为我的猜测"] },
            created_at: "2026-10-19T02:00:00Z",
            question: {
              question: "要把你说过的这句话记录为你自己的猜测吗？",
              answer_type: "single_choice",
              options: ["记录为我的猜测", "不记录"],
              purpose: "body_state_suggestion",
              quote: "我觉得是久坐导致的",
              state_binding: {
                revision: "body-state-suggestion-v1",
                kind: "hypothesis",
                source: {
                  message_id: "msg-user-1",
                  message_seq: 1,
                  span_start: 0,
                  span_end: 9,
                  excerpt_sha256: "a".repeat(64),
                },
                base_body_state_revision: 0,
              },
            },
            body_state_receipt: {
              kind: "hypothesis",
              commit_id: "commit-1",
              hypothesis_id: "hyp-1",
              source_evidence_id: "ev-1",
              revision: 1,
              event_id: "event-1",
              committed_at: "2026-10-19T02:00:00Z",
            },
          },
        ],
      }),
    );
    expect(screen.getByTestId("body-state-suggestion-saved")).toBeDefined();
    expect(screen.getByText("已保存为你的猜测")).toBeDefined();
  });
});
