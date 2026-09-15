import { create } from "zustand";
import { useAuthStore } from "./authStore";
import type {
  FileType,
  UserUpload,
} from "@/features/profile/types/upload.types";
import {
  parseOCRResult,
  parsePostureAnalysis,
} from "@/features/profile/types/upload.parsers";
import {
  createUpload,
  deleteUpload,
  listUploads,
} from "@/generated/api/bodysense";
import { CreateUploadRequest as CreateUploadRequestSchema } from "@/generated/api/model/createUploadRequest.zod";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";

type UploadWire = Awaited<ReturnType<typeof createUpload>>;

function toUserUpload(upload: UploadWire): UserUpload {
  return {
    id: upload.id,
    file_type: upload.file_type,
    original_name: upload.original_name,
    file_size: upload.file_size,
    mime_type: upload.mime_type,
    ocr_result: parseOCRResult(upload.ocr_result),
    ocr_status: upload.ocr_status,
    analysis_status: upload.analysis_status,
    analysis_result: parsePostureAnalysis(upload.analysis_result),
    created_at: upload.created_at,
    updated_at: upload.updated_at,
  };
}

interface UploadState {
  uploads: UserUpload[];
  isLoading: boolean;
  error: string | null;

  fetchUploads: () => Promise<void>;
  uploadFile: (file: File, fileType: FileType) => Promise<UserUpload>;
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

      const response = await withOpenApiError(() =>
        listUploads(undefined, openApiAuthFetch),
      );
      set({ uploads: response.uploads.map(toUserUpload), isLoading: false });
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

  uploadFile: async (file: File, fileType: FileType) => {
    set({ isLoading: true, error: null });

    try {
      const { accessToken } = useAuthStore.getState();
      if (!accessToken) {
        throw new Error("Not authenticated");
      }

      const upload = toUserUpload(
        await withOpenApiError(() =>
          createUpload(
            CreateUploadRequestSchema.parse({ file, file_type: fileType }),
            undefined,
            openApiAuthFetch,
          ),
        ),
      );
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

      await withOpenApiError(() =>
        deleteUpload(id, undefined, openApiAuthFetch),
      );
      set((state) => ({
        uploads: state.uploads.filter((upload) => upload.id !== id),
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
