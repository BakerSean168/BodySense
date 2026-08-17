import React, { useState } from "react";
import { useNavigate, useLocation } from "react-router";
import { useAuthStore } from "@/stores/authStore";

interface MainLayoutProps {
  children: React.ReactNode;
  fullHeight?: boolean;
}

export function MainLayout({ children, fullHeight = false }: MainLayoutProps) {
  const { user, logout } = useAuthStore();
  const navigate = useNavigate();
  const location = useLocation();
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);

  const navigation = [
    {
      name: "首页",
      href: "/dashboard",
      icon: "M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6",
    },
    {
      name: "身体档案",
      href: "/profile",
      icon: "M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z",
    },
    {
      name: "智能问诊",
      href: "/consultation",
      icon: "M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z",
    },
    {
      name: "健康评估",
      href: "/assessment",
      icon: "M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01",
    },
  ];

  const handleLogout = () => {
    logout();
    navigate("/login");
  };

  return (
    <div
      className={`bg-slate-50 flex ${fullHeight ? "h-screen overflow-hidden" : "min-h-screen"}`}
    >
      {/* Mobile sidebar overlay */}
      {isMobileMenuOpen && (
        <div
          className="fixed inset-0 z-40 bg-slate-900/50 backdrop-blur-sm lg:hidden"
          onClick={() => setIsMobileMenuOpen(false)}
        />
      )}

      {/* Sidebar */}
      <div
        className={`fixed inset-y-0 left-0 z-50 w-64 bg-[#F7F5F0] border-r border-[#E5E3DF] transform transition-transform duration-300 ease-in-out lg:translate-x-0 lg:static lg:inset-0 ${isMobileMenuOpen ? "translate-x-0" : "-translate-x-full"}`}
      >
        <div className="h-full flex flex-col">
          {/* Logo area */}
          <div className="flex h-16 shrink-0 items-center px-6 border-b border-[#E5E3DF]">
            <div className="flex items-center gap-2.5">
              <div className="w-8 h-8 rounded-xl bg-primary-700 flex items-center justify-center shadow-sm">
                <svg
                  className="w-4 h-4 text-[#FBFBFA]"
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
              <span className="text-xl font-display font-bold tracking-tight text-[#1A221E]">
                体悟
              </span>
            </div>
          </div>

          {/* Navigation */}
          <nav className="flex-1 overflow-y-auto px-4 py-6 space-y-1.5">
            {navigation.map((item) => {
              const isActive = location.pathname.startsWith(item.href);
              return (
                <button
                  key={item.name}
                  onClick={() => {
                    navigate(item.href);
                    setIsMobileMenuOpen(false);
                  }}
                  className={`w-full flex items-center gap-3.5 px-4 py-3 rounded-full text-sm font-semibold tracking-wide transition-all duration-300 ${
                    isActive
                      ? "bg-primary-700 text-[#FBFBFA] shadow-sm shadow-[#2a4b3a]/15"
                      : "text-[#4A554E] hover:bg-primary-100/50 hover:text-[#1A221E]"
                  }`}
                >
                  <svg
                    className={`w-4.5 h-4.5 transition-colors ${isActive ? "text-[#FBFBFA]" : "text-[#709a83]"}`}
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d={item.icon}
                    />
                  </svg>
                  {item.name}
                </button>
              );
            })}
          </nav>

          {/* User area */}
          <div className="p-4 border-t border-[#E5E3DF]">
            <div className="flex items-center gap-3 px-3 py-2 rounded-2xl bg-white/40 border border-[#E5E3DF]/50 shadow-[0_2px_10px_rgba(40,50,40,0.01)]">
              <div className="w-9 h-9 rounded-full bg-[#e2ebe5] border border-[#c5d7cc] flex items-center justify-center shrink-0 shadow-sm">
                <span className="text-sm font-bold text-primary-800">
                  {user?.email?.charAt(0).toUpperCase() || "U"}
                </span>
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-xs font-semibold text-primary-900 truncate">
                  {user?.email || "普通用户"}
                </p>
                <p className="text-[10px] font-semibold text-[#709a83] uppercase tracking-wider">
                  普通会员
                </p>
              </div>
            </div>
            <button
              onClick={handleLogout}
              className="mt-3 w-full flex items-center gap-3 px-4 py-2.5 rounded-full text-xs font-bold text-[#709a83] hover:bg-[#B65E49]/10 hover:text-[#B65E49] transition-all duration-300 border border-transparent hover:border-[#B65E49]/20"
            >
              <svg
                className="w-4 h-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"
                />
              </svg>
              退出登录
            </button>
          </div>
        </div>
      </div>

      {/* Main content */}
      <div className="flex-1 flex flex-col min-w-0 overflow-hidden bg-[#FBFBFA]">
        {/* Mobile header */}
        <div className="lg:hidden flex items-center justify-between h-16 px-4 bg-white/90 backdrop-blur-md border-b border-[#E5E3DF] sticky top-0 z-30">
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-xl bg-primary-700 flex items-center justify-center">
              <svg
                className="w-4 h-4 text-[#FBFBFA]"
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
            <span className="text-lg font-display font-bold text-[#1A221E]">
              体悟
            </span>
          </div>
          <button
            onClick={() => setIsMobileMenuOpen(true)}
            className="p-2 rounded-xl text-[#4A554E] hover:bg-primary-50 transition-colors border border-transparent hover:border-[#E5E3DF]"
          >
            <svg
              className="w-6 h-6"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M4 6h16M4 12h16M4 18h16"
              />
            </svg>
          </button>
        </div>

        {/* Page content */}
        <main
          className={`flex-1 ${fullHeight ? "flex flex-col min-h-0 overflow-hidden" : "overflow-y-auto"}`}
        >
          <div
            className={
              fullHeight
                ? "flex-1 flex flex-col min-h-0 h-full"
                : "w-full px-6 lg:px-10 py-8 animate-in fade-in slide-in-from-bottom-4 duration-500"
            }
          >
            {children}
          </div>
        </main>
      </div>
    </div>
  );
}
