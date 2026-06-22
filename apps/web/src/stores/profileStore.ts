import { create } from 'zustand';
import { useAuthStore } from './authStore';

export interface UserProfile {
  id: string;
  user_id: string;
  gender?: string;
  age?: number;
  height_cm?: number;
  weight_kg?: number;
  bmi?: number;
  occupation?: string;
  sleep_time?: string;
  wake_time?: string;
  exercise_type?: string;
  exercise_frequency?: string;
  injury_history?: string;
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

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080';

export const useProfileStore = create<ProfileState>()((set) => ({
  profile: null,
  isLoading: false,
  error: null,

  fetchProfile: async () => {
    set({ isLoading: true, error: null });

    try {
      const { accessToken } = useAuthStore.getState();

      if (!accessToken) {
        set({ profile: null, isLoading: false });
        return;
      }

      const response = await fetch(`${API_BASE_URL}/api/v1/profile`, {
        headers: {
          Authorization: `Bearer ${accessToken}`,
        },
      });

      if (!response.ok) {
        throw new Error('Failed to fetch profile');
      }

      // Response may be null for new users (no profile yet)
      const profile = await response.json();
      set({ profile: profile || null, isLoading: false });
    } catch (error) {
      console.error('Failed to fetch profile:', error);
      set({
        profile: null,
        isLoading: false,
        error: error instanceof Error ? error.message : 'Failed to fetch profile',
      });
    }
  },

  updateProfile: async (data: Partial<UserProfile>) => {
    set({ isLoading: true, error: null });

    try {
      const { accessToken } = useAuthStore.getState();

      if (!accessToken) {
        throw new Error('Not authenticated');
      }

      const response = await fetch(`${API_BASE_URL}/api/v1/profile`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${accessToken}`,
        },
        body: JSON.stringify(data),
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || 'Failed to update profile');
      }

      const profile = await response.json();
      set({ profile, isLoading: false });
    } catch (error) {
      set({
        isLoading: false,
        error: error instanceof Error ? error.message : 'Failed to update profile',
      });
      throw error;
    }
  },

  clearError: () => set({ error: null }),
}));
