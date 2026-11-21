import {
  bodyFactRecordsService,
  type BodyFactRecordsPage,
} from "@/features/profile/services/bodyFactRecordsService";

/** One live, user-scoped ledger read; Habits is a filtered view of the same key. */
export const bodyFactRecordsKey = (owner: string | undefined) =>
  ["workspace", "body-fact-records", owner ?? "signed-out"] as const;

/**
 * One page of the owner's ledger. A page that names another identity or a
 * non-advancing cursor is rejected rather than displayed or written against.
 */
export async function fetchBodyFactRecordsPage(
  owner: string | undefined,
  beforeRevision: number | undefined,
): Promise<BodyFactRecordsPage> {
  const page = await bodyFactRecordsService.list(beforeRevision);
  if (page.user_id !== owner) throw new Error("档案身份已变化，请重新读取。");
  if (
    page.has_more &&
    (!page.next_before_revision ||
      (beforeRevision !== undefined &&
        page.next_before_revision >= beforeRevision))
  ) {
    throw new Error("记录分页信息无效，请重新读取。");
  }
  return page;
}
