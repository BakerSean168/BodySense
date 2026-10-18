import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiRequestError } from "@/lib/api-client";
import {
  PROPOSAL_A,
  PROPOSAL_B,
  RUN_ID,
  makeDecisionReceipt,
  makeDiagnosis,
  makeRun,
  makeSettlementReceipt,
} from "@/features/body-canvas/model/__fixtures__/nativeDiagnosisFixtures";
import { nativeDiagnosisService } from "../services/nativeDiagnosisService";
import { useDiagnosisActions } from "./useDiagnosisActions";

vi.mock("../services/nativeDiagnosisService", () => ({
  nativeDiagnosisService: {
    current: vi.fn(),
    latestRun: vi.fn(),
    run: vi.fn(),
    decide: vi.fn(),
    settle: vi.fn(),
  },
}));

const api = vi.mocked(nativeDiagnosisService);

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

async function ready() {
  const hook = renderHook(() => useDiagnosisActions(), { wrapper });
  await waitFor(() =>
    expect(hook.result.current.view.run?.canDecide).toBe(true),
  );
  return hook;
}

function row(hook: Awaited<ReturnType<typeof ready>>, proposalId: string) {
  return hook.result.current.view.run?.proposals.find(
    (item) => item.proposalId === proposalId,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  api.current.mockResolvedValue(makeDiagnosis());
  api.latestRun.mockResolvedValue({
    runId: RUN_ID,
    trigger: "new_analysis",
    basedOnBodyStateRevision: 5,
    basedOnDiagnosisRevision: 2,
    startedAt: "2026-10-18T03:00:00Z",
    state: "open",
  });
  api.run.mockResolvedValue(makeRun());
});

describe("useDiagnosisActions on the V2 decision API", () => {
  it("records only the selected proposal with idempotency key and expected revision", async () => {
    api.decide.mockImplementation(async (_runId, command) =>
      makeDecisionReceipt({
        proposalId: command.proposalId,
        contentFingerprint: command.presentedFingerprint,
        decision: command.decision,
        expectedRevision: command.expectedRevision,
        idempotencyKey: command.idempotencyKey,
        toolCallId: command.toolCallId,
      }),
    );
    const hook = await ready();
    act(() => hook.result.current.select(PROPOSAL_A, "accept"));
    await act(() => hook.result.current.recordSelection());

    expect(api.decide).toHaveBeenCalledTimes(1);
    expect(api.decide).toHaveBeenCalledWith(
      RUN_ID,
      expect.objectContaining({
        proposalId: PROPOSAL_A,
        decision: "accept",
        expectedRevision: 2,
        idempotencyKey: expect.any(String),
        toolCallId: expect.stringMatching(/^web:diagnosis-window:/),
      }),
    );
    await waitFor(() => expect(row(hook, PROPOSAL_A)?.state).toBe("pending"));
    expect(row(hook, PROPOSAL_B)?.state).toBe("undecided");
    expect(hook.result.current.view.run?.closesRemaining).toBe(true);
  });

  it("marks the proposal changed on a stale 409 and records nothing", async () => {
    api.decide.mockRejectedValue(
      new ApiRequestError("changed", 409, "REVISION_CONFLICT"),
    );
    const hook = await ready();
    act(() => hook.result.current.select(PROPOSAL_A, "accept"));
    await act(() => hook.result.current.recordSelection());

    await waitFor(() => expect(row(hook, PROPOSAL_A)?.state).toBe("changed"));
    expect(row(hook, PROPOSAL_A)?.decision).toBeNull();
    expect(hook.result.current.selection).toEqual({});
    expect(hook.result.current.error).toMatch(/已经变化/);
    expect(api.run.mock.calls.length).toBeGreaterThan(1);
  });

  it("reuses the idempotency key when retrying the same choice", async () => {
    api.decide.mockRejectedValueOnce(new ApiRequestError("down", 503));
    const hook = await ready();
    act(() => hook.result.current.select(PROPOSAL_A, "reject"));
    await act(() => hook.result.current.recordSelection());
    await waitFor(() =>
      expect(hook.result.current.view.run?.canDecide).toBe(true),
    );
    api.decide.mockImplementation(async (_runId, command) =>
      makeDecisionReceipt({
        decision: command.decision,
        idempotencyKey: command.idempotencyKey,
      }),
    );
    await act(() => hook.result.current.recordSelection());

    const [first, second] = api.decide.mock.calls.map((call) => call[1]);
    expect(second?.idempotencyKey).toBe(first?.idempotencyKey);
    await waitFor(() => expect(row(hook, PROPOSAL_A)?.state).toBe("rejected"));
  });

  it("does not show committed without a matching settlement receipt", async () => {
    let decisionId = "";
    api.decide.mockImplementation(async (_runId, command) => {
      const receipt = makeDecisionReceipt({
        idempotencyKey: command.idempotencyKey,
      });
      decisionId = receipt.decisionId;
      return receipt;
    });
    const hook = await ready();
    act(() => hook.result.current.select(PROPOSAL_A, "accept"));
    await act(() => hook.result.current.recordSelection());
    await waitFor(() =>
      expect(hook.result.current.view.run?.canSettle).toBe(true),
    );

    // Revision does not advance: an inconsistent "committed" receipt is refused.
    api.settle.mockResolvedValueOnce(
      makeSettlementReceipt({ revision: 2, decisionIds: [decisionId] }),
    );
    await act(() => hook.result.current.settle());
    await waitFor(() => expect(hook.result.current.busy).toBe(false));
    expect(row(hook, PROPOSAL_A)?.state).toBe("pending");
    expect(hook.result.current.view.run?.committedReceiptId).toBeNull();
    expect(hook.result.current.error).toMatch(/回执/);

    api.settle.mockImplementation(async (_runId, command) =>
      makeSettlementReceipt({
        decisionIds: [decisionId],
        idempotencyKey: command.idempotencyKey,
      }),
    );
    await waitFor(() =>
      expect(hook.result.current.view.run?.canSettle).toBe(true),
    );
    await act(() => hook.result.current.settle());
    expect(api.settle).toHaveBeenLastCalledWith(
      RUN_ID,
      expect.objectContaining({ expectedRevision: 2, closeRemaining: true }),
    );
    await waitFor(() => expect(row(hook, PROPOSAL_A)?.state).toBe("committed"));
  });

  it("keeps decisions read-only while Safety review is required", async () => {
    const hook = renderHook(() => useDiagnosisActions({ safetyReview: true }), {
      wrapper,
    });
    await waitFor(() => expect(hook.result.current.view.run).not.toBeNull());
    expect(hook.result.current.view.run?.canDecide).toBe(false);
    act(() => hook.result.current.select(PROPOSAL_A, "accept"));
    await act(() => hook.result.current.recordSelection());
    expect(api.decide).not.toHaveBeenCalled();
  });

  it("submit records the stepper choices then settles the whole run", async () => {
    const decisionIds: string[] = [];
    api.decide.mockImplementation(async (_runId, command) => {
      const receipt = makeDecisionReceipt({
        proposalId: command.proposalId,
        contentFingerprint: command.presentedFingerprint,
        decision: command.decision,
        idempotencyKey: command.idempotencyKey,
      });
      decisionIds.push(receipt.decisionId);
      return receipt;
    });
    api.settle.mockImplementation(async (_runId, command) =>
      makeSettlementReceipt({
        decisionIds,
        idempotencyKey: command.idempotencyKey,
      }),
    );
    const hook = await ready();
    act(() => hook.result.current.select(PROPOSAL_A, "accept"));
    await act(() =>
      hook.result.current.submit({ proposalId: PROPOSAL_B, choice: "reject" }),
    );

    expect(api.decide).toHaveBeenCalledTimes(2);
    expect(api.decide.mock.calls.map((call) => call[1].decision)).toEqual([
      "accept",
      "reject",
    ]);
    expect(api.settle).toHaveBeenCalledWith(
      RUN_ID,
      expect.objectContaining({ expectedRevision: 2, closeRemaining: false }),
    );
    await waitFor(() => expect(row(hook, PROPOSAL_A)?.state).toBe("committed"));
    expect(row(hook, PROPOSAL_B)?.state).toBe("rejected");
    expect(hook.result.current.view.run?.phase).toBe("committed");

    act(() => hook.result.current.dismissOutcome());
    expect(hook.result.current.selection).toEqual({});
  });

  it("submit with undecided proposals closes them and stops on a failed decision", async () => {
    api.decide.mockRejectedValueOnce(new ApiRequestError("down", 503));
    const hook = await ready();
    act(() => hook.result.current.select(PROPOSAL_A, "accept"));
    await act(() => hook.result.current.submit());
    expect(api.settle).not.toHaveBeenCalled();
    expect(hook.result.current.error).toBeTruthy();

    api.decide.mockImplementation(async (_runId, command) =>
      makeDecisionReceipt({ idempotencyKey: command.idempotencyKey }),
    );
    api.settle.mockImplementation(async (_runId, command) =>
      makeSettlementReceipt({ idempotencyKey: command.idempotencyKey }),
    );
    await waitFor(() =>
      expect(hook.result.current.view.run?.canDecide).toBe(true),
    );
    await act(() => hook.result.current.submit());
    expect(api.settle).toHaveBeenCalledWith(
      RUN_ID,
      expect.objectContaining({ closeRemaining: true }),
    );
  });
});
