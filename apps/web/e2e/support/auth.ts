import { expect, type Page } from "@playwright/test";

/**
 * Supabase JS handles refresh-token rotation and persists its own browser
 * session. E2E tests read the current access token from that same browser
 * origin; there is no longer a Go /api/v1/auth/refresh endpoint.
 *
 * Never copy this helper into application code. Product code uses the SDK.
 */
export async function refreshBrowserAccessToken(
  page: Page,
  _apiBase: string,
): Promise<string> {
  const accessToken = await page.evaluate(() => {
    const key = Object.keys(localStorage).find(
      (name) => name.startsWith("sb-") && name.endsWith("-auth-token"),
    );
    if (!key) return null;
    try {
      const session = JSON.parse(localStorage.getItem(key) || "{}") as {
        access_token?: unknown;
      };
      return typeof session.access_token === "string"
        ? session.access_token
        : null;
    } catch {
      return null;
    }
  });
  expect(
    accessToken,
    "Supabase browser session must have an access JWT",
  ).toBeTruthy();
  return accessToken || "";
}
