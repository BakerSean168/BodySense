# Episodic S6-A — durable user deletion, owner-only inspect/list (unwired)

Parent: [#498](https://github.com/BakerSean168/BodySense/issues/498) ·
user controls [#516](https://github.com/BakerSean168/BodySense/issues/516) ·
ADR #495. This is the **database/repository** prerequisite for a reviewed
Person/Chat control interface, not the user-facing release.

## Delivery and authorization boundary

- `apps/api/migrations/000006_episodic_user_deletion` creates
  `episodic_note_deletions` with **only** user ID, note ID, Conversation ID,
  source sequence range and deletion time. It stores **no quoted text,
  message UUIDs, clinical data, model output or asset IDs**.
- `EpisodicNoteControlsRepository.ListOwned` returns no other user's notes,
  active `shadow` notes only, sorted newest first. The current S2-A storage
  cap is 48 notes; no serving uses this list.
- `DeleteOwned` accepts an authenticated owner identity from a *future*
  application call. It locks the owner Conversation as the S2 writer and
  retention machinery do, records a durable tombstone, then removes the
  selected note **and all overlapping notes in that Conversation** atomically.
  Other account notes, BodyState facts, Diagnosis, Safety records and raw
  messages are unchanged. Repeat deletes by that owner are idempotent; absent
  and foreign IDs are indistinguishable.
- The S2 writer checks tombstone overlap **before** idempotent replay and
  source loading. It returns `ErrEpisodicNoteUserDeleted` if anyone tries to
  regenerate any overlapping source window, even with a different generator
  or retention-policy revision.
- The SQL `BEFORE INSERT OR UPDATE` guard takes a Conversation
  `FOR SHARE` lock (conflicting with the writer/deleter's `NO KEY UPDATE`
  lock) and checks range overlap. This prevents direct SQL resurrection by
  bypassing Go, including concurrent deletion/write under the same owner.
- The tombstone is not subject to normal chat prune or episodic-note
  oldest-first rollover. User/account deletion destroys it via a composite
  Conversation-owner FK `ON DELETE CASCADE`. Its retention is therefore
  **the life of the account**, pending a reviewed privacy policy.
- Migration `000006.down.sql` refuses downgrade when any deletion tombstone
  remains. Dropping this table would erase the user's explicit suppression
  choices and permit regeneration after a rollback.

## Important limitations

No user-facing endpoint or Person category was added. Product and privacy
owners have not approved the copy, consent, retention or actual user data
handling. The existing Person window has exactly five established categories;
adding a sixth would change the product's navigation contract. In a later
S6-B PR, add a versioned, generated-OpenAPI **owner-authenticated** read and
delete interface and a minimal previewable UI, then obtain design/privacy
sign-off *before* enabling on real accounts.

This feature does not authorize:
- Live model generation, private patient data egress or health-context memory.
- An Agent reader, vector index or diagnosis/safety authority.
- A production database upgrade or change of staging credentials.
- Erasure of BodyState or diagnosis through the note delete action. Full
  account erasure and source-statement/source-asset erasure remain distinct.

## Evidence

Disposable PG18 full up/down/replay of migrations 1..6, plus isolated tests
for owner-scoped list, masked cross-account 404 semantics, idempotent deletion,
overlapping note cleanup, Go replay rejection including version upgrade,
direct SQL trigger rejection, Conversation lock serialization, unchanged raw
chat/BodyState evidence, account cascade, and S3 source erasure regressions.
The migration checksums, deterministic database schema snapshot, release
dry-run exact file allowlist and dev Knowledge Candidate migration gate must
be updated in the same PR. PR CI and merged-main CI are separate acceptance
steps; do not claim one substitutes for the other.
