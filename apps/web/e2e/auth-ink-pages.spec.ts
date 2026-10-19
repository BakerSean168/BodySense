import { expect, test } from "@playwright/test";

/**
 * Penpot "01 Auth" ink proposal boards (Sign in / Register · Proposal, desktop 1440 and
 * mobile 390): layout landmarks, a11y names, links and visual screenshots.
 */
const viewports = [
  { name: "1440", width: 1440, height: 900 },
  { name: "390", width: 390, height: 844 },
] as const;

for (const vp of viewports) {
  test.describe(`auth ink pages @${vp.name}`, () => {
    test.use({
      viewport: { width: vp.width, height: vp.height },
      reducedMotion: "reduce",
    });

    test(`sign in matches the ink proposal layout (${vp.name})`, async ({
      page,
    }) => {
      await page.goto("/login");
      const pageRoot = page.getByTestId("auth-login-page");
      await expect(pageRoot).toBeVisible({ timeout: 30_000 });
      await expect(page.getByRole("heading", { name: "登录" })).toBeVisible();
      await expect(page.getByTestId("auth-dantian-dot")).toBeVisible();
      await expect(page.getByLabel("邮箱地址")).toHaveAttribute(
        "placeholder",
        "邮箱",
      );
      await expect(page.getByLabel("密码", { exact: true })).toHaveAttribute(
        "type",
        "password",
      );
      await expect(page.getByRole("button", { name: "登录" })).toHaveCount(1);
      await expect(page.getByText("体悟", { exact: true })).toHaveCount(1);
      await expect(page.getByTestId("auth-ink-stage")).toBeVisible();

      const card = page.locator("main");
      const box = await card.boundingBox();
      expect(box).not.toBeNull();
      if (vp.name === "1440") {
        expect(Math.round(box!.x)).toBe(900);
        expect(Math.round(box!.width)).toBe(452);
        await expect(
          page.getByText("先看见身体，再决定怎么练。"),
        ).toBeVisible();
      } else {
        expect(Math.round(box!.y)).toBeGreaterThanOrEqual(380);
        expect(Math.round(box!.width)).toBe(390);
      }

      await page.getByRole("button", { name: "显示密码" }).click();
      await expect(page.getByLabel("密码", { exact: true })).toHaveAttribute(
        "type",
        "text",
      );
      await page.getByRole("button", { name: "隐藏密码" }).click();

      await page.getByLabel("邮箱地址").focus();
      await page.screenshot({
        path: `test-results/auth-ink/login-${vp.name}.png`,
        animations: "disabled",
      });

      await page.getByRole("button", { name: "登录" }).click();
      await expect(page.getByText("请输入邮箱地址")).toBeVisible();

      await page.getByRole("link", { name: "注册" }).click();
      await expect(page).toHaveURL(/\/register$/);
    });

    test(`register matches the ink proposal layout (${vp.name})`, async ({
      page,
    }) => {
      await page.goto("/register");
      await expect(page.getByTestId("auth-register-page")).toBeVisible({
        timeout: 30_000,
      });
      await expect(page.getByRole("heading", { name: "注册" })).toBeVisible();
      await expect(page.getByLabel("密码", { exact: true })).toHaveAttribute(
        "placeholder",
        "密码（至少 8 位）",
      );
      const confirm = page.getByLabel("确认密码", { exact: true });
      await expect(confirm).toHaveAttribute("placeholder", "确认密码");
      await expect(confirm).toHaveAttribute("type", "password");
      await expect(
        page.getByRole("checkbox", { name: /同意/ }),
      ).not.toBeChecked();

      // Confirm sits below password with the same Penpot field rhythm.
      const pwBox = await page
        .getByLabel("密码", { exact: true })
        .boundingBox();
      const confirmBox = await confirm.boundingBox();
      const emailBox = await page.getByLabel("邮箱地址").boundingBox();
      expect(pwBox && confirmBox && emailBox).toBeTruthy();
      expect(Math.round(confirmBox!.y - pwBox!.y)).toBe(
        Math.round(pwBox!.y - emailBox!.y),
      );

      // Submit button and the bottom link stay inside the first screen.
      const submitBox = await page
        .getByRole("button", { name: "创建账号" })
        .boundingBox();
      const linkBox = await page
        .getByRole("link", { name: "登录" })
        .boundingBox();
      expect(submitBox!.y + submitBox!.height).toBeLessThanOrEqual(vp.height);
      expect(linkBox!.y + linkBox!.height).toBeLessThanOrEqual(vp.height);
      expect(
        linkBox!.y - (submitBox!.y + submitBox!.height),
      ).toBeGreaterThanOrEqual(24);

      await page.getByTestId("confirmPassword-visibility").click();
      await expect(confirm).toHaveAttribute("type", "text");
      await expect(page.getByLabel("密码", { exact: true })).toHaveAttribute(
        "type",
        "password",
      );
      await page.getByTestId("confirmPassword-visibility").click();

      await page.screenshot({
        path: `test-results/auth-ink/register-${vp.name}.png`,
        animations: "disabled",
      });

      await page.getByRole("button", { name: "创建账号" }).click();
      await expect(
        page.getByText("请先阅读并同意服务条款和健康数据处理说明"),
      ).toBeVisible();

      await page.getByLabel("密码", { exact: true }).fill("BodySenseE2E!123");
      await confirm.fill("BodySenseE2E!124");
      await page.getByRole("button", { name: "创建账号" }).click();
      await expect(page.getByText("两次输入的密码不一致")).toBeVisible();

      await page.getByRole("link", { name: "登录" }).click();
      await expect(page).toHaveURL(/\/login$/);
    });
  });
}
