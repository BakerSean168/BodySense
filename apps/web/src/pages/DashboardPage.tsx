import { useEffect, useState } from 'react';
import { useAuthStore } from '@/stores/authStore';
import { useProfileStore } from '@/stores/profileStore';
import { useNavigate } from 'react-router';

export function DashboardPage() {
  const { user, logout } = useAuthStore();
  const { profile, isLoading, fetchProfile } = useProfileStore();
  const navigate = useNavigate();
  const [profileChecked, setProfileChecked] = useState(false);

  useEffect(() => {
    const loadProfile = async () => {
      await fetchProfile();
      setProfileChecked(true);
    };
    loadProfile();
  }, [fetchProfile]);

  // If user has no profile, redirect to onboarding
  useEffect(() => {
    if (profileChecked && !isLoading && profile === null) {
      navigate('/onboarding');
    }
  }, [profileChecked, isLoading, profile, navigate]);

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  return (
    <div className="min-h-screen bg-gray-50">
      <nav className="bg-white shadow">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="flex h-16 justify-between">
            <div className="flex items-center">
              <h1 className="text-xl font-bold text-gray-900">BodySense</h1>
            </div>
            <div className="flex items-center space-x-4">
              <span className="text-sm text-gray-600">{user?.email}</span>
              <button
                onClick={handleLogout}
                className="rounded-md bg-gray-100 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-200"
              >
                Logout
              </button>
            </div>
          </div>
        </div>
      </nav>

      <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        <div className="rounded-lg bg-white p-6 shadow">
          <h2 className="text-2xl font-bold text-gray-900">Dashboard</h2>
          <p className="mt-2 text-gray-600">
            Welcome back, {user?.email}! You are now logged in.
          </p>

          <div className="mt-6 grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3">
            <button
              type="button"
              onClick={() => navigate('/profile')}
              className="rounded-lg border border-gray-200 p-4 text-left hover:border-blue-300 hover:bg-blue-50 transition-colors"
            >
              <h3 className="font-medium text-gray-900">身体档案</h3>
              <p className="mt-1 text-sm text-gray-500">查看和编辑您的身体信息</p>
            </button>
            <button
              type="button"
              onClick={() => navigate('/onboarding')}
              className="rounded-lg border border-gray-200 p-4 text-left hover:border-blue-300 hover:bg-blue-50 transition-colors"
            >
              <h3 className="font-medium text-gray-900">填写档案</h3>
              <p className="mt-1 text-sm text-gray-500">新用户填写身体信息</p>
            </button>
            <div className="rounded-lg border border-gray-200 p-4">
              <h3 className="font-medium text-gray-900">咨询问诊</h3>
              <p className="mt-1 text-sm text-gray-500">开始新的 AI 咨询</p>
            </div>
            <div className="rounded-lg border border-gray-200 p-4">
              <h3 className="font-medium text-gray-900">历史记录</h3>
              <p className="mt-1 text-sm text-gray-500">查看过往会话</p>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
