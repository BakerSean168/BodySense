import { expect, test } from "@playwright/test";

// Bounded Web transport test. Fixtures exercise the generated native client;
// they are not evidence of a real Go/PostgreSQL commit (covered by API tests).
test("Person hypothesis create, receipt, correction and withdrawal through native transport", async ({
  page,
}) => {
  test.setTimeout(60_000);
  const owner = "11111111-1111-4111-8111-111111111111";
  const id = "22222222-2222-4222-8222-222222222222";
  const successorId = "33333333-3333-4333-8333-333333333333";
  const now = "2026-10-17T00:00:00Z";
  let revision = 0;
  let hypotheses: object[] = [];
  const commands: { operation: string; body: Record<string, unknown> }[] = [];
  let releaseReceipt!: () => void;
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
      hypotheses =
        operation === "withdraw"
          ? []
          : [
              {
                id: resultId,
                user_id: owner,
                lineage_id: id,
                statement:
                  operation === "correct"
                    ? body.replacement.statement
                    : body.statement,
                source: { actor: "user", kind: "direct_edit", id, version: 1 },
                related_fact_ids: [],
                related_observation_ids: [],
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
    if (path.endsWith("/body-state/concern-groups"))
      return reply({ user_id: owner, current_revision: revision, groups: [] });
    for (const [suffix, field] of [
      ["records", "facts"],
      ["observation-records", "observations"],
      ["episodes", "episodes"],
    ]) {
      if (path.endsWith(`/body-state/${suffix}`))
        return reply({
          user_id: owner,
          current_revision: revision,
          [field]: [],
          has_more: false,
        });
    }
    // Unrelated features remain unavailable; this test sends no requests to a live backend.
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
    await expect(panel.getByText("还没有你提出的猜测。")).toBeVisible({
      timeout: 30_000,
    });
    await panel.getByRole("button", { name: "添加我的猜测" }).click();
    await panel.getByLabel("我的完整猜测").fill("我猜与久坐有关");
    await panel.getByRole("button", { name: "保存猜测" }).click();
    await expect(panel.getByRole("button", { name: "正在提交" })).toBeVisible();
    await expect(panel.getByText("你的猜测已保存。")).toHaveCount(0);
    releaseReceipt();
    await expect(panel.getByText("你的猜测已保存。")).toBeVisible();
    await expect(
      panel.getByText("我猜与久坐有关", { exact: true }),
    ).toBeVisible();
    await panel.getByRole("button", { name: "更正猜测" }).click();
    await panel.getByLabel("我的完整猜测").fill("我猜与运动有关");
    await panel.getByRole("button", { name: "保存猜测更正" }).click();
    await expect(
      panel.getByText("我猜与运动有关", { exact: true }),
    ).toBeVisible();
    await panel.getByRole("button", { name: "撤回猜测" }).click();
    await panel.getByRole("button", { name: "确认撤回猜测" }).click();
    await expect(panel.getByText("还没有你提出的猜测。")).toBeVisible();
    expect(commands.map((command) => command.operation)).toEqual([
      "record",
      "correct",
      "withdraw",
    ]);
    expect(commands.map((command) => command.body.expected_revision)).toEqual([
      0, 1, 2,
    ]);
  } finally {
    releaseReceipt();
  }
});
