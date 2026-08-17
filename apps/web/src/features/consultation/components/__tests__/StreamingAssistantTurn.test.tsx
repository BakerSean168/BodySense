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
    expect(screen.getByText("参考知识")).toBeDefined();
    expect(screen.getByText("Posture Guide")).toBeDefined();
  });

  it("shows loading dots when running but no content", () => {
    const { container } = renderWithState(
      stateWith({ status: "streaming", text: "", toolCallsById: {} }),
    );
    const dots = container.querySelector(".animate-bounce");
    expect(dots).not.toBeNull();
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
    expect(screen.getByText("问诊追问")).toBeDefined();
    expect(screen.getByText("你的回答：无")).toBeDefined();
  });
});
