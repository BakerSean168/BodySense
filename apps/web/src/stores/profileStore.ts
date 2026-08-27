import { create } from "zustand";
import { authFetch } from "@/features/auth/services/authService";
import { useAuthStore } from "./authStore";
import { safeJson, extractErrorMessage } from "@/lib/api-url";

export interface UserProfile {
  id: string;
  user_id: string;
  gender?: string;
  birth_date?: string;
  age_years?: number;
  height_cm?: number;
  weight_kg?: number;
  bmi?: number;
  activity_pattern?: string;
  sleep_pattern?: string;
  exercise_type?: string;
  exercise_frequency?: string;
  injury_history?: string;
  /** @deprecated Legacy profile fields kept for API compatibility only. */
  age?: number;
  /** @deprecated Use activity_pattern. */
  occupation?: string;
  /** @deprecated Use sleep_pattern. */
  sleep_time?: string;
  /** @deprecated Use sleep_pattern. */
  wake_time?: string;
  /** @deprecated Current concerns belong in BodyState / consultation. */
  self_description?: string;
  created_at: string;
  updated_at: string;
}

interface ProfileState {
  profile: UserProfile | null;
  isLoading: boolean;
  error: string | null;

  // Actions
  fetchProfile: () => Promise<void>;
  updateProfile: (data: Partial<UserProfile>) => Promise<void>;
  clearError: () => void;
}

export const useProfileStore = create<ProfileState>()((set) => ({
  profile: null,
  isLoading: false,
  error: null,

  fetchProfile: async () => {
    set({ isLoading: true, error: null });

    try {
      const { isAuthenticated } = useAuthStore.getState();

      if (!isAuthenticated) {
        set({ profile: null, isLoading: false });
        return;
      }

      // Use authFetch for automatic 401 handling (token refresh + logout on failure)
      const response = await authFetch("/api/v1/profile");

      if (response.status === 401) {
        // Token invalid or user doesn't exist — authFetch already tried refresh,
        // if we still get 401, the user is gone. authFetch handles logout.
        set({ profile: null, isLoading: false });
        return;
      }

      if (!response.ok) {
        throw new Error("Failed to fetch profile");
      }

      // Response may be null for new users (no profile yet)
      const profile = await safeJson<UserProfile | null>(response);
      set({ profile: profile || null, isLoading: false });
    } catch (error) {
      console.error("Failed to fetch profile:", error);
      set({
        profile: null,
        isLoading: false,
        error:
          error instanceof Error ? error.message : "Failed to fetch profile",
      });
    }
  },

  updateProfile: async (data: Partial<UserProfile>) => {
    set({ isLoading: true, error: null });

    try {
      const { isAuthenticated } = useAuthStore.getState();

      if (!isAuthenticated) {
        throw new Error("Not authenticated");
      }

      // Use authFetch for automatic 401 handling
      const response = await authFetch("/api/v1/profile", {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(data),
      });

      if (response.status === 401) {
        set({ isLoading: false });
        throw new Error("Session expired, please login again");
      }

      if (!response.ok) {
        throw new Error(await extractErrorMessage(response));
      }

      const profile = await safeJson<UserProfile>(response);
      set({ profile, isLoading: false });
    } catch (error) {
      set({
        isLoading: false,
        error:
          error instanceof Error ? error.message : "Failed to update profile",
      });
      throw error;
    }
  },

  clearError: () => set({ error: null }),
}));
