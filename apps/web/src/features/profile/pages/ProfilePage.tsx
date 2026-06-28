import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router';
import { useProfileStore } from '@/stores/profileStore';
import type { UserProfile } from '@/stores/profileStore';
import { useUploadStore } from '@/stores/uploadStore';
import { ProfileView } from '../components/profile/ProfileView';
import { ProfileEdit } from '../components/profile/ProfileEdit';
import { FileUploader, UploadList } from '../components/uploads';
import { MainLayout } from '@/components/layout/MainLayout';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/button';

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
      <MainLayout>
        <div className="flex items-center justify-center min-h-[60vh]">
          <div className="text-center">
            <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-600 mx-auto"></div>
            <p className="mt-4 text-slate-500 font-medium">加载中...</p>
          </div>
        </div>
      </MainLayout>
    );
  }

  return (
    <MainLayout>
      <div className="w-full">
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-2xl font-bold text-slate-900 tracking-tight">身体档案</h1>
            <p className="text-slate-500 mt-1">管理您的生理指标、运动状态及健康文档。</p>
          </div>
          {profile && !isEditing && activeTab === 'profile' && (
            <Button onClick={() => setIsEditing(true)}>
              编辑档案
            </Button>
          )}
        </div>

        {error && (
          <div className="mb-6 rounded-xl bg-red-50 p-4 border border-red-100 flex justify-between items-center">
            <p className="text-sm font-medium text-red-800">{error}</p>
            <button
              type="button"
              onClick={clearError}
              className="text-sm text-red-600 hover:text-red-500 font-medium"
            >
              关闭
            </button>
          </div>
        )}

        {/* Tabs */}
        <div className="mb-8 border-b border-slate-200">
          <nav className="-mb-px flex space-x-8">
            <button
              type="button"
              onClick={() => setActiveTab('profile')}
              className={`py-3 px-1 border-b-2 font-medium text-sm transition-colors ${
                activeTab === 'profile'
                  ? 'border-primary-500 text-primary-600'
                  : 'border-transparent text-slate-500 hover:text-slate-700 hover:border-slate-300'
              }`}
            >
              个人信息
            </button>
            <button
              type="button"
              onClick={() => setActiveTab('uploads')}
              className={`py-3 px-1 border-b-2 font-medium text-sm transition-colors flex items-center ${
                activeTab === 'uploads'
                  ? 'border-primary-500 text-primary-600'
                  : 'border-transparent text-slate-500 hover:text-slate-700 hover:border-slate-300'
              }`}
            >
              文件管理
              {uploads.length > 0 && (
                <span className={`ml-2 py-0.5 px-2.5 rounded-full text-xs font-semibold ${
                  activeTab === 'uploads' ? 'bg-primary-100 text-primary-700' : 'bg-slate-100 text-slate-600'
                }`}>
                  {uploads.length}
                </span>
              )}
            </button>
          </nav>
        </div>

        <div className="animate-in fade-in slide-in-from-bottom-4 duration-500">
          {/* Profile Tab */}
          {activeTab === 'profile' && (
            <Card className="p-8">
              {!profile && !isEditing ? (
                <div className="text-center py-12">
                  <div className="w-16 h-16 rounded-full bg-primary-50 text-primary-500 flex items-center justify-center mx-auto mb-4">
                    <svg className="w-8 h-8" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                    </svg>
                  </div>
                  <h2 className="text-xl font-bold text-slate-900 mb-2">
                    还未填写身体档案
                  </h2>
                  <p className="text-slate-500 mb-6 max-w-sm mx-auto">
                    完善信息，获得更精准的健康建议
                  </p>
                  <Button onClick={() => navigate('/onboarding')} size="lg">
                    开始填写
                  </Button>
                </div>
              ) : isEditing && profile ? (
                <div className="max-w-2xl mx-auto">
                  <h2 className="text-xl font-bold text-slate-900 mb-6 border-b border-slate-100 pb-4">编辑档案</h2>
                  <ProfileEdit
                    profile={profile}
                    onSave={handleSave}
                    onCancel={() => setIsEditing(false)}
                    isLoading={isLoading}
                  />
                </div>
              ) : profile ? (
                <ProfileView profile={profile} onEdit={() => setIsEditing(true)} />
              ) : null}
            </Card>
          )}

          {/* Uploads Tab */}
          {activeTab === 'uploads' && (
            <div className="space-y-6">
              <Card className="p-6">
                <h3 className="text-lg font-bold text-slate-900 mb-4 flex items-center gap-2">
                  <svg className="w-5 h-5 text-primary-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
                  </svg>
                  上传文件
                </h3>
                <div className="bg-slate-50 border-2 border-dashed border-slate-200 rounded-xl p-4 transition-colors hover:border-primary-300 hover:bg-primary-50">
                  <FileUploader onUploadComplete={() => fetchUploads()} />
                </div>
              </Card>

              <Card className="p-6">
                <h3 className="text-lg font-bold text-slate-900 mb-4 flex items-center gap-2">
                  <svg className="w-5 h-5 text-indigo-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 8h14M5 8a2 2 0 110-4h14a2 2 0 110 4M5 8v10a2 2 0 002 2h10a2 2 0 002-2V8m-9 4h4" />
                  </svg>
                  已上传文件
                </h3>
                <UploadList />
              </Card>
            </div>
          )}
        </div>
      </div>
    </MainLayout>
  );
}

