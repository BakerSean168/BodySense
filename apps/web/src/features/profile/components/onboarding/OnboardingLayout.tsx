import type { ReactNode } from 'react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/button';

interface OnboardingLayoutProps {
  children: ReactNode;
  currentStep: number;
  totalSteps: number;
  onNext: () => void;
  onBack: () => void;
  onSubmit: () => void;
  isLoading: boolean;
  isLastStep: boolean;
  canProceed: boolean;
}

export function OnboardingLayout({
  children,
  currentStep,
  totalSteps,
  onNext,
  onBack,
  onSubmit,
  isLoading,
  isLastStep,
  canProceed,
}: OnboardingLayoutProps) {
  return (
    <div className="min-h-screen bg-slate-50 flex flex-col justify-center py-12 sm:px-6 lg:px-8 relative overflow-hidden">
      {/* Background decorations */}
      <div className="absolute top-0 right-0 w-1/2 h-full bg-gradient-to-l from-primary-50 to-transparent z-0 pointer-events-none" />
      <div className="absolute -top-40 -right-40 w-96 h-96 bg-primary-200 rounded-full mix-blend-multiply filter blur-3xl opacity-50 z-0 pointer-events-none" />
      <div className="absolute top-1/3 -left-40 w-96 h-96 bg-indigo-200 rounded-full mix-blend-multiply filter blur-3xl opacity-50 z-0 pointer-events-none" />

      <div className="sm:mx-auto sm:w-full sm:max-w-md relative z-10">
        <div className="w-16 h-16 rounded-2xl bg-gradient-to-br from-primary-500 to-indigo-600 flex items-center justify-center mx-auto mb-6 shadow-lg shadow-primary-500/30">
          <svg className="w-8 h-8 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
          </svg>
        </div>
        <h1 className="text-center text-3xl font-bold text-slate-900 tracking-tight mb-2">
          身体档案
        </h1>
        <p className="text-center text-slate-500 mb-8">
          完善信息，获得更精准的健康建议
        </p>
      </div>

      <div className="sm:mx-auto sm:w-full sm:max-w-xl relative z-10 animate-in fade-in slide-in-from-bottom-4 duration-500">
        {/* Progress bar */}
        <div className="mb-6 px-4 sm:px-0">
          <div className="flex justify-between text-sm font-medium text-slate-500 mb-3">
            <span>步骤 {currentStep + 1} / {totalSteps}</span>
            <span className="text-primary-600">{Math.round(((currentStep + 1) / totalSteps) * 100)}%</span>
          </div>
          <div className="w-full bg-slate-200/60 rounded-full h-2.5 overflow-hidden backdrop-blur-sm">
            <div
              className="bg-gradient-to-r from-primary-500 to-indigo-500 h-full rounded-full transition-all duration-500 ease-out"
              style={{ width: `${((currentStep + 1) / totalSteps) * 100}%` }}
            />
          </div>
        </div>

        {/* Content card */}
        <Card className="py-8 px-6 sm:px-10 shadow-xl border-slate-100/50 bg-white/90 backdrop-blur-md">
          <div className="min-h-[300px]">
            {children}
          </div>
        </Card>

        {/* Navigation buttons */}
        <div className="mt-8 flex justify-between px-4 sm:px-0 gap-4">
          <Button
            variant="outline"
            onClick={onBack}
            disabled={currentStep === 0}
            className="w-1/3 bg-white"
          >
            上一步
          </Button>

          <Button
            onClick={isLastStep ? onSubmit : onNext}
            disabled={!canProceed || isLoading}
            isLoading={isLoading}
            className="w-2/3 shadow-md shadow-primary-500/20"
          >
            {isLastStep ? '完成' : '下一步'}
          </Button>
        </div>
      </div>
    </div>
  );
}
