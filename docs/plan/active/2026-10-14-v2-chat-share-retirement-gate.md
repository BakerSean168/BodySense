# BodySense V2 · PC-D8 share-retirement transition gate

**Scope:** bounded, default-off server-side privacy switch for the
*legacy* public ConversationShare API. This is NOT V2 activation,
historical token revocation, erasure, or a release admission.

## Explicit runtime behavior

- Environment variable: `BODYSENSE_V2_RETIRE_CHAT_SHARES=true`.
  Only explicit `true` enables retirement. Absent/false preserves the
  existing pre-V2 service until migration is approved. An invalid value
  causes an immediate startup configuration error; it must never
  silently enable the public API by falling back to false.
- When enabled, `GET /api/v1/shared-conversations/:token` returns
  an identity-free `404` without reading the token, snapshot title,
  message content, or a legacy provider. The denial precedes the
  public OpenAPI validator.
- `POST /api/v1/conversations/:id/share` returns `404`, even for
  an authenticated owner; no new public snapshot can be minted.
- Owner-authorized `DELETE /api/v1/conversations/:id/share` remains
  available as a transitional **revocation-only** capability. The
  authenticated route must never make a new token. The final V2
  removal of this endpoint is a separate authorized cutover.
- Other generated OpenAPI routes remain registered exactly once.
  Default-off legacy operation is intentionally unchanged so that
  this PR does not unknowingly break pre-V2 clients.

## Required before activation

1. Complete the PC-001 actual-database owner/history and old
   `conversation_shares` inventory (including copied content).
2. Take approved backups/rollback evidence and revoke legacy public
   tokens/snapshots in the scoped database. A `404` response alone
   does **not** mean the copied medical content was erased.
3. Verify the disabled mode at the **actual** V2 deployment identity
   and all exposed ingress routes, not merely at unit-test router
   construction. Verify direct service access and cache/object copies
   cannot still reveal snapshots.
4. Remove obsolete OpenAPI/client sharing operations and the temporary
   revocation capability only after full PC-D8 and downstream caller
   migration. Do not remove them by editing generated code directly.

**Testing:** Go route tests demonstrate non-invocation of share provider
for public reads and new POST creation, with independent owner DELETE
revocation, and the server configuration parser rejects invalid values.
The existing default-on public share test remains a necessary
backward-compatibility test until cutover.

**Formal gate:** remain blocked. No environment variable is set in this
PR, no staging/production deployment is performed, and the entire
BodySense V2 contract remains unaccepted until full integration review.
