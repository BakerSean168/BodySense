import { useState, useEffect } from "react";
import { useNavigate } from "react-router";
import { useProfileStore } from "@/stores/profileStore";
import { OnboardingLayout } from "./OnboardingLayout";
import { GenderStep } from "./steps/GenderStep";
import { AgeStep } from "./steps/AgeStep";
import { HeightWeightStep } from "./steps/HeightWeightStep";
import { OccupationStep } from "./steps/OccupationStep";
import { SleepStep } from "./steps/SleepStep";
import { ExerciseStep } from "./steps/ExerciseStep";
import { InjuryStep } from "./steps/InjuryStep";
import { UploadStep } from "./steps/UploadStep";
import { assessmentApi } from "@/features/assessment/services/assessmentService";

import { toast } from "sonner";

const TOTAL_STEPS = 8;

interface FormData {
  gender: string;
  age: number | undefined;
  height_cm: number | undefined;
  weight_kg: number | undefined;
  occupation: string;
  exercise_type: string;
  exercise_frequency: string;
  sleep_time: string;
  wake_time: string;
  injury_history: string;
  self_description: string;
}

const INITIAL_DATA: FormData = {
  gender: "",
  age: undefined,
  height_cm: undefined,
  weight_kg: undefined,
  occupation: "",
  exercise_type: "",
  exercise_frequency: "",
  sleep_time: "",
  wake_time: "",
  injury_history: "",
  self_description: "",
};

export function OnboardingWizard() {
  const navigate = useNavigate();
  const { updateProfile, isLoading: isUpdatingProfile } = useProfileStore();

  const [currentStep, setCurrentStep] = useState(0);
  const [formData, setFormData] = useState<FormData>(INITIAL_DATA);
  const [isGeneratingReport, setIsGeneratingReport] = useState(false);
  const [loadingPhraseIndex, setLoadingPhraseIndex] = useState(0);

  const loadingPhrases = [
    "“体悟” AI 正在分析您的生理指标...",
    "正在进行体相结构多模态检测...",
    "正在提取体检报告中的关键健康数据...",
    "结合知识库，正在为您生成个性化体态分析报告...",
  ];

  useEffect(() => {
    if (!isGeneratingReport) return;
    const interval = setInterval(() => {
      setLoadingPhraseIndex((prev) => (prev + 1) % loadingPhrases.length);
    }, 2500);
    return () => clearInterval(interval);
  }, [isGeneratingReport]);

  const updateField = <K extends keyof FormData>(
    field: K,
    value: FormData[K],
  ) => {
    setFormData((prev) => ({ ...prev, [field]: value }));
  };

  const canProceed = (): boolean => {
    switch (currentStep) {
      case 0: // Gender
        return formData.gender !== "";
      case 1: // Age
        return (
          formData.age !== undefined && formData.age >= 1 && formData.age <= 150
        );
      case 2: // Height/Weight
        return (
          formData.height_cm !== undefined && formData.weight_kg !== undefined
        );
      case 3: // Occupation
        return formData.exercise_frequency !== "";
      case 4: // Sleep
        return true; // Optional
      case 5: // Exercise
        return true; // Optional
      case 6: // Injury
        return true; // Optional
      case 7: // Upload
        return true; // Optional
      default:
        return false;
    }
  };

  const handleNext = () => {
    if (currentStep < TOTAL_STEPS - 1) {
      setCurrentStep((prev) => prev + 1);
    }
  };

  const handleBack = () => {
    if (currentStep > 0) {
      setCurrentStep((prev) => prev - 1);
    }
  };

  const handleSubmit = async () => {
    setIsGeneratingReport(true);
    try {
      // 1. 保存身体档案信息
      await updateProfile({
        gender: formData.gender || undefined,
        age: formData.age,
        height_cm: formData.height_cm,
        weight_kg: formData.weight_kg,
        occupation: formData.occupation || undefined,
        exercise_type: formData.exercise_type || undefined,
        exercise_frequency: formData.exercise_frequency || undefined,
        sleep_time: formData.sleep_time || undefined,
        wake_time: formData.wake_time || undefined,
        injury_history: formData.injury_history || undefined,
        self_description: formData.self_description || undefined,
      });

      // 2. 生成健康评估报告
      const report = await assessmentApi.generate();

      toast.success("评估报告生成成功！");
      // 3. 跳转到报告详情页
      navigate(`/assessment/${report.id}`);
    } catch (err) {
      console.error("Onboarding submission failed:", err);
      setIsGeneratingReport(false);
      toast.error(
        err instanceof Error ? err.message : "生成评估报告失败，请稍后重试",
      );
    }
  };

  const renderStep = () => {
    switch (currentStep) {
      case 0:
        return (
          <GenderStep
            value={formData.gender}
            onChange={(v) => updateField("gender", v)}
          />
        );
      case 1:
        return (
          <AgeStep
            value={formData.age}
            onChange={(v) => updateField("age", v)}
          />
        );
      case 2:
        return (
          <HeightWeightStep
            height={formData.height_cm}
            weight={formData.weight_kg}
            onHeightChange={(v) => updateField("height_cm", v)}
            onWeightChange={(v) => updateField("weight_kg", v)}
          />
        );
      case 3:
        return (
          <OccupationStep
            occupation={formData.occupation}
            activityType={formData.exercise_frequency}
            onOccupationChange={(v) => updateField("occupation", v)}
            onActivityTypeChange={(v) => updateField("exercise_frequency", v)}
          />
        );
      case 4:
        return (
          <SleepStep
            sleepTime={formData.sleep_time}
            wakeTime={formData.wake_time}
            onSleepTimeChange={(v) => updateField("sleep_time", v)}
            onWakeTimeChange={(v) => updateField("wake_time", v)}
          />
        );
      case 5:
        return (
          <ExerciseStep
            exerciseType={formData.exercise_type}
            exerciseFrequency={formData.exercise_frequency}
            onExerciseTypeChange={(v) => updateField("exercise_type", v)}
            onExerciseFrequencyChange={(v) =>
              updateField("exercise_frequency", v)
            }
          />
        );
      case 6:
        return (
          <InjuryStep
            injuryHistory={formData.injury_history}
            selfDescription={formData.self_description}
            onInjuryHistoryChange={(v) => updateField("injury_history", v)}
            onSelfDescriptionChange={(v) => updateField("self_description", v)}
          />
        );
      case 7:
        return <UploadStep />;
      default:
        return null;
    }
  };

  if (isGeneratingReport) {
    return (
      <div className="min-h-screen bg-slate-50 flex flex-col items-center justify-center p-4">
        <div className="max-w-md w-full text-center space-y-6 animate-in fade-in zoom-in-95 duration-500">
          <div className="relative w-24 h-24 mx-auto">
            <div className="absolute inset-0 rounded-full border-4 border-[#CD7B67]/20" />
            <div className="absolute inset-0 rounded-full border-4 border-t-[#CD7B67] animate-spin" />
            <div className="absolute inset-0 flex items-center justify-center">
              <svg
                className="w-8 h-8 text-[#CD7B67]"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={1.8}
                  d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
                />
              </svg>
            </div>
          </div>

          <div className="space-y-2">
            <h2 className="text-xl font-bold text-slate-800">
              正在生成您的健康评估
            </h2>
            <p className="text-sm text-slate-500 font-medium h-6 transition-all duration-300">
              {loadingPhrases[loadingPhraseIndex]}
            </p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <OnboardingLayout
      currentStep={currentStep}
      totalSteps={TOTAL_STEPS}
      onNext={handleNext}
      onBack={handleBack}
      onSubmit={handleSubmit}
      isLoading={isUpdatingProfile}
      isLastStep={currentStep === TOTAL_STEPS - 1}
      canProceed={canProceed()}
    >
      {renderStep()}
    </OnboardingLayout>
  );
}
