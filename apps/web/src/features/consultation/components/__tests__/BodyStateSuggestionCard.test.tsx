import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AskUserCard } from "../AskUserCard";
import { AskUserStatusCard } from "../AskUserStatusCard";
import type {
  AskUserQuestion,
  BodyStateSuggestionReceipt,
  PendingInteraction,
} from "../../types/consultation";

const QUOTE = "我觉得是久坐导致的";

const suggestionQuestion: AskUserQuestion = {
  question: "要把你说过的这句话记录为你自己的猜测吗？",
  answer_type: "single_choice",
  options: ["记录为我的猜测", "不记录"],
  allow_custom_input: false,
  required: true,
  purpose: "body_state_suggestion",
  quote: QUOTE,
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
};

const receipt: BodyStateSuggestionReceipt = {
  kind: "hypothesis",
  commit_id: "commit-1",
  hypothesis_id: "hyp-1",
  source_evidence_id: "ev-1",
  revision: 1,
  event_id: "event-1",
  committed_at: "2026-10-19T02:00:00Z",
};

function interaction(
  overrides: Partial<PendingInteraction> = {},
): PendingInteraction {
  return {
    id: "int-1",
    run_id: "run-1",
    conversation_id: "conv-1",
    tool_call_id: "tc-1",
    tool_name: "ask_user",
    question: suggestionQuestion,
    status: "answered",
    answer: { text: "记录为我的猜测", selected: ["记录为我的猜测"] },
    created_at: "2026-10-19T02:00:00Z",
    ...overrides,
  };
}

describe("BodyStateSuggestionCard via AskUserCard", () => {
  it("shows the user's own words and Go's fixed options", () => {
    render(<AskUserCard question={suggestionQuestion} onSubmit={vi.fn()} />);
    expect(screen.getByTestId("body-state-suggestion-card")).toBeDefined();
    expect(
      screen.getByTestId("body-state-suggestion-quote").textContent,
    ).toContain(QUOTE);
    expect(screen.getByText("记录建议 · 你的猜测")).toBeDefined();
    // No free-text answer can be smuggled into a bound suggestion.
    expect(screen.queryByRole("textbox")).toBeNull();
    expect(screen.queryByText(/已保存为/)).toBeNull();
  });

  it("confirms and declines with Go's exact options", async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    render(<AskUserCard question={suggestionQuestion} onSubmit={onSubmit} />);
    await user.click(screen.getByRole("button", { name: "记录为我的猜测" }));
    await user.click(screen.getByRole("button", { name: "不记录" }));
    expect(onSubmit.mock.calls).toEqual([
      [{ text: "记录为我的猜测", selected: ["记录为我的猜测"] }],
      [{ text: "不记录", selected: ["不记录"] }],
    ]);
  });

  it("offers editing only when the composer can take the words back", async () => {
    const onEdit = vi.fn();
    const user = userEvent.setup();
    const { rerender } = render(
      <AskUserCard question={suggestionQuestion} onSubmit={vi.fn()} />,
    );
    expect(screen.queryByRole("button", { name: "改一改再说" })).toBeNull();
    rerender(
      <AskUserCard
        question={suggestionQuestion}
        onSubmit={vi.fn()}
        onEdit={onEdit}
      />,
    );
    await user.click(screen.getByRole("button", { name: "改一改再说" }));
    expect(onEdit).toHaveBeenCalledWith(QUOTE);
  });

  it("falls back to the ordinary card when Go did not bind the suggestion", () => {
    render(
      <AskUserCard
        question={{ ...suggestionQuestion, state_binding: undefined }}
        onSubmit={vi.fn()}
      />,
    );
    expect(screen.queryByTestId("body-state-suggestion-card")).toBeNull();
  });

  it("disables every answer while submitting", () => {
    render(
      <AskUserCard
        question={suggestionQuestion}
        onSubmit={vi.fn()}
        onEdit={vi.fn()}
        isSubmitting
      />,
    );
    for (const button of screen.getAllByRole("button")) {
      expect((button as HTMLButtonElement).disabled).toBe(true);
    }
  });
});

describe("BodyStateSuggestionStatus via AskUserStatusCard", () => {
  it("says saved only with Go's live receipt", () => {
    render(
      <AskUserStatusCard
        interaction={interaction({ body_state_receipt: receipt })}
      />,
    );
    expect(screen.getByTestId("body-state-suggestion-saved")).toBeDefined();
    expect(screen.getByText("已保存为你的猜测")).toBeDefined();
  });

  it("says saved from the receipt kept in interaction history", () => {
    render(
      <AskUserStatusCard
        interaction={interaction({ metadata: { body_state_receipt: receipt } })}
      />,
    );
    expect(screen.getByTestId("body-state-suggestion-saved")).toBeDefined();
  });

  it("never claims saved for a confirm answer without a receipt", () => {
    render(<AskUserStatusCard interaction={interaction()} />);
    expect(screen.getByTestId("body-state-suggestion-answered")).toBeDefined();
    expect(screen.queryByText(/已保存为/)).toBeNull();
  });

  it("shows a decline as nothing written", () => {
    render(
      <AskUserStatusCard
        interaction={interaction({
          answer: { text: "不记录", selected: ["不记录"] },
          body_state_receipt: null,
        })}
      />,
    );
    expect(screen.getByTestId("body-state-suggestion-declined")).toBeDefined();
    expect(screen.getByText("这句话没有写入你的身体记录。")).toBeDefined();
  });

  it("shows an expired suggestion as not saved", () => {
    render(
      <AskUserStatusCard
        interaction={interaction({ status: "expired", answer: undefined })}
      />,
    );
    expect(screen.getByTestId("body-state-suggestion-expired")).toBeDefined();
  });
});
