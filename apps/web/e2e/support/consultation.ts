import { expect, test, type APIRequestContext } from "@playwright/test";

type Headers = Record<string, string>;

/**
 * The user's single conversation. GET /api/v1/conversation returns it even
 * before the first message, so specs that only need a conversation id never
 * send a chat message (whose reply the real model decides).
 */
export async function conversationIdFor(
  request: APIRequestContext,
  apiBase: string,
  headers: Headers,
): Promise<string> {
  const response = await request.get(`${apiBase}/api/v1/conversation`, {
    headers,
  });
  const body = await response.text();
  expect(response.ok(), body).toBeTruthy();
  const id = (JSON.parse(body) as { conversation: { id: string } }).conversation
    .id;
  expect(id).toBeTruthy();
  return id;
}

/**
 * Leaves no Run open in the conversation. A Run that ended its stream waiting
 * on a question (a real model may ask a follow-up at any turn) still owns the
 * conversation, so the next message would get RUN_IN_PROGRESS. Seeding steps
 * do not test that question: they cancel its Run explicitly. Everything the
 * Run already committed (facts, safety screens) stays committed. Returns how
 * many Runs were cancelled (0 with the deterministic stub, which asks nothing
 * unscripted).
 */
export async function settleConversation(
  request: APIRequestContext,
  apiBase: string,
  headers: Headers,
  conversationId: string,
): Promise<number> {
  const response = await request.get(
    `${apiBase}/api/v1/consultations/${conversationId}`,
    { headers },
  );
  // A conversation that never started a Run has no consultation session yet.
  if (response.status() === 404) return 0;
  const body = await response.text();
  expect(response.ok(), body).toBeTruthy();
  const session = JSON.parse(body) as {
    pending_interactions: Array<{ run_id: string; status: string }>;
  };
  const runIds = new Set(
    session.pending_interactions
      .filter((interaction) => interaction.status === "pending")
      .map((interaction) => interaction.run_id),
  );
  for (const runId of runIds) {
    const cancel = await request.post(
      `${apiBase}/api/v1/consultation-runs/${runId}/cancel`,
      { headers, data: {} },
    );
    expect(cancel.ok(), await cancel.text()).toBeTruthy();
  }
  return runIds.size;
}

/** Events of a consultation SSE response body, in order. */
export function streamEvents<T = Record<string, unknown>>(body: string) {
  return body
    .split(/\r?\n/)
    .filter((line) => line.startsWith("data:"))
    .map((line) => JSON.parse(line.slice(5).trim()) as T);
}

/**
 * Specs that only the deterministic stub model can drive: their subject is a
 * reply it is scripted to emit for a marker message (an Ask User question, a
 * BodyState suggestion), or they need turns a real model cannot finish in
 * time. These cannot be seeded through the API. Set E2E_REAL_MODEL=1 when
 * running the suite against a deployment with a real model (staging) to skip
 * them explicitly, with the reason in the report.
 */
export function requiresDeterministicModel(
  reason = "asserts a scripted reply of the deterministic E2E model",
) {
  test.skip(process.env.E2E_REAL_MODEL === "1", reason);
}
