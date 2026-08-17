import { create } from "zustand";
import { useAuthStore } from "./authStore";
import type { UserUpload } from "@/features/profile/types/upload.types";
import { apiUrl, safeJson, extractErrorMessage } from "@/lib/api-url";

interface UploadState {
  uploads: UserUpload[];
  isLoading: boolean;
  error: string | null;

  // Actions
  fetchUploads: () => Promise<void>;
  uploadFile: (file: File, fileType: string) => Promise<UserUpload>;
  deleteUpload: (id: string) => Promise<void>;
  clearError: () => void;
}

export const useUploadStore = create<UploadState>()((set) => ({
  uploads: [],
  isLoading: false,
  error: null,

  fetchUploads: async () => {
    set({ isLoading: true, error: null });

    try {
      const { accessToken } = useAuthStore.getState();

      if (!accessToken) {
        set({ uploads: [], isLoading: false });
        return;
      }

      const response = await fetch(apiUrl("/api/v1/uploads"), {
        headers: {
          Authorization: `Bearer ${accessToken}`,
        },
      });

      if (!response.ok) {
        throw new Error("Failed to fetch uploads");
      }

      const uploads = await safeJson<UserUpload[]>(response);
      set({ uploads: uploads || [], isLoading: false });
    } catch (error) {
      console.error("Failed to fetch uploads:", error);
      set({
        uploads: [],
        isLoading: false,
        error:
          error instanceof Error ? error.message : "Failed to fetch uploads",
      });
    }
  },

  uploadFile: async (file: File, fileType: string) => {
    set({ isLoading: true, error: null });

    try {
      const { accessToken } = useAuthStore.getState();

      if (!accessToken) {
        throw new Error("Not authenticated");
      }

      const formData = new FormData();
      formData.append("file", file);
      formData.append("file_type", fileType);

      const response = await fetch(apiUrl("/api/v1/uploads"), {
        method: "POST",
        headers: {
          Authorization: `Bearer ${accessToken}`,
        },
        body: formData,
      });

      if (!response.ok) {
        throw new Error(await extractErrorMessage(response));
      }

      const upload = await safeJson<UserUpload>(response);
      set((state) => ({
        uploads: [upload, ...state.uploads],
        isLoading: false,
      }));

      return upload;
    } catch (error) {
      set({
        isLoading: false,
        error: error instanceof Error ? error.message : "Failed to upload file",
      });
      throw error;
    }
  },

  deleteUpload: async (id: string) => {
    set({ isLoading: true, error: null });

    try {
      const { accessToken } = useAuthStore.getState();

      if (!accessToken) {
        throw new Error("Not authenticated");
      }

      const response = await fetch(apiUrl(`/api/v1/uploads/${id}`), {
        method: "DELETE",
        headers: {
          Authorization: `Bearer ${accessToken}`,
        },
      });

      if (!response.ok) {
        throw new Error(await extractErrorMessage(response));
      }

      set((state) => ({
        uploads: state.uploads.filter((u) => u.id !== id),
        isLoading: false,
      }));
    } catch (error) {
      set({
        isLoading: false,
        error:
          error instanceof Error ? error.message : "Failed to delete upload",
      });
      throw error;
    }
  },

  clearError: () => set({ error: null }),
}));
