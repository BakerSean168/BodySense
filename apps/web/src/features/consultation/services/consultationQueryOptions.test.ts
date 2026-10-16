import { describe, expect, it } from "vitest";
import { consultationKeys } from "./consultationQueryKeys";
import {
  consultationThreadQueryOptions,
  conversationQueryOptions,
  diagnosisHistoryQueryOptions,
} from "./consultationQueryOptions";

describe("consultation query options", () => {
  it("uses one canonical key factory per server projection", () => {
    expect(conversationQueryOptions().queryKey).toEqual(
      consultationKeys.conversation(),
    );
    expect(consultationThreadQueryOptions("conversation-1").queryKey).toEqual(
      consultationKeys.thread("conversation-1"),
    );
    expect(diagnosisHistoryQueryOptions(20).queryKey).toEqual(
      consultationKeys.diagnosisHistory(20),
    );
  });
});
