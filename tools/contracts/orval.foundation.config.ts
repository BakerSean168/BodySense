import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "orval";

const here = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  foundation: {
    input: { target: path.join(here, "foundation/openapi.yaml") },
    output: {
      client: "fetch",
      mode: "single",
      target: path.join(here, "generated/openapi-web/client.ts"),
      schemas: {
        path: path.join(here, "generated/openapi-web/model"),
        type: "zod",
      },
      override: {
        zod: {
          version: 4,
          variant: "mini",
          strict: { response: true, body: true, param: true },
          generateReusableSchemas: true,
        },
        fetch: {
          runtimeValidation: { strategy: "throw" },
          forceSuccessResponse: true,
          includeHttpResponseReturnType: false,
        },
      },
    },
  },
});
