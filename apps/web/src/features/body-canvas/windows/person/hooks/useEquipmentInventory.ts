import { useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  availabilityChange,
  equipmentKeys,
  equipmentService,
  type EquipmentAsset,
} from "@/features/profile/services/equipmentService";
import { ApiRequestError, errorMessage } from "@/lib/api-client";

export function useEquipmentInventory(canEdit: boolean) {
  const queryClient = useQueryClient();
  const inventory = useQuery({
    queryKey: equipmentKeys.all,
    queryFn: equipmentService.list,
  });
  const equipment = inventory.data?.items ?? [];
  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: equipmentKeys.all });
  const [busyId, setBusyId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);
  const [name, setName] = useState("");
  const [notes, setNotes] = useState("");
  // One key per add attempt, so a network retry replays instead of duplicating.
  const createKey = useRef<string | null>(null);
  const onConflict = (cause: unknown) => {
    if (cause instanceof ApiRequestError && cause.status === 409) {
      void refresh();
    }
  };
  const create = useMutation({
    mutationFn: equipmentService.create,
    onSuccess: refresh,
    onError: onConflict,
  });
  const update = useMutation({
    mutationFn: ({
      asset,
      availability,
    }: {
      asset: EquipmentAsset;
      availability: EquipmentAsset["availability"];
    }) =>
      equipmentService.update(
        asset.id,
        availabilityChange(asset, availability),
      ),
    onSuccess: refresh,
    onError: onConflict,
  });
  const pending = create.isPending || update.isPending;

  const changeAvailability = async (
    asset: EquipmentAsset,
    availability: EquipmentAsset["availability"],
  ) => {
    if (!canEdit || pending) return;
    setBusyId(asset.id);
    setError(null);
    try {
      await update.mutateAsync({ asset, availability });
    } catch (cause) {
      setError(errorMessage(cause, "器材状态没有保存，请重试。"));
    } finally {
      setBusyId(null);
    }
  };

  const resetForm = () => {
    setAdding(false);
    setName("");
    setNotes("");
    createKey.current = null;
  };

  const add = async () => {
    const trimmed = name.trim();
    if (!canEdit || !trimmed || pending) return;
    setError(null);
    if (
      equipment.some(
        (asset) =>
          asset.display_name.toLocaleLowerCase() ===
          trimmed.toLocaleLowerCase(),
      )
    ) {
      setError("这件器材已经在当前清单中。");
      return;
    }
    createKey.current ??= crypto.randomUUID();
    try {
      await create.mutateAsync({
        idempotency_key: createKey.current,
        display_name: trimmed,
        notes: notes.trim() || null,
        access_kind: "owned",
        availability: "available",
      });
      resetForm();
    } catch (cause) {
      setError(errorMessage(cause, "器材没有保存，请重试。"));
    }
  };

  const changeName = (value: string) => {
    setName(value);
    createKey.current = null;
  };
  const changeNotes = (value: string) => {
    setNotes(value);
    createKey.current = null;
  };
  return {
    inventory,
    equipment,
    pending,
    busyId,
    error,
    adding,
    setAdding,
    name,
    changeName,
    notes,
    changeNotes,
    isCreating: create.isPending,
    resetForm,
    add,
    changeAvailability,
  };
}
