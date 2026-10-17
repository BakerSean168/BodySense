import { expect, test, type Page } from "@playwright/test";

// Native transport fixtures only: no live backend, no claimed Go/PG persistence or Penpot approval.
async function expectPersonFits(page: Page) {
  const geometry = await page.locator(".bc-person-body").evaluate((element) => {
    const bounds = element.getBoundingClientRect();
    return {
      horizontalOverflow: element.scrollWidth - element.clientWidth,
      pageOverflow: document.documentElement.scrollWidth - window.innerWidth,
      clipped: Array.from(
        element.querySelectorAll(
          "button,textarea,input,select,fieldset,p,strong,h4",
        ),
      )
        .filter((node) => node.getClientRects().length > 0)
        .filter((node) => {
          const rect = node.getBoundingClientRect();
          return rect.left < bounds.left - 1 || rect.right > bounds.right + 1;
        })
        .map((node) => node.textContent?.slice(0, 80) || node.tagName),
    };
  });
  expect(geometry).toEqual({
    horizontalOverflow: 0,
    pageOverflow: 0,
    clipped: [],
  });
}

test("Person responsive keyboard, source badges, stale retry and receipt flow at 390/840/1280px", async ({
  page,
}, testInfo) => {
  test.setTimeout(90_000);
  await page.setViewportSize({ width: 390, height: 844 });
  const owner = "11111111-1111-4111-8111-111111111111";
  const id = "22222222-2222-4222-8222-222222222222";
  const successorId = "33333333-3333-4333-8333-333333333333";
  const factId = "44444444-4444-4444-8444-444444444444";
  const observationId = "55555555-5555-4555-8555-555555555555";
  const now = "2026-10-17T00:00:00Z";
  const statement = "我猜与久坐有关_" + "LongUnbrokenUserStatement".repeat(12);
  const fact = {
    id: factId,
    user_id: owner,
    kind: "symptom",
    subject: "self",
    statement: "我的肩部记录_" + "LongUnbrokenSource".repeat(12),
    body_region_ids: [],
    temporal: { precision: "unknown" },
    details: { symptom_type: "肩部不适", presence: "present" },
    source_evidence_id: factId,
    assertion_status: "asserted",
    created_revision: 1,
    recorded_at: now,
  };
  const observation = {
    id: observationId,
    user_id: owner,
    kind: "body_weight",
    statement: "我今天称重",
    raw_value: "67",
    raw_unit: "kg",
    observed_at: { precision: "unknown" },
    source: { kind: "user_report", id: observationId, version: 1 },
    assertion_status: "asserted",
    created_revision: 2,
    recorded_at: now,
  };
  const episodes = ["ongoing", "ended", "unknown"].map((state, index) => ({
    id: `66666666-6666-4666-8666-66666666666${index}`,
    user_id: owner,
    symptom_type: "肩部不适_" + "LongSymptom".repeat(6),
    body_region_ids: [],
    laterality: "unknown",
    identity_status: "current",
    created_revision: 3,
    created_at: now,
    links: [
      {
        fact_id: factId,
        role: "observation",
        association_method: "explicit",
        is_primary: true,
        linked_revision: 3,
      },
    ],
    projection: {
      state,
      reason:
        state === "ongoing"
          ? "explicit_current_report"
          : state === "ended"
            ? "explicit_resolution"
            : "conflicting_evidence",
      basis_fact_ids: [factId],
      based_on_revision: 3,
      ...(state !== "unknown" ? { state_as_of: "2026-10-15" } : {}),
    },
  }));
  let revision = 3;
  let hypotheses: { id: string; statement: string }[] = [];
  const commands: { operation: string; body: Record<string, unknown> }[] = [];
  let staleOnce = true;
  let groupsUnavailable = true;
  let releaseGroups!: () => void;
  let releaseReceipt!: () => void;
  const groupGate = new Promise<void>((resolve) => {
    releaseGroups = resolve;
  });
  const receiptGate = new Promise<void>((resolve) => {
    releaseReceipt = resolve;
  });
  await page.route("**/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    const reply = (body: unknown, status = 200) =>
      route.fulfill({ status, json: body });
    if (path.endsWith("/auth/refresh"))
      return reply({ access_token: "fixture-token", expires_in: 3600 });
    if (path.endsWith("/me"))
      return reply({ id: owner, email: "synthetic@example.test" });
    if (path.endsWith("/health-workspace"))
      return reply({
        generated_at: now,
        profile_ready: true,
        treatment_revisions: [],
        recent_outcomes: [],
        trends: [],
        actions: [],
        capabilities: {
          can_continue_consultation: true,
          can_edit_body_state: true,
          can_request_diagnosis: false,
          can_review_diagnosis: false,
          can_generate_treatment: false,
          can_accept_treatment: false,
          can_execute_treatment: false,
          can_record_outcome: false,
          can_review_treatment: false,
          requires_safety_review: false,
          requires_diagnosis_review: false,
          requires_treatment_review: false,
        },
      });
    if (path.endsWith("/profile"))
      return reply({
        profile: {
          id: owner,
          user_id: owner,
          gender: "male",
          birth_date: "2000-01-01",
          created_at: now,
          updated_at: now,
        },
      });
    if (
      path.endsWith("/body-state/hypothesis-records") &&
      route.request().method() === "GET"
    )
      return reply({ user_id: owner, current_revision: revision, hypotheses });
    if (
      path.includes("/body-state/hypothesis-records") &&
      route.request().method() === "POST"
    ) {
      const body = route.request().postDataJSON();
      const operation = path.endsWith("/correct")
        ? "correct"
        : path.endsWith("/withdraw")
          ? "withdraw"
          : "record";
      commands.push({ operation, body });
      if (staleOnce) {
        staleOnce = false;
        revision += 1;
        return reply(
          { error: { code: "REVISION_CONFLICT", message: "stale" } },
          409,
        );
      }
      const target = operation === "correct" ? id : successorId;
      const resultId = operation === "record" ? id : successorId;
      if (operation !== "record")
        expect(path).toContain(`/hypothesis-records/${target}/`);
      if (body.expected_revision !== revision)
        return reply(
          { error: { code: "REVISION_CONFLICT", message: "stale" } },
          409,
        );
      if (operation === "record") await receiptGate;
      revision += 1;
      const replacement = operation === "correct" ? body.replacement : body;
      hypotheses =
        operation === "withdraw"
          ? []
          : [
              {
                id: resultId,
                user_id: owner,
                lineage_id: id,
                statement: replacement.statement,
                source: { actor: "user", kind: "direct_edit", id, version: 1 },
                related_fact_ids: replacement.related_fact_ids,
                related_observation_ids: replacement.related_observation_ids,
                assertion_status: "asserted",
                created_revision: revision,
                recorded_at: now,
                ...(operation === "correct"
                  ? { supersedes_hypothesis_id: id }
                  : {}),
              },
            ];
      return reply({
        id,
        user_id: owner,
        operation,
        hypothesis_id: resultId,
        ...(operation !== "record" ? { target_hypothesis_id: target } : {}),
        source_evidence_id: id,
        revision,
        revision_id: id,
        event_id: id,
        committed_at: now,
      });
    }
    if (path.endsWith("/body-state/concern-groups")) {
      await groupGate;
      if (groupsUnavailable)
        return reply(
          { error: { code: "UNAVAILABLE", message: "offline" } },
          503,
        );
      return reply({
        user_id: owner,
        current_revision: revision,
        groups: hypotheses.length
          ? [
              {
                id,
                display_title: "相关记录_" + "LongGroupTitle".repeat(12),
                latest_revision: revision,
                related_hypothesis_ids: [hypotheses[0].id],
                related_fact_ids: [factId],
                related_observation_ids: [observationId],
                related_episode_ids: [episodes[0].id],
              },
            ]
          : [],
      });
    }
    for (const [suffix, field, rows] of [
      ["records", "facts", [fact]],
      ["observation-records", "observations", [observation]],
      ["episodes", "episodes", episodes],
    ] as const) {
      if (path.endsWith(`/body-state/${suffix}`))
        return reply({
          user_id: owner,
          current_revision: revision,
          [field]: rows,
          has_more: false,
        });
    }
    return reply(
      { error: { code: "FIXTURE_UNAVAILABLE", message: "outside Web test" } },
      503,
    );
  });
  try {
    await page.goto("/consultation?view=state", {
      waitUntil: "domcontentloaded",
    });
    const panel = page.getByRole("region", { name: "我的猜测与相关记录" });
    await expect(panel.getByText("正在读取猜测与分组…")).toBeVisible();
    await expectPersonFits(page);
    releaseGroups();
    await expect(panel.getByRole("alert")).toContainText("读取失败");
    groupsUnavailable = false;
    await panel.getByRole("button", { name: "重新读取猜测与分组" }).click();
    await expect(panel.getByText("还没有你提出的猜测。")).toBeVisible();
    await expect(panel.getByText("暂无相关记录分组。")).toBeVisible();
    await page.getByRole("button", { name: "查看与关联经历" }).click();
    await expect(
      page.getByText("截至明确报告时仍在持续", { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByText("有明确的结束报告", { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByText("持续状态尚未确认", { exact: true }),
    ).toBeVisible();
    const canvas = await page.locator(".bc-scene canvas").elementHandle();
    expect(canvas).not.toBeNull();
    await panel.getByRole("button", { name: "添加我的猜测" }).focus();
    await page.keyboard.press("Enter");
    await expect(panel.getByLabel("我的完整猜测")).toBeFocused();
    await panel.getByLabel("我的完整猜测").fill(statement);
    await page.keyboard.press("Escape");
    await expect(
      panel.getByRole("button", { name: "添加我的猜测" }),
    ).toBeFocused();
    await expect(page.getByTestId("person-window")).toBeVisible();
    await page.keyboard.press("Enter");
    await panel.getByLabel("我的完整猜测").fill(statement);
    for (const width of [390, 840, 1280, 390]) {
      await page.setViewportSize({ width, height: 844 });
      await expect(panel.getByLabel("我的完整猜测")).toHaveValue(statement);
      await expectPersonFits(page);
      expect(await canvas!.evaluate((node) => node.isConnected)).toBe(true);
      await page.screenshot({
        path: testInfo.outputPath(`person-${width}.png`),
      });
    }
    await panel.getByRole("button", { name: "保存猜测" }).click();
    await expect(panel.getByRole("alert")).toContainText("记录已变化或不可用");
    await expect(panel.getByText("你的猜测已保存。")).toHaveCount(0);
    await expect(
      panel.getByRole("button", { name: "添加我的猜测" }),
    ).toBeEnabled();
    await panel.getByRole("button", { name: "添加我的猜测" }).click();
    await panel.getByLabel("我的完整猜测").fill(statement);
    await panel.getByLabel(fact.statement, { exact: true }).check();
    await panel.getByLabel(observation.statement, { exact: true }).check();
    await panel.getByRole("button", { name: "保存猜测" }).click();
    await expect(panel.getByText("正在提交，等待保存回执。")).toBeVisible();
    await expect(panel.getByText("你的猜测已保存。")).toHaveCount(0);
    await page.keyboard.press("Escape");
    await expect(
      panel.getByRole("form", { name: "编辑我的猜测" }),
    ).toBeVisible();
    releaseReceipt();
    await expect(panel.getByText("你的猜测已保存。")).toBeFocused();
    await expect(panel.getByText(statement, { exact: true })).toBeVisible();
    await expect(panel.getByText(`来源事实：${fact.statement}`)).toBeVisible();
    await expect(
      panel.getByText("来源观察：我今天称重 · 67 · kg"),
    ).toBeVisible();
    await expectPersonFits(page);
    await panel.getByRole("button", { name: "更正猜测" }).click();
    await expect(panel.getByLabel("我的完整猜测")).toBeFocused();
    await panel.getByLabel("我的完整猜测").fill("我猜与运动有关");
    await panel.getByRole("button", { name: "保存猜测更正" }).click();
    await expect(
      panel.getByText("我猜与运动有关", { exact: true }),
    ).toBeVisible();
    await panel.getByRole("button", { name: "撤回猜测" }).click();
    await expect(
      panel.getByRole("button", { name: "确认撤回猜测" }),
    ).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(panel.getByText("还没有你提出的猜测。")).toBeVisible();
    await expectPersonFits(page);
    expect(commands.map((command) => command.body.expected_revision)).toEqual([
      3, 4, 5, 6,
    ]);
    expect(commands[1].body).toMatchObject({
      related_fact_ids: [factId],
      related_observation_ids: [observationId],
    });
  } finally {
    releaseGroups();
    releaseReceipt();
  }
});
