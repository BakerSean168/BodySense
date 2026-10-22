import { useEffect, useMemo, useRef, useState } from "react";
import {
  assetErasureService,
  type AssetErasureReceipt,
} from "@/features/profile/services/assetErasureService";
import {
  groupAssetErasure,
  projectAssetErasure,
} from "@/features/profile/components/uploads/assetErasureViewModel";
import { errorMessage } from "@/lib/api-client";
import { useUploadStore } from "@/stores/uploadStore";
import type { ProfileUploadLike } from "../../../../model/profileCategories";

export type AssetDeletionScene =
  | "scope"
  | "confirm-source"
  | "confirm-erase"
  | "source-removed"
  | "receipt";

/**
 * Keep erasure commands and the single idempotency key mounted across all
 * scenes. The UI may only report full erasure after a verified server receipt.
 */
export function useAssetDeletionFlow(upload: ProfileUploadLike) {
  const removeSource = useUploadStore((state) => state.deleteUpload);
  const live = useUploadStore((state) =>
    state.uploads.find((item) => item.id === upload.id),
  );
  const sourceGone =
    (live?.source_availability ?? upload.source_availability) ===
    "source_unavailable";
  const [scene, setScene] = useState<AssetDeletionScene>(
    sourceGone ? "source-removed" : "scope",
  );
  const [scope, setScope] = useState<"source" | "erase">("source");
  const [receipt, setReceipt] = useState<AssetErasureReceipt | null>(null);
  const [working, setWorking] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const key = useRef(crypto.randomUUID());
  const mounted = useRef(true);

  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);

  const view = useMemo(
    () => (receipt ? projectAssetErasure(receipt) : null),
    [receipt],
  );
  const families = useMemo(
    () => (receipt ? groupAssetErasure(receipt) : []),
    [receipt],
  );

  async function confirmSourceRemoval() {
    setWorking(true);
    setError(null);
    try {
      await removeSource(upload.id);
      if (mounted.current) setScene("source-removed");
    } catch (cause) {
      if (mounted.current)
        setError(errorMessage(cause, "原件没有移除，请重试。"));
    } finally {
      if (mounted.current) setWorking(false);
    }
  }

  async function requestErasure() {
    setWorking(true);
    setError(null);
    try {
      const next = await assetErasureService.request(upload.id, key.current);
      if (!mounted.current) return;
      if (next.asset_id !== upload.id)
        throw new Error("清理回执与这个文件不一致。");
      setReceipt(next);
      setScene("receipt");
    } catch (cause) {
      if (mounted.current)
        setError(
          errorMessage(cause, "尚未确认清理结果。重试会继续同一次清理。"),
        );
    } finally {
      if (mounted.current) setWorking(false);
    }
  }

  return {
    scene,
    setScene,
    scope,
    setScope,
    receipt,
    working,
    error,
    sourceGone,
    view,
    families,
    confirmSourceRemoval,
    requestErasure,
  };
}

export type AssetDeletionFlow = ReturnType<typeof useAssetDeletionFlow>;
