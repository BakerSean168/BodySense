import { describe, expect, it } from "vitest";
import { makeWorkspace } from "@/features/body-canvas/model/__fixtures__/workspaceFixtures";
import {
  requiresWorkspaceReview,
  selectWorkspaceSummary,
} from "./workspaceSelectors";

describe("workspace selectors", () => {
  it("uses server capabilities for review instead of interpreting body records", () => {
    const workspace = makeWorkspace();
    workspace.capabilities.requires_safety_review = true;
    expect(requiresWorkspaceReview(workspace.capabilities)).toBe(true);
    expect(selectWorkspaceSummary(workspace)).toEqual({
      canStartTraining: true,
      requiresReview: true,
    });
  });
  it("does not enable execution without a workspace", () => {
    expect(selectWorkspaceSummary(null).canStartTraining).toBe(false);
  });
});
