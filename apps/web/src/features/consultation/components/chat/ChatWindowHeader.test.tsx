import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ChatWindowHeader } from "./ChatWindowHeader";

describe("Chat window header", () => {
  it("keeps the 今天 ▾ picker when the conversation has no earlier day", () => {
    render(
      <ChatWindowHeader
        history={{ todayKey: "2026-10-20", days: [] }}
        onSelectDay={vi.fn()}
      />,
    );
    expect(screen.getByText("对话")).toBeVisible();
    const picker = screen.getByRole("combobox", { name: "跳到某天的对话" });
    expect(picker).toHaveDisplayValue("今天");
    expect(screen.getAllByRole("option")).toHaveLength(1);
  });

  it("jumps to a day the user wrote in from the 今天 picker", async () => {
    const user = userEvent.setup();
    const onSelectDay = vi.fn();
    render(
      <ChatWindowHeader
        history={{
          todayKey: "2026-10-20",
          days: [
            {
              day: "2026-10-14",
              label: "10月14日",
              count: 2,
              preview: "昨天有头痛",
              firstMessageId: "a",
            },
          ],
        }}
        selectedDay="2026-10-20"
        onSelectDay={onSelectDay}
      />,
    );
    const picker = screen.getByRole("combobox", { name: "跳到某天的对话" });
    expect(picker).toHaveDisplayValue("今天");
    await user.selectOptions(picker, "10月14日");
    expect(onSelectDay).toHaveBeenCalledWith("2026-10-14");
  });
});
