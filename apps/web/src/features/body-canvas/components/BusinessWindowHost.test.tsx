import { useEffect, useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BusinessWindowHost } from "./BusinessWindowHost";

describe("responsive Body Canvas business windows", () => {
  it("does not steal focus when inactive windows mount or refresh", async () => {
    const toolbar = <button type="button">返回画布</button>;
    const window = (minimized: boolean) => (
      <>
        {toolbar}
        <button type="button" className="bc-dock-chat">
          对话入口
        </button>
        <BusinessWindowHost
          windowId="chat"
          stackIndex={0}
          minimized={minimized}
          isActive={false}
          onClose={vi.fn()}
        >
          <label>
            未提交的内容
            <input defaultValue="草稿" />
          </label>
        </BusinessWindowHost>
      </>
    );

    const { rerender } = render(window(true));
    expect(screen.getByRole("button", { name: "对话入口" })).not.toHaveFocus();
    await userEvent.click(screen.getByRole("button", { name: "返回画布" }));
    rerender(window(true));
    expect(screen.getByRole("button", { name: "返回画布" })).toHaveFocus();
    expect(
      screen.getByRole("textbox", { name: "未提交的内容", hidden: true }),
    ).toHaveValue("草稿");
  });

  it("never returns focus to another Dock when switching between mounted windows", () => {
    const view = (current: "chat" | "diagnosis") => (
      <>
        <button type="button" className="bc-dock-chat">
          对话入口
        </button>
        <button type="button" className="bc-dock-diagnosis">
          诊断入口
        </button>
        <BusinessWindowHost
          windowId="chat"
          stackIndex={0}
          minimized={current !== "chat"}
          isActive={current === "chat"}
          onClose={vi.fn()}
        >
          <input aria-label="对话草稿" defaultValue="待发送" />
        </BusinessWindowHost>
        <BusinessWindowHost
          windowId="diagnosis"
          stackIndex={1}
          minimized={current !== "diagnosis"}
          isActive={current === "diagnosis"}
          onClose={vi.fn()}
        >
          <p>诊断窗口</p>
        </BusinessWindowHost>
      </>
    );
    const { rerender } = render(view("chat"));
    expect(screen.getByRole("region", { name: "对话" })).toHaveFocus();

    rerender(view("diagnosis"));
    expect(screen.getByRole("region", { name: "综合诊断" })).toHaveFocus();

    rerender(view("chat"));
    expect(screen.getByRole("region", { name: "对话" })).toHaveFocus();
    expect(screen.getByRole("textbox", { name: "对话草稿" })).toHaveValue(
      "待发送",
    );
  });

  it("restores focus from a hidden Ask User input to the existing Dock without losing its draft", async () => {
    const view = (hidden: boolean) => (
      <>
        <button type="button" className="bc-dock-chat">
          对话入口
        </button>
        <BusinessWindowHost
          windowId="chat"
          stackIndex={0}
          minimized={hidden}
          isActive={!hidden}
          onClose={vi.fn()}
        >
          <label>
            待回答的内容
            <input defaultValue="尚未提交的回答" />
          </label>
        </BusinessWindowHost>
      </>
    );
    const { rerender } = render(view(false));
    const host = screen.getByRole("region", { name: "对话" });
    expect(host).toHaveFocus();
    const input = screen.getByRole("textbox", { name: "待回答的内容" });
    await userEvent.click(input);
    expect(input).toHaveFocus();
    rerender(view(true));
    expect(host).not.toBeVisible();
    expect(screen.getByRole("button", { name: "对话入口" })).toHaveFocus();
    expect(input).toHaveValue("尚未提交的回答");
    rerender(view(false));
    expect(host).toHaveFocus();
    expect(input).toHaveValue("尚未提交的回答");
    await userEvent.click(input);
    rerender(view(false));
    expect(input).toHaveFocus();
  });

  it("keeps deliberate toolbar focus and restores a hidden mobile Trainer to Training Dock", async () => {
    const view = (hidden: boolean) => (
      <>
        <button className="bc-dock-training">训练入口</button>
        <button>返回画布</button>
        <BusinessWindowHost
          windowId="trainer"
          mobile
          stackIndex={0}
          minimized={hidden}
          isActive={!hidden}
          onClose={vi.fn()}
        >
          <button>暂停训练</button>
        </BusinessWindowHost>
      </>
    );
    const { rerender } = render(view(false));
    expect(screen.getByRole("dialog", { name: "训练执行" })).toHaveFocus();
    await userEvent.click(screen.getByRole("button", { name: "返回画布" }));
    rerender(view(true));
    expect(screen.getByRole("button", { name: "返回画布" })).toHaveFocus();
    rerender(view(false));
    await userEvent.click(screen.getByRole("button", { name: "暂停训练" }));
    rerender(view(true));
    expect(screen.getByRole("button", { name: "训练入口" })).toHaveFocus();
  });
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
          <input
            value={text}
            onChange={(event) => setText(event.target.value)}
          />
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
    await userEvent.type(
      screen.getByRole("textbox", { name: "未发送的对话" }),
      "待发送的内容",
    );
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
    expect(screen.getByRole("textbox", { name: "未发送的对话" })).toHaveValue(
      "待发送的内容",
    );
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
