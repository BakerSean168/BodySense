import { create } from "zustand";
import { getUserProfile, updateUserProfile } from "@/generated/api/bodysense";
import { ApiRequestError } from "@/lib/api-client";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";
import type { UnitPreference } from "@/features/profile/model/profilePreferences";
import { useAuthStore } from "./authStore";

// UserProfile is the minimal optional Profile (PC-D1): display identity and
// preferences. Mutable health state (height, weight, lifestyle, history) lives
// in BodyState-backed projections and is never part of this store.
export interface UserProfile {
  id: string;
  user_id: string;
  display_name?: string;
  avatar_asset_id?: string;
  /** Self-reported; never a physiological or medical determination. */
  gender?: "male" | "female";
  birth_date?: string;
  age_years?: number;
  preferences: {
    locale?: string;
    time_zone?: string;
    unit_preference?: UnitPreference;
  };
  /** Concurrency token for updates; never shown to the user. */
  revision: number;
  created_at: string;
  updated_at: string;
}

/** Every editable field; null means "not provided" (the PUT is a full replacement). */
export interface ProfileEditable {
  display_name: string | null;
  avatar_asset_id: string | null;
  gender: "male" | "female" | null;
  birth_date: string | null;
  locale: string | null;
  time_zone: string | null;
  unit_preference: UnitPreference | null;
}

/** Legacy partial edit (drawer): merged over the current values before saving. */
export type UpdateUserProfileInput = Partial<ProfileEditable>;

/** Raised when the profile changed elsewhere; the store already holds the latest. */
export class ProfileConflictError extends Error {
  constructor() {
    super("资料已在别处更新，已载入最新内容。");
    this.name = "ProfileConflictError";
  }
}

interface ProfileState {
  profile: UserProfile | null;
  isLoading: boolean;
  error: string | null;
  fetchProfile: () => Promise<void>;
  saveProfile: (next: ProfileEditable) => Promise<UserProfile>;
  updateProfile: (data: UpdateUserProfileInput) => Promise<void>;
  clearError: () => void;
}

type ApiProfile = NonNullable<
  Awaited<ReturnType<typeof getUserProfile>>["profile"]
>;

function projectProfile(profile: ApiProfile): UserProfile {
  return {
    id: profile.id,
    user_id: profile.user_id,
    display_name: profile.display_name,
    avatar_asset_id: profile.avatar_asset_id,
    gender: profile.gender,
    birth_date: profile.birth_date,
    age_years: profile.age_years,
    preferences: {
      locale: profile.preferences.locale,
      time_zone: profile.preferences.time_zone,
      unit_preference: profile.preferences.unit_preference,
    },
    revision: profile.revision,
    created_at: profile.created_at,
    updated_at: profile.updated_at,
  };
}

export function editableProfile(profile: UserProfile | null): ProfileEditable {
  return {
    display_name: profile?.display_name ?? null,
    avatar_asset_id: profile?.avatar_asset_id ?? null,
    gender: profile?.gender ?? null,
    birth_date: profile?.birth_date ?? null,
    locale: profile?.preferences.locale ?? null,
    time_zone: profile?.preferences.time_zone ?? null,
    unit_preference: profile?.preferences.unit_preference ?? null,
  };
}

async function readProfile(): Promise<UserProfile | null> {
  const response = await withOpenApiError(() =>
    getUserProfile(undefined, openApiAuthFetch),
  );
  return response.profile ? projectProfile(response.profile) : null;
}

export const useProfileStore = create<ProfileState>()((set, get) => ({
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
      set({ profile: await readProfile(), isLoading: false });
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

  saveProfile: async (next) => {
    const { isAuthenticated } = useAuthStore.getState();
    if (!isAuthenticated) throw new Error("Not authenticated");
    set({ isLoading: true, error: null });
    try {
      const response = await withOpenApiError(() =>
        updateUserProfile(
          {
            expected_revision: get().profile?.revision ?? 0,
            display_name: next.display_name?.trim() || null,
            avatar_asset_id: next.avatar_asset_id,
            gender: next.gender,
            birth_date: next.birth_date || null,
            preferences: {
              locale: next.locale,
              time_zone: next.time_zone,
              unit_preference: next.unit_preference,
            },
          },
          undefined,
          openApiAuthFetch,
        ),
      );
      const saved = projectProfile(response.profile);
      set({ profile: saved, isLoading: false });
      return saved;
    } catch (error) {
      if (error instanceof ApiRequestError && error.status === 409) {
        // Never overwrite a newer profile: reload it and let the user decide.
        const latest = await readProfile().catch(() => get().profile);
        set({ profile: latest, isLoading: false });
        throw new ProfileConflictError();
      }
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

  updateProfile: async (data) => {
    await get().saveProfile({ ...editableProfile(get().profile), ...data });
  },

  clearError: () => set({ error: null }),
}));
