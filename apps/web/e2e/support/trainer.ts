import { expect, type APIRequestContext } from "@playwright/test";

type Headers = Record<string, string>;

export interface TrainerPlanRecord {
  planId: string;
  status: string;
  currentAcceptedRevisionId: string;
  currentContent: { sessions: Array<{ templateId: string }> };
  review: {
    recommended: boolean;
    reason?: string;
    sessionFeedbackCount?: number;
  };
  executionGate: { permitsExecution: boolean; safetyRevision: number };
}

export interface TrainerSessionRecord {
  sessionId: string;
  state: string;
  revision: number;
  feedback?: { text: string };
}

async function json<T>(
  response: Awaited<ReturnType<APIRequestContext["get"]>>,
  label: string,
): Promise<T> {
  const body = await response.text();
  expect(
    response.ok(),
    `${label} failed with ${response.status()}: ${body}`,
  ).toBeTruthy();
  return JSON.parse(body) as T;
}

export async function getTrainerPlan(
  request: APIRequestContext,
  apiBase: string,
  headers: Headers,
  planId: string,
): Promise<TrainerPlanRecord> {
  return json<TrainerPlanRecord>(
    await request.get(`${apiBase}/api/v1/training-plans/${planId}`, {
      headers,
    }),
    "read training plan record",
  );
}

/**
 * Ends the plan's Trainer session through the Go runtime by explicit
 * commands, then records the user's verbatim feedback. The session the
 * browser started from the TrainingPlan window is reused; otherwise one is
 * started at the revision and Safety revision the user saw. Nothing is
 * inferred and no Outcome is written.
 */
export async function finishTrainerSessionWithFeedback(
  request: APIRequestContext,
  apiBase: string,
  headers: Headers,
  planId: string,
  text: string,
): Promise<TrainerSessionRecord> {
  const plan = await getTrainerPlan(request, apiBase, headers, planId);
  expect(plan.executionGate.permitsExecution).toBe(true);
  const { sessions } = await json<{
    sessions: Array<TrainerSessionRecord & { planId: string }>;
  }>(
    await request.get(`${apiBase}/api/v1/training-sessions`, { headers }),
    "list training sessions",
  );
  const live = sessions.find(
    (session) => session.planId === planId && session.state !== "ended",
  );
  const started =
    live ??
    (await json<TrainerSessionRecord>(
      await request.post(`${apiBase}/api/v1/training-sessions`, {
        headers,
        data: {
          planId,
          expectedPlanRevisionId: plan.currentAcceptedRevisionId,
          templateId: plan.currentContent.sessions[0]!.templateId,
          timeZone: "Asia/Shanghai",
          expectedSafetyRevision: plan.executionGate.safetyRevision,
          idempotencyKey: crypto.randomUUID(),
        },
      }),
      "start training session",
    ));
  const ended = await json<TrainerSessionRecord>(
    await request.post(
      `${apiBase}/api/v1/training-sessions/${started.sessionId}/finish`,
      {
        headers,
        data: {
          endedReason: "stopped_early",
          expectedSessionRevision: started.revision,
          idempotencyKey: crypto.randomUUID(),
        },
      },
    ),
    "finish training session",
  );
  expect(ended.state).toBe("ended");
  const withFeedback = await json<TrainerSessionRecord>(
    await request.post(
      `${apiBase}/api/v1/training-sessions/${started.sessionId}/feedback`,
      { headers, data: { text, idempotencyKey: crypto.randomUUID() } },
    ),
    "record training session feedback",
  );
  expect(withFeedback.feedback?.text).toBe(text);
  return withFeedback;
}

/** The user's own words about a body change, recorded through BodyState. */
export async function recordUserSymptom(
  request: APIRequestContext,
  apiBase: string,
  headers: Headers,
  statement: string,
): Promise<{ revision: number; fact_ids: string[] }> {
  const records = await json<{ current_revision: number }>(
    await request.get(`${apiBase}/api/v1/body-state/records`, { headers }),
    "read BodyState revision",
  );
  return json<{ revision: number; fact_ids: string[] }>(
    await request.post(`${apiBase}/api/v1/body-state/records`, {
      headers,
      data: {
        idempotency_key: crypto.randomUUID(),
        expected_revision: records.current_revision,
        facts: [
          {
            kind: "symptom",
            statement,
            body_region_ids: [],
            temporal: { precision: "unknown" },
            details: { presence: "present" },
          },
        ],
      },
    }),
    "record the user's symptom",
  );
}
