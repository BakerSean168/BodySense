import { create } from "zustand";
import { getUserProfile, updateUserProfile } from "@/generated/api/bodysense";
import { ApiRequestError } from "@/lib/api-client";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";
import { useAuthStore } from "./authStore";

// UserProfile intentionally contains stable identity context only. Mutable
// health state lives in BodyState-backed projections such as body metrics and
// lifestyle.
export interface UserProfile {
  id: string;
  user_id: string;
  gender?: "male" | "female";
  birth_date?: string;
  age_years?: number;
  created_at: string;
  updated_at: string;
}

export interface UpdateUserProfileInput {
  gender?: "male" | "female" | null;
  birth_date?: string | null;
}

interface ProfileState {
  profile: UserProfile | null;
  isLoading: boolean;
  error: string | null;
  fetchProfile: () => Promise<void>;
  updateProfile: (data: UpdateUserProfileInput) => Promise<void>;
  clearError: () => void;
}

function projectProfile(
  profile: NonNullable<Awaited<ReturnType<typeof getUserProfile>>["profile"]>,
): UserProfile {
  return {
    id: profile.id,
    user_id: profile.user_id,
    gender: profile.gender,
    birth_date: profile.birth_date,
    age_years: profile.age_years,
    created_at: profile.created_at,
    updated_at: profile.updated_at,
  };
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
      const response = await withOpenApiError(() =>
        getUserProfile(undefined, openApiAuthFetch),
      );
      set({
        profile: response.profile ? projectProfile(response.profile) : null,
        isLoading: false,
      });
    } catch (error) {
      if (error instanceof ApiRequestError && error.status === 401) {
        set({ profile: null, isLoading: false });
        return;
      }
      console.error("Failed to fetch profile:", error);
      set({
        profile: null,
        isLoading: false,
        error:
          error instanceof Error ? error.message : "Failed to fetch profile",
      });
    }
  },

  updateProfile: async (data) => {
    set({ isLoading: true, error: null });
    try {
      const { isAuthenticated } = useAuthStore.getState();
      if (!isAuthenticated) throw new Error("Not authenticated");
      const response = await withOpenApiError(() =>
        updateUserProfile(data, undefined, openApiAuthFetch),
      );
      set({ profile: projectProfile(response.profile), isLoading: false });
    } catch (error) {
      const message =
        error instanceof ApiRequestError && error.status === 401
          ? "Session expired, please login again"
          : error instanceof Error
            ? error.message
            : "Failed to update profile";
      set({ isLoading: false, error: message });
      throw error;
    }
  },

  clearError: () => set({ error: null }),
}));
