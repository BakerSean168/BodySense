import { expect, type Page } from "@playwright/test";

/**
 * Acquire a short-lived API token without weakening the production browser
 * contract. The page keeps the refresh credential in its HttpOnly cookie;
 * BrowserContext.request shares that cookie jar and receives the rotated cookie.
 */
export async function refreshBrowserAccessToken(
  page: Page,
  apiBase: string,
): Promise<string> {
  const origin = new URL(page.url()).origin;
  const response = await page.context().request.post(
    `${apiBase}/api/v1/auth/refresh`,
    {
      headers: { Origin: origin },
    },
  );
  const body = await response.text();
  expect(
    response.ok(),
    `browser refresh failed with ${response.status()}: ${body}`,
  ).toBeTruthy();
  const payload = JSON.parse(body) as { access_token?: string };
  expect(payload.access_token).toBeTruthy();
  return payload.access_token ?? "";
}
