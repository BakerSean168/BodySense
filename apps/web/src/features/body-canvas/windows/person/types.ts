import type { BodyRegionId } from "@/features/body-explorer/model/bodyRegionOntology";
import type { ProfileCategoryId } from "../../model/profileCategories";

/**
 * Header identity: the Profile nickname, else the account email's local part,
 * else a placeholder (personDisplayName). Never invented.
 */
export interface PersonProfileIdentity {
  nickname: string;
  /** The signed-in account's email; only its local part is displayed. */
  accountEmail?: string;
}

export interface PersonWindowProps {
  canEdit: boolean;
  /** Opens the 身体档案 drawer from the Identity popover. */
  onOpenRecords?: () => void;
  /** Popover state is owned by the header that renders the identity trigger. */
  identityInfoOpen: boolean;
  onIdentityInfoOpenChange: (open: boolean) => void;
  /**
   * Records is the authoritative body-state surface, so the window embeds the
   * existing workbench instead of reimplementing record management. Selecting a
   * region only produces context; it never asserts that a problem exists.
   */
  selectedRegionId?: BodyRegionId | null;
  onSelectRegion?: (regionId: BodyRegionId | null) => void;
  onAskRegion?: (regionId: BodyRegionId) => void;
  /** The header picker owns the category; uncontrolled use keeps local state. */
  category?: ProfileCategoryId;
  onCategoryChange?: (category: ProfileCategoryId) => void;
}
