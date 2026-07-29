import { useEffect, useState } from 'react';
import { useAuthStore } from '@/stores/authStore';
import { useProfileStore } from '@/stores/profileStore';
import { useNavigate } from 'react-router';
import { MainLayout } from '@/components/layout/MainLayout';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { JourneyNextStepCard, useJourneyState } from '@/features/journey';

export function DashboardPage() {
  const { user } = useAuthStore();
  const { profile, isLoading, fetchProfile } = useProfileStore();
  const navigate = useNavigate();
  const [profileChecked, setProfileChecked] = useState(false);
  const {
    journey,
    isLoading: journeyLoading,
    error: journeyError,
    refresh: refreshJourney,
  } = useJourneyState();

  useEffect(() => {
    const loadProfile = async () => {
      await fetchProfile();
      setProfileChecked(true);
    };
    loadProfile();
  }, [fetchProfile]);

  // Onboarding is the one redirect the dashboard still owns: without a profile
  // there is no journey to render. Every later step comes from the backend's
  // `available_actions` instead of being inferred here.
  useEffect(() => {
    if (profileChecked && !isLoading && profile === null) {
      navigate('/onboarding');
    }
  }, [profileChecked, isLoading, profile, navigate]);

  const widgets = [
    {
      title: '身体档案',
      desc: '查看并编辑您的个人生理指标、运动状态与健康目标。',
      icon: 'M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z',
      color: 'from-[#709a83] to-[#9ebdad]',
      href: '/profile',
    },
    {
      title: '智能问诊',
      desc: '与 AI 健康助手交流，实时解答您的姿态困惑或身体不适。',
      icon: 'M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z',
      color: 'from-[#CD7B67] to-[#E5A899]',
      href: '/consultation',
    },
    {
      title: '姿态评估',
      desc: '生成基于智能 AI 的全面身体姿态、骨骼平衡及理疗建议报告。',
      icon: 'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01',
      color: 'from-primary-700 to-primary-500',
      href: '/assessment',
    },
    {
      title: '历史记录',
      desc: '浏览您过往的问诊对话、健康诊断结果和身体姿态改善趋势。',
      icon: 'M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z',
      color: 'from-[#b69a84] to-[#d6c5b3]',
      href: '/history',
    },
  ];

  return (
    <MainLayout>
      <div className="space-y-8">
        {/* Welcome Section */}
        <div className="relative overflow-hidden rounded-[32px] bg-white text-[#2E3C36] p-8 sm:p-10 shadow-sm border border-[#E5E3DF]">
          {/* Subtle light morphing background blob */}
          <div className="absolute top-0 right-0 w-64 h-64 bg-gradient-to-tr from-[#709a83]/5 to-[#CD7B67]/5 rounded-full filter blur-3xl opacity-60 translate-x-1/3 -translate-y-1/3 pointer-events-none" />
          <div className="organic-blob absolute top-1/4 right-1/4 w-32 h-32 bg-[#709a83]/5 filter blur-2xl opacity-40 pointer-events-none" />

          <div className="relative z-10 max-w-2xl">
            <h1 className="text-3xl sm:text-4xl font-display font-semibold tracking-tight mb-3 text-[#2E3C36]">
              欢迎回来，<span className="text-[#CD7B67]">{user?.email?.split('@')[0]}</span>
            </h1>
            <p className="text-[#5D6B63] text-base sm:text-lg leading-relaxed font-medium">
              这是您的“体悟”个人健康管理中心。下方“下一步”由后端旅程状态驱动，快捷操作可随时进入各功能。
            </p>
          </div>
        </div>

        {/* Backend-derived next step — the only primary "what now?" surface.
            available_actions from GET /journey own the main CTAs; the hero no
            longer hard-codes consultation/profile buttons. */}
        <JourneyNextStepCard
          journey={journey}
          isLoading={journeyLoading}
          error={journeyError}
          onRetry={refreshJourney}
        />

        {/* Quick Actions Grid */}
        <div>
          <h2 className="text-xl font-display font-semibold text-[#2E3C36] mb-4">快捷操作</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
            {widgets.map((widget) => (
              <Card 
                key={widget.title}
                className="group relative overflow-hidden cursor-pointer"
                onClick={() => navigate(widget.href)}
              >
                <div className={`absolute top-0 right-0 w-32 h-32 bg-gradient-to-br ${widget.color} opacity-5 rounded-full blur-2xl -translate-y-1/2 translate-x-1/2 transition-transform duration-700 group-hover:scale-150`} />
                
                <div className="p-6 relative z-10">
                  <div className={`w-12 h-12 rounded-2xl bg-gradient-to-br ${widget.color} p-0.5 mb-4 shadow-sm shadow-black/5`}>
                    <div className="w-full h-full bg-[#FBFBFA] rounded-[14px] flex items-center justify-center">
                      <svg className="w-5.5 h-5.5 text-[#2E3C36]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d={widget.icon} />
                      </svg>
                    </div>
                  </div>
                  <h3 className="text-lg font-bold text-[#2E3C36] mb-2">{widget.title}</h3>
                  <p className="text-xs text-[#5E7D6F] font-semibold uppercase tracking-wider mb-2">专属服务</p>
                  <p className="text-sm text-[#5D6B63] font-medium leading-relaxed line-clamp-2">{widget.desc}</p>
                  
                  <div className="mt-5 flex items-center text-sm font-semibold text-accent-terracotta opacity-0 transform translate-x-[-10px] transition-all duration-300 group-hover:opacity-100 group-hover:translate-x-0">
                    立即前往
                    <svg className="w-4 h-4 ml-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                    </svg>
                  </div>
                </div>
              </Card>
            ))}
          </div>
        </div>

        {/* Recent Activity & Health Score */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          <Card className="lg:col-span-2 p-6">
            <div className="flex items-center justify-between mb-6 border-b border-[#E5E3DF] pb-4">
              <h3 className="text-lg font-display font-semibold text-[#2E3C36]">近期活动</h3>
              <Button variant="ghost" size="sm" className="text-accent-terracotta font-semibold hover:text-[#B65E49]" onClick={() => navigate('/history')}>查看全部</Button>
            </div>
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <div className="w-16 h-16 rounded-full bg-[#f1f5f2] border border-[#c5d7cc]/30 flex items-center justify-center mb-4">
                <svg className="w-8 h-8 text-primary-700" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.8} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              </div>
              <p className="text-[#2E3C36] font-semibold">暂无近期活动记录</p>
              <p className="text-sm text-[#5E7D6F] mt-1">开始一次健康评估或咨询，您的记录将展示在这里。</p>
            </div>
          </Card>
          
          <Card className="p-6">
            <h3 className="text-lg font-display font-semibold text-[#2E3C36] mb-6 border-b border-[#E5E3DF] pb-4">健康评分</h3>
            <div className="flex flex-col items-center justify-center py-8">
              <div className="relative w-32 h-32 flex items-center justify-center">
                <svg className="w-full h-full transform -rotate-90" viewBox="0 0 36 36">
                  <path
                    className="text-[#E5E3DF]"
                    d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2.5"
                  />
                  <path
                    className="text-accent-terracotta"
                    strokeDasharray="0, 100"
                    d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2.5"
                  />
                </svg>
                <div className="absolute flex flex-col items-center justify-center">
                  <span className="text-4xl font-display font-bold text-[#2E3C36]">--</span>
                  <span className="text-[10px] text-[#5E7D6F] uppercase font-bold tracking-widest mt-0.5">/ 100</span>
                </div>
              </div>
              <p className="mt-6 text-sm text-[#5D6B63] font-medium text-center">完成一次姿态评估，获取您的专业评分</p>
            </div>
          </Card>
        </div>
      </div>
    </MainLayout>
  );
}
