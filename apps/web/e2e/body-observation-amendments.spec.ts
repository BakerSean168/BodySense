import { expect, test } from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";

// Direct owner + PostgreSQL + generated Go routes. This is deliberately not
// a Penpot editor test: the final Observation UI belongs to Track B.
test.describe.configure({ timeout: 95_000 });
const api = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";
const observations = `${api}/api/v1/body-state/observation-records`;

test("v12 actual user corrects original observation then withdraws exact successor, no medical invention", async ({
  page,
  request,
}) => {
  await page.goto("/register");
  await page
    .getByLabel("邮箱地址")
    .fill(`observation-amend-${crypto.randomUUID()}@example.test`);
  await page.getByLabel("密码", { exact: true }).fill("BodySenseE2E!123");
  await page.getByLabel("确认密码").fill("BodySenseE2E!123");
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding)/);
  const token = await refreshBrowserAccessToken(page, api);
  const headers = { Authorization: `Bearer ${token}` };

  const report = {
    kind: "body_weight",
    statement: "I read a synthetic weight of 67.5 kg",
    raw_value: "67.5",
    raw_unit: "kg",
    observed_at: { precision: "day", start: "2026-10-13" },
  };
  const first = await request.post(observations, {
    headers,
    data: {
      idempotency_key: crypto.randomUUID(),
      expected_revision: 0,
      observation: report,
    },
  });
  expect(first.status(), await first.text()).toBe(200);
  const original = await first.json();

  const correction = {
    idempotency_key: crypto.randomUUID(),
    expected_revision: 1,
    replacement: {
      ...report,
      statement: "I misread my synthetic weight, it was 65.7 kg",
      raw_value: "65.7",
    },
  };
  const route = `${observations}/${original.observation_id}/correct`;
  const changed = await request.post(route, { headers, data: correction });
  expect(changed.status(), await changed.text()).toBe(200);
  const receipt = await changed.json();
  expect(receipt.revision).toBe(2);
  expect(receipt.observation_id).not.toBe(original.observation_id);
  expect(receipt.source_evidence_id).not.toBe(original.source_evidence_id);
  expect(receipt).not.toHaveProperty("raw_value");

  const replay = await request.post(route, { headers, data: correction });
  expect(replay.status()).toBe(200);
  expect(await replay.json()).toEqual(receipt);
  const spoofed = await request.post(route, {
    headers,
    data: {
      ...correction,
      idempotency_key: crypto.randomUUID(),
      replacement: {
        ...correction.replacement,
        source_kind: "device",
        normalized_value: 65.7,
      },
    },
  });
  expect(spoofed.status()).toBe(400);

  const current = await request.get(observations, { headers });
  expect(current.status()).toBe(200);
  const snapshot = await current.json();
  expect(snapshot.current_revision).toBe(2);
  expect(snapshot.observations).toHaveLength(1);
  expect(snapshot.observations[0]).toMatchObject({
    id: receipt.observation_id,
    raw_value: "65.7",
    source: { kind: "user_report", id: receipt.source_evidence_id },
  });
  expect(JSON.stringify(snapshot.observations)).not.toContain("67.5");

  const withdrawRoute = `${observations}/${receipt.observation_id}/withdraw`;
  const withdrawal = {
    idempotency_key: crypto.randomUUID(),
    expected_revision: 2,
  };
  const withdrawn = await request.post(withdrawRoute, {
    headers,
    data: withdrawal,
  });
  expect(withdrawn.status(), await withdrawn.text()).toBe(200);
  const removed = await withdrawn.json();
  expect(removed).toMatchObject({
    observation_id: receipt.observation_id,
    revision: 3,
  });
  const repeated = await request.post(withdrawRoute, {
    headers,
    data: withdrawal,
  });
  expect(repeated.status()).toBe(200);
  expect(await repeated.json()).toEqual(removed);

  await page.reload();
  // Reusing the still-valid owner JWT after browser reload prevents a second
  // refresh-cookie rotation racing the browser's auth bootstrap.
  const hidden = await request.get(observations, { headers });
  expect(hidden.status()).toBe(200);
  expect(await hidden.json()).toMatchObject({
    current_revision: 3,
    observations: [],
  });

  const noResolutionFact = await request.get(
    `${api}/api/v1/body-state/records`,
    { headers },
  );
  expect(noResolutionFact.status()).toBe(200);
  expect((await noResolutionFact.json()).facts).toEqual([]);
  const unauthorized = await request.post(withdrawRoute, { data: withdrawal });
  expect(unauthorized.status()).toBe(401);
});
