import { expect, type APIRequestContext } from "@playwright/test";

/**
 * Structured safety coverage is recorded only from the runtime-bound symptom
 * intake. Run the deterministic E2E intake in the given conversation and
 * answer its checklist with "none of the above".
 */
export async function completeStructuredSafetyIntake(
  request: APIRequestContext,
  apiBase: string,
  headers: Record<string, string>,
  conversationId: string,
) {
  const run = await request.post(`${apiBase}/api/v1/consultation-runs`, {
    headers,
    data: {
      conversationId,
      clientMessageId: `e2e-intake-${crypto.randomUUID()}`,
      requestId: crypto.randomUUID(),
      message: {
        role: "user",
        parts: [{ type: "text", text: "E2E_SYMPTOM_INTAKE" }],
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
          fields: { safety_signals: ["以上均无"], duration: "2–7天" },
        },
      },
      timeout: 60_000,
    },
  );
  const answerBody = await answer.text();
  expect(answer.ok(), answerBody).toBeTruthy();
  expect(answerBody).toContain("event: stream.done");
}
