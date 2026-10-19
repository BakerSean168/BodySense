import { Link } from "react-router";
import { LoginForm } from "@/features/auth/components/LoginForm";
import { AuthInkLayout } from "@/features/auth/components/AuthInkLayout";

export function LoginPage() {
  return (
    <AuthInkLayout
      title="登录"
      testId="auth-login-page"
      footer={
        <>
          还没有账号？
          <Link
            to="/register"
            className="auth-ink-link font-bold text-[#2C694F] hover:underline hover:underline-offset-4"
          >
            注册
          </Link>
        </>
      }
    >
      <LoginForm />
    </AuthInkLayout>
  );
}
