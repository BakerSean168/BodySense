import { afterEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { createRef } from "react";
import { MemoryRouter } from "react-router";
import userEvent from "@testing-library/user-event";

// The real altcha module registers a web component that fetches and
// solves proof-of-work challenges; the tests drive the element's events
// directly instead.
vi.mock("altcha", () => ({}));
vi.mock("altcha/i18n/zh-cn", () => ({}));

import {
  AuthCaptcha,
  CAPTCHA_SLOT_CLASS,
  type AuthCaptchaHandle,
} from "../AuthCaptcha";

function widgetEl(container: HTMLElement): HTMLElement {
  const el = container.querySelector("altcha-widget");
  if (!(el instanceof HTMLElement)) {
    throw new Error("widget element not rendered");
  }
  return el;
}

function fireVerified(el: HTMLElement, payload: string) {
  el.dispatchEvent(new CustomEvent("verified", { detail: { payload } }));
}

function fireState(el: HTMLElement, state: string) {
  el.dispatchEvent(new CustomEvent("statechange", { detail: { state } }));
}

afterEach(() => {
  vi.useRealTimers();
});

describe("AuthCaptcha", () => {
  it("renders the widget in the fixed-height slot, solving on load against the API", () => {
    const { container } = render(<AuthCaptcha captchaRef={createRef()} />);
    const slot = screen.getByTestId("auth-captcha");
    expect(slot.className).toContain(CAPTCHA_SLOT_CLASS);
    expect(slot.className).toContain("h-[72px]");
    const el = widgetEl(container);
    expect(el.getAttribute("challenge")).toContain("/api/v1/captcha/challenge");
    expect(el.getAttribute("auto")).toBe("onload");
  });

  it("exposes the solved payload through the handle", async () => {
    const ref = createRef<AuthCaptchaHandle>();
    const { container } = render(<AuthCaptcha captchaRef={ref} />);
    expect(ref.current?.getProof()).toBeNull();
    fireVerified(widgetEl(container), "solved-payload");
    expect(ref.current?.getProof()).toBe("solved-payload");
    await expect(ref.current?.waitForProof(1_000)).resolves.toBe(
      "solved-payload",
    );
  });

  it("waitForProof resolves when the widget finishes after the call", async () => {
    const ref = createRef<AuthCaptchaHandle>();
    const { container } = render(<AuthCaptcha captchaRef={ref} />);
    const pending = ref.current?.waitForProof(5_000);
    fireVerified(widgetEl(container), "late-payload");
    await expect(pending).resolves.toBe("late-payload");
  });

  it("waitForProof resolves null on timeout", async () => {
    const ref = createRef<AuthCaptchaHandle>();
    render(<AuthCaptcha captchaRef={ref} />);
    await expect(ref.current?.waitForProof(10)).resolves.toBeNull();
  });

  it("shows a visible failure with retry when the widget errors", async () => {
    const onLoadFailureChange = vi.fn();
    const { container } = render(
      <AuthCaptcha
        captchaRef={createRef()}
        onLoadFailureChange={onLoadFailureChange}
      />,
    );
    fireState(widgetEl(container), "error");
    const failedSlot = await screen.findByTestId("auth-captcha-failed");
    expect(failedSlot.className).toContain(CAPTCHA_SLOT_CLASS);
    expect(failedSlot.className).toContain("h-[72px]");
    expect(screen.getByRole("alert").textContent).toContain("人机验证加载失败");
    expect(onLoadFailureChange).toHaveBeenCalledWith(true);
    expect(container.querySelector("altcha-widget")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "重新加载验证" }));
    expect(await screen.findByTestId("auth-captcha")).toBeTruthy();
    expect(screen.queryByTestId("auth-captcha-failed")).toBeNull();
    expect(onLoadFailureChange).toHaveBeenCalledWith(false);
  });

  it("fails visibly when the widget produces no event within the timeout", () => {
    vi.useFakeTimers();
    render(<AuthCaptcha captchaRef={createRef()} />);
    expect(screen.queryByTestId("auth-captcha-failed")).toBeNull();
    act(() => {
      vi.advanceTimersByTime(10_000);
    });
    expect(screen.getByTestId("auth-captcha-failed")).toBeTruthy();
  });

  it("stays quiet when the widget reports progress in time", () => {
    vi.useFakeTimers();
    const { container } = render(<AuthCaptcha captchaRef={createRef()} />);
    fireState(widgetEl(container), "verifying");
    act(() => {
      vi.advanceTimersByTime(10_000);
    });
    expect(screen.queryByTestId("auth-captcha-failed")).toBeNull();
    expect(screen.getByTestId("auth-captcha")).toBeTruthy();
  });

  it("register form passes the solved proof to the store", async () => {
    const { RegisterPage } = await import("@/pages/RegisterPage");
    const { useAuthStore } = await import("@/stores/authStore");
    const initial = useAuthStore.getState();
    const register = vi.fn().mockResolvedValue(undefined);
    act(() => useAuthStore.setState({ register }));
    try {
      const { container } = render(
        <MemoryRouter>
          <RegisterPage />
        </MemoryRouter>,
      );
      fireEvent.change(screen.getByLabelText("邮箱地址"), {
        target: { value: "a@b.co" },
      });
      fireEvent.change(screen.getByLabelText("密码"), {
        target: { value: "longenough1" },
      });
      fireEvent.change(screen.getByLabelText("确认密码"), {
        target: { value: "longenough1" },
      });
      await userEvent.click(screen.getByRole("checkbox", { name: /同意/ }));
      fireVerified(widgetEl(container), "form-payload");
      fireEvent.click(screen.getByRole("button", { name: "创建账号" }));
      await vi.waitFor(() => {
        expect(register).toHaveBeenCalledWith(
          "a@b.co",
          "longenough1",
          "form-payload",
        );
      });
    } finally {
      act(() => useAuthStore.setState(initial, true));
    }
  });

  it("register form reports the load failure instead of asking to complete a missing widget", async () => {
    const { RegisterPage } = await import("@/pages/RegisterPage");
    const { useAuthStore } = await import("@/stores/authStore");
    const initial = useAuthStore.getState();
    const register = vi.fn().mockResolvedValue(undefined);
    act(() => useAuthStore.setState({ register }));
    try {
      const { container } = render(
        <MemoryRouter>
          <RegisterPage />
        </MemoryRouter>,
      );
      fireState(widgetEl(container), "error");
      expect(await screen.findByTestId("auth-captcha-failed")).toBeTruthy();
      fireEvent.change(screen.getByLabelText("邮箱地址"), {
        target: { value: "a@b.co" },
      });
      fireEvent.change(screen.getByLabelText("密码"), {
        target: { value: "longenough1" },
      });
      fireEvent.change(screen.getByLabelText("确认密码"), {
        target: { value: "longenough1" },
      });
      await userEvent.click(screen.getByRole("checkbox", { name: /同意/ }));
      fireEvent.click(screen.getByRole("button", { name: "创建账号" }));
      expect(
        await screen.findByText("人机验证加载失败，请重新加载验证后再提交"),
      ).toBeTruthy();
      expect(screen.queryByText("请完成人机验证")).toBeNull();
      expect(register).not.toHaveBeenCalled();
    } finally {
      act(() => useAuthStore.setState(initial, true));
    }
  });
});
