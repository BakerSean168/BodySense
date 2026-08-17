import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { describe, expect, it, vi } from "vitest";
import { SharePage } from "../SharePage";

vi.mock("../../services/consultationService", () => ({
  consultationApi: {
    getSharedConversation: vi.fn().mockResolvedValue({
      title: "肩颈对话",
      messages: [
        {
          id: "message-1",
          conversation_id: "conversation-1",
          turn_id: "turn-1",
          role: "assistant",
          status: "completed",
          seq: 1,
          parts: [{ type: "text", text: "这里是共享的对话内容。" }],
          content_text: "这里是共享的对话内容。",
          model: null,
          provider: null,
          input_tokens: null,
          output_tokens: null,
          total_tokens: null,
          error: null,
          metadata: {},
          created_at: "2026-08-16T00:00:00Z",
          updated_at: "2026-08-16T00:00:00Z",
        },
      ],
      metadata: {
        diagnosis: {
          candidates: [
            { name: "不应公开的候选", confidence: "中", basis: "private" },
          ],
        },
      },
    }),
  },
}));

describe("SharePage", () => {
  it("renders only the conversation snapshot and never health-domain metadata", async () => {
    render(
      <MemoryRouter initialEntries={["/consultation/share/token-1"]}>
        <Routes>
          <Route path="/consultation/share/:token" element={<SharePage />} />
        </Routes>
      </MemoryRouter>,
    );

    expect(
      await screen.findByText("这里是共享的对话内容。"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/不包含 BodyState、Diagnosis 或 Treatment/),
    ).toBeInTheDocument();
    expect(screen.queryByText("不应公开的候选")).not.toBeInTheDocument();
  });
});
