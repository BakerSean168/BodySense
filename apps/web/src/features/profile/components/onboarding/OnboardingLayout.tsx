import type { ReactNode } from 'react';

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
    <div className="min-h-screen bg-gray-50 flex flex-col justify-center py-12 sm:px-6 lg:px-8">
      <div className="sm:mx-auto sm:w-full sm:max-w-md">
        <h1 className="text-center text-2xl font-bold text-gray-900 mb-2">
          身体档案
        </h1>
        <p className="text-center text-sm text-gray-500 mb-8">
          完善信息，获得更精准的健康建议
        </p>
      </div>

      <div className="sm:mx-auto sm:w-full sm:max-w-md">
        {/* Progress bar */}
        <div className="mb-8 px-4">
          <div className="flex justify-between text-xs text-gray-500 mb-2">
            <span>步骤 {currentStep + 1} / {totalSteps}</span>
            <span>{Math.round(((currentStep + 1) / totalSteps) * 100)}%</span>
          </div>
          <div className="w-full bg-gray-200 rounded-full h-2">
            <div
              className="bg-blue-600 h-2 rounded-full transition-all duration-300"
              style={{ width: `${((currentStep + 1) / totalSteps) * 100}%` }}
            />
          </div>
        </div>

        {/* Content card */}
        <div className="bg-white py-8 px-6 shadow sm:rounded-lg sm:px-10">
          {children}
        </div>

        {/* Navigation buttons */}
        <div className="mt-6 flex justify-between px-4">
          <button
            type="button"
            onClick={onBack}
            disabled={currentStep === 0}
            className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            上一步
          </button>

          <button
            type="button"
            onClick={isLastStep ? onSubmit : onNext}
            disabled={!canProceed || isLoading}
            className="px-6 py-2 text-sm font-medium text-white bg-blue-600 border border-transparent rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isLoading ? '保存中...' : isLastStep ? '完成' : '下一步'}
          </button>
        </div>
      </div>
    </div>
  );
}
