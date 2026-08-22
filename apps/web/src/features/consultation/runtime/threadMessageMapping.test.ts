import { describe, expect, it } from "vitest";
import {
  buildActiveTurnSeedFromRuntimeEvents,
  toInitialThreadTimeline,
  toInitialThreadMessage,
} from "./threadMessageMapping";
import type {
  InteractionHistoryItem,
  Message,
  PendingInteraction,
  StreamEvent,
} from "../types/consultation";

function makeMessage(overrides: Partial<Message> = {}): Message {
  return {
    id: "msg-1",
    conversation_id: "conv-1",
    turn_id: "turn-1",
    role: "assistant",
    status: "completed",
    seq: 2,
    parts: [],
    content_text: "",
    model: null,
    provider: null,
    input_tokens: null,
    output_tokens: null,
    total_tokens: null,
    error: null,
    metadata: {},
    created_at: "2026-07-01T00:00:00.000Z",
    updated_at: "2026-07-01T00:00:00.000Z",
    ...overrides,
  };
}

function makeStreamEvent(
  type: StreamEvent["type"],
  payload: StreamEvent["payload"],
  channel: StreamEvent["channel"],
  ids: Partial<StreamEvent["ids"]> = {},
  seq = 1,
): StreamEvent {
  return {
    version: 1,
    seq,
    channel,
    type,
    ids: {
      conversation_id: "conv-1",
      run_id: "run-1",
      turn_id: "turn-1",
      message_id: "msg-streaming",
      tool_call_id: null,
      interaction_id: null,
      ...ids,
    },
    payload,
  } as StreamEvent;
}

describe("threadMessageMapping", () => {
  it("merges canonical tool-call and tool-result parts into one tool-call part", () => {
    const message = makeMessage({
      parts: [
        {
          type: "tool-call",
          toolName: "search_knowledge",
          toolCallId: "tool-1",
          args: { query: "肩部前倾" },
        },
        {
          type: "tool-result",
          toolName: "search_knowledge",
          toolCallId: "tool-1",
          result: { found: 2 },
        },
      ],
    });

    const mapped = toInitialThreadMessage(message);

    expect(mapped.role).toBe("assistant");
    expect(Array.isArray(mapped.content)).toBe(true);
    const content = mapped.content as Array<{
      type: string;
      toolName?: string;
      result?: unknown;
    }>;
    expect(content).toHaveLength(1);
    expect(content[0]).toMatchObject({
      type: "tool-call",
      toolName: "search_knowledge",
      result: { found: 2 },
    });
  });

  it("preserves structured source metadata for historical citations", () => {
    const message = makeMessage({
      parts: [
        {
          type: "source",
          id: "src-1",
          title: "头前伸自测方法",
          url: "https://example.com/guide",
          providerMetadata: {
            bodysense: {
              summary: "判断耳垂与肩峰的相对位置",
            },
          },
        },
      ],
    });

    const mapped = toInitialThreadMessage(message);
    expect(Array.isArray(mapped.content)).toBe(true);
    const content = mapped.content as unknown as ReadonlyArray<{
      type: string;
      providerMetadata?: unknown;
    }>;

    expect(content[0]).toMatchObject({
      type: "source",
      title: "头前伸自测方法",
      url: "https://example.com/guide",
    });
    expect(content[0]?.providerMetadata).toEqual({
      bodysense: {
        summary: "判断耳垂与肩峰的相对位置",
      },
    });
  });

  it("maps aborted assistant messages to incomplete cancelled status", () => {
    const mapped = toInitialThreadMessage(
      makeMessage({
        status: "aborted",
        parts: [{ type: "text", text: "请先补充年龄信息。" }],
      }),
    );

    expect(mapped.status).toEqual({
      type: "incomplete",
      reason: "cancelled",
    });
  });

  it("rebuilds an interrupted active turn from runtime events", () => {
    const pendingInteraction: PendingInteraction = {
      id: "int-1",
      run_id: "run-1",
      conversation_id: "conv-1",
      tool_call_id: "tc-ask",
      tool_name: "ask_user",
      question: {
        question: "你的年龄是多少？",
        answer_type: "number",
        required: true,
      },
      status: "pending",
      created_at: "2026-07-01T00:01:00.000Z",
    };

    const seed = buildActiveTurnSeedFromRuntimeEvents(
      [
        makeStreamEvent(
          "run.started",
          { status: "running", source: "start_turn" },
          "run",
          {},
          1,
        ),
        makeStreamEvent(
          "message.created",
          { role: "assistant", status: "streaming" },
          "message",
          {},
          2,
        ),
        makeStreamEvent(
          "message.text.delta",
          { delta: "为了继续分析，我还需要你的年龄。" },
          "message",
          {},
          3,
        ),
        makeStreamEvent(
          "tool.call",
          { tool: "search_knowledge", args: { query: "头前伸" } },
          "tool",
          { tool_call_id: "tc-1" },
          4,
        ),
        makeStreamEvent(
          "tool.result",
          { tool: "search_knowledge", result: { found: 1 } },
          "tool",
          { tool_call_id: "tc-1" },
          5,
        ),
        makeStreamEvent(
          "state.interaction.required",
          {
            interaction_id: "int-1",
            created_at: pendingInteraction.created_at,
            question: pendingInteraction.question,
          },
          "state",
          { tool_call_id: "tc-ask", interaction_id: "int-1" },
          6,
        ),
        makeStreamEvent(
          "run.interrupted",
          { status: "waiting_user", interaction_id: "int-1" },
          "run",
          { interaction_id: "int-1" },
          7,
        ),
        makeStreamEvent("stream.done", {}, "stream", {}, 8),
      ],
      [pendingInteraction],
    );

    expect(seed).not.toBeNull();
    expect(seed?.consumedMessageId).toBe("msg-streaming");
    expect(seed?.activeTurn.status).toBe("interrupted");
    expect(seed?.activeTurn.text).toContain("还需要你的年龄");
    expect(seed?.activeTurn.pendingInteraction?.id).toBe("int-1");
    expect(seed?.activeTurn.pendingInteraction?.created_at).toBe(
      "2026-07-01T00:01:00.000Z",
    );
    expect(seed?.activeTurn.toolCallsById["tc-1"]).toMatchObject({
      tool: "search_knowledge",
      status: "completed",
    });
  });

  it("includes answered interaction history in the initial timeline", () => {
    const history: InteractionHistoryItem[] = [
      {
        id: "int-answered",
        run_id: "run-1",
        conversation_id: "conv-1",
        tool_call_id: "tc-ask",
        tool_name: "ask_user",
        question: {
          question: "是否感觉到颈部或肩部不适？",
          answer_type: "single_choice",
          options: ["有", "无"],
        },
        status: "answered",
        answer: { text: "无", selected: ["无"] },
        created_at: "2026-07-01T00:01:00.000Z",
        answered_at: "2026-07-01T00:02:00.000Z",
      },
    ];

    const timeline = toInitialThreadTimeline([makeMessage()], history);
    const interactionMessage = timeline.find(
      (message) => message.id === "interaction-int-answered",
    );

    expect(interactionMessage).toBeDefined();
    expect(interactionMessage?.role).toBe("assistant");
    expect(interactionMessage?.metadata).toMatchObject({
      custom: {
        interaction_history: true,
        interaction: {
          id: "int-answered",
          status: "answered",
        },
      },
    });
  });
});
