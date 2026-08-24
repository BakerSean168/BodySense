import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { expect, test, type Page } from "@playwright/test";

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
  await page.goto("/consultation");
  await expect(
    page.getByPlaceholder("描述您的症状、体态问题或身体感受，也可附上照片..."),
  ).toBeVisible();
}

async function startLongRun(page: Page, suffix: string): Promise<void> {
  const composer = page.getByPlaceholder(
    "描述您的症状、体态问题或身体感受，也可附上照片...",
  );
  await composer.fill(`E2E_HOLD_RUN_LONG ${suffix}`);
  await page.getByRole("button", { name: "发送" }).click();
  await expect(
    page.getByRole("button", { name: "取消本次执行" }),
  ).toBeVisible();
}

test("user can explicitly cancel a live Consultation run", async ({ page }) => {
  await registerBrowser(page);
  await startLongRun(page, "cancel");

  await page.getByRole("button", { name: "取消本次执行" }).click();

  await expect(page.getByText("本次执行已取消")).toBeVisible();
  await expect(page.getByRole("button", { name: "取消本次执行" })).toHaveCount(
    0,
  );
  await expect(
    page.getByPlaceholder("描述您的症状、体态问题或身体感受，也可附上照片..."),
  ).toBeEnabled();

  await page.reload();
  await expect(page.getByText("本次执行已取消")).toBeVisible();
  await expect(
    page.getByPlaceholder("描述您的症状、体态问题或身体感受，也可附上照片..."),
  ).toBeEnabled();
});

test("browser recovers execution_lost across an API process restart", async ({
  page,
}) => {
  const restartCommand = process.env.E2E_RESTART_API_COMMAND;
  test.skip(!restartCommand, "local validator restart command is not configured");

  await registerBrowser(page);
  await startLongRun(page, "restart-recovery");

  await execFileAsync("bash", ["-lc", restartCommand!], {
    cwd: process.cwd(),
    timeout: 60_000,
  });

  await expect(page.getByText("本次执行已安全停止")).toBeVisible({
    timeout: 30_000,
  });
  await expect(page.getByText(/系统已安全回收/)).toBeVisible();
  await expect(
    page.getByPlaceholder("描述您的症状、体态问题或身体感受，也可附上照片..."),
  ).toBeEnabled();
});
