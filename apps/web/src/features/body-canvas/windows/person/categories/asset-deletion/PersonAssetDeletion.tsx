import type { ProfileUploadLike } from "../../../../model/profileCategories";
import { AssetDeletionChoiceScenes } from "./AssetDeletionChoiceScenes";
import { AssetDeletionResultScenes } from "./AssetDeletionResultScenes";
import { useAssetDeletionFlow } from "./useAssetDeletionFlow";

/**
 * Stable deletion-window shell shared by the choice and receipt scenes.
 * The element, keyboard exit and data-phase must persist across scene changes.
 */
export function PersonAssetDeletion({
  upload,
  backLabel,
  onBack,
  onErased,
}: {
  upload: ProfileUploadLike;
  backLabel: string;
  onBack: () => void;
  /** Only after the verified erasure receipt and user's confirmation. */
  onErased: (assetId: string) => void;
}) {
  const flow = useAssetDeletionFlow(upload);
  const choosing =
    flow.scene === "scope" ||
    flow.scene === "confirm-source" ||
    flow.scene === "confirm-erase";
  const phase =
    flow.scene === "source-removed"
      ? "source_removed"
      : flow.scene === "receipt"
        ? (flow.view?.phase ?? "in_progress")
        : flow.scene;

  return (
    <div
      className="bc-person-category bc-person-detail bc-asset-scene"
      data-testid="person-asset-deletion"
      data-scene={flow.scene}
      data-phase={phase}
      onKeyDown={(event) => {
        if (event.key === "Escape" && flow.scene === "scope" && !flow.working) {
          event.preventDefault();
          event.stopPropagation();
          onBack();
        }
      }}
    >
      {choosing ? (
        <AssetDeletionChoiceScenes
          upload={upload}
          flow={flow}
          onBack={onBack}
        />
      ) : (
        <AssetDeletionResultScenes
          upload={upload}
          flow={flow}
          backLabel={backLabel}
          onBack={onBack}
          onErased={onErased}
        />
      )}
    </div>
  );
}
