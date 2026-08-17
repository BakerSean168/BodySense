import { describe, expect, it } from "vitest";
import { expectJson } from "./api-client";

describe("expectJson", () => {
  it("preserves structured API error code and message", async () => {
    const response = new Response(
      JSON.stringify({
        error: {
          code: "BODY_STATE_REVISION_CONFLICT",
          message: "state changed",
        },
      }),
      {
        status: 409,
        headers: { "content-type": "application/json" },
      },
    );

    await expect(expectJson(response)).rejects.toMatchObject({
      status: 409,
      code: "BODY_STATE_REVISION_CONFLICT",
      message: "state changed",
    });
  });
});
