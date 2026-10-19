import { expect, test } from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";

// PC3-19 / PC-D8: the target product has no public health-conversation share.
// Every old share entry point fails closed on the real stack: the API routes
// are gone (404, with or without a session), and an old share link in the
// browser lands on sign-in or the user's own workbench, never on someone
// else's transcript.
const api = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";
const token = "legacy-public-share-token";

test("old public share links and share endpoints fail closed", async ({
  page,
}) => {
  const request = page.context().request;
  for (const path of [
    `/api/v1/shared-conversations/${token}`,
    `/api/v1/conversations/share/${token}`,
  ]) {
    const response = await request.get(`${api}${path}`);
    expect(response.status(), path).toBe(404);
  }

  for (const path of [
    `/consultation/share/${token}`,
    `/shared-conversations/${token}`,
    `/share/${token}`,
  ]) {
    await page.goto(path);
    await page.waitForURL(/\/login/);
  }

  await page.goto("/register");
  await page
    .getByLabel("邮箱地址")
    .fill(`share-${crypto.randomUUID()}@example.test`);
  await page.getByLabel("密码", { exact: true }).fill("BodySenseE2E!123");
  await page.getByRole("checkbox", { name: /同意/ }).check();
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding)/);
  const headers = {
    Authorization: `Bearer ${await refreshBrowserAccessToken(page, api)}`,
  };
  const conversation = await request.get(`${api}/api/v1/conversation`, {
    headers,
  });
  expect(conversation.ok(), await conversation.text()).toBeTruthy();
  const {
    conversation: { id },
  } = (await conversation.json()) as { conversation: { id: string } };

  for (const method of ["post", "delete"] as const) {
    const response = await request[method](
      `${api}/api/v1/conversations/${id}/share`,
      { headers },
    );
    expect(response.status(), `${method} share`).toBe(404);
  }
  const shared = await request.get(
    `${api}/api/v1/shared-conversations/${token}`,
    { headers },
  );
  expect(shared.status()).toBe(404);

  // Signed in, an old share link opens nothing shared: it falls back to the
  // user's own workbench route.
  await page.goto(`/consultation/share/${token}`);
  await expect(page).toHaveURL(/\/(consultation|onboarding)(\/|\?|$)/);
  await expect(page).not.toHaveURL(/share/);
});
