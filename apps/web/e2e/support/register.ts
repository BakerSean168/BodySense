import type { Page } from "@playwright/test";

/**
 * Fills the /register password, confirm-password and consent controls.
 * Shared by every spec that registers through the UI, so a register-form change
 * lands in one place. The caller fills the email and submits.
 */
export async function fillRegisterPassword(
  page: Page,
  password: string,
): Promise<void> {
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByLabel("确认密码", { exact: true }).fill(password);
  await page.getByRole("checkbox", { name: /同意/ }).check();
}
