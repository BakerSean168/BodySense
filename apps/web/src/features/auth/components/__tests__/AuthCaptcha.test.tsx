import { afterEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { createRef } from "react";
import { MemoryRouter } from "react-router";
import userEvent from "@testing-library/user-event";

const turnstileBehavior = vi.hoisted(() => ({
  mode: "load" as "load" | "error" | "hang",
}));

vi.mock("@marsidev/react-turnstile", async () => {
  const React = await import("react");
  return {
    Turnstile: React.forwardRef(
      (props: {
        onWidgetLoad?: (widgetId: string) => void;
        onError?: (errorCode: string) => void;
      }) => {
        React.useEffect(() => {
          if (turnstileBehavior.mode === "load") {
            props.onWidgetLoad?.("widget-1");
          }
          if (turnstileBehavior.mode === "error") {
            props.onError?.("200500");
          }
        }, []);
        return React.createElement("div", { "data-testid": "mock-turnstile" });
      },
    ),
  };
});

async function loadAuthCaptcha(siteKey: string) {
  vi.resetModules();
  vi.stubEnv("VITE_TURNSTILE_SITE_KEY", siteKey);
  return import("../AuthCaptcha");
}

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllEnvs();
});

describe("AuthCaptcha", () => {
  it("shows a visible failure with retry when the widget errors", async () => {
    turnstileBehavior.mode = "error";
    const { AuthCaptcha } = await loadAuthCaptcha("test-site-key");
    render(<AuthCaptcha captchaRef={createRef()} />);
    expect(await screen.findByTestId("auth-turnstile-failed")).toBeTruthy();
    expect(screen.getByRole("alert").textContent).toContain("人机验证加载失败");
    expect(screen.queryByTestId("mock-turnstile")).toBeNull();

    turnstileBehavior.mode = "load";
    fireEvent.click(screen.getByRole("button", { name: "重新加载验证" }));
    expect(await screen.findByTestId("mock-turnstile")).toBeTruthy();
    expect(screen.queryByTestId("auth-turnstile-failed")).toBeNull();
  });

  it("fails visibly when the widget never loads within the timeout", async () => {
    turnstileBehavior.mode = "hang";
    const { AuthCaptcha } = await loadAuthCaptcha("test-site-key");
    vi.useFakeTimers();
    render(<AuthCaptcha captchaRef={createRef()} />);
    expect(screen.queryByTestId("auth-turnstile-failed")).toBeNull();
    act(() => {
      vi.advanceTimersByTime(10_000);
    });
    expect(screen.getByTestId("auth-turnstile-failed")).toBeTruthy();
  });

  it("stays quiet when the widget loads in time", async () => {
    turnstileBehavior.mode = "load";
    const { AuthCaptcha } = await loadAuthCaptcha("test-site-key");
    vi.useFakeTimers();
    render(<AuthCaptcha captchaRef={createRef()} />);
    act(() => {
      vi.advanceTimersByTime(10_000);
    });
    expect(screen.queryByTestId("auth-turnstile-failed")).toBeNull();
    expect(screen.getByTestId("mock-turnstile")).toBeTruthy();
  });

  it("renders an explicit note instead of a silent gap without a site key", async () => {
    turnstileBehavior.mode = "load";
    const { AuthCaptcha } = await loadAuthCaptcha("");
    render(<AuthCaptcha captchaRef={createRef()} />);
    expect(screen.getByTestId("auth-turnstile-unconfigured")).toBeTruthy();
    expect(screen.queryByTestId("mock-turnstile")).toBeNull();
  });

  it("gives the widget and the failure state one shared fixed-height slot", async () => {
    turnstileBehavior.mode = "load";
    const loaded = await loadAuthCaptcha("test-site-key");
    render(<loaded.AuthCaptcha captchaRef={createRef()} />);
    const widgetSlot = screen.getByTestId("auth-turnstile");
    expect(widgetSlot.className).toContain(loaded.CAPTCHA_SLOT_CLASS);
    expect(widgetSlot.className).toContain("h-[72px]");

    turnstileBehavior.mode = "error";
    const errored = await loadAuthCaptcha("test-site-key");
    render(<errored.AuthCaptcha captchaRef={createRef()} />);
    const failedSlot = await screen.findByTestId("auth-turnstile-failed");
    expect(failedSlot.className).toContain(errored.CAPTCHA_SLOT_CLASS);
    expect(failedSlot.className).toContain("h-[72px]");
  });

  it("keeps the unconfigured note visible without consuming in-flow height", async () => {
    turnstileBehavior.mode = "load";
    const { AuthCaptcha } = await loadAuthCaptcha("");
    render(<AuthCaptcha captchaRef={createRef()} />);
    const note = screen.getByTestId("auth-turnstile-unconfigured");
    expect(note.textContent).toContain("人机验证未启用");
    // The auth sheets follow a fixed ink board with zero spare vertical
    // room, so the note's wrapper must stay heightless (the note floats
    // inside the margin band) instead of pushing the form taller.
    expect(note.parentElement?.className).toContain("h-0");
  });

  it("register form reports the load failure instead of asking to complete a missing widget", async () => {
    turnstileBehavior.mode = "error";
    vi.resetModules();
    vi.stubEnv("VITE_TURNSTILE_SITE_KEY", "test-site-key");
    const { RegisterPage } = await import("@/pages/RegisterPage");
    const { useAuthStore } = await import("@/stores/authStore");
    const initial = useAuthStore.getState();
    const register = vi.fn().mockResolvedValue(undefined);
    act(() => useAuthStore.setState({ register }));
    try {
      render(
        <MemoryRouter>
          <RegisterPage />
        </MemoryRouter>,
      );
      expect(await screen.findByTestId("auth-turnstile-failed")).toBeTruthy();
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
