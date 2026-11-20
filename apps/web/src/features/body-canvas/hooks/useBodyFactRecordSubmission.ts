import { useCallback, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  bodyFactRecordsService,
  type BodyFactRecordInput,
  type BodyFactRecordsPage,
} from "@/features/profile/services/bodyFactRecordsService";
import { ApiRequestError, errorMessage } from "@/lib/api-client";
import { type BodyFactKind as FactKind } from "../model/bodyFactPresentation";

/**
 * One durable statement write plus the receipt state the surface needs so an
 * older snapshot is never shown as current. `pending` keeps the exact command
 * across an uncertain transport failure instead of creating a second fact.
 */
export function useBodyFactRecordSubmission({
  owner,
  ownerRef,
  canEdit,
  page,
  pagesDisagree,
  habitsOnly,
  kind,
  statement,
  when,
  onSaved,
  refetch,
}: {
  owner: string | undefined;
  /** Commit identity before asynchronous receipts can update this surface. */
  ownerRef: { current: string | undefined };
  canEdit: boolean;
  page: BodyFactRecordsPage | undefined;
  pagesDisagree: boolean;
  habitsOnly: boolean;
  kind: FactKind;
  statement: string;
  when: string;
  /** Clear the draft and close the form after a durable receipt. */
  onSaved: () => void;
  refetch: () => void;
}) {
  const queryClient = useQueryClient();
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  // A durable receipt can arrive before an invalidated ledger query refetches.
  // Do not call an older snapshot empty or use its revision for another write.
  const [savedRevision, setSavedRevision] = useState<number | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const pending = useRef<{
    owner: string;
    content: string;
    command: BodyFactRecordInput;
  } | null>(null);

  const resetPending = useCallback(() => {
    pending.current = null;
  }, []);

  const reset = useCallback(() => {
    pending.current = null;
    setNotice(null);
    setSavedRevision(null);
    setSaveError(null);
    setSaving(false);
  }, []);

  const snapshotBehindCommit =
    savedRevision !== null && (page?.current_revision ?? -1) < savedRevision;

  async function submit() {
    if (
      !owner ||
      !canEdit ||
      !page ||
      pagesDisagree ||
      snapshotBehindCommit ||
      saving ||
      !statement.trim()
    )
      return;
    const submittingOwner = owner;
    const effectiveKind = habitsOnly ? "lifestyle" : kind;
    const content = JSON.stringify([
      effectiveKind,
      statement.trim(),
      when.trim(),
    ]);
    if (
      pending.current?.owner !== owner ||
      pending.current.content !== content
    ) {
      pending.current = {
        owner,
        content,
        command: {
          idempotency_key: crypto.randomUUID(),
          expected_revision: page.current_revision,
          facts: [
            {
              kind: effectiveKind,
              statement: statement.trim(),
              body_region_ids: [],
              temporal: {
                precision: "unknown",
                ...(when.trim() ? { original_expression: when.trim() } : {}),
              },
              details: {},
            },
          ],
        },
      };
    }
    const command = pending.current.command;
    setSaving(true);
    setSaveError(null);
    setNotice(null);
    try {
      const receipt = await bodyFactRecordsService.record(command);
      if (ownerRef.current !== submittingOwner) return;
      if (receipt.user_id !== submittingOwner)
        throw new Error("保存回执的档案身份不一致。");
      // The write is committed before refetch. A failed read must never turn it
      // into another write or claim that persistence failed.
      pending.current = null;
      onSaved();
      setSavedRevision(receipt.revision);
      setNotice("已保存");
      void queryClient.invalidateQueries({ queryKey: ["workspace"] });
    } catch (cause) {
      if (ownerRef.current !== submittingOwner) return;
      if (cause instanceof ApiRequestError && cause.status === 409) {
        pending.current = null;
        setSaveError("记录已发生变化。已重新读取，请核对内容后再次保存。");
        refetch();
      } else {
        setSaveError(
          errorMessage(
            cause,
            "尚未确认保存结果。重试会使用同一条请求，不会重复创建。",
          ),
        );
      }
    } finally {
      if (ownerRef.current === submittingOwner) setSaving(false);
    }
  }

  return {
    submit,
    saving,
    saveError,
    notice,
    setNotice,
    savedRevision,
    setSavedRevision,
    setSaveError,
    snapshotBehindCommit,
    reset,
    resetPending,
  };
}
