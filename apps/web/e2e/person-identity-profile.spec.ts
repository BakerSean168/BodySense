import { expect, test, type APIResponse } from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { profileReplacement } from "./support/profile";
import { fillRegisterPassword } from "./support/register";

// AS-001 #306: the minimal optional Profile is edited in the Person identity
// popover at phone width. Registration, real Go/DB calls and reloads.
test.describe.configure({ timeout: 90_000 });
test.use({ viewport: { width: 390, height: 844 } });

const apiBase = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";

interface ProfileBody {
  profile: {
    display_name?: string;
    gender?: string;
    birth_date?: string;
    preferences: { time_zone?: string; unit_preference?: string };
    revision: number;
  } | null;
}

async function expectJson<T>(response: APIResponse, label: string): Promise<T> {
  const body = await response.text();
  expect(
    response.ok(),
    `${label} failed with ${response.status()}: ${body}`,
  ).toBeTruthy();
  return JSON.parse(body) as T;
}

test("Person identity edits optional Profile fields and preferences without touching BodyState", async ({
  page,
  request,
}) => {
  const email = `profile-identity-${crypto.randomUUID()}@example.test`;
  await page.goto("/register");
  await page.getByLabel("邮箱地址").fill(email);
  await fillRegisterPassword(page, "BodySenseE2E!123");
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding|consultation)/);
  const accessToken = await refreshBrowserAccessToken(page, apiBase);
  const headers = { Authorization: `Bearer ${accessToken}` };
  await expectJson(
    await request.put(`${apiBase}/api/v1/profile`, {
      headers,
      data: profileReplacement({ gender: "male", birth_date: "1996-08-27" }),
    }),
    "seed onboarding identity",
  );
  // Height/weight are BodyState observations, not Profile fields.
  const metricsBefore = await expectJson<Record<string, unknown>>(
    await request.put(`${apiBase}/api/v1/body-metrics`, {
      headers,
      data: { expected_revision: 0, height_cm: 172, weight_kg: 66.5 },
    }),
    "seed body metrics",
  );

  await page.goto("/consultation?view=person");
  const person = page.getByTestId("person-window");
  await expect(person).toBeVisible();
  // No display name yet: the header falls back to the real account name.
  const trigger = page.getByRole("button", { name: email });
  await expect(trigger).toBeVisible();
  await trigger.click();

  const identity = page.getByRole("dialog", { name: "个人资料" });
  await expect(identity).toBeVisible();
  const box = await identity.boundingBox();
  expect(box, "identity popover has a layout box").not.toBeNull();
  expect(box!.x).toBeGreaterThanOrEqual(0);
  expect(box!.x + box!.width).toBeLessThanOrEqual(390);
  await expect(identity).toContainText("资料缺失不会由 AI 猜测补全");
  await expect(identity).not.toContainText(/BMI|身高|体重|revision|修订/);
  await expect(identity.getByLabel("自报性别")).toHaveValue("male");
  await expect(identity.getByLabel("生日")).toHaveValue("1996-08-27");
  // Unset preferences are shown as identifiable defaults.
  await expect(identity.getByLabel("显示时区")).toHaveValue("");
  await expect(identity.getByLabel("显示时区")).toContainText("跟随设备");
  await expect(identity.getByLabel("单位")).toHaveValue("");

  await identity.getByLabel("昵称").fill("BakerSean");
  await identity.getByLabel("显示时区").selectOption("Asia/Tokyo");
  await identity.getByLabel("单位").selectOption("imperial");
  await identity.getByLabel("自报性别").selectOption("");
  await identity.getByRole("button", { name: "保存" }).click();
  await expect(identity.getByRole("status")).toHaveText("已保存");
  await expect(page.getByRole("button", { name: "BakerSean" })).toBeVisible();

  const stored = await expectJson<ProfileBody>(
    await request.get(`${apiBase}/api/v1/profile`, { headers }),
    "read profile",
  );
  expect(stored.profile).toMatchObject({
    display_name: "BakerSean",
    birth_date: "1996-08-27",
    preferences: { time_zone: "Asia/Tokyo", unit_preference: "imperial" },
  });
  expect(stored.profile?.gender).toBeUndefined();
  // Preferences never rewrite the recorded measurement values or units.
  const metricsAfter = await expectJson<Record<string, unknown>>(
    await request.get(`${apiBase}/api/v1/body-metrics`, { headers }),
    "read body metrics",
  );
  expect(metricsAfter).toEqual(metricsBefore);

  // A write from elsewhere moves the revision; the open editor must not
  // overwrite it and reloads the newer profile instead.
  await expectJson(
    await request.put(`${apiBase}/api/v1/profile`, {
      headers,
      data: {
        ...profileReplacement({
          display_name: "Elsewhere",
          birth_date: "1996-08-27",
        }),
        expected_revision: stored.profile!.revision,
        preferences: {
          locale: null,
          time_zone: "Asia/Tokyo",
          unit_preference: "imperial",
        },
      },
    }),
    "concurrent profile write",
  );
  await identity.getByLabel("昵称").fill("Mine");
  await identity.getByRole("button", { name: "保存" }).click();
  await expect(identity.getByRole("alert")).toContainText("资料已在别处更新");
  await expect(identity.getByLabel("昵称")).toHaveValue("Elsewhere");

  // Survives reload; clearing the nickname falls back to the account name.
  await page.reload();
  await expect(page.getByTestId("person-window")).toBeVisible();
  await page.getByRole("button", { name: "Elsewhere" }).click();
  await expect(identity.getByLabel("显示时区")).toHaveValue("Asia/Tokyo");
  await identity.getByLabel("昵称").fill("");
  await identity.getByRole("button", { name: "保存" }).click();
  await expect(identity.getByRole("status")).toHaveText("已保存");
  await expect(page.getByRole("button", { name: email })).toBeVisible();
  await identity.getByRole("button", { name: "关闭" }).click();
  await expect(identity).toBeHidden();
});
