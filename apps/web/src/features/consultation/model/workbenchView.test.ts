import { describe, expect, it } from "vitest";
import { parseWorkspaceView } from "./workbenchView";

describe("parseWorkspaceView", () => {
  it("accepts known route-addressable views", () => {
    expect(parseWorkspaceView("diagnosis")).toBe("diagnosis");
    expect(parseWorkspaceView("treatment")).toBe("treatment");
    expect(parseWorkspaceView("progress")).toBe("progress");
  });

  it("falls back to the durable state workspace", () => {
    expect(parseWorkspaceView(null)).toBe("state");
    expect(parseWorkspaceView("unknown")).toBe("state");
  });
});
