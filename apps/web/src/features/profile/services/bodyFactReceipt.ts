import { BodyFactCommit } from "@/generated/api/model/bodyFactCommit.zod";

/** Only a Go-generated commit result can unlock saved feedback/invalidation. */
export function readBodyFactCommit(result: unknown) {
  if (
    !result ||
    typeof result !== "object" ||
    !("status" in result) ||
    result.status !== "persisted" ||
    !("receipt" in result)
  )
    return null;
  const parsed = BodyFactCommit.safeParse(result.receipt);
  return parsed.success ? parsed.data : null;
}
