import { useEffect, useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BusinessWindowHost } from "./BusinessWindowHost";

describe("responsive Body Canvas business windows", () => {
  it("preserves the same live task and unsent draft across desktop/mobile resize", async () => {
    const onClose = vi.fn();
    const mounted = vi.fn();
    const unmounted = vi.fn();

    function Draft() {
      const [text, setText] = useState("");
      useEffect(() => {
        mounted();
        return () => unmounted();
      }, []);
      return (
        <label>
          未发送的对话
          <input value={text} onChange={(event) => setText(event.target.value)} />
        </label>
      );
    }

    const task = (mobile: boolean, minimized = false) => (
      <BusinessWindowHost
        windowId="chat"
        stackIndex={0}
        onClose={onClose}
        isActive={!minimized}
        mobile={mobile}
        minimized={minimized}
      >
        <Draft />
      </BusinessWindowHost>
    );

    const { rerender, container } = render(task(false));
    const desktop = screen.getByRole("region", { name: "对话" });
    await userEvent.type(screen.getByRole("textbox", { name: "未发送的对话" }), "待发送的内容");
    expect(mounted).toHaveBeenCalledTimes(1);

    rerender(task(true));
    const sheet = screen.getByRole("dialog", { name: "对话" });
    expect(sheet).toBe(desktop);
    expect(sheet).toHaveAttribute("aria-modal", "false");
    expect(within(sheet).getByRole("textbox")).toHaveValue("待发送的内容");
    expect(mounted).toHaveBeenCalledTimes(1);
    expect(unmounted).not.toHaveBeenCalled();

    // Switching away from Chat hides the same node but must not destroy its draft.
    rerender(task(true, true));
    expect(sheet).not.toBeVisible();
    expect(container.querySelector("input")).toHaveValue("待发送的内容");

    rerender(task(false));
    expect(screen.getByRole("region", { name: "对话" })).toBe(sheet);
    expect(screen.getByRole("textbox", { name: "未发送的对话" }))
      .toHaveValue("待发送的内容");
    expect(mounted).toHaveBeenCalledTimes(1);
    expect(unmounted).not.toHaveBeenCalled();

    await userEvent.click(screen.getByRole("button", { name: "关闭对话" }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("keeps the responsive header, visibility and mobile-only actions in one shell", () => {
    const onClose = vi.fn();
    const view = (mobile: boolean) => (
      <BusinessWindowHost
        windowId="person"
        stackIndex={0}
        onClose={onClose}
        mobile={mobile}
        footer={<button type="button">保存草稿</button>}
      >
        <p>已加载的个人档案</p>
      </BusinessWindowHost>
    );

    const { rerender } = render(view(false));
    expect(screen.getByRole("region", { name: "个人档案" })).toBeVisible();
    expect(screen.queryByRole("button", { name: "保存草稿" })).toBeNull();

    rerender(view(true));
    expect(screen.getByRole("dialog", { name: "个人档案" })).toBeVisible();
    expect(screen.getByRole("button", { name: "保存草稿" })).toBeVisible();
    expect(screen.getByRole("button", { name: "关闭个人档案" })).toBeVisible();
    expect(screen.getByText("已加载的个人档案")).toBeVisible();
  });
});
