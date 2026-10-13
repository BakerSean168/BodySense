import { describe, expect, it } from "vitest";
import {
  classifyLoadedBodyList,
  commandOutcome,
  personHabitsVM,
} from "./presentation-v2";
import { enabled, meta, receipt } from "./presentation-v2.fixtures";

const error = {
  code: "network",
  message: "连接中断，结果尚待核对",
  retry: "same_request",
};

describe("reconcile the concurrent runnable BodyFact projection without replacing it", () => {
  it("does not label unscanned Habits history globally empty", () => {
    personHabitsVM.parse({
      state: "ready",
      meta,
      data: { habits: [], nextCursor: "before:4", canRecord: enabled },
    });
    expect(classifyLoadedBodyList(0, "before:4")).toBe("partial_empty");
    expect(classifyLoadedBodyList(0, null)).toBe("empty");
    expect(classifyLoadedBodyList(2, "before:4")).toBe("populated");
  });
  it("does not accept a fabricated negative loaded count", () => {
    expect(() => classifyLoadedBodyList(-1, null)).toThrow();
  });
  it("uncertain commit retains the original idempotency key and draft", () => {
    commandOutcome.parse({
      state: "commit_uncertain",
      idempotencyKey: "same-request",
      error,
      retainDraft: true,
      recovery: "same_idempotency_key",
    });
  });
  it("uncertain commit cannot discard its identity or become an automatic new write", () => {
    expect(
      commandOutcome.safeParse({
        state: "commit_uncertain",
        error,
        retainDraft: true,
        recovery: "same_idempotency_key",
      }).success,
    ).toBe(false);
    expect(
      commandOutcome.safeParse({
        state: "commit_uncertain",
        idempotencyKey: "same-request",
        error,
        retainDraft: true,
        recovery: "new_command",
      }).success,
    ).toBe(false);
  });
  it("successful write followed by a failed read still preserves the real receipt", () => {
    commandOutcome.parse({
      state: "committed_refresh_failed",
      receipt,
      refresh: ["diagnosis.current"],
      error,
      recovery: "refresh_queries_only",
    });
  });
  it("committed-refresh-failed cannot be claimed without a receipt", () => {
    expect(
      commandOutcome.safeParse({
        state: "committed_refresh_failed",
        refresh: ["diagnosis.current"],
        error,
        recovery: "refresh_queries_only",
      }).success,
    ).toBe(false);
  });
  it("a failed refresh must not resubmit the already-committed mutation", () => {
    expect(
      commandOutcome.safeParse({
        state: "committed_refresh_failed",
        receipt,
        refresh: ["diagnosis.current"],
        error,
        recovery: "same_idempotency_key",
      }).success,
    ).toBe(false);
  });
});
