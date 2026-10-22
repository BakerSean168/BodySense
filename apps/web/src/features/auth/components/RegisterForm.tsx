import { useEffect, useState } from "react";
import { useNavigate } from "react-router";
import { useAuthStore } from "@/stores/authStore";
import { InkButton, InkField } from "./InkField";
import { Checkbox } from "@/components/ui/checkbox";

type RegisterErrors = {
  email?: string;
  password?: string;
  confirmPassword?: string;
  consent?: string;
};

export function RegisterForm() {
  const navigate = useNavigate();
  const { register, isLoading, error, clearError } = useAuthStore();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [consent, setConsent] = useState(false);
  const [validationErrors, setValidationErrors] = useState<RegisterErrors>({});

  // The auth store error is global: reset it (and this form) whenever the
  // form mounts or unmounts so a register error never leaks onto /login.
  useEffect(() => {
    clearError();
    setEmail("");
    setPassword("");
    setConfirmPassword("");
    setConsent(false);
    setValidationErrors({});
    return () => clearError();
  }, [clearError]);

  const validateForm = (): boolean => {
    const errors: RegisterErrors = {};

    if (!email) {
      errors.email = "请输入邮箱地址";
    } else if (!/\S+@\S+\.\S+/.test(email)) {
      errors.email = "邮箱格式不正确";
    }

    if (!password) {
      errors.password = "请输入密码";
    } else if (password.length < 8) {
      errors.password = "密码长度不能少于 8 位";
    }

    if (!confirmPassword) {
      errors.confirmPassword = "请再次输入密码";
    } else if (confirmPassword !== password) {
      errors.confirmPassword = "两次输入的密码不一致";
    }

    if (!consent) {
      errors.consent = "请先阅读并同意服务条款和健康数据处理说明";
    }

    setValidationErrors(errors);
    return Object.keys(errors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    clearError();

    if (!validateForm()) return;

    try {
      await register(email, password);
      navigate("/consultation");
    } catch {
      // Error is handled by the store
    }
  };

  return (
    <form onSubmit={handleSubmit} noValidate data-testid="auth-register-form">
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
        autoComplete="new-password"
        className="mt-7 lg:mt-[34px]"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        placeholder="密码（至少 8 位）"
        error={validationErrors.password}
      />

      <InkField
        id="confirmPassword"
        kind="password"
        accessibleLabel="确认密码"
        visibilityLabel="确认密码"
        autoComplete="new-password"
        className="mt-7 lg:mt-[34px]"
        value={confirmPassword}
        onChange={(e) => setConfirmPassword(e.target.value)}
        placeholder="确认密码"
        error={validationErrors.confirmPassword}
      />

      <div className="mt-[22px] lg:mt-6">
        <label className="flex cursor-pointer items-start gap-[10px] text-[13px] leading-[1.2] text-[#52665A]">
          <Checkbox
            checked={consent}
            onCheckedChange={(checked) => setConsent(checked)}
            aria-invalid={validationErrors.consent ? true : undefined}
            aria-describedby={
              validationErrors.consent ? "consent-error" : undefined
            }
            data-testid="auth-register-consent"
            className="mt-px size-4 rounded-[4px] border-[#AEC0B0] bg-white focus-visible:ring-offset-[#FBFAF5]"
          />
          <span>同意《服务条款》和《健康数据处理说明》</span>
        </label>
        {validationErrors.consent && (
          <p
            id="consent-error"
            role="alert"
            className="mt-2 text-[13px] leading-[1.2] font-semibold text-[#9A3F35]"
          >
            {validationErrors.consent}
          </p>
        )}
      </div>

      {error && (
        <p
          role="alert"
          data-testid="auth-register-error"
          className="mt-3 text-[13px] leading-[1.2] font-semibold text-[#9A3F35]"
        >
          {error}
        </p>
      )}

      <div className="mt-[14px] lg:mt-[26px]">
        <InkButton
          isLoading={isLoading}
          loadingLabel="注册中…"
          testId="auth-register-submit"
        >
          创建账号
        </InkButton>
      </div>
    </form>
  );
}
