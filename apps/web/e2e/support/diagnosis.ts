import { expect, type APIRequestContext } from "@playwright/test";

interface NativeRunProposal {
  proposalId: string;
  contentFingerprint: string;
  state: string;
}

interface NativeRun {
  runId: string;
  state: string;
  proposals: NativeRunProposal[];
}

export interface AdoptedDiagnosis {
  runId: string;
  revision: number;
  conditionIds: string[];
}

async function currentDiagnosisRevision(
  request: APIRequestContext,
  apiBase: string,
  headers: Record<string, string>,
): Promise<number> {
  const response = await request.get(`${apiBase}/api/v1/diagnosis`, {
    headers,
  });
  expect(response.ok(), await response.text()).toBeTruthy();
  return ((await response.json()) as { currentRevision: number })
    .currentRevision;
}

/**
 * Seeds an adopted native Diagnosis exactly as the Diagnosis window does:
 * start one formal analysis run from the user's own statement, accept every
 * pending proposal as the user's explicit choice, then settle the whole run.
 * Only the committed settlement adopts Conditions.
 */
export async function adoptNativeDiagnosis(
  request: APIRequestContext,
  apiBase: string,
  headers: Record<string, string>,
  requestEvidenceId: string,
): Promise<AdoptedDiagnosis> {
  const expectedRevision = await currentDiagnosisRevision(
    request,
    apiBase,
    headers,
  );
  const start = await request.post(
    `${apiBase}/api/v1/diagnosis/analysis-runs`,
    {
      headers,
      data: {
        requestEvidenceId,
        trigger: "new_analysis",
        expectedRevision,
        idempotencyKey: `e2e-dx-start-${crypto.randomUUID()}`,
      },
      timeout: 60_000,
    },
  );
  const startBody = await start.text();
  expect(start.status(), startBody).toBe(201);
  const run = JSON.parse(startBody) as NativeRun;
  expect(run.state).toBe("open");
  const pending = run.proposals.filter(
    (proposal) => proposal.state === "pending",
  );
  expect(pending.length).toBeGreaterThan(0);

  for (const proposal of pending) {
    const decision = await request.post(
      `${apiBase}/api/v1/diagnosis/analysis-runs/${run.runId}/decisions`,
      {
        headers,
        data: {
          proposalId: proposal.proposalId,
          presentedFingerprint: proposal.contentFingerprint,
          expectedRevision,
          decision: "accept",
          toolCallId: `e2e:diagnosis:${crypto.randomUUID()}`,
          idempotencyKey: `e2e-dx-decide-${crypto.randomUUID()}`,
        },
      },
    );
    expect(decision.status(), await decision.text()).toBe(200);
  }

  const settlement = await request.post(
    `${apiBase}/api/v1/diagnosis/analysis-runs/${run.runId}/settlement`,
    {
      headers,
      data: {
        expectedRevision,
        toolCallId: `e2e:diagnosis:${crypto.randomUUID()}`,
        idempotencyKey: `e2e-dx-settle-${crypto.randomUUID()}`,
        closeRemaining: false,
      },
    },
  );
  const settlementBody = await settlement.text();
  expect(settlement.status(), settlementBody).toBe(200);
  const receipt = JSON.parse(settlementBody) as {
    outcome: string;
    revision: number;
    conditionIds: string[];
  };
  expect(receipt.outcome, settlementBody).toBe("committed");
  expect(receipt.conditionIds.length).toBeGreaterThan(0);
  return {
    runId: run.runId,
    revision: receipt.revision,
    conditionIds: receipt.conditionIds,
  };
}

/** The user's own statement evidence for a BodyFact (the run's request source). */
export async function factStatementEvidenceId(
  request: APIRequestContext,
  apiBase: string,
  headers: Record<string, string>,
  factId: string,
): Promise<string> {
  const records = await request.get(`${apiBase}/api/v1/body-state/records`, {
    headers,
  });
  expect(records.ok()).toBeTruthy();
  const fact = (
    (await records.json()) as {
      facts: Array<{ id: string; source_evidence_id?: string }>;
    }
  ).facts.find((row) => row.id === factId);
  expect(fact?.source_evidence_id).toBeTruthy();
  return fact!.source_evidence_id!;
}
