import { expect, type APIRequestContext } from "@playwright/test";
import { settleConversation, streamEvents } from "./consultation";

interface RuntimeEvent {
  type: string;
  payload: {
    interaction_id?: string;
    question?: { fields?: Array<{ key?: string }> };
  };
}

// A real model turn can take over a minute; Go ends it at its own 120 s AI read
// deadline, so wait just past that to see the Run outcome rather than abort it.
const TURN_TIMEOUT = 125_000;

const isSafetyChecklist = (event: RuntimeEvent) =>
  event.type === "state.interaction.required" &&
  (event.payload.question?.fields ?? []).some(
    (field) => field.key === "safety_signals",
  );

/**
 * Structured safety coverage is recorded only from the Go-bound symptom
 * intake. Report a present symptom in the given conversation (the v3 runtime
 * commits it and asks the safety screen for it) and answer the checklist with
 * the specified signals (default: "none of the above").
 *
 * Works with the deterministic stub and with a real model: the conversation is
 * settled first, a turn that does not reach the checklist (the model asked
 * something else first) is cancelled and the statement is sent again, and any
 * follow-up question after the answer is cancelled so the next message can
 * start a Run. Only the user's checklist answer records the screen.
 */
export async function completeStructuredSafetyIntake(
  request: APIRequestContext,
  apiBase: string,
  headers: Record<string, string>,
  conversationId: string,
  signals: string[] = ["以上均无"],
  statement = "我昨天肩膀不舒服。",
) {
  let interactionId: string | undefined;
  let lastRunBody = "";
  for (let attempt = 0; attempt < 3 && !interactionId; attempt += 1) {
    await settleConversation(request, apiBase, headers, conversationId);
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
      timeout: TURN_TIMEOUT,
    });
    lastRunBody = await run.text();
    expect(run.ok(), lastRunBody).toBeTruthy();
    interactionId =
      streamEvents<RuntimeEvent>(lastRunBody).find(isSafetyChecklist)?.payload
        .interaction_id;
  }
  expect(interactionId, lastRunBody).toBeTruthy();
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
      timeout: TURN_TIMEOUT,
    },
  );
  const answerBody = await answer.text();
  expect(answer.ok(), answerBody).toBeTruthy();
  expect(answerBody).toContain("event: stream.done");
  await settleConversation(request, apiBase, headers, conversationId);

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
