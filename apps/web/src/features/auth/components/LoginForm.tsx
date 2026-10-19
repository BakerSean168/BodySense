import { useState } from "react";
import { useNavigate } from "react-router";
import { useAuthStore } from "@/stores/authStore";
import { InkButton, InkField } from "./InkField";

export function LoginForm() {
  const navigate = useNavigate();
  const { login, isLoading, error, clearError } = useAuthStore();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [validationErrors, setValidationErrors] = useState<{
    email?: string;
    password?: string;
  }>({});

  const validateForm = (): boolean => {
    const errors: { email?: string; password?: string } = {};

    if (!email) {
      errors.email = "请输入邮箱地址";
    } else if (!/\S+@\S+\.\S+/.test(email)) {
      errors.email = "邮箱格式不正确";
    }

    if (!password) {
      errors.password = "请输入密码";
    }

    setValidationErrors(errors);
    return Object.keys(errors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    clearError();

    if (!validateForm()) return;

    try {
      await login(email, password);
      navigate("/consultation");
    } catch {
      // Error is handled by the store
    }
  };

  // Penpot "Sign in · Error": the server error sits under the password field.
  const passwordError = validationErrors.password ?? error ?? undefined;

  return (
    <form onSubmit={handleSubmit} noValidate data-testid="auth-login-form">
      <InkField
        id="email"
        kind="email"
        accessibleLabel="邮箱地址"
        autoComplete="email"
        value={email}
        onChange={(e) => setEmail(e.target.value)}
        placeholder="邮箱"
        error={validationErrors.email}
      />

      <InkField
        id="password"
        kind="password"
        accessibleLabel="密码"
        autoComplete="current-password"
        className="mt-7 lg:mt-[34px]"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        placeholder="密码"
        error={passwordError}
      />

      <div className="mt-[52px] lg:mt-[56px]">
        <InkButton
          isLoading={isLoading}
          loadingLabel="登录中…"
          testId="auth-login-submit"
        >
          登录
        </InkButton>
      </div>
    </form>
  );
}
