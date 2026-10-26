import tseslint from "typescript-eslint";

// B8 (VNext Oxc evaluation): Oxlint is the primary lint gate. Its rule set
// in .oxlintrc.json mirrors, rule for rule, the error-level rules this
// config used to enforce via @eslint/js + typescript-eslint recommended —
// equivalence was established empirically with a probe suite, not assumed
// (see docs/architecture/delivery-platform-v3.md, B8 section).
//
// ESLint remains only for the single rule oxlint 1.89 cannot yet enforce
// with this repo's options: `preserve-caught-error` never fires in oxlint
// for `{ requireCatchParameter: false }` (throwing a new error inside a
// catch block without attaching the caught error as `cause`). The rule is
// syntactic, so this config intentionally runs without type information
// (no projectService), which keeps the residual ESLint pass fast.
// If a future oxlint implements that rule faithfully, delete this config
// and the Nx lint targets become pure oxlint runs.
export default tseslint.config(
  {
    ignores: [
      "**/dist/**",
      "**/storybook-static/**",
      "**/node_modules/**",
      "**/.nx/**",
      "**/coverage/**",
      "**/generated/**",
      "tools/contracts/generated/**",
      "apps/web/e2e/**",
      "**/*.config.{js,mjs,cjs,ts,mts,cts}",
    ],
  },
  {
    files: ["**/*.{ts,tsx,mts,cts}"],
    languageOptions: {
      parser: tseslint.parser,
    },
    rules: {
      "preserve-caught-error": [
        "error",
        { requireCatchParameter: false, errorClassNames: [] },
      ],
    },
  },
  {
    files: ["**/*.{js,mjs,cjs,jsx}"],
    rules: {
      "preserve-caught-error": [
        "error",
        { requireCatchParameter: false, errorClassNames: [] },
      ],
    },
  },
);
