import {
  expect,
  test,
  type APIRequestContext,
  type Page,
} from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";
import { profileReplacement } from "./support/profile";

// AS-002 deletion flows for an uploaded file in the profile drawer: source
// removal (the original goes, derived records stay with a source-unavailable
// marker) versus verified erasure (every controlled copy, reported done only
// once every target verified). The real Go/PostgreSQL path, plus the receipt
// states a fresh account cannot reach (retrying, review) via route fixtures.
test.describe.configure({ timeout: 90_000 });

const api = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";

// Two distinct 1x1 PNGs, so the uploads never share a content hash.
const PNG_A =
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==";
const PNG_B =
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==";

type Headers = { Authorization: string };

async function register(page: Page, request: APIRequestContext) {
  const email = `asset-erasure-${crypto.randomUUID()}@example.test`;
  await page.goto("/register");
  await page.getByLabel("邮箱地址").fill(email);
  await page.getByLabel("密码", { exact: true }).fill("BodySenseE2E!123");
  await page.getByRole("checkbox", { name: /同意/ }).check();
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding)/);
  const accessToken = await refreshBrowserAccessToken(page, api);
  const headers = { Authorization: `Bearer ${accessToken}` };
  const profile = await request.put(`${api}/api/v1/profile`, {
    headers,
    data: profileReplacement({ gender: "male", birth_date: "2000-01-01" }),
  });
  expect(profile.ok()).toBeTruthy();
  return headers;
}

async function upload(
  request: APIRequestContext,
  headers: Headers,
  name: string,
  png: string,
): Promise<string> {
  const response = await request.post(`${api}/api/v1/uploads`, {
    headers,
    multipart: {
      file: {
        name,
        mimeType: "image/png",
        buffer: Buffer.from(png, "base64"),
      },
      file_type: "consultation_photo",
    },
  });
  expect(response.status(), await response.text()).toBe(201);
  return ((await response.json()) as { id: string }).id;
}

async function openFiles(page: Page) {
  await page.goto("/consultation?view=person");
  await page.locator(".bc-person-identity").click();
  await page.getByRole("button", { name: "身体档案与文件" }).click();
  await page.getByRole("button", { name: /^文件管理/ }).click();
}

function item(page: Page, name: string) {
  return page.getByTestId("upload-item").filter({ hasText: name });
}

async function listed(request: APIRequestContext, headers: Headers) {
  const response = await request.get(`${api}/api/v1/uploads`, { headers });
  expect(response.ok()).toBeTruthy();
  return (
    (await response.json()) as {
      uploads: Array<{ id: string; source_availability: string }>;
    }
  ).uploads;
}

test("source removal keeps the record listed; erasure verifies every copy and drops it", async ({
  page,
  request,
}, testInfo) => {
  const headers = await register(page, request);
  const removedId = await upload(request, headers, "source-removal.png", PNG_A);
  const erasedId = await upload(request, headers, "full-erasure.png", PNG_B);
  await openFiles(page);

  const removed = item(page, "source-removal.png");
  await expect(removed).toBeVisible();
  page.once("dialog", (dialog) => {
    expect(dialog.message()).toContain("只删除原始文件");
    void dialog.accept();
  });
  await removed.getByRole("button", { name: "移除原件" }).click();
  await expect(
    removed.getByText("原件已移除 · 整理出的记录保留"),
  ).toBeVisible();
  await expect(removed.getByRole("button", { name: "移除原件" })).toHaveCount(
    0,
  );
  await expect(removed.getByRole("button", { name: "彻底删除" })).toBeVisible();

  const erased = item(page, "full-erasure.png");
  await erased.getByRole("button", { name: "彻底删除" }).click();
  const panel = erased.getByTestId("asset-erasure");
  await expect(panel).toHaveAttribute("data-phase", "confirm");
  await panel.getByRole("button", { name: "确认彻底删除" }).click();
  await expect(panel).toHaveAttribute("data-phase", "erased");
  await expect(panel.getByText("已彻底删除")).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("asset-erasure-done.png"),
    fullPage: true,
  });
  await panel.getByRole("button", { name: "完成" }).click();
  await expect(item(page, "full-erasure.png")).toHaveCount(0);
  await expect(item(page, "source-removal.png")).toBeVisible();

  const uploads = await listed(request, headers);
  expect(uploads.map((u) => u.id)).toEqual([removedId]);
  expect(uploads[0].source_availability).toBe("source_unavailable");
  const receipt = await request.get(
    `${api}/api/v1/assets/${erasedId}/erasure`,
    {
      headers,
    },
  );
  expect(receipt.ok()).toBeTruthy();
  const body = await receipt.json();
  expect(body.status).toBe("erased");
  expect(body.outstanding_targets).toEqual([]);
});

const targetNames = [
  "source_objects",
  "derived_jobs",
  "posture_analyses",
  "diagnosis_records",
  "document_extractions",
  "assessment_reports",
  "runtime_checkpoints",
  "asset_manifest",
];

function syntheticReceipt(
  assetId: string,
  status: string,
  overrides: Record<string, Record<string, unknown>> = {},
  reviews: Array<Record<string, unknown>> = [],
) {
  const targets = targetNames.map((target) => {
    const row: Record<string, unknown> = {
      target,
      state: "verified",
      erased_count: 0,
      outstanding_count: 0,
      verified_at: "2026-10-19T00:00:00Z",
      ...overrides[target],
    };
    if (row.state !== "verified") delete row.verified_at;
    return row;
  });
  return {
    request_id: crypto.randomUUID(),
    asset_id: assetId,
    status,
    inventory_revision: "asset-erasure-inventory-v1",
    requested_at: "2026-10-19T00:00:00Z",
    targets,
    outstanding_targets: targets
      .filter((t) => t.state !== "verified")
      .map((t) => t.target),
    reviews,
  };
}

test("asset erasure receipt shows retrying and review states and never claims done early", async ({
  page,
  request,
}) => {
  const headers = await register(page, request);
  await upload(request, headers, "pinned-report.png", PNG_A);
  const states: Array<(assetId: string) => object> = [
    (id) =>
      syntheticReceipt(id, "outstanding", {
        derived_jobs: {
          state: "failed",
          failure_code: "derived_work_active",
          outstanding_count: 1,
        },
        asset_manifest: {
          state: "blocked",
          failure_code: "targets_outstanding",
          outstanding_count: 1,
        },
      }),
    (id) =>
      syntheticReceipt(
        id,
        "outstanding",
        {
          diagnosis_records: {
            state: "blocked",
            failure_code: "diagnosis_review_required",
            outstanding_count: 1,
          },
          asset_manifest: {
            state: "blocked",
            failure_code: "targets_outstanding",
            outstanding_count: 1,
          },
        },
        [{ subject: "diagnosis_condition", subject_id: crypto.randomUUID() }],
      ),
  ];
  let call = 0;
  await page.route("**/api/v1/assets/*/erasure", async (route) => {
    const assetId = route.request().url().split("/assets/")[1].split("/")[0];
    const body = states[Math.min(call, states.length - 1)](assetId);
    call += 1;
    await route.fulfill({ status: 202, json: body });
  });
  await openFiles(page);

  const pinned = item(page, "pinned-report.png");
  await pinned.getByRole("button", { name: "彻底删除" }).click();
  const panel = pinned.getByTestId("asset-erasure");
  await panel.getByRole("button", { name: "确认彻底删除" }).click();
  await expect(panel).toHaveAttribute("data-phase", "outstanding");
  await expect(
    panel.getByText("识别或分析仍在进行，完成后会自动重试"),
  ).toBeVisible();
  await expect(panel.getByText("已彻底删除")).toHaveCount(0);

  await panel.getByRole("button", { name: "再次检查" }).click();
  await expect(panel).toHaveAttribute("data-phase", "review");
  await expect(panel.getByText("需要你先处理")).toBeVisible();
  await expect(
    panel.getByText("诊断条目：请在诊断窗口修改诊断或撤回该条目"),
  ).toBeVisible();
  await panel.getByRole("button", { name: "稍后再看" }).click();
  await expect(pinned).toBeVisible();
});
