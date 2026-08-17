import { authFetch } from "@/features/auth/services/authService";
import type { Outcome, TreatmentRevision } from "@/features/workspace";
import { safeJson } from "@/lib/api-url";

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

export interface TrainingFeedbackResult {
  outcome?: Outcome;
  treatment_status?: string;
  review_recommended?: boolean;
  paused?: boolean;
  proposal?: TreatmentRevision;
  has_proposal?: boolean;
  requires_new_diagnosis?: boolean;
}

export interface TrainingLogUpdateResponse {
  message: string;
  has_proposal: boolean;
  result: TrainingFeedbackResult;
  proposal?: TreatmentRevision;
}

export const trainingApi = {
  getPlan: async (id: string): Promise<TrainingPlan> => {
    const response = await authFetch(`/api/v1/training/${id}`);
    if (!response.ok) throw new Error("Failed to get plan");
    return safeJson(response);
  },

  listPlans: async (): Promise<TrainingPlan[]> => {
    const response = await authFetch("/api/v1/training");
    if (!response.ok) throw new Error("Failed to list plans");
    const data = await safeJson<{ plans: TrainingPlan[] }>(response);
    return data.plans;
  },

  getTodayTask: async (planId: string): Promise<TrainingLog> => {
    const response = await authFetch(`/api/v1/training/${planId}/today`);
    if (!response.ok) throw new Error("Failed to get today task");
    return safeJson(response);
  },

  checkIn: async (planId: string): Promise<void> => {
    const response = await authFetch(`/api/v1/training/${planId}/checkin`, {
      method: "POST",
    });
    if (!response.ok) throw new Error("Failed to check in");
  },

  updateLog: async (
    planId: string,
    notes: string,
    exercises: TrainingLog["exercises"],
  ): Promise<TrainingLogUpdateResponse> => {
    const response = await authFetch(`/api/v1/training/${planId}/log`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ notes, exercises }),
    });
    if (!response.ok) throw new Error("Failed to update log");
    return safeJson(response);
  },

  getProgress: async (planId: string): Promise<TrainingProgress> => {
    const response = await authFetch(`/api/v1/training/${planId}/progress`);
    if (!response.ok) throw new Error("Failed to get progress");
    return safeJson(response);
  },

  submitReassessment: async (
    planId: string,
    feedback: {
      symptom_changes: string;
      training_feeling: string;
      difficulties: string;
    },
  ): Promise<TrainingFeedbackResult> => {
    const response = await authFetch(`/api/v1/training/${planId}/reassess`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ feedback }),
    });
    if (!response.ok) throw new Error("Failed to submit reassessment");
    return safeJson(response);
  },
};
