import { expect, test } from "@playwright/test";
import { refreshBrowserAccessToken } from "./support/auth";

// Genuine browser cookie/JWT + Go validation + PostgreSQL 18 source/receipt.
// The final observation editor is Track B's Penpot scope, so this tests only
// the honest v11 public backend behavior without fabricating a UI screen.
test.describe.configure({ timeout: 95_000 });

const api = process.env.E2E_API_BASE_URL || "http://127.0.0.1:8080";
const route = `${api}/api/v1/body-state/observation-records`;

test("direct-user BodyObservation keeps raw source, exact replay and isolation across browser reload", async ({
  page,
  request,
}) => {
  const email = `v11-observation-${crypto.randomUUID()}@example.test`;
  await page.goto("/register");
  await page.getByLabel("邮箱地址").fill(email);
  await page.getByLabel("密码", { exact: true }).fill("BodySenseE2E!123");
  await page.getByLabel("确认密码").fill("BodySenseE2E!123");
  await page.getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL(/\/(dashboard|onboarding)/);

  const accessToken = await refreshBrowserAccessToken(page, api);
  const headers = { Authorization: `Bearer ${accessToken}` };
  const key = crypto.randomUUID();
  const observation = {
    kind: "body_weight",
    statement: "My home scale displayed 67.5 kilograms",
    raw_value: "67.5",
    raw_unit: "kg",
    method: "home scale",
    observed_at: { precision: "day", start: "2026-10-13" },
  };
  const original = {
    idempotency_key: key,
    expected_revision: 0,
    observation,
  };

  const saved = await request.post(route, { headers, data: original });
  expect(saved.status(), await saved.text()).toBe(200);
  const receipt = await saved.json();
  expect(receipt).toMatchObject({ revision: 1 });
  expect(receipt.id).toMatch(/^[0-9a-f-]{36}$/);
  expect(receipt.observation_id).toMatch(/^[0-9a-f-]{36}$/);
  expect(receipt.source_evidence_id).toMatch(/^[0-9a-f-]{36}$/);
  expect(receipt).not.toHaveProperty("raw_value");
  expect(receipt).not.toHaveProperty("statement");
  expect(receipt).not.toHaveProperty("clinical_confidence");
  expect(JSON.stringify(receipt)).not.toContain("67.5");

  const replay = await request.post(route, { headers, data: original });
  expect(replay.status()).toBe(200);
  expect(await replay.json()).toEqual(receipt);
  const changed = await request.post(route, {
    headers,
    data: { ...original, observation: { ...observation, raw_value: "68" } },
  });
  expect(changed.status()).toBe(409);
  const stale = await request.post(route, {
    headers,
    data: { ...original, idempotency_key: crypto.randomUUID() },
  });
  expect(stale.status()).toBe(409);

  const forged = await request.post(route, {
    headers,
    data: {
      ...original,
      idempotency_key: crypto.randomUUID(),
      observation: {
        ...observation,
        source_kind: "device",
        normalized_value: 67.5,
      },
    },
  });
  expect(forged.status()).toBe(400);

  await page.reload();
  const renewedToken = await refreshBrowserAccessToken(page, api);
  const list = await request.get(route, {
    headers: { Authorization: `Bearer ${renewedToken}` },
  });
  expect(list.status()).toBe(200);
  const pageData = await list.json();
  expect(pageData).toMatchObject({
    current_revision: 1,
    has_more: false,
    observations: [
      {
        id: receipt.observation_id,
        kind: "body_weight",
        raw_value: "67.5",
        raw_unit: "kg",
        statement: observation.statement,
        assertion_status: "asserted",
        observed_at: observation.observed_at,
        source: {
          kind: "user_report",
          id: receipt.source_evidence_id,
          version: 1,
        },
      },
    ],
  });
  expect(pageData.observations[0]).not.toHaveProperty("normalized_value");
  expect(pageData.observations[0]).not.toHaveProperty("diagnosis");

  const wrongOwner = await request.get(route);
  expect(wrongOwner.status()).toBe(401);

  const legacyFacts = await request.get(`${api}/api/v1/body-state/records`, {
    headers: { Authorization: `Bearer ${renewedToken}` },
  });
  expect(legacyFacts.status()).toBe(200);
  expect((await legacyFacts.json()).facts).toEqual([]);
});
