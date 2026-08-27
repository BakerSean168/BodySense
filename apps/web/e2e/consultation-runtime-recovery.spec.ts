import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { expect, test, type Page } from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";

const apiBase = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";

const execFileAsync = promisify(execFile);

async function registerBrowser(page: Page): Promise<void> {
  const email = `runtime-${Date.now()}-${Math.random().toString(16).slice(2)}@example.com`;
  const password = "BodySenseE2E!123";
  await page.goto("/register");
  await page.getByLabel("邮箱地址").fill(email);
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByLabel("确认密码").fill(password);
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding)/);

  const accessToken = await refreshBrowserAccessToken(page, apiBase);
  const profile = await page
    .context()
    .request.put(`${apiBase}/api/v1/profile`, {
      headers: { Authorization: `Bearer ${accessToken}` },
      data: { gender: "male", birth_date: "1996-08-27" },
    });
  expect(profile.ok(), await profile.text()).toBeTruthy();

  await page.goto("/consultation");
  await expect(
    page.getByPlaceholder("和 BodySense 说说你的身体感受…"),
  ).toBeVisible();
}

async function startLongRun(page: Page, suffix: string): Promise<void> {
  const composer = page.getByPlaceholder("和 BodySense 说说你的身体感受…");
  await composer.fill(`E2E_HOLD_RUN_LONG ${suffix}`);
  await page.getByRole("button", { name: "发送" }).click();
  await expect(
    page.getByRole("button", { name: "停止", exact: true }),
  ).toBeVisible();
}

test("user can explicitly cancel a live Consultation run", async ({ page }) => {
  await registerBrowser(page);
  await startLongRun(page, "cancel");

  await page.getByRole("button", { name: "停止", exact: true }).click();

  await expect(page.getByText("本次处理已停止")).toBeVisible();
  await expect(
    page.getByRole("button", { name: "停止", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByPlaceholder("和 BodySense 说说你的身体感受…"),
  ).toBeEnabled();

  await page.reload();
  await expect(page.getByText("本次处理已停止")).toBeVisible();
  await expect(
    page.getByPlaceholder("和 BodySense 说说你的身体感受…"),
  ).toBeEnabled();
});

test("browser recovers execution_lost across an API process restart", async ({
  page,
}) => {
  const restartCommand = process.env.E2E_RESTART_API_COMMAND;
  test.skip(
    !restartCommand,
    "local validator restart command is not configured",
  );

  await registerBrowser(page);
  await startLongRun(page, "restart-recovery");

  await execFileAsync("bash", ["-lc", restartCommand!], {
    cwd: process.cwd(),
    timeout: 60_000,
  });

  await expect(page.getByText("本次处理已安全停止")).toBeVisible({
    timeout: 30_000,
  });
  await expect(page.getByText(/系统已安全回收/)).toBeVisible();
  await expect(
    page.getByPlaceholder("和 BodySense 说说你的身体感受…"),
  ).toBeEnabled();
});
