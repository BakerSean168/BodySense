import { describe, expect, it } from "vitest";
import { chatDayLabel, groupChatHistory } from "./chatHistory";

const msg = (
  id: string,
  created_at: string,
  text: string,
  extra: Record<string, unknown> = {},
) => ({ id, role: "user", created_at, content_text: text, ...extra });

describe("groupChatHistory", () => {
  const now = new Date("2026-10-20T02:00:00Z"); // 10:00 on 10-20 in Shanghai

  it("groups the user's own messages by day on the display clock", () => {
    const history = groupChatHistory(
      [
        msg("a", "2026-10-14T01:00:00Z", "昨天有头痛，和之前那次有关系吗？"),
        msg("b", "2026-10-14T03:00:00Z", "还有一点头晕"),
        // 23:30 UTC on 10-19 is already 10-20 in Shanghai.
        msg("c", "2026-10-19T23:30:00Z", "今天好多了"),
        { ...msg("d", "2026-10-19T12:00:00Z", "回答"), role: "assistant" },
        msg("e", "2026-10-19T12:00:00Z", "曾经结束", {
          metadata: { is_interaction_answer: true },
        }),
      ],
      "Asia/Shanghai",
      now,
    );
    expect(history.todayKey).toBe("2026-10-20");
    expect(history.days).toEqual([
      {
        day: "2026-10-20",
        label: "今天",
        count: 1,
        preview: "今天好多了",
        firstMessageId: "c",
      },
      {
        day: "2026-10-14",
        label: "10月14日",
        count: 2,
        preview: "昨天有头痛，和之前那次有关系吗？",
        firstMessageId: "a",
      },
    ]);
  });

  it("returns no days for an empty conversation", () => {
    expect(groupChatHistory([], "Asia/Shanghai", now).days).toEqual([]);
  });

  it("labels yesterday and other years", () => {
    expect(chatDayLabel("2026-10-19", "2026-10-20")).toBe("昨天");
    expect(chatDayLabel("2025-10-19", "2026-10-20")).toBe("2025年10月19日");
    expect(chatDayLabel("2026-03-01", "2026-03-02")).toBe("昨天");
  });
});
