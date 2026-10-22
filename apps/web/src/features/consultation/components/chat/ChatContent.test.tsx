import { useLayoutEffect } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { AssistantRuntimeProvider, useLocalRuntime } from "@assistant-ui/react";
import {
  ActiveTurnProvider,
  useActiveTurnActions,
} from "../../context/ActiveTurnContext";
import { INITIAL_ACTIVE_TURN_STATE } from "../../runtime/activeTurnReducer";
import type { PendingInteraction } from "../../types/consultation";
import { ChatContent } from "./ChatContent";

const pending: PendingInteraction = {
  id: "question-1",
  run_id: "run-1",
  conversation_id: "conversation-1",
  tool_call_id: "call-1",
  tool_name: "ask_user",
  created_at: "2026-10-21T00:00:00Z",
  status: "pending",
  question: { question: "哪里不舒服？", answer_type: "text" },
};

function Question() {
  const { hydrateTurn } = useActiveTurnActions();
  useLayoutEffect(() => {
    hydrateTurn({
      ...INITIAL_ACTIVE_TURN_STATE,
      status: "interrupted",
      pendingInteraction: pending,
    });
  }, [hydrateTurn]);
  return null;
}

function Harness({
  resume,
}: {
  resume: React.ComponentProps<typeof ChatContent>["onResumeInteraction"];
}) {
  const runtime = useLocalRuntime({
    async run() {
      return { content: [] };
    },
  });
  return (
    <ActiveTurnProvider>
      <AssistantRuntimeProvider runtime={runtime}>
        <Question />
        <ChatContent
          conversationId="conversation-1"
          onResumeInteraction={resume}
        />
      </AssistantRuntimeProvider>
    </ActiveTurnProvider>
  );
}

describe("Chat pending answer recovery", () => {
  it("resubmits the failed answer when Retry is clicked", async () => {
    const resume = vi
      .fn()
      .mockRejectedValueOnce(new Error("连接中断"))
      .mockResolvedValue(undefined);
    const user = userEvent.setup();
    render(<Harness resume={resume} />);
    await user.type(
      screen.getByRole("textbox", { name: "哪里不舒服？" }),
      "左肩",
    );
    await user.click(screen.getByRole("button", { name: "提交" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("连接中断");
    await user.click(screen.getByRole("button", { name: "重试" }));
    await waitFor(() => expect(resume).toHaveBeenCalledTimes(2));
    expect(resume.mock.calls[1]).toEqual(resume.mock.calls[0]);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});
