import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { TrainingSessionRecord } from "../services/trainingService";

const api = vi.hoisted(() => ({ listSessions: vi.fn() }));
vi.mock("../services/trainingService", () => ({ trainingApi: api }));

import { pickLiveSession, TrainerStatusChip } from "./TrainerStatusChip";

function session(id: string, state: TrainingSessionRecord["state"], startedAt: string): TrainingSessionRecord {
  return {
    sessionId: id,
    planId: "plan-1",
    planRevisionId: "rev-1",
    templateId: "a",
    timeZone: "Asia/Shanghai",
    trainingDate: "2026-10-19",
    state,
    revision: 1,
    startedAt,
    startGate: { result: "allow", safetyRevision: 1, policyRevision: "p1" },
    actions: [],
    transitions: [],
  };
}

const t = (minute: number) => new Date(Date.UTC(2026, 9, 19, 1, minute)).toISOString();

function renderChip(onOpen = vi.fn()) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <TrainerStatusChip onOpen={onOpen} />
    </QueryClientProvider>,
  );
  return onOpen;
}

describe("Home training chip (Penpot Home / trainer-active · trainer-paused)", () => {
  it("points at the in-progress session first, else the newest paused one, never an ended one", () => {
    const ended = session("e", "ended", t(30));
    const oldPaused = session("p1", "paused", t(1));
    const newPaused = session("p2", "paused", t(5));
    const live = session("l", "in_progress", t(2));
    expect(pickLiveSession([ended, oldPaused, live, newPaused])?.sessionId).toBe("l");
    expect(pickLiveSession([ended, oldPaused, newPaused])?.sessionId).toBe("p2");
    expect(pickLiveSession([ended])).toBeNull();
    expect(pickLiveSession(undefined)).toBeNull();
  });

  it("shows the paused state and reopens that session on click", async () => {
    const paused = session("p2", "paused", t(5));
    api.listSessions.mockResolvedValue([paused]);
    const onOpen = renderChip();
    const chip = await screen.findByRole("button", { name: "回到训练 · 训练已暂停" });
    expect(chip).toHaveAttribute("data-state", "paused");
    await userEvent.click(chip);
    expect(onOpen).toHaveBeenCalledWith(paused);
  });

  it("renders nothing without a live session", async () => {
    api.listSessions.mockResolvedValue([session("e", "ended", t(3))]);
    renderChip();
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(screen.queryByTestId("home-trainer-chip")).not.toBeInTheDocument();
  });
});
