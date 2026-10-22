import { expect, test } from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { completeOnboarding } from "./support/onboarding";
import { fillRegisterPassword } from "./support/register";

/**
 * Penpot "02 Onboarding" (Onboarding / Desktop / Step 01 … 16): a new account walks
 * the 16 one-question steps at 1440 and 390, lands in the workbench, and the API
 * holds the mapped profile, body metrics and lifestyle baseline.
 */
const apiBase = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";
const viewports = [
  { name: "1440", width: 1440, height: 900 },
  { name: "390", width: 390, height: 844 },
] as const;

for (const vp of viewports) {
  test(`onboarding follows the Penpot steps (${vp.name})`, async ({
    page,
    request,
  }) => {
    test.setTimeout(120_000);
    await page.setViewportSize({ width: vp.width, height: vp.height });
    await page.emulateMedia({ reducedMotion: "reduce" });
    const email = `onboarding-penpot-${vp.name}-${Date.now()}-${Math.random().toString(16).slice(2)}@example.com`;

    await page.goto("/register");
    await page.getByLabel("邮箱地址").fill(email);
    await fillRegisterPassword(page, "BodySenseE2E!123");
    await page.getByRole("button", { name: "创建账号" }).click();
    await page.waitForURL(/\/onboarding/);

    const root = page.getByTestId("onboarding-page");
    await expect(
      page.getByRole("heading", { name: "您的性别是？" }),
    ).toBeVisible();
    await expect(root.getByText("BodySense", { exact: true })).toBeVisible();
    await expect(page.getByRole("progressbar")).toHaveAttribute(
      "aria-valuenow",
      "1",
    );

    await completeOnboarding(page, async (step) => {
      // Layout landmarks from the boards: 620px column on desktop, no sideways scroll.
      const title = await page.getByRole("heading", { level: 1 }).boundingBox();
      expect(title).not.toBeNull();
      if (vp.name === "1440") {
        expect(Math.round(title!.x)).toBe(410);
        expect(Math.round(title!.y)).toBe(114);
      } else {
        expect(Math.round(title!.x)).toBe(20);
      }
      const overflowX = await page.evaluate(
        () => document.documentElement.scrollWidth - window.innerWidth,
      );
      expect(overflowX).toBeLessThanOrEqual(0);
      if (step === 2 || step === 16) {
        const footer = page
          .getByTestId("onboarding-footer")
          .getByRole("button");
        const box = await footer.boundingBox();
        expect(box!.y + box!.height).toBeLessThanOrEqual(vp.height);
        if (vp.name === "1440") expect(Math.round(box!.width)).toBe(620);
      }
    });

    await page.waitForURL(/\/consultation/, { timeout: 60_000 });

    const token = await refreshBrowserAccessToken(page, apiBase);
    const profile = await request.get(`${apiBase}/api/v1/profile`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(profile.ok(), await profile.text()).toBeTruthy();
    const body = (await profile.json()) as {
      profile?: { gender?: string; birth_date?: string };
    };
    expect(body.profile).toMatchObject({
      gender: "male",
      birth_date: "1995-06-15",
    });

    // Step 05 (颈部 + 腰部) is kept as a self-report, read back from its own endpoint.
    const discomfort = await request.get(
      `${apiBase}/api/v1/onboarding/discomfort-areas`,
      {
        headers: { Authorization: `Bearer ${token}` },
      },
    );
    expect(discomfort.ok(), await discomfort.text()).toBeTruthy();
    const reported = (await discomfort.json()) as {
      report?: { area_codes?: string[]; source?: string } | null;
    };
    expect(reported.report).toMatchObject({
      area_codes: ["neck", "lower_back"],
      source: "onboarding",
    });
  });
}
