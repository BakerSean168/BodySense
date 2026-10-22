import {
  expect,
  test,
  type APIRequestContext,
  type Page,
} from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { profileReplacement } from "./support/profile";
import { fillRegisterPassword } from "./support/register";
import { choosePersonCategory, personCategoryTrigger } from "./support/person";
import { selectOption } from "./support/select";

// UI-001 #309: Penpot Person / Records + Body / Episode scenes on real Go/PG.
// Registration, real receipts, full reloads and 390px geometry; no fixtures.
test.describe.configure({ timeout: 120_000 });

const api = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";

async function register(page: Page, request: APIRequestContext) {
  await page.goto("/register");
  await page
    .getByLabel("邮箱地址")
    .fill(`person-penpot-${crypto.randomUUID()}@example.test`);
  await fillRegisterPassword(page, "BodySenseE2E!123");
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding|consultation)/);
  const accessToken = await refreshBrowserAccessToken(page, api);
  const headers = { Authorization: `Bearer ${accessToken}` };
  const profile = await request.put(`${api}/api/v1/profile`, {
    headers,
    data: profileReplacement({ gender: "male", birth_date: "2000-01-01" }),
  });
  expect(profile.ok(), await profile.text()).toBeTruthy();
  return headers;
}

async function ledger(
  request: APIRequestContext,
  headers: Record<string, string>,
) {
  const response = await request.get(`${api}/api/v1/body-state/records`, {
    headers,
  });
  expect(response.ok()).toBeTruthy();
  return (await response.json()) as {
    current_revision: number;
    facts: { id: string; statement: string; kind: string }[];
  };
}

/** Nothing in the Person window may overflow the viewport or its sheet. */
async function expectFits(page: Page, width: number) {
  const geometry = await page.evaluate(() => {
    const body = document.querySelector(".bc-person-body");
    const bounds = body?.getBoundingClientRect();
    return {
      page: document.documentElement.scrollWidth - window.innerWidth,
      body: body ? body.scrollWidth - body.clientWidth : -1,
      clipped: bounds
        ? Array.from(body!.querySelectorAll("button,p,h3,h4,span,select"))
            .filter((node) => node.getClientRects().length > 0)
            .filter((node) => {
              const rect = node.getBoundingClientRect();
              return (
                rect.left < bounds.left - 1 || rect.right > bounds.right + 1
              );
            })
            .map((node) => node.textContent?.slice(0, 40) || node.tagName)
        : ["no person body"],
    };
  });
  expect(geometry).toEqual({ page: 0, body: 0, clipped: [] });
  expect(width).toBeGreaterThan(0);
}

test("Person Records timeline, Episode scene and hypotheses persist through real receipts at 390px", async ({
  page,
  request,
}, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const headers = await register(page, request);
  await page.goto("/consultation?view=person");
  const person = page.getByTestId("person-window");
  await expect(person).toBeVisible();
  const canvas = await page.locator(".bc-scene canvas").elementHandle();
  expect(
    canvas,
    "the 3D Body Canvas stays mounted behind the window",
  ).not.toBeNull();

  // Penpot header: "nickname / 身体记录"; no tab strip.
  await expect(personCategoryTrigger(page)).toHaveAttribute(
    "data-value",
    "records",
  );
  await expect(person.locator(".bc-person-tabs")).toHaveCount(0);
  const records = page.getByTestId("body-fact-records");
  await expect(records.getByText("还没有身体记录")).toBeVisible();

  // Records add: saved appears only with the Go receipt, then survives reload.
  await page.getByTestId("person-add").click();
  await selectOption(
    page.getByRole("combobox", { name: "记录类别" }),
    "symptom",
  );
  await page
    .getByLabel("你的描述")
    .fill("昨天晚上开始头痛，两边太阳穴一跳一跳的。");
  await page.getByLabel(/发生时间/).fill("昨天晚上");
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(records.getByText("已保存", { exact: true })).toBeVisible();
  const first = await ledger(request, headers);
  expect(first.facts).toHaveLength(1);
  await page.reload();
  await expect(records.getByText("1 条", { exact: true })).toBeVisible();
  await expect(records.getByRole("heading", { level: 4 })).toContainText(
    /今天 · \d+月\d+日/,
  );
  await expect(
    records.getByText(/发生：昨天晚上 · 记录 \d\d:\d\d/),
  ).toBeVisible();
  await expectFits(page, 390);

  // The generic Records form keeps no structured symptom details, so it never
  // seeds an Episode. The user's own structured symptom statement does.
  const symptom = await request.post(`${api}/api/v1/body-state/records`, {
    headers,
    data: {
      idempotency_key: crypto.randomUUID(),
      expected_revision: first.current_revision,
      facts: [
        {
          kind: "symptom",
          statement: "头痛又来了，两边太阳穴胀痛。",
          body_region_ids: [],
          temporal: { precision: "unknown" },
          details: { symptom_type: "头痛", presence: "present" },
        },
      ],
    },
  });
  expect(symptom.ok(), await symptom.text()).toBeTruthy();
  const symptomReceipt = (await symptom.json()) as {
    fact_ids: string[];
    revision: number;
  };
  const factId = symptomReceipt.fact_ids[0];
  // An Episode is an explicit user command on the asserted symptom.
  const episode = await request.post(`${api}/api/v1/body-state/episodes`, {
    headers,
    data: {
      fact_id: factId,
      expected_revision: symptomReceipt.revision,
      idempotency_key: crypto.randomUUID(),
    },
  });
  expect(episode.ok(), await episode.text()).toBeTruthy();
  await page.reload();
  await expect(records.getByText("2 条", { exact: true })).toBeVisible();
  await records.getByRole("button", { name: /^查看记录：头痛又来了/ }).click();
  const detail = page.getByTestId("body-fact-detail");
  await expect(detail.getByRole("heading", { name: "身体感受" })).toBeVisible();
  await expect(
    detail.getByText("“头痛又来了，两边太阳穴胀痛。”"),
  ).toBeVisible();
  await expect(detail.getByRole("button", { name: "撤回陈述" })).toBeVisible();
  await expectFits(page, 390);
  await detail.getByRole("button", { name: "头痛经历" }).click();

  const scene = page.getByTestId("person-episode");
  await expect(scene.getByRole("heading", { name: "头痛经历" })).toBeVisible();
  // One unqualified symptom never reads as ongoing or recovered.
  await expect(scene.getByTitle("持续状态尚未确认")).toHaveText(
    "是否持续 · 未知",
  );
  await expect(scene.getByText("1 条陈述")).toBeVisible();
  await expect(scene.getByRole("region", { name: "信息缺口" })).toContainText(
    "现在是否还在持续",
  );
  const footer = scene.locator(".bc-person-footer");
  const footerBox = await footer.boundingBox();
  expect(footerBox).not.toBeNull();
  expect(footerBox!.y + footerBox!.height).toBeLessThanOrEqual(844);
  await expectFits(page, 390);
  await page.screenshot({
    path: testInfo.outputPath("person-episode-390.png"),
  });
  await scene.getByRole("button", { name: "返回记录" }).click();

  // The user's own hypothesis: saved only after the receipt, then derived group.
  await person.getByRole("button", { name: "我的猜测 0 条" }).click();
  const panel = page.getByRole("region", { name: "我的猜测与相关记录" });
  await page.getByTestId("person-add").click();
  await panel.getByLabel("我的完整猜测").fill("我猜头痛和最近睡得晚有关。");
  await panel
    .getByRole("checkbox", {
      name: "头痛又来了，两边太阳穴胀痛。",
      exact: true,
    })
    .check();
  await panel.getByRole("button", { name: "保存猜测" }).click();
  await expect(panel.getByText("你的猜测已保存。")).toBeVisible();
  const stored = await (
    await request.get(`${api}/api/v1/body-state/hypothesis-records`, {
      headers,
    })
  ).json();
  expect(stored.hypotheses).toHaveLength(1);
  expect(stored.hypotheses[0]).toMatchObject({
    statement: "我猜头痛和最近睡得晚有关。",
    related_fact_ids: [factId],
  });
  await page.reload();
  await person.getByRole("button", { name: "我的猜测 1 条" }).click();
  await expect(
    panel.locator(".bc-record-statement", {
      hasText: "我猜头痛和最近睡得晚有关。",
    }),
  ).toBeVisible();
  const group = panel.locator(".bc-person-group");
  await expect(group).toHaveCount(1);
  await expect(group).toContainText("相关经历：头痛");
  await expectFits(page, 390);
  await person.getByRole("button", { name: "返回记录" }).click();

  // The Episode scene shows the same derived group as a related chip.
  await person.getByRole("button", { name: "经历 1 段" }).click();
  await page
    .getByTestId("person-episodes")
    .getByRole("button", { name: /头痛经历/ })
    .click();
  await expect(
    scene.getByRole("region", { name: "相关" }).getByRole("button", {
      name: "我猜头痛和最近睡得晚有关。",
    }),
  ).toBeVisible();

  // Category switching keeps the canvas; Habits is the same ledger filtered.
  // (Reloads above replaced the document, so take the live canvas again.)
  const liveCanvas = await page.locator(".bc-scene canvas").elementHandle();
  expect(liveCanvas).not.toBeNull();
  await choosePersonCategory(page, "habits");
  await expect(records.getByText("还没有生活习惯")).toBeVisible();
  await choosePersonCategory(page, "records");
  await expect(records.getByText("2 条", { exact: true })).toBeVisible();
  expect(await liveCanvas!.evaluate((node) => node.isConnected)).toBe(true);
  await page.setViewportSize({ width: 1280, height: 844 });
  await expectFits(page, 1280);
  expect(await liveCanvas!.evaluate((node) => node.isConnected)).toBe(true);
});

test("Person Records shows the failed read and keeps the last list as possibly stale", async ({
  page,
  request,
}) => {
  const headers = await register(page, request);
  const first = await ledger(request, headers);
  const seeded = await request.post(`${api}/api/v1/body-state/records`, {
    headers,
    data: {
      idempotency_key: crypto.randomUUID(),
      expected_revision: first.current_revision,
      facts: [
        {
          kind: "activity",
          statement: "早上快走了 40 分钟。",
          body_region_ids: [],
          temporal: { precision: "unknown", original_expression: "今天早上" },
          details: {},
        },
      ],
    },
  });
  expect(seeded.ok(), await seeded.text()).toBeTruthy();
  await page.goto("/consultation?view=person");
  const records = page.getByTestId("body-fact-records");
  await expect(records.getByText("早上快走了 40 分钟。")).toBeVisible();

  // A failed refresh keeps the last read and says it may be out of date.
  await page.route("**/api/v1/body-state/records*", (route) =>
    route.request().method() === "GET"
      ? route.fulfill({
          status: 503,
          json: { error: { code: "UNAVAILABLE", message: "down" } },
        })
      : route.continue(),
  );
  await choosePersonCategory(page, "equipment");
  await choosePersonCategory(page, "records");
  await expect(records.getByRole("alert")).toContainText("可能不是最新");
  await expect(records.getByText("早上快走了 40 分钟。")).toBeVisible();

  // With no prior read the window says so instead of calling it empty.
  await page.reload();
  await expect(records.getByRole("alert")).toContainText("记录没有读取出来");
  await expect(records.getByText("还没有身体记录")).toHaveCount(0);
  await page.unroute("**/api/v1/body-state/records*");
  await records.getByRole("button", { name: "重新读取" }).click();
  await expect(records.getByText("早上快走了 40 分钟。")).toBeVisible();
});
