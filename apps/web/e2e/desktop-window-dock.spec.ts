import {
  expect,
  test,
  type APIRequestContext,
  type Page,
} from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { profileReplacement } from "./support/profile";
import { completeStructuredSafetyIntake } from "./support/safety";

// Desktop business windows dock like the Penpot "Desktop / <Window> / …"
// boards (1440 x 900, screen coordinates): Person on the left, Diagnosis,
// Training and Trainer on the right, all from y 92 to 792 so the body stays
// visible beside them. The global safety banner never covers a window header.
test.describe.configure({ timeout: 150_000 });

const api = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";
const VIEWPORT = { width: 1440, height: 900 };
const TOLERANCE = 4;

type DockedWindow = "person" | "diagnosis" | "training" | "trainer";
type Box = { x: number; y: number; width: number; height: number };

const PENPOT_DOCK: Record<DockedWindow, Box> = {
  person: { x: 40, y: 92, width: 860, height: 700 },
  diagnosis: { x: 800, y: 92, width: 600, height: 700 },
  training: { x: 800, y: 92, width: 600, height: 700 },
  trainer: { x: 800, y: 92, width: 600, height: 700 },
};

async function register(page: Page, request: APIRequestContext) {
  await page.goto("/register");
  await page
    .getByLabel("邮箱地址")
    .fill(`window-dock-${crypto.randomUUID()}@example.test`);
  await page.getByLabel("密码", { exact: true }).fill("BodySenseE2E!123");
  await page.getByLabel("确认密码").fill("BodySenseE2E!123");
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding|consultation)/);
  const accessToken = await refreshBrowserAccessToken(page, api);
  const headers = {
    Authorization: `Bearer ${accessToken}`,
    "Content-Type": "application/json",
  };
  const profile = await request.put(`${api}/api/v1/profile`, {
    headers,
    data: profileReplacement({ gender: "female", birth_date: "1994-03-12" }),
  });
  expect(profile.ok(), await profile.text()).toBeTruthy();
  return headers;
}

/** Opens one window by its deep link and returns its desktop element. */
async function openWindow(page: Page, id: DockedWindow) {
  await page.goto(`/consultation?view=${id}`);
  const window = page.locator(`.bc-business-window[data-window="${id}"]`);
  await expect(window).toBeVisible();
  await expect(window).toHaveAttribute(
    "data-dock",
    id === "person" ? "left" : "right",
  );
  // Let the layout settle (lazy windows, banner ResizeObserver).
  await page.waitForTimeout(300);
  return window;
}

function expectClose(actual: number, expected: number, label: string) {
  expect(
    Math.abs(actual - expected),
    `${label}: ${actual} vs Penpot ${expected}`,
  ).toBeLessThanOrEqual(TOLERANCE);
}

function intersects(a: Box, b: Box) {
  return (
    a.x < b.x + b.width &&
    b.x < a.x + a.width &&
    a.y < b.y + b.height &&
    b.y < a.y + a.height
  );
}

test("desktop windows dock at the Penpot geometry and leave the body visible", async ({
  page,
  request,
}) => {
  await page.setViewportSize(VIEWPORT);
  await register(page, request);
  for (const id of Object.keys(PENPOT_DOCK) as DockedWindow[]) {
    const window = await openWindow(page, id);
    await expect(page.getByTestId("safety-banner")).toHaveCount(0);
    const box = (await window.boundingBox())!;
    const penpot = PENPOT_DOCK[id];
    expectClose(box.x, penpot.x, `${id} x`);
    expectClose(box.y, penpot.y, `${id} top`);
    expectClose(box.width, penpot.width, `${id} width`);
    expectClose(box.height, penpot.height, `${id} height`);
    // The window ends above the Dock instead of running under it.
    const dock = (await page.locator(".bc-dock").boundingBox())!;
    expect(box.y + box.height, `${id} bottom above Dock`).toBeLessThanOrEqual(
      dock.y,
    );
  }
});

test("the desktop safety banner never covers a window header", async ({
  page,
  request,
}) => {
  await page.setViewportSize(VIEWPORT);
  const headers = await register(page, request);
  const opening = await request.post(`${api}/api/v1/consultation-runs`, {
    headers,
    data: {
      conversationId: null,
      clientMessageId: `e2e-dock-open-${crypto.randomUUID()}`,
      requestId: crypto.randomUUID(),
      message: { role: "user", parts: [{ type: "text", text: "你好" }] },
    },
    timeout: 60_000,
  });
  expect(opening.ok(), await opening.text()).toBeTruthy();
  const conversation = await request.get(`${api}/api/v1/conversation`, {
    headers,
  });
  expect(conversation.ok()).toBeTruthy();
  const { id: conversationId } = (
    (await conversation.json()) as { conversation: { id: string } }
  ).conversation;
  // A positive red-flag answer leaves the workspace safety review pending,
  // which shows the global banner on every surface.
  await completeStructuredSafetyIntake(
    request,
    api,
    headers,
    conversationId,
    ["麻木"],
    "我今天肩膀不舒服。",
  );

  for (const id of Object.keys(PENPOT_DOCK) as DockedWindow[]) {
    const window = await openWindow(page, id);
    const banner = page.getByTestId("safety-banner");
    await expect(banner).toBeVisible();
    const bannerBox = (await banner.boundingBox())!;
    const box = (await window.boundingBox())!;
    const header = (await window
      .locator(".bc-business-window-header")
      .boundingBox())!;
    expect(intersects(bannerBox, header), `${id} header under the banner`).toBe(
      false,
    );
    expect(bannerBox.y + bannerBox.height).toBeLessThanOrEqual(box.y);
    // Still docked to its Penpot side; only the top gives way to the banner.
    const penpot = PENPOT_DOCK[id];
    expectClose(box.x, penpot.x, `${id} x`);
    expectClose(box.width, penpot.width, `${id} width`);
    expectClose(box.y + box.height, penpot.y + penpot.height, `${id} bottom`);
  }
});
