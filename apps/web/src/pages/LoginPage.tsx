import { Link } from "react-router";
import { LoginForm } from "@/features/auth/components/LoginForm";

export function LoginPage() {
  return (
    <div className="min-h-screen flex bg-[#FBFBFA]">
      {/* Left side - Visual/Branding */}
      <div className="hidden lg:flex lg:w-1/2 relative bg-[#F7F5F0] overflow-hidden border-r border-[#E5E3DF]">
        {/* Slow-morphing organic light sculpture (Signature) */}
        <div className="absolute inset-0 overflow-hidden z-10 flex items-center justify-center">
          <div className="organic-blob absolute w-[450px] h-[450px] bg-gradient-to-tr from-[#709a83]/8 via-[#CD7B67]/5 to-[#9ebdad]/8 filter blur-3xl opacity-70" />
          <div
            className="organic-blob absolute w-[350px] h-[350px] bg-[#709a83]/5 filter blur-3xl opacity-40"
            style={{ animationDirection: "reverse", animationDuration: "18s" }}
          />
        </div>

        <div className="relative z-20 flex flex-col justify-between p-16 text-[#2E3C36] w-full h-full">
          {/* Logo Monogram */}
          <div className="flex items-center gap-2.5">
            <div className="w-10 h-10 rounded-2xl bg-primary-700 flex items-center justify-center shadow-sm">
              <svg
                className="w-5 h-5 text-white"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2.5}
                  d="M13 10V3L4 14h7v7l9-11h-7z"
                />
              </svg>
            </div>
            <span className="text-xl font-display font-bold tracking-tight text-[#2E3C36]">
              体悟
            </span>
          </div>

          <div className="max-w-md my-auto">
            <h1 className="text-4xl md:text-5xl font-display font-semibold leading-[1.2] mb-6 text-[#2E3C36] tracking-tight">
              深刻理解您的身体，
              <br />
              <span className="text-[#CD7B67]">前所未有。</span>
            </h1>
            <p className="text-base text-[#5D6B63] leading-relaxed font-medium">
              加入“体悟”，获取基于 AI
              的姿态分析、个性化健康评估以及专业的专家在线咨询服务。
            </p>
          </div>

          {/* Minimalist Footer inside Left Side */}
          <div className="text-xs text-[#7A9E8C] font-medium tracking-wide">
            © {new Date().getFullYear()} 体悟 · 智能辅助设计
          </div>
        </div>
      </div>

      {/* Right side - Form */}
      <div className="w-full lg:w-1/2 flex items-center justify-center p-8 sm:p-12 lg:p-16 bg-[#FBFBFA]">
        <div className="w-full max-w-md space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-700">
          <div className="text-center lg:text-left">
            <div className="lg:hidden w-12 h-12 rounded-2xl bg-primary-700 flex items-center justify-center mx-auto mb-6 shadow-md shadow-[#2a4b3a]/10">
              <svg
                className="w-6 h-6 text-white"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M13 10V3L4 14h7v7l9-11h-7z"
                />
              </svg>
            </div>
            <h2 className="text-3xl font-display font-semibold text-[#1A221E] tracking-tight">
              欢迎回来
            </h2>
            <p className="mt-2 text-sm font-semibold text-[#709a83] uppercase tracking-wider">
              请登录您的账号以继续
            </p>
          </div>

          <div className="bg-white/40 border border-[#E5E3DF] p-6 sm:p-8 rounded-[24px] shadow-[0_4px_25px_rgba(40,50,40,0.02)] backdrop-blur-sm">
            <LoginForm />
          </div>

          <div className="text-center">
            <p className="text-sm text-[#4A554E] font-medium">
              还没有账号？{" "}
              <Link
                to="/register"
                className="font-semibold text-accent-terracotta hover:text-accent-clay hover:underline underline-offset-4 transition-all"
              >
                立即注册
              </Link>
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
