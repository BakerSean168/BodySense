import { Link } from "react-router";
import { RegisterForm } from "@/features/auth/components/RegisterForm";
import { AuthInkLayout } from "@/features/auth/components/AuthInkLayout";

export function RegisterPage() {
  return (
    <AuthInkLayout
      compactMobile
      title="注册"
      testId="auth-register-page"
      footer={
        <>
          已有账号？
          <Link
            to="/login"
            className="auth-ink-link font-bold text-[#2C694F] hover:underline hover:underline-offset-4"
          >
            登录
          </Link>
        </>
      }
    >
      <RegisterForm />
    </AuthInkLayout>
  );
}
