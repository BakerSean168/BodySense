import {
  createEquipmentAsset,
  listEquipmentAssets,
  updateEquipmentAsset,
} from "@/generated/api/bodysense";
import { CreateEquipmentAssetRequest } from "@/generated/api/model/createEquipmentAssetRequest.zod";
import { UpdateEquipmentAssetRequest } from "@/generated/api/model/updateEquipmentAssetRequest.zod";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";

// EquipmentAsset (PC-D2) is the user's own resource inventory, not BodyState.
// The generated public contract is the sole browser DTO authority.
export type EquipmentAssetList = Awaited<
  ReturnType<typeof listEquipmentAssets>
>;
export type EquipmentAsset = EquipmentAssetList["items"][number];
export type EquipmentAssetCreateInput = Parameters<
  typeof createEquipmentAsset
>[0];
export type EquipmentAssetUpdateInput = Parameters<
  typeof updateEquipmentAsset
>[1];

export const equipmentKeys = {
  all: ["equipment"] as const,
};

export const equipmentService = {
  list: (): Promise<EquipmentAssetList> =>
    withOpenApiError(() => listEquipmentAssets(undefined, openApiAuthFetch)),

  create: (input: EquipmentAssetCreateInput): Promise<EquipmentAsset> =>
    withOpenApiError(() =>
      createEquipmentAsset(
        CreateEquipmentAssetRequest.parse(input),
        undefined,
        openApiAuthFetch,
      ),
    ),

  update: (
    id: string,
    input: EquipmentAssetUpdateInput,
  ): Promise<EquipmentAsset> =>
    withOpenApiError(() =>
      updateEquipmentAsset(
        id,
        UpdateEquipmentAssetRequest.parse(input),
        undefined,
        openApiAuthFetch,
      ),
    ),
};

/** The update body that keeps every field and changes only availability. */
export function availabilityChange(
  asset: EquipmentAsset,
  availability: EquipmentAssetUpdateInput["availability"],
): EquipmentAssetUpdateInput {
  return {
    expected_revision: asset.revision,
    display_name: asset.display_name,
    notes: asset.notes,
    access_kind: asset.access_kind,
    availability,
    valid_from: asset.valid_from,
    valid_until: asset.valid_until,
  };
}
