import type { ProfileCategoryId } from "../../model/profileCategories";
import { PersonCategoryMenu } from "./PersonCategoryMenu";
import { PersonIdentityTrigger } from "./PersonIdentityTrigger";
import { usePersonCategoryCounts } from "./hooks/usePersonCategoryCounts";
import type { PersonProfileIdentity } from "./types";

/**
 * Penpot Person header: "BakerSean / 身体记录 ▾". The category picker replaces
 * the old tab strip; it opens Penpot's icon menu, built as an ARIA listbox.
 */
export function PersonWindowHeader({
  identity,
  open,
  onToggle,
  onOpenSettings,
  onRequestLogout,
  category,
  onCategoryChange,
}: {
  identity: PersonProfileIdentity;
  open: boolean;
  onToggle: () => void;
  onOpenSettings?: () => void;
  onRequestLogout?: () => void;
  category: ProfileCategoryId;
  onCategoryChange: (category: ProfileCategoryId) => void;
}) {
  const counts = usePersonCategoryCounts();
  return (
    <div className="bc-window-context bc-person-context">
      <PersonIdentityTrigger
        identity={identity}
        open={open}
        onToggle={onToggle}
        onOpenSettings={onOpenSettings}
        onRequestLogout={onRequestLogout}
      />
      <span className="bc-window-context-sep" aria-hidden="true">
        /
      </span>
      <PersonCategoryMenu
        value={category}
        onChange={onCategoryChange}
        counts={counts}
      />
    </div>
  );
}
