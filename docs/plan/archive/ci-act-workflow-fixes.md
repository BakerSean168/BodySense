# CI act workflow fixes

## Goal

Use `act` to reproduce the current workflow failures locally, then fix the CI blockers until the relevant jobs pass.

## Checklist

- [x] Reproduce the reported `web:lint` failure locally.
- [x] Add a stable lint target/configuration for the web app.
- [x] Re-run web lint/typecheck/build locally.
- [x] Re-run the CI web job with `act`.
- [x] Check remaining workflow jobs and fix any additional failures.
- [x] Archive this plan when complete.
