import { useCallback, useEffect, useRef, useState } from "react";
import { useUploadStore } from "@/stores/uploadStore";
import type { ProfileUploadLike } from "../../../model/profileCategories";

export interface PersonWindowData {
  uploads: ProfileUploadLike[];
  loading: boolean;
  error: string | null;
  retry: () => Promise<void>;
}

/**
 * Media and Reports share the same uploads read; neither category should
 * request the same list independently. The Store intentionally catches
 * network failures, so its error state is part of this hook's result.
 */
export function usePersonWindowData(enabled = true): PersonWindowData {
  const uploads = useUploadStore((state) => state.uploads);
  const readError = useUploadStore((state) =>
    state.hasFetched ? null : state.error,
  );
  const fetchUploads = useUploadStore((state) => state.fetchUploads);
  const started = useRef(false);
  const [loading, setLoading] = useState(true);
  const [requestError, setRequestError] = useState<string | null>(null);

  const retry = useCallback(async () => {
    setLoading(true);
    setRequestError(null);
    try {
      await fetchUploads();
    } catch {
      // Third-party implementations may throw rather than populate the Store.
      setRequestError("部分文件暂时无法读取。");
    } finally {
      setLoading(false);
    }
  }, [fetchUploads]);

  useEffect(() => {
    if (!enabled || started.current) return;
    started.current = true;
    void retry();
  }, [enabled, retry]);

  return {
    uploads: uploads as ProfileUploadLike[],
    loading,
    error: requestError || (readError ? "部分文件暂时无法读取。" : null),
    retry,
  };
}
