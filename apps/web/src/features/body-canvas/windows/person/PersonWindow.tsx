import { useEffect } from "react";
import { LoaderCircle } from "lucide-react";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { BodyFactRecords } from "../../components/BodyFactRecords";
import {
  getProfileCategoryMeta,
  type ProfileCategoryId,
} from "../../model/profileCategories";
import { PersonRecordsView } from "./categories/records/PersonRecordsView";
import { EquipmentCategory } from "./categories/EquipmentCategory";
import { UploadsCategory } from "./categories/UploadsCategory";
import { PersonDataError } from "./categories/PersonDataError";
import { PersonIdentityPopover } from "./PersonIdentityPopover";
import { usePersonWindowData } from "./hooks/usePersonWindowData";
import type { PersonWindowProps } from "./types";

/**
 * Person/Profile business window.
 *
 * Layout contract (Penpot Desktop / Mobile Person boards):
 * - header: avatar + nickname, a static "/", then the category picker
 *   ("BakerSean / 身体记录"); Close on the right; no duplicate title, no
 *   body-status card and no tab strip;
 * - clicking avatar/nickname opens the Identity popover (minimal optional
 *   Profile: nickname, birth date + self-reported gender, display time zone +
 *   units); outside click closes it;
 * - exactly five categories; the picker label is the only place the category
 *   name appears — no duplicate H1 and no generic explanation beneath it;
 * - adding is only the header "+" (before Close), controlled here by
 *   `addOpen`; empty states carry no add buttons. Equipment grows naturally
 *   (Desktop 4 columns, Mobile 2 columns) with no fixed slot count.
 */
export function PersonWindow({
  canEdit,
  onOpenRecords,
  identityInfoOpen: infoOpen,
  onIdentityInfoOpenChange,
  category: controlledCategory,
  addOpen = false,
  onAddOpenChange,
  onAddLabelChange,
}: PersonWindowProps) {
  const mobile = useMediaQuery("(max-width: 760px)");
  const category: ProfileCategoryId = controlledCategory ?? "records";
  const setInfoOpen = onIdentityInfoOpenChange;
  // The window owns its own reads: nothing loads until the window is open, and
  // a failure is surfaced instead of substituted with placeholder values.
  const data = usePersonWindowData(
    category === "media" || category === "reports",
  );
  const meta = getProfileCategoryMeta(category);

  // Category switching must never leave the identity popover floating over a
  // different category.
  useEffect(() => {
    setInfoOpen(false);
  }, [category]);

  return (
    <section
      className="bc-person"
      aria-label="个人档案"
      data-testid="person-window"
      data-mobile={mobile ? "true" : "false"}
    >
      {infoOpen ? (
        <PersonIdentityPopover
          onClose={() => setInfoOpen(false)}
          onOpenRecords={onOpenRecords}
        />
      ) : null}

      <div
        className="bc-person-body"
        tabIndex={0}
        role="region"
        aria-label={meta.regionLabel}
        data-category={category}
      >
        {category === "records" ? (
          <PersonRecordsView
            canEdit={canEdit}
            addOpen={addOpen}
            onAddOpenChange={onAddOpenChange}
            onAddLabelChange={onAddLabelChange}
          />
        ) : category === "habits" ? (
          <BodyFactRecords
            habitsOnly
            canEdit={canEdit}
            addOpen={addOpen}
            onAddOpenChange={onAddOpenChange}
          />
        ) : category === "equipment" ? (
          <EquipmentCategory
            canEdit={canEdit}
            addOpen={addOpen}
            onAddOpenChange={onAddOpenChange}
          />
        ) : data.loading ? (
          <p role="status" className="bc-person-loading">
            <LoaderCircle size={16} className="animate-spin" />
            正在读取档案
          </p>
        ) : data.error ? (
          <PersonDataError
            message={data.error}
            onRetry={() => void data.retry()}
          />
        ) : category === "media" ? (
          <UploadsCategory
            uploads={data.uploads}
            category="media"
            mobile={mobile}
            addOpen={addOpen}
            onAddOpenChange={onAddOpenChange}
          />
        ) : category === "reports" ? (
          <UploadsCategory
            uploads={data.uploads}
            category="reports"
            addOpen={addOpen}
            onAddOpenChange={onAddOpenChange}
          />
        ) : null}
      </div>
    </section>
  );
}
