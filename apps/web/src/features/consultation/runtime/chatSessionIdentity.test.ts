import { describe, expect, it } from "vitest";
import { shouldPromoteProvisionalChatSession } from "./chatSessionIdentity";

describe("shouldPromoteProvisionalChatSession", () => {
  it("promotes only after the matching server thread reaches a terminal assistant state", () => {
    expect(
      shouldPromoteProvisionalChatSession({
        chatSessionKey: "new",
        pendingConversationId: "conv-1",
        displayedConversationId: "conv-1",
        activeTurnRunId: null,
        latestAssistantStatus: "failed",
      }),
    ).toBe(true);
  });

  it("keeps the provisional runtime mounted while the server still owns an active run", () => {
    expect(
      shouldPromoteProvisionalChatSession({
        chatSessionKey: "new",
        pendingConversationId: "conv-1",
        displayedConversationId: "conv-1",
        activeTurnRunId: "run-1",
        latestAssistantStatus: "streaming",
      }),
    ).toBe(false);
  });

  it("does not let an older terminal message promote a still-streaming latest assistant", () => {
    expect(
      shouldPromoteProvisionalChatSession({
        chatSessionKey: "new",
        pendingConversationId: "conv-1",
        displayedConversationId: "conv-1",
        activeTurnRunId: null,
        latestAssistantStatus: "streaming",
      }),
    ).toBe(false);
  });

  it("does not promote a different conversation or an already-stable runtime", () => {
    expect(
      shouldPromoteProvisionalChatSession({
        chatSessionKey: "new",
        pendingConversationId: "conv-1",
        displayedConversationId: "conv-2",
        activeTurnRunId: null,
        latestAssistantStatus: "completed",
      }),
    ).toBe(false);
    expect(
      shouldPromoteProvisionalChatSession({
        chatSessionKey: "conversation:conv-1",
        pendingConversationId: "conv-1",
        displayedConversationId: "conv-1",
        activeTurnRunId: null,
        latestAssistantStatus: "completed",
      }),
    ).toBe(false);
  });
});
