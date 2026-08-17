import { describe, expect, it } from "vitest";
import { consultationKeys } from "./consultationQueryKeys";
import {
  consultationThreadQueryOptions,
  conversationsQueryOptions,
  diagnosisHistoryQueryOptions,
} from "./consultationQueryOptions";

describe("consultation query options", () => {
  it("uses one canonical key factory per server projection", () => {
    expect(conversationsQueryOptions().queryKey).toEqual(
      consultationKeys.conversations(),
    );
    expect(consultationThreadQueryOptions("conversation-1").queryKey).toEqual(
      consultationKeys.thread("conversation-1"),
    );
    expect(diagnosisHistoryQueryOptions(20).queryKey).toEqual(
      consultationKeys.diagnosisHistory(20),
    );
  });
});
