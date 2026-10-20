import {
  expect,
  test,
  type APIRequestContext,
  type Page,
} from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { profileReplacement } from "./support/profile";
import { fillRegisterPassword } from "./support/register";

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
  await fillRegisterPassword(page, "BodySenseE2E!123");
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

async function safetyReviewPending(
  request: APIRequestContext,
  headers: Record<string, string>,
) {
  const workspace = await request.get(`${api}/api/v1/health-workspace`, {
    headers,
  });
  expect(workspace.ok(), await workspace.text()).toBeTruthy();
  const body = (await workspace.json()) as {
    capabilities: { requires_safety_review: boolean };
  };
  return body.capabilities.requires_safety_review;
}

/**
 * Records an unscreened symptom straight into BodyState. No consultation Run
 * is started, so the spec does not depend on how the model answers (a real
 * model may ask a follow-up question and keep the Run open). An unscreened
 * symptom owes a Safety review, which shows the global banner everywhere.
 */
async function recordUnscreenedSymptom(
  request: APIRequestContext,
  headers: Record<string, string>,
) {
  const write = await request.post(`${api}/api/v1/body-state/records`, {
    headers,
    data: {
      idempotency_key: `e2e-dock-symptom-${crypto.randomUUID()}`,
      expected_revision: 0,
      facts: [
        {
          kind: "symptom",
          statement: "我今天肩膀不舒服。",
          body_region_ids: [],
          temporal: { precision: "unknown" },
          details: {},
        },
      ],
    },
  });
  expect(write.ok(), await write.text()).toBeTruthy();
  expect(await safetyReviewPending(request, headers)).toBe(true);
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
  // A never-written BodyState owes no Safety review, so no chat is needed.
  expect(await safetyReviewPending(request, headers)).toBe(false);
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
  // An unscreened symptom leaves the workspace safety review pending, which
  // shows the global banner on every surface.
  await recordUnscreenedSymptom(request, headers);

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
  // A never-written BodyState owes no Safety review, so no chat is needed.
  expect(await safetyReviewPending(request, headers)).toBe(false);

  // Home: motion ribbon, Tools rail and the icon-only Dock (/ D P M).
  await page.goto("/consultation");
  const intro = page.getByRole("group", { name: "参考动作" });
  const tools = page.getByRole("toolbar", { name: "身体画布工具" });
  const dock = page.getByRole("navigation", { name: "身体工作区导航" });
  await expect(intro).toBeVisible();
  await expect(tools).toBeVisible();
  await expect(dock.locator(".bc-dock-item")).toHaveText([
    /对话/,
    /诊断/,
    /训练/,
    /个人/,
  ]);
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

  // Penpot "Home · Dock collapsed" / "Home · Tools collapsed": the chevrons
  // fold the Dock to a small pill and the rail to a side tab, and restore.
  await dock.getByRole("button", { name: "收起导航" }).click();
  const pill = dock.getByRole("button", { name: "展开导航" });
  await expect(pill).toBeFocused();
  await expect(dock.getByRole("button", { name: "个人档案" })).toHaveCount(0);
  const pillBox = (await dock.boundingBox())!;
  expectClose(pillBox.height, 32, "collapsed Dock height");
  expect(pillBox.width).toBeLessThanOrEqual(60);
  await pill.click();
  await expect(
    dock.getByRole("button", { name: "个人档案", exact: true }),
  ).toBeVisible();
  await tools.getByRole("button", { name: "收起工具栏" }).click();
  await expect(tools).toHaveCount(0);
  const tab = page.getByRole("button", { name: "展开工具栏" });
  await expect(tab).toBeFocused();
  await tab.click();
  await expect(tools).toBeVisible();

  // Right-docked windows: the body keeps its place (Penpot does not move it),
  // the motion ribbon stays above the windows (Penpot boards), the rail stays.
  for (const id of ["diagnosis", "training", "trainer"] as const) {
    await openWindow(page, id);
    await expect(intro).toBeVisible();
    await expect(tools).toBeVisible();
  }

  // Left-docked Person: the Tools rail collapses instead of peeking out.
  const person = await openWindow(page, "person");
  await expect(intro).toBeVisible();
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

  // Mobile keeps its sheet and an icon Dock (Penpot Mobile / Home, M P D /);
  // nothing desktop leaks in.
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/consultation");
  await expect(dock.locator(".bc-dock-item")).toHaveCount(4);
  await expect(dock.locator(".bc-dock-item").first()).toHaveAttribute(
    "aria-label",
    "个人档案",
  );
  await expect(
    dock.getByRole("button", { name: "个人档案", exact: true }),
  ).toBeVisible();
  await expect(intro).toBeVisible();
  await expect(tools).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth + 1,
    ),
  ).toBe(true);
});
