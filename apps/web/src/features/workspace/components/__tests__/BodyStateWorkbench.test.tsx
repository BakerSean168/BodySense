import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { BodyStateWorkbench } from "../BodyStateWorkbench";
import { workspaceApi } from "../../api/workspaceApi";

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

describe("BodyStateWorkbench observation review", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("keeps assessment observations pending until explicit confirmation", async () => {
    const review = vi
      .spyOn(workspaceApi, "reviewObservation")
      .mockResolvedValue({});
    const user = userEvent.setup();

    const queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <BodyStateWorkbench
          snapshot={{
            user_id: "user-1",
            current_revision: 5,
            safety_state: {},
            facts: [],
            observations: [],
            pending_observations: [
              {
                id: "observation-1",
                kind: "posture_alignment",
                body_region: "肩部",
                method: "posture_photo_front",
                value: {
                  label: "高低肩倾向",
                  description: "右侧肩峰略高",
                },
                review_state: "unverified",
                lifecycle_state: "active",
                excluded_from_reasoning: true,
                updated_revision: 5,
              },
            ],
            hypotheses: [],
            recent_revisions: [],
          }}
        />
      </QueryClientProvider>,
    );

    expect(screen.getByText("高低肩倾向")).toBeInTheDocument();
    expect(
      screen.getByText(/需要你确认后才会用于后续分析/),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "确认" }));

    expect(review).toHaveBeenCalledWith("observation-1", 5, "confirmed");
  });
});
