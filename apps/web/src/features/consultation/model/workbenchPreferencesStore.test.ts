import { describe, expect, it } from "vitest";
import { clampChatSize } from "./workbenchPreferencesStore";

describe("clampChatSize", () => {
  it("keeps a useful desktop range", () => {
    expect(clampChatSize(12)).toBe(24);
    expect(clampChatSize(38.26)).toBe(38.3);
    expect(clampChatSize(70)).toBe(52);
  });

  it("recovers from invalid persisted values", () => {
    expect(clampChatSize(Number.NaN)).toBe(38);
  });
});
