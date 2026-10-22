import { expect, test, type APIResponse } from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { profileReplacement } from "./support/profile";
import { fillRegisterPassword } from "./support/register";
import { selectOption } from "./support/select";

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
  // No display name yet: the header shows the account email's local part
  // (Penpot draws a name), never the whole address.
  const localPart = email.split("@")[0];
  const trigger = page.getByRole("button", { name: localPart, exact: true });
  await expect(trigger).toBeVisible();
  await expect(trigger).toHaveAttribute("data-name-source", "account");
  await expect(trigger).not.toContainText("@");
  await trigger.click();
  await page.getByRole("menuitem", { name: "个人资料" }).click();

  const identity = page.getByRole("dialog", { name: "个人资料" });
  await expect(identity).toBeVisible();
  const box = await identity.boundingBox();
  expect(box, "identity popover has a layout box").not.toBeNull();
  expect(box!.x).toBeGreaterThanOrEqual(0);
  expect(box!.x + box!.width).toBeLessThanOrEqual(390);
  // The empty nickname input previews the account-name fallback.
  await expect(identity.getByLabel("昵称")).toHaveAttribute(
    "placeholder",
    localPart,
  );
  await expect(identity).not.toContainText("AI 猜测");
  await expect(identity).not.toContainText(/BMI|身高|体重|revision|修订/);
  const gender = identity.getByRole("combobox", { name: "自报性别" });
  const zone = identity.getByRole("combobox", { name: "显示时区" });
  const units = identity.getByRole("combobox", { name: "单位" });
  await expect(gender).toHaveAttribute("data-value", "male");
  await expect(identity.getByRole("button", { name: "生日" })).toHaveAttribute(
    "data-value",
    "1996-08-27",
  );
  // Unset preferences are shown as identifiable defaults.
  await expect(zone).toHaveAttribute("data-value", "");
  await expect(zone).toContainText("跟随设备");
  await expect(units).toHaveAttribute("data-value", "");

  await identity.getByLabel("昵称").fill("BakerSean");
  await selectOption(zone, "Asia/Tokyo");
  await selectOption(units, "imperial");
  await selectOption(gender, "");
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
  await page.getByRole("menuitem", { name: "个人资料" }).click();
  await expect(
    identity.getByRole("combobox", { name: "显示时区" }),
  ).toHaveAttribute("data-value", "Asia/Tokyo");
  await identity.getByLabel("昵称").fill("");
  await identity.getByRole("button", { name: "保存" }).click();
  await expect(identity.getByRole("status")).toHaveText("已保存");
  await expect(
    page.getByRole("button", { name: localPart, exact: true }),
  ).toBeVisible();
  await identity.getByRole("button", { name: "关闭" }).click();
  await expect(identity).toBeHidden();
});
