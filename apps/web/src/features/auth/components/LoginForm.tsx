import { useState } from "react";
import { useNavigate } from "react-router";
import { useAuthStore } from "@/stores/authStore";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";

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

  return (
    <form onSubmit={handleSubmit} className="space-y-5">
      <Input
        id="email"
        type="email"
        label="邮箱地址"
        value={email}
        onChange={(e) => setEmail(e.target.value)}
        placeholder="your@email.com"
        error={validationErrors.email}
      />

      <Input
        id="password"
        type="password"
        label="密码"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        placeholder="••••••••"
        error={validationErrors.password}
      />

      {error && (
        <div className="rounded-xl bg-red-50 p-4 border border-red-100">
          <p className="text-sm font-medium text-red-800">{error}</p>
        </div>
      )}

      <Button
        type="submit"
        isLoading={isLoading}
        className="w-full mt-2"
        size="lg"
      >
        登录
      </Button>
    </form>
  );
}
