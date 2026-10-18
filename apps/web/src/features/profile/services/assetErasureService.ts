import {
  getAssetErasure,
  requestAssetErasure,
} from "@/generated/api/bodysense";
import { RequestAssetErasureRequest } from "@/generated/api/model/requestAssetErasureRequest.zod";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";

// AS-002 per-asset erasure. The generated contract is the DTO authority; a
// repeated request (same asset, any key) retries the same erasure.
export type AssetErasureReceipt = Awaited<
  ReturnType<typeof requestAssetErasure>
>;

export const assetErasureService = {
  request: (
    assetId: string,
    idempotencyKey: string,
  ): Promise<AssetErasureReceipt> =>
    withOpenApiError(() =>
      requestAssetErasure(
        assetId,
        RequestAssetErasureRequest.parse({ idempotency_key: idempotencyKey }),
        undefined,
        openApiAuthFetch,
      ),
    ),
  receipt: (assetId: string): Promise<AssetErasureReceipt> =>
    withOpenApiError(() =>
      getAssetErasure(assetId, undefined, openApiAuthFetch),
    ),
};
