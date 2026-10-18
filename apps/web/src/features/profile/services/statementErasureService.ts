import {
  getStatementErasure,
  requestStatementErasure,
} from "@/generated/api/bodysense";
import { RequestStatementErasureRequest } from "@/generated/api/model/requestStatementErasureRequest.zod";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";

// AS-002 per-statement erasure. The generated contract is the DTO authority;
// a repeated request (same statement, any key) retries the same erasure.
export type StatementErasureReceipt = Awaited<
  ReturnType<typeof requestStatementErasure>
>;

export const statementErasureService = {
  request: (
    statementId: string,
    idempotencyKey: string,
  ): Promise<StatementErasureReceipt> =>
    withOpenApiError(() =>
      requestStatementErasure(
        statementId,
        RequestStatementErasureRequest.parse({
          idempotency_key: idempotencyKey,
        }),
        undefined,
        openApiAuthFetch,
      ),
    ),
  receipt: (statementId: string): Promise<StatementErasureReceipt> =>
    withOpenApiError(() =>
      getStatementErasure(statementId, undefined, openApiAuthFetch),
    ),
};
