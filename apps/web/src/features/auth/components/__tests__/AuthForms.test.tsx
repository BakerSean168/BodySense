import { afterEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { LoginPage } from "@/pages/LoginPage";
import { RegisterPage } from "@/pages/RegisterPage";
import { useAuthStore } from "@/stores/authStore";
import userEvent from "@testing-library/user-event";

const initial = useAuthStore.getState();

function renderAt(node: React.ReactNode) {
  return render(<MemoryRouter>{node}</MemoryRouter>);
}

afterEach(() => {
  act(() => useAuthStore.setState(initial, true));
  vi.restoreAllMocks();
});

describe("LoginPage (Penpot ink sign-in)", () => {
  it("renders label-less fields with accessible names and one primary action", () => {
    renderAt(<LoginPage />);
    expect(screen.getByRole("heading", { name: "登录" })).toBeTruthy();
    expect(screen.getByLabelText("邮箱地址").getAttribute("placeholder")).toBe(
      "邮箱",
    );
    expect(screen.getByLabelText("密码").getAttribute("type")).toBe("password");
    expect(screen.getAllByRole("button", { name: "登录" })).toHaveLength(1);
    expect(
      screen.getByRole("link", { name: "注册" }).getAttribute("href"),
    ).toBe("/register");
    expect(screen.queryByText("确认密码")).toBeNull();
  });

  it("toggles password visibility with the eye button", () => {
    renderAt(<LoginPage />);
    const toggle = screen.getByRole("button", { name: "显示密码" });
    fireEvent.click(toggle);
    expect(screen.getByLabelText("密码").getAttribute("type")).toBe("text");
    expect(
      screen
        .getByRole("button", { name: "隐藏密码" })
        .getAttribute("aria-pressed"),
    ).toBe("true");
  });

  it("keeps validation inline and does not call login", () => {
    const login = vi.fn();
    act(() => useAuthStore.setState({ login }));
    renderAt(<LoginPage />);
    fireEvent.click(screen.getByRole("button", { name: "登录" }));
    expect(screen.getByText("请输入邮箱地址")).toBeTruthy();
    expect(screen.getByText("请输入密码")).toBeTruthy();
    expect(screen.getByLabelText("密码").getAttribute("aria-invalid")).toBe(
      "true",
    );
    expect(login).not.toHaveBeenCalled();
  });

  it("shows the store error under the password field and the submitting label", () => {
    renderAt(<LoginPage />);
    act(() => useAuthStore.setState({ error: "登录失败", isLoading: true }));
    expect(screen.getByRole("alert").textContent).toBe("登录失败");
    expect(screen.getByTestId("auth-login-submit").textContent).toBe("登录中…");
  });
});

describe("RegisterPage (Penpot ink register)", () => {
  it("links back to sign in and has a label-less confirm-password field", () => {
    renderAt(<RegisterPage />);
    expect(screen.getByRole("heading", { name: "注册" })).toBeTruthy();
    expect(screen.getByLabelText("密码").getAttribute("placeholder")).toBe(
      "密码（至少 8 位）",
    );
    const confirm = screen.getByLabelText("确认密码");
    expect(confirm.getAttribute("placeholder")).toBe("确认密码");
    expect(confirm.getAttribute("type")).toBe("password");
    expect(confirm.getAttribute("autocomplete")).toBe("new-password");
    expect(screen.queryByText("确认密码")).toBeNull();
    expect(
      screen.getByRole("link", { name: "登录" }).getAttribute("href"),
    ).toBe("/login");
  });

  it("requires 8+ char password and consent before registering", async () => {
    const register = vi.fn().mockResolvedValue(undefined);
    act(() => useAuthStore.setState({ register }));
    renderAt(<RegisterPage />);
    fireEvent.change(screen.getByLabelText("邮箱地址"), {
      target: { value: "a@b.co" },
    });
    fireEvent.change(screen.getByLabelText("密码"), {
      target: { value: "short" },
    });
    fireEvent.click(screen.getByRole("button", { name: "创建账号" }));
    expect(screen.getByText("密码长度不能少于 8 位")).toBeTruthy();
    expect(
      screen.getByText("请先阅读并同意服务条款和健康数据处理说明"),
    ).toBeTruthy();
    expect(register).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("密码"), {
      target: { value: "longenough1" },
    });
    fireEvent.change(screen.getByLabelText("确认密码"), {
      target: { value: "longenough1" },
    });
    await userEvent.click(screen.getByRole("checkbox", { name: /同意/ }));
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "创建账号" }));
    });
    expect(register).toHaveBeenCalledWith("a@b.co", "longenough1");
  });

  it("requires the confirm password", () => {
    const register = vi.fn();
    act(() => useAuthStore.setState({ register }));
    renderAt(<RegisterPage />);
    fireEvent.click(screen.getByRole("button", { name: "创建账号" }));
    expect(screen.getByText("请再次输入密码")).toBeTruthy();
    expect(screen.getByLabelText("确认密码").getAttribute("aria-invalid")).toBe(
      "true",
    );
    expect(register).not.toHaveBeenCalled();
  });

  it("shows a mismatch error inline under confirm and does not register", async () => {
    const register = vi.fn();
    act(() => useAuthStore.setState({ register }));
    renderAt(<RegisterPage />);
    fireEvent.change(screen.getByLabelText("邮箱地址"), {
      target: { value: "a@b.co" },
    });
    fireEvent.change(screen.getByLabelText("密码"), {
      target: { value: "longenough1" },
    });
    fireEvent.change(screen.getByLabelText("确认密码"), {
      target: { value: "longenough2" },
    });
    await userEvent.click(screen.getByRole("checkbox", { name: /同意/ }));
    fireEvent.click(screen.getByRole("button", { name: "创建账号" }));
    const error = screen.getByText("两次输入的密码不一致");
    expect(error.getAttribute("id")).toBe("confirmPassword-error");
    expect(error.getAttribute("role")).toBe("alert");
    expect(
      screen.getByLabelText("确认密码").getAttribute("aria-describedby"),
    ).toBe("confirmPassword-error");
    expect(
      screen.getByLabelText("密码").getAttribute("aria-invalid"),
    ).toBeNull();
    expect(register).not.toHaveBeenCalled();
  });

  it("registers when the two passwords match", async () => {
    const register = vi.fn().mockResolvedValue(undefined);
    act(() => useAuthStore.setState({ register }));
    renderAt(<RegisterPage />);
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
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "创建账号" }));
    });
    expect(screen.queryByText("两次输入的密码不一致")).toBeNull();
    expect(register).toHaveBeenCalledWith("a@b.co", "longenough1");
  });

  it("toggles the confirm field with its own eye button", () => {
    renderAt(<RegisterPage />);
    fireEvent.click(screen.getByRole("button", { name: "显示确认密码" }));
    expect(screen.getByLabelText("确认密码").getAttribute("type")).toBe("text");
    expect(screen.getByLabelText("密码").getAttribute("type")).toBe("password");
    expect(
      screen
        .getByRole("button", { name: "隐藏确认密码" })
        .getAttribute("aria-pressed"),
    ).toBe("true");
    expect(
      screen
        .getByRole("button", { name: "显示密码" })
        .getAttribute("aria-pressed"),
    ).toBe("false");
  });
});
