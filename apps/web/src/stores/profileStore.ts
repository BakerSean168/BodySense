import { create } from "zustand";
import { authFetch } from "@/features/auth/services/authService";
import { useAuthStore } from "./authStore";
import { safeJson, extractErrorMessage } from "@/lib/api-url";

// UserProfile intentionally contains stable identity context only. Mutable
// health state lives in BodyState-backed projections such as body metrics and
// lifestyle.
export interface UserProfile {
  id: string;
  user_id: string;
  gender?: string;
  birth_date?: string;
  age_years?: number;
  created_at: string;
  updated_at: string;
}

interface ProfileState {
  profile: UserProfile | null;
  isLoading: boolean;
  error: string | null;
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
      const response = await authFetch("/api/v1/profile");
      if (response.status === 401) {
        set({ profile: null, isLoading: false });
        return;
      }
      if (!response.ok) throw new Error("Failed to fetch profile");
      const profile = await safeJson<UserProfile | null>(response);
      set({ profile: profile || null, isLoading: false });
    } catch (error) {
      console.error("Failed to fetch profile:", error);
      set({
        profile: null,
        isLoading: false,
        error: error instanceof Error ? error.message : "Failed to fetch profile",
      });
    }
  },

  updateProfile: async (data: Partial<UserProfile>) => {
    set({ isLoading: true, error: null });
    try {
      const { isAuthenticated } = useAuthStore.getState();
      if (!isAuthenticated) throw new Error("Not authenticated");
      const response = await authFetch("/api/v1/profile", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(data),
      });
      if (response.status === 401) {
        set({ isLoading: false });
        throw new Error("Session expired, please login again");
      }
      if (!response.ok) throw new Error(await extractErrorMessage(response));
      const profile = await safeJson<UserProfile>(response);
      set({ profile, isLoading: false });
    } catch (error) {
      set({
        isLoading: false,
        error: error instanceof Error ? error.message : "Failed to update profile",
      });
      throw error;
    }
  },

  clearError: () => set({ error: null }),
}));
