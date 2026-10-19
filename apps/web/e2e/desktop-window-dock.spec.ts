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

// Penpot "Desktop / Home" Dock bar: y 800, 72 tall (icon-only).
const PENPOT_DOCK_BAR = { y: 800, height: 72 };
// Penpot Body / Episode, Body / Region and Assets scenes: 40/92/560/700.
const PENPOT_PERSON_SCENE: Box = { x: 40, y: 92, width: 560, height: 700 };

async function register(page: Page, request: APIRequestContext) {
  await page.goto("/register");
  await page
    .getByLabel("邮箱地址")
    .fill(`window-dock-${crypto.randomUUID()}@example.test`);
  await page.getByLabel("密码", { exact: true }).fill("BodySenseE2E!123");
  await page.getByRole("checkbox", { name: /同意/ }).check();
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

/**
 * Answers the structured Safety screening in a fresh conversation. A new
 * account has no screening yet, so the workspace starts with a safety review
 * (and the global banner); "none of the above" clears it.
 */
async function screenSafety(
  request: APIRequestContext,
  headers: Record<string, string>,
  signals?: string[],
  statement?: string,
) {
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
  const { id } = (
    (await conversation.json()) as { conversation: { id: string } }
  ).conversation;
  await completeStructuredSafetyIntake(
    request,
    api,
    headers,
    id,
    signals,
    statement,
  );
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
  const headers = await register(page, request);
  await screenSafety(request, headers);
  for (const id of Object.keys(PENPOT_DOCK) as DockedWindow[]) {
    const window = await openWindow(page, id);
    await expect(page.getByTestId("safety-banner")).toHaveCount(0);
    const box = (await window.boundingBox())!;
    const penpot = PENPOT_DOCK[id];
    expectClose(box.x, penpot.x, `${id} x`);
    expectClose(box.y, penpot.y, `${id} top`);
    expectClose(box.width, penpot.width, `${id} width`);
    // Penpot ends the window at y 792, 8px above its 72px icon Dock (y 800
    // to 872); the shipped Dock matches it, so windows get the full 700px.
    expectClose(box.height, penpot.height, `${id} height`);
    const dock = (await page.locator(".bc-dock").boundingBox())!;
    expectClose(dock.y, PENPOT_DOCK_BAR.y, "Dock top");
    expectClose(dock.height, PENPOT_DOCK_BAR.height, "Dock height");
    expect(
      box.y + box.height + 8,
      `${id} bottom above Dock`,
    ).toBeLessThanOrEqual(dock.y + 1);
  }
});

test("the desktop safety banner never covers a window header", async ({
  page,
  request,
}) => {
  await page.setViewportSize(VIEWPORT);
  const headers = await register(page, request);
  // A positive red-flag answer leaves the workspace safety review pending,
  // which shows the global banner on every surface.
  await screenSafety(request, headers, ["麻木"], "我今天肩膀不舒服。");

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
    const dock = (await page.locator(".bc-dock").boundingBox())!;
    expectClose(
      box.y + box.height,
      Math.min(penpot.y + penpot.height, dock.y - 8),
      `${id} bottom`,
    );
  }
});

test("desktop chrome follows the Penpot window boards", async ({
  page,
  request,
}) => {
  await page.setViewportSize(VIEWPORT);
  const headers = await register(page, request);
  await screenSafety(request, headers);

  // Home: intro, Tools rail and the icon-only Dock with accessible names.
  await page.goto("/consultation");
  const intro = page.locator(".bc-intro");
  const tools = page.getByRole("toolbar", { name: "身体画布工具" });
  const dock = page.getByRole("navigation", { name: "身体工作区导航" });
  await expect(intro).toBeVisible();
  await expect(tools).toBeVisible();
  for (const name of ["个人档案", "训练计划", "综合诊断", "对话"]) {
    const item = dock.getByRole("button", { name, exact: true });
    await expect(item).toBeVisible();
    await expect(item).toHaveAttribute("title", new RegExp(`^${name} · `));
    // The text label is only visually hidden on desktop (Penpot icon Dock).
    const label = (await item.locator(".bc-dock-label").boundingBox())!;
    expect(label.width).toBeLessThanOrEqual(1);
  }
  const bar = (await dock.boundingBox())!;
  expectClose(bar.y, PENPOT_DOCK_BAR.y, "Dock top");
  expectClose(bar.height, PENPOT_DOCK_BAR.height, "Dock height");
  // Keyboard users still reach every Dock entry.
  await dock.getByRole("button", { name: "个人档案", exact: true }).focus();
  await expect(
    dock.getByRole("button", { name: "个人档案", exact: true }),
  ).toBeFocused();

  // Right-docked windows: the body keeps its place (Penpot does not move it),
  // the intro gives way, the Tools rail stays.
  for (const id of ["diagnosis", "training", "trainer"] as const) {
    await openWindow(page, id);
    await expect(intro).toBeHidden();
    await expect(tools).toBeVisible();
  }

  // Left-docked Person: the Tools rail collapses instead of peeking out.
  const person = await openWindow(page, "person");
  await expect(intro).toBeHidden();
  await expect(tools).toBeHidden();
  expectClose((await person.boundingBox())!.width, 860, "person list width");

  // Person detail scenes are 560 wide from the left dock, then back to 860.
  await person.getByRole("button", { name: "经历 0 段" }).click();
  await expect(page.getByTestId("person-episodes")).toBeVisible();
  await expect
    .poll(async () => Math.round((await person.boundingBox())!.width))
    .toBeLessThanOrEqual(PENPOT_PERSON_SCENE.width + TOLERANCE);
  const scene = (await person.boundingBox())!;
  expectClose(scene.x, PENPOT_PERSON_SCENE.x, "scene x");
  expectClose(scene.y, PENPOT_PERSON_SCENE.y, "scene top");
  expectClose(scene.width, PENPOT_PERSON_SCENE.width, "scene width");
  expectClose(scene.height, PENPOT_PERSON_SCENE.height, "scene height");
  await person.getByRole("button", { name: "返回记录" }).click();
  await expect
    .poll(async () => Math.round((await person.boundingBox())!.width))
    .toBeGreaterThanOrEqual(860 - TOLERANCE);

  // Closing the window brings the intro and the Tools rail back.
  await person.getByRole("button", { name: "关闭个人档案" }).click();
  await expect(person).toBeHidden();
  await expect(intro).toBeVisible();
  await expect(tools).toBeVisible();

  // Reduced motion: the chrome changes without animating.
  await page.emulateMedia({ reducedMotion: "reduce" });
  const duration = await tools.evaluate(
    (node) => getComputedStyle(node).transitionDuration,
  );
  expect(
    duration.split(",").every((value) => parseFloat(value) < 0.01),
    duration,
  ).toBe(true);
  await page.emulateMedia({ reducedMotion: null });

  // Mobile keeps its labelled Dock and its sheet; nothing desktop leaks in.
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/consultation");
  await expect(dock.locator(".bc-dock-item")).toHaveCount(4);
  await expect(dock.getByText("个人", { exact: true })).toBeVisible();
  await expect(intro).toBeVisible();
  await expect(tools).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth + 1,
    ),
  ).toBe(true);
});
