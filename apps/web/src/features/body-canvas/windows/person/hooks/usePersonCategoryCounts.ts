import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import {
  equipmentKeys,
  equipmentService,
} from "@/features/profile/services/equipmentService";
import { bodyFactRecordsKey } from "../../../components/BodyFactRecords";
import { bodyFactRecordsService } from "@/features/profile/services/bodyFactRecordsService";
import { projectBodyFactSurface } from "../../../model/bodyFactViewModel";
import { useAuthStore } from "@/stores/authStore";
import { useUploadStore } from "@/stores/uploadStore";
import {
  selectUploadsByCategory,
  type ProfileCategoryId,
  type ProfileUploadLike,
} from "../../../model/profileCategories";

/**
 * Counts for the category menu, only from data the app already read: the
 * upload list once fetched, the equipment inventory, and the body-fact ledger
 * (records = every fact, habits = lifestyle facts, the same split the two
 * categories render). A ledger count is shown only once every page is loaded
 * at one revision; a partial or stale read shows no number, never a guess.
 */
export function usePersonCategoryCounts(): Partial<
  Record<ProfileCategoryId, string>
> {
  const uploads = useUploadStore(
    (state) => state.uploads,
  ) as ProfileUploadLike[];
  const fetched = useUploadStore((state) => state.hasFetched);
  const owner = useAuthStore((state) => state.user?.id);
  const equipment = useQuery({
    queryKey: equipmentKeys.all,
    queryFn: equipmentService.list,
    enabled: Boolean(owner),
  });
  // Read-only view of the ledger the Records / Habits categories fetch.
  const ledger = useInfiniteQuery({
    queryKey: bodyFactRecordsKey(owner),
    initialPageParam: undefined as number | undefined,
    queryFn: async ({ pageParam }) => {
      const page = await bodyFactRecordsService.list(pageParam);
      if (page.user_id !== owner)
        throw new Error("档案身份已变化，请重新读取。");
      return page;
    },
    getNextPageParam: (page) =>
      page.has_more ? page.next_before_revision : undefined,
    enabled: false,
  });
  const counts: Partial<Record<ProfileCategoryId, string>> = {};
  if (fetched) {
    counts.media = `${selectUploadsByCategory(uploads, "media").length} 个`;
    counts.reports = `${selectUploadsByCategory(uploads, "reports").length} 份`;
  }
  if (equipment.data) counts.equipment = `${equipment.data.items.length} 件`;
  const pages = ledger.data?.pages;
  if (owner && pages?.length && pages.every((page) => page.user_id === owner)) {
    const surface = (kind: "records" | "habits") =>
      projectBodyFactSurface({
        pages,
        surface: kind,
        hasOwner: true,
        isPending: false,
        isError: false,
      });
    const records = surface("records");
    const habits = surface("habits");
    if (!records.hasMore && !records.pagesDisagree) {
      counts.records = `${records.records.length} 条`;
      counts.habits = `${habits.records.length} 项`;
    }
  }
  return counts;
}
