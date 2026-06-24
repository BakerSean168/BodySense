import { authFetch } from '@/features/auth/services/authService';

export interface TrainingPlan {
  id: string;
  user_id: string;
  consultation_id?: string;
  goal: string;
  duration_weeks: number;
  current_week: number;
  phases: TrainingPhase[];
  created_at: string;
}

export interface TrainingPhase {
  week: number;
  focus: string;
  exercises: TrainingExercise[];
}

export interface TrainingExercise {
  name: string;
  description: string;
  sets: string;
  reps: string;
  notes?: string;
}

export interface TrainingLog {
  id: string;
  plan_id: string;
  date: string;
  exercises: { name: string; completed: boolean }[];
  notes?: string;
  is_checked_in: boolean;
}

export interface TrainingProgress {
  consecutive_days: number;
  total_checkins: number;
  current_week: number;
  total_weeks: number;
}

export interface TrainingReassessmentResult {
  analysis: string;
  adjustments: {
    difficulty: string;
    duration: string;
    exercise_changes: { action: string; exercise: string; reason: string }[];
  };
  next_phase_plan: {
    focus: string;
    exercises: TrainingExercise[];
  };
  motivation: string;
}

export interface TrainingGenerationParams {
  consultation_id?: string;
  diagnosis?: Record<string, unknown>;
  preferences?: Record<string, unknown>;
}

export const trainingApi = {
  generate: async (params: TrainingGenerationParams): Promise<TrainingPlan> => {
    const response = await authFetch('/api/v1/training/generate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(params),
    });
    if (!response.ok) throw new Error('Failed to generate plan');
    return response.json();
  },

  getPlan: async (id: string): Promise<TrainingPlan> => {
    const response = await authFetch(`/api/v1/training/${id}`);
    if (!response.ok) throw new Error('Failed to get plan');
    return response.json();
  },

  listPlans: async (): Promise<TrainingPlan[]> => {
    const response = await authFetch('/api/v1/training');
    if (!response.ok) throw new Error('Failed to list plans');
    const data = await response.json();
    return data.plans;
  },

  getTodayTask: async (planId: string): Promise<TrainingLog> => {
    const response = await authFetch(`/api/v1/training/${planId}/today`);
    if (!response.ok) throw new Error('Failed to get today task');
    return response.json();
  },

  checkIn: async (planId: string): Promise<void> => {
    const response = await authFetch(`/api/v1/training/${planId}/checkin`, {
      method: 'POST',
    });
    if (!response.ok) throw new Error('Failed to check in');
  },

  updateLog: async (
    planId: string,
    notes: string,
    exercises: TrainingLog['exercises'],
  ): Promise<void> => {
    const response = await authFetch(`/api/v1/training/${planId}/log`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ notes, exercises }),
    });
    if (!response.ok) throw new Error('Failed to update log');
  },

  getProgress: async (planId: string): Promise<TrainingProgress> => {
    const response = await authFetch(`/api/v1/training/${planId}/progress`);
    if (!response.ok) throw new Error('Failed to get progress');
    return response.json();
  },

  submitReassessment: async (
    planId: string,
    feedback: { symptom_changes: string; training_feeling: string; difficulties: string },
  ): Promise<TrainingReassessmentResult> => {
    const response = await authFetch(`/api/v1/training/${planId}/reassess`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ feedback }),
    });
    if (!response.ok) throw new Error('Failed to submit reassessment');
    return response.json();
  },
};
