import { expect, type APIRequestContext } from "@playwright/test";

/**
 * Structured safety coverage is recorded only from the Go-bound symptom
 * intake. Report a present symptom in the given conversation (the v3 runtime
 * commits it and asks the safety screen for it) and answer the checklist with
 * the specified signals (default: "none of the above").
 */
export async function completeStructuredSafetyIntake(
  request: APIRequestContext,
  apiBase: string,
  headers: Record<string, string>,
  conversationId: string,
  signals: string[] = ["以上均无"],
  statement = "我昨天肩膀不舒服。",
) {
  const run = await request.post(`${apiBase}/api/v1/consultation-runs`, {
    headers,
    data: {
      conversationId,
      clientMessageId: `e2e-intake-${crypto.randomUUID()}`,
      requestId: crypto.randomUUID(),
      message: {
        role: "user",
        parts: [{ type: "text", text: statement }],
      },
    },
    timeout: 60_000,
  });
  const runBody = await run.text();
  expect(run.ok(), runBody).toBeTruthy();
  const pending = runBody
    .split(/\r?\n/)
    .filter((line) => line.startsWith("data:"))
    .map(
      (line) =>
        JSON.parse(line.slice(5).trim()) as {
          type: string;
          payload: { interaction_id?: string };
        },
    )
    .find((event) => event.type === "state.interaction.required");
  const interactionId = pending?.payload.interaction_id;
  expect(interactionId, runBody).toBeTruthy();
  const answer = await request.post(
    `${apiBase}/api/v1/consultations/${conversationId}/interrupts/${interactionId}/answers`,
    {
      headers,
      data: {
        requestId: crypto.randomUUID(),
        answer: {
          fields: { safety_signals: signals },
        },
      },
      timeout: 60_000,
    },
  );
  const answerBody = await answer.text();
  expect(answer.ok(), answerBody).toBeTruthy();
  expect(answerBody).toContain("event: stream.done");

  const records = await request.get(`${apiBase}/api/v1/body-state/records`, {
    headers,
  });
  expect(records.ok(), await records.text()).toBeTruthy();
  const body = (await records.json()) as {
    facts: Array<{ id: string; statement: string; created_revision: number }>;
  };
  const symptoms = body.facts.filter((fact) => fact.statement === statement);
  // The accepted checklist must screen the committed fact without duplicating it.
  expect(symptoms).toHaveLength(1);
  return symptoms[0];
}
