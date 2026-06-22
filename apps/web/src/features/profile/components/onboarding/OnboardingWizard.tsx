import { useState } from 'react';
import { useNavigate } from 'react-router';
import { useProfileStore } from '@/stores/profileStore';
import { OnboardingLayout } from './OnboardingLayout';
import { GenderStep } from './steps/GenderStep';
import { AgeStep } from './steps/AgeStep';
import { HeightWeightStep } from './steps/HeightWeightStep';
import { OccupationStep } from './steps/OccupationStep';
import { SleepStep } from './steps/SleepStep';
import { ExerciseStep } from './steps/ExerciseStep';
import { InjuryStep } from './steps/InjuryStep';

const TOTAL_STEPS = 7;

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
  gender: '',
  age: undefined,
  height_cm: undefined,
  weight_kg: undefined,
  occupation: '',
  exercise_type: '',
  exercise_frequency: '',
  sleep_time: '',
  wake_time: '',
  injury_history: '',
  self_description: '',
};

export function OnboardingWizard() {
  const navigate = useNavigate();
  const { updateProfile, isLoading } = useProfileStore();

  const [currentStep, setCurrentStep] = useState(0);
  const [formData, setFormData] = useState<FormData>(INITIAL_DATA);

  const updateField = (field: keyof FormData, value: any) => {
    setFormData((prev) => ({ ...prev, [field]: value }));
  };

  const canProceed = (): boolean => {
    switch (currentStep) {
      case 0: // Gender
        return formData.gender !== '';
      case 1: // Age
        return formData.age !== undefined && formData.age >= 1 && formData.age <= 150;
      case 2: // Height/Weight
        return formData.height_cm !== undefined && formData.weight_kg !== undefined;
      case 3: // Occupation
        return formData.exercise_frequency !== '';
      case 4: // Sleep
        return true; // Optional
      case 5: // Exercise
        return true; // Optional
      case 6: // Injury
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
    try {
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
      navigate('/dashboard');
    } catch {
      // Error is handled by the store
    }
  };

  const renderStep = () => {
    switch (currentStep) {
      case 0:
        return (
          <GenderStep
            value={formData.gender}
            onChange={(v) => updateField('gender', v)}
          />
        );
      case 1:
        return (
          <AgeStep
            value={formData.age}
            onChange={(v) => updateField('age', v)}
          />
        );
      case 2:
        return (
          <HeightWeightStep
            height={formData.height_cm}
            weight={formData.weight_kg}
            onHeightChange={(v) => updateField('height_cm', v)}
            onWeightChange={(v) => updateField('weight_kg', v)}
          />
        );
      case 3:
        return (
          <OccupationStep
            occupation={formData.occupation}
            activityType={formData.exercise_frequency}
            onOccupationChange={(v) => updateField('occupation', v)}
            onActivityTypeChange={(v) => updateField('exercise_frequency', v)}
          />
        );
      case 4:
        return (
          <SleepStep
            sleepTime={formData.sleep_time}
            wakeTime={formData.wake_time}
            onSleepTimeChange={(v) => updateField('sleep_time', v)}
            onWakeTimeChange={(v) => updateField('wake_time', v)}
          />
        );
      case 5:
        return (
          <ExerciseStep
            exerciseType={formData.exercise_type}
            exerciseFrequency={formData.exercise_frequency}
            onExerciseTypeChange={(v) => updateField('exercise_type', v)}
            onExerciseFrequencyChange={(v) => updateField('exercise_frequency', v)}
          />
        );
      case 6:
        return (
          <InjuryStep
            injuryHistory={formData.injury_history}
            selfDescription={formData.self_description}
            onInjuryHistoryChange={(v) => updateField('injury_history', v)}
            onSelfDescriptionChange={(v) => updateField('self_description', v)}
          />
        );
      default:
        return null;
    }
  };

  return (
    <OnboardingLayout
      currentStep={currentStep}
      totalSteps={TOTAL_STEPS}
      onNext={handleNext}
      onBack={handleBack}
      onSubmit={handleSubmit}
      isLoading={isLoading}
      isLastStep={currentStep === TOTAL_STEPS - 1}
      canProceed={canProceed()}
    >
      {renderStep()}
    </OnboardingLayout>
  );
}
