import { defineConfig } from "orval";

export default defineConfig({
  bodysensePublicApi: {
    input: {
      target: "../../packages/contracts/openapi/bodysense.v1.openapi.yaml",
    },
    output: {
      client: "fetch",
      mode: "single",
      target: "./src/generated/api/bodysense.ts",
      schemas: { path: "./src/generated/api/model", type: "zod" },
      clean: true,
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
          useRuntimeFetcher: true,
        },
      },
    },
  },
});
