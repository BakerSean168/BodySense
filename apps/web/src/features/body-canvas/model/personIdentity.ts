/**
 * The name the Person header shows (Penpot draws a name, "BakerSean"). Order:
 * the Profile nickname, else the local part of the account email ("baker" for
 * baker@example.com), else a placeholder. The fallback is derived from the
 * real account, never invented, and the full email stays in the title.
 */
export interface PersonDisplayName {
  name: string;
  source: "profile" | "account" | "placeholder";
}

export const PERSON_NAME_PLACEHOLDER = "未设置昵称";

export function emailLocalPart(email: string | null | undefined): string {
  const trimmed = (email ?? "").trim();
  const at = trimmed.indexOf("@");
  return (at > 0 ? trimmed.slice(0, at) : trimmed).trim();
}

export function personDisplayName({
  nickname,
  accountEmail,
}: {
  nickname?: string | null;
  accountEmail?: string | null;
}): PersonDisplayName {
  const profileName = (nickname ?? "").trim();
  if (profileName) return { name: profileName, source: "profile" };
  const local = emailLocalPart(accountEmail);
  if (local) return { name: local, source: "account" };
  return { name: PERSON_NAME_PLACEHOLDER, source: "placeholder" };
}
