import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { ApiRequestError } from "@/lib/api-client";
import {
  NEXT_REVISION_ID,
  PLAN_ID,
  PROPOSAL_ID,
  makePlan,
  makeProposal,
  updatedContent,
} from "@/features/body-canvas/model/__fixtures__/trainingPlanFixtures";

const api = vi.hoisted(() => ({
  listPlanRecords: vi.fn(),
  listProposals: vi.fn(),
  acceptProposal: vi.fn(),
  rejectProposal: vi.fn(),
}));
vi.mock("../services/trainingService", () => ({
  trainingApi: api,
  newPlanDecisionIdentity: () => ({
    decisionId: crypto.randomUUID(),
    idempotencyKey: crypto.randomUUID(),
  }),
}));

import { projectTrainingWindow } from "@/features/body-canvas/model/trainingPlans";
import { useTrainingPlanActions, type TrainingPlanActions } from "./useTrainingPlanActions";

const view = (actions: TrainingPlanActions) => projectTrainingWindow(actions.state);

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

const receipt = {
  receiptId: "66666666-6666-4666-8666-666666666666",
  proposalId: PROPOSAL_ID,
  decisionId: "77777777-7777-4777-8777-777777777777",
  planId: PLAN_ID,
  revisionId: NEXT_REVISION_ID,
  idempotencyKey: "k",
  decidedAt: "2026-10-18T12:00:00+08:00",
  executionGate: makePlan().executionGate,
};

function setup(onPlanAccepted = vi.fn()) {
  return renderHook(
    () =>
      useTrainingPlanActions({
        enabled: true,
        selectedPlanId: null,
        selectedTemplateId: null,
        onPlanAccepted,
      }),
    { wrapper },
  );
}

describe("useTrainingPlanActions", () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true, now: new Date("2026-10-18T12:00:00+08:00") });
    Object.values(api).forEach((fn) => fn.mockReset());
    api.listPlanRecords.mockResolvedValue([makePlan()]);
    api.listProposals.mockResolvedValue([makeProposal()]);
  });

  it("shows the new revision only after the receipt and the refreshed record agree", async () => {
    const accepted = vi.fn();
    const { result } = setup(accepted);
    await waitFor(() => expect(view(result.current).scene).toBe("plan"));
    act(() => result.current.openProposal({ proposalId: PROPOSAL_ID }));
    expect(view(result.current).scene).toBe("proposal");

    api.acceptProposal.mockResolvedValue(receipt);
    api.listPlanRecords.mockResolvedValue([
      makePlan({ currentAcceptedRevisionId: NEXT_REVISION_ID, currentContent: updatedContent() }),
    ]);
    api.listProposals.mockResolvedValue([makeProposal({ state: "accepted", decisionId: receipt.decisionId })]);
    await act(async () => result.current.accept(PROPOSAL_ID));

    await waitFor(() => expect(view(result.current).scene).toBe("updated"));
    expect(accepted).toHaveBeenCalledWith(PLAN_ID);
    expect(view(result.current).selected!.revisionId).toBe(NEXT_REVISION_ID);
    const [record, identity] = api.acceptProposal.mock.calls[0]!;
    expect(record.proposalId).toBe(PROPOSAL_ID);
    expect(identity.idempotencyKey).toEqual(expect.any(String));
  });

  it("reuses one decision identity when the same decision is retried", async () => {
    const { result } = setup();
    await waitFor(() => expect(view(result.current).scene).toBe("plan"));
    api.acceptProposal.mockRejectedValueOnce(new Error("network")).mockResolvedValueOnce(receipt);
    await act(async () => result.current.accept(PROPOSAL_ID));
    expect(result.current.error).toBe("network");
    await act(async () => result.current.accept(PROPOSAL_ID));
    const [, first] = api.acceptProposal.mock.calls[0]!;
    const [, second] = api.acceptProposal.mock.calls[1]!;
    expect(second).toEqual(first);
  });

  it("ignores a second click while the decision is in flight", async () => {
    const { result } = setup();
    await waitFor(() => expect(view(result.current).scene).toBe("plan"));
    let resolve!: (value: typeof receipt) => void;
    api.acceptProposal.mockReturnValue(new Promise((done) => (resolve = done)));
    act(() => {
      result.current.accept(PROPOSAL_ID);
      result.current.accept(PROPOSAL_ID);
    });
    await waitFor(() => expect(result.current.busy).toBe(true));
    await act(async () => resolve(receipt));
    expect(api.acceptProposal).toHaveBeenCalledTimes(1);
  });

  it("turns a stale 409 into the Expired board and keeps the plan unchanged", async () => {
    const { result } = setup();
    await waitFor(() => expect(view(result.current).scene).toBe("plan"));
    api.acceptProposal.mockRejectedValue(new ApiRequestError("stale", 409, "REVISION_CONFLICT"));
    await act(async () => result.current.accept(PROPOSAL_ID));
    await waitFor(() => expect(view(result.current).scene).toBe("expired"));
    expect(view(result.current).proposal!.expiredReason).toBe("base_revision_changed");
    expect(view(result.current).selected!.revisionId).toBe(makePlan().currentAcceptedRevisionId);
    expect(api.listPlanRecords.mock.calls.length).toBeGreaterThan(1);
  });

  it("records a rejection only from its receipt", async () => {
    const { result } = setup();
    await waitFor(() => expect(view(result.current).scene).toBe("plan"));
    api.rejectProposal.mockResolvedValue({ ...receipt, proposalId: "other" });
    await act(async () => result.current.reject(PROPOSAL_ID));
    expect(result.current.notice).toBeNull();
    expect(result.current.error).toMatch(/回执与本次操作不一致/);
    api.rejectProposal.mockResolvedValue({ ...receipt });
    await act(async () => result.current.reject(PROPOSAL_ID));
    expect(result.current.notice).toBe("rejected");
  });
});
