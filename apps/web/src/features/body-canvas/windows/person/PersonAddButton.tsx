import { Plus } from "lucide-react";
import type { ProfileCategoryId } from "../../model/profileCategories";
import { IconTip } from "@/components/ui/tooltip";

/** What the header "+" does in each category (also its accessible name). */
export const PERSON_ADD_LABELS: Record<ProfileCategoryId, string> = {
  records: "补充记录",
  habits: "补充生活习惯",
  equipment: "添加器材",
  media: "上传照片",
  reports: "上传报告",
};

/**
 * The only add entry of the Person window: an icon button in the header,
 * before Close. Empty states and lists carry no add buttons of their own.
 */
export function PersonAddButton({
  category,
  label: labelOverride,
  disabled,
  onAdd,
}: {
  category: ProfileCategoryId;
  /** A subview's own add action (我的猜测) replaces the category default. */
  label?: string | null;
  disabled?: boolean;
  onAdd: () => void;
}) {
  const label = labelOverride ?? PERSON_ADD_LABELS[category];
  return (
    <IconTip label={label}>
      <button
        type="button"
        className="bc-icon-button bc-header-add"
        aria-label={label}
        disabled={disabled}
        onClick={onAdd}
        data-testid="person-add"
      >
        <Plus size={19} strokeWidth={1.9} aria-hidden="true" />
      </button>
    </IconTip>
  );
}
