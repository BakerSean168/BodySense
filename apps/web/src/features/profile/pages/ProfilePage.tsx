import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router';
import { useProfileStore } from '@/stores/profileStore';
import type { UserProfile } from '@/stores/profileStore';
import { ProfileView } from '../components/profile/ProfileView';
import { ProfileEdit } from '../components/profile/ProfileEdit';

export function ProfilePage() {
  const navigate = useNavigate();
  const { profile, isLoading, error, fetchProfile, updateProfile, clearError } = useProfileStore();
  const [isEditing, setIsEditing] = useState(false);

  useEffect(() => {
    fetchProfile();
  }, [fetchProfile]);

  const handleSave = async (data: Partial<UserProfile>) => {
    try {
      await updateProfile(data);
      setIsEditing(false);
    } catch {
      // Error is handled by the store
    }
  };

  if (isLoading && !profile) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto"></div>
          <p className="mt-4 text-gray-500">加载中...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-2xl mx-auto">
        <div className="mb-8">
          <button
            type="button"
            onClick={() => navigate('/dashboard')}
            className="text-sm text-blue-600 hover:text-blue-500"
          >
            ← 返回仪表盘
          </button>
        </div>

        {error && (
          <div className="mb-6 rounded-md bg-red-50 p-4">
            <p className="text-sm text-red-800">{error}</p>
            <button
              type="button"
              onClick={clearError}
              className="mt-2 text-sm text-red-600 underline hover:text-red-500"
            >
              关闭
            </button>
          </div>
        )}

        <div className="bg-white shadow sm:rounded-lg">
          <div className="px-6 py-8">
            {!profile && !isEditing ? (
              <div className="text-center">
                <h2 className="text-lg font-medium text-gray-900 mb-2">
                  还未填写身体档案
                </h2>
                <p className="text-sm text-gray-500 mb-6">
                  完善信息，获得更精准的健康建议
                </p>
                <button
                  type="button"
                  onClick={() => navigate('/onboarding')}
                  className="px-6 py-3 text-sm font-medium text-white bg-blue-600 border border-transparent rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
                >
                  开始填写
                </button>
              </div>
            ) : isEditing && profile ? (
              <ProfileEdit
                profile={profile}
                onSave={handleSave}
                onCancel={() => setIsEditing(false)}
                isLoading={isLoading}
              />
            ) : profile ? (
              <ProfileView profile={profile} onEdit={() => setIsEditing(true)} />
            ) : null}
          </div>
        </div>
      </div>
    </div>
  );
}
