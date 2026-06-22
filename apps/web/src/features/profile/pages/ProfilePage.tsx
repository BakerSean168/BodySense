import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router';
import { useProfileStore } from '@/stores/profileStore';
import type { UserProfile } from '@/stores/profileStore';
import { useUploadStore } from '@/stores/uploadStore';
import { ProfileView } from '../components/profile/ProfileView';
import { ProfileEdit } from '../components/profile/ProfileEdit';
import { FileUploader, UploadList } from '../components/uploads';

export function ProfilePage() {
  const navigate = useNavigate();
  const { profile, isLoading, error, fetchProfile, updateProfile, clearError } = useProfileStore();
  const { uploads, fetchUploads } = useUploadStore();
  const [isEditing, setIsEditing] = useState(false);
  const [activeTab, setActiveTab] = useState<'profile' | 'uploads'>('profile');

  useEffect(() => {
    fetchProfile();
    fetchUploads();
  }, [fetchProfile, fetchUploads]);

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

        {/* Tabs */}
        <div className="mb-6 border-b border-gray-200">
          <nav className="-mb-px flex space-x-8">
            <button
              type="button"
              onClick={() => setActiveTab('profile')}
              className={`py-2 px-1 border-b-2 font-medium text-sm ${
                activeTab === 'profile'
                  ? 'border-blue-500 text-blue-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
              }`}
            >
              身体档案
            </button>
            <button
              type="button"
              onClick={() => setActiveTab('uploads')}
              className={`py-2 px-1 border-b-2 font-medium text-sm ${
                activeTab === 'uploads'
                  ? 'border-blue-500 text-blue-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
              }`}
            >
              文件管理
              {uploads.length > 0 && (
                <span className="ml-2 bg-gray-100 text-gray-600 py-0.5 px-2 rounded-full text-xs">
                  {uploads.length}
                </span>
              )}
            </button>
          </nav>
        </div>

        {/* Profile Tab */}
        {activeTab === 'profile' && (
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
        )}

        {/* Uploads Tab */}
        {activeTab === 'uploads' && (
          <div className="space-y-6">
            <div className="bg-white shadow sm:rounded-lg p-6">
              <h3 className="text-lg font-medium text-gray-900 mb-4">
                上传文件
              </h3>
              <FileUploader onUploadComplete={() => fetchUploads()} />
            </div>

            <div className="bg-white shadow sm:rounded-lg p-6">
              <h3 className="text-lg font-medium text-gray-900 mb-4">
                已上传文件
              </h3>
              <UploadList />
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
