import { useEffect, useState } from "react";
import { useUploadStore } from "@/stores/uploadStore";
import type { ProfileUploadLike } from "../model/profileCategories";

export interface PersonWindowData {
  uploads: ProfileUploadLike[];
  loading: boolean;
  error: string | null;
}

/**
 * Ancillary Person reads (Media/Reports uploads). Records/Habits use the
 * BodyFact ledger, and the identity popover reads the Profile store; body
 * measurements are BodyState records, not identity, and are not loaded here.
 */
export function usePersonWindowData(): PersonWindowData {
  const uploads = useUploadStore((state) => state.uploads);
  const fetchUploads = useUploadStore((state) => state.fetchUploads);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        await fetchUploads();
      } catch {
        if (!cancelled) setError("部分文件暂时无法读取。");
      }
      if (!cancelled) setLoading(false);
    })();
    return () => {
      cancelled = true;
    };
  }, [fetchUploads]);

  return { uploads: uploads as ProfileUploadLike[], loading, error };
}
