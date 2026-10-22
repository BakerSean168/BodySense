import { expect, test, type Page } from "@playwright/test";
import {
  adoptNativeDiagnosis,
  factStatementEvidenceId,
  seedUserWithSymptom,
} from "./support/diagnosis";
import { completeStructuredSafetyIntake } from "./support/safety";

const apiBase = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";

async function openDiagnosis(page: Page, conversationId: string) {
  await page.goto(`/consultation/${conversationId}?view=diagnosis`);
  const window = page.getByTestId("diagnosis-window");
  // The lazy Diagnosis chunk can be cold on the first Vite request.
  await expect(window).toBeVisible({ timeout: 30_000 });
  return window;
}

const posted = (page: Page, path: string) =>
  page.waitForResponse(
    (response) =>
      response.url().includes(path) && response.request().method() === "POST",
  );

test("Chat request starts a formal analysis; decisions land in History", async ({
  page,
  request,
}) => {
  test.setTimeout(180_000);
  const seeded = await seedUserWithSymptom(page, request, apiBase);
  const window = await openDiagnosis(page, seeded.conversationId);
  await expect(window.getByTestId("diagnosis-empty")).toBeVisible();

  // Empty → Chat: the user's own latest message is the only candidate request.
  await window.getByRole("button", { name: "在对话里请求正式分析" }).click();
  const entry = page.getByTestId("diagnosis-request");
  await expect(entry).toBeVisible();
  await expect(entry.getByTestId("diagnosis-request-quote")).toContainText(
    "我昨天肩膀不舒服。",
  );

  const bound = posted(page, "/api/v1/diagnosis/analysis-requests");
  const started = posted(page, "/api/v1/diagnosis/analysis-runs");
  await entry.getByRole("button", { name: "开始正式分析" }).click();
  const binding = await bound;
  // The intake already bound this exact message verbatim: replayed (200).
  expect([200, 201]).toContain(binding.status());
  const request_ = (await binding.json()) as {
    excerpt: string;
    requestEvidenceId: string;
  };
  expect(request_.excerpt).toBe("我昨天肩膀不舒服。");
  const run = await started;
  expect(run.status()).toBe(201);
  expect(
    ((await run.json()) as { requestEvidenceId: string }).requestEvidenceId,
  ).toBe(request_.requestEvidenceId);
  await expect(entry).toHaveAttribute("data-state", "started");
  await expect(entry).toContainText("待你决定");

  // Decisions stay in the Diagnosis window; nothing is adopted before the receipt.
  await entry.getByRole("button", { name: /去决定/ }).click();
  const panel = window.getByTestId("diagnosis-proposals");
  await expect(panel).toHaveAttribute("data-phase", "deciding");
  const footer = window.getByTestId("diagnosis-decide");
  while (
    (await footer.getByRole("button", { name: /采纳并提交$/ }).count()) === 0
  )
    await footer.getByRole("button", { name: /^采纳$/ }).click();
  const settled = posted(page, "/settlement");
  await footer.getByRole("button", { name: /采纳并提交$/ }).click();
  expect(((await (await settled).json()) as { outcome: string }).outcome).toBe(
    "committed",
  );
  await expect(window.getByTestId("diagnosis-receipt")).toBeVisible();
  await window.getByRole("button", { name: "查看当前" }).click();

  // History: the adopted decision from Go records, reached from the header.
  await page.getByRole("button", { name: "查看诊断历史" }).click();
  const history = window.getByTestId("diagnosis-history");
  await expect(history).toBeVisible();
  const rows = history.locator(".bc-dx-timeline-row");
  await expect(rows.first()).toHaveAttribute("data-outcome", "adopted");
  await expect(rows.first()).toContainText("采纳");
  await expect(
    page.getByRole("combobox", { name: "选择已确认病情" }),
  ).toHaveAttribute("data-value", "__history__");
  await expect(
    page.getByRole("button", { name: "与助手讨论当前诊断" }),
  ).toHaveCount(0);
});

test("review prompt can be put aside without changing the Condition", async ({
  page,
  request,
}) => {
  test.setTimeout(180_000);
  const seeded = await seedUserWithSymptom(page, request, apiBase);
  const evidence = await factStatementEvidenceId(
    request,
    apiBase,
    seeded.headers,
    seeded.symptomId,
  );
  const adopted = await adoptNativeDiagnosis(
    request,
    apiBase,
    seeded.headers,
    evidence,
  );
  // A later Safety screen is relevant after the analysis: review recommended.
  await completeStructuredSafetyIntake(
    request,
    apiBase,
    seeded.headers,
    seeded.conversationId,
    ["以上均无"],
    "我今天肩膀不舒服。",
  );
  const window = await openDiagnosis(page, seeded.conversationId);
  const banner = window.getByTestId("diagnosis-review");
  await expect(banner).toBeVisible();
  await expect(
    window.locator(`[data-condition-id="${adopted.conditionIds[0]}"]`),
  ).toHaveAttribute("data-freshness", "review_recommended");

  const dismissed = posted(page, "/review-prompt-dismissals");
  await banner.getByRole("button", { name: "暂不处理" }).click();
  const receipt = await dismissed;
  expect(receipt.status(), await receipt.text()).toBe(200);
  await expect(window.getByTestId("diagnosis-review")).toHaveCount(0);

  // The Condition itself never changed.
  const after = await request.get(`${apiBase}/api/v1/diagnosis`, {
    headers: seeded.headers,
  });
  const body = (await after.json()) as {
    currentRevision: number;
    conditionEntries: Array<{
      disposition: string;
      freshness: { state: string };
      reviewPromptDismissal?: { dismissalId: string };
    }>;
  };
  expect(body.currentRevision).toBe(adopted.revision);
  expect(body.conditionEntries[0]!.disposition).toBe("adopted");
  expect(body.conditionEntries[0]!.freshness.state).toBe("review_recommended");
  expect(body.conditionEntries[0]!.reviewPromptDismissal).toBeTruthy();
  // Re-raising on newer evidence is proven against PG18 in
  // TestDiagnosisReviewPromptDismissalPostgres.
});

test("Safety stopped opens Safety / Restricted and fits 390px", async ({
  page,
  request,
}) => {
  test.setTimeout(180_000);
  await page.setViewportSize({ width: 390, height: 844 });
  const seeded = await seedUserWithSymptom(page, request, apiBase);
  const evidence = await factStatementEvidenceId(
    request,
    apiBase,
    seeded.headers,
    seeded.symptomId,
  );
  const diagnosis = await request.get(`${apiBase}/api/v1/diagnosis`, {
    headers: seeded.headers,
  });
  const { currentRevision } = (await diagnosis.json()) as {
    currentRevision: number;
  };
  const start = await request.post(
    `${apiBase}/api/v1/diagnosis/analysis-runs`,
    {
      headers: seeded.headers,
      data: {
        requestEvidenceId: evidence,
        trigger: "new_analysis",
        expectedRevision: currentRevision,
        idempotencyKey: `e2e-dxh-start-${crypto.randomUUID()}`,
      },
      timeout: 60_000,
    },
  );
  expect(start.status(), await start.text()).toBe(201);
  await completeStructuredSafetyIntake(
    request,
    apiBase,
    seeded.headers,
    seeded.conversationId,
    ["麻木"],
    "我今天肩膀不舒服。",
  );

  const window = await openDiagnosis(page, seeded.conversationId);
  await expect(window.getByTestId("diagnosis-safety")).toBeVisible();
  const open = window.getByTestId("diagnosis-safety-open");
  expect((await open.boundingBox())!.height).toBeGreaterThanOrEqual(44);
  await open.click();

  const restricted = page.getByTestId("safety-restricted");
  await expect(restricted).toBeVisible();
  await expect(page.getByTestId("training-mode")).toHaveText("安全限制");
  await expect(restricted).toContainText("麻木");
  await expect(restricted).toContainText("暂不能执行");
  // The mobile safety banner stays above the sheet: it never covers the sheet
  // header or the status chips (暂不能执行 / 全部训练).
  const banner = page.getByTestId("safety-banner");
  await expect(banner).toBeVisible();
  const bannerBox = (await banner.boundingBox())!;
  const sheetBox = (await page
    .locator('.bc-business-sheet[data-window="training"]')
    .boundingBox())!;
  expect(bannerBox.y + bannerBox.height).toBeLessThanOrEqual(sheetBox.y);
  const chips = restricted.locator(".bc-sr-item .bc-dx-status .bc-dx-chip");
  await expect(chips.first()).toHaveText("暂不能执行");
  await expect(chips.nth(1)).toHaveText("全部训练");
  for (const chip of await chips.all()) {
    const box = (await chip.boundingBox())!;
    const overlaps =
      box.x < bannerBox.x + bannerBox.width &&
      bannerBox.x < box.x + box.width &&
      box.y < bannerBox.y + bannerBox.height &&
      bannerBox.y < box.y + box.height;
    expect(overlaps, `${await chip.textContent()} under the banner`).toBe(
      false,
    );
  }
  const overflow = await page.evaluate(() => {
    const body = document.querySelector(
      '[data-testid="safety-restricted"] .bc-window-body',
    ) as HTMLElement;
    return {
      doc: document.documentElement.scrollWidth,
      body: body.scrollWidth,
      client: body.clientWidth,
    };
  });
  expect(overflow.doc).toBeLessThanOrEqual(390);
  expect(overflow.body).toBeLessThanOrEqual(overflow.client);
  for (const button of await restricted
    .locator(".bc-dx-footer")
    .getByRole("button")
    .all()) {
    const box = await button.boundingBox();
    expect(box!.height).toBeGreaterThanOrEqual(44);
    expect(box!.x + box!.width).toBeLessThanOrEqual(390);
  }
  await restricted.getByRole("button", { name: "返回计划" }).click();
  await expect(page.getByTestId("safety-restricted")).toHaveCount(0);
});
