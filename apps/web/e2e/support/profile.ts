/**
 * Body of the first PUT /api/v1/profile for a freshly registered user: the
 * endpoint is a full replacement under optimistic concurrency, so every
 * editable field is sent (null = not provided) against revision 0.
 */
export function profileReplacement(identity: {
  gender?: "male" | "female" | null;
  birth_date?: string | null;
  display_name?: string | null;
}) {
  return {
    expected_revision: 0,
    display_name: identity.display_name ?? null,
    avatar_asset_id: null,
    gender: identity.gender ?? null,
    birth_date: identity.birth_date ?? null,
    preferences: { locale: null, time_zone: null, unit_preference: null },
  };
}
