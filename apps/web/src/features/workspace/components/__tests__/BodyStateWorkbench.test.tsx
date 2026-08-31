import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
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

  // 生活习惯类的内容不应该搭配 后来改善，后来加重，后来恢复 之类的按钮
  it("does not show symptom trend actions for lifestyle facts", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    });

    render(
      <QueryClientProvider client={queryClient}>
        <BodyStateWorkbench
          snapshot={{
            user_id: "user-1",
            current_revision: 10,
            safety_state: {},
            facts: [
              {
                id: "lifestyle-1",
                kind: "lifestyle.activity",
                body_region: "",
                value: "日常活动以久坐为主",
                origin: "user_reported",
                review_state: "confirmed",
                lifecycle_state: "active",
                trend: "stable",
                updated_revision: 10,
              },
            ],
            observations: [],
            pending_observations: [],
            hypotheses: [],
            recent_revisions: [],
          }}
        />
      </QueryClientProvider>,
    );
    expect(screen.getByText("日常活动以久坐为主")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "后来改善" }),
    ).not.toBeInTheDocument();

    expect(
      screen.queryByRole("button", { name: "后来加重" }),
    ).not.toBeInTheDocument();

    expect(
      screen.queryByRole("button", { name: "后来恢复" }),
    ).not.toBeInTheDocument();

    expect(
      screen.getByRole("button", { name: "纠正记录" }),
    ).toBeInTheDocument();

    expect(
      screen.getByRole("button", { name: "更新现状" }),
    ).toBeInTheDocument();
  });

  // 不适症状类的内容应该搭配 后来改善，后来加重，后来恢复 之类的按钮
  it("keeps symptom trend actions for discomfort facts", () => {
    const queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    });

    render(
      <QueryClientProvider client={queryClient}>
        <BodyStateWorkbench
          snapshot={{
            user_id: "user-1",
            current_revision: 10,
            safety_state: {},
            facts: [
              {
                id: "symptom-1",
                kind: "discomfort",
                body_region: "右臀",
                value: "久坐后右臀疼痛",
                origin: "user_reported",
                review_state: "confirmed",
                lifecycle_state: "active",
                trend: "stable",
                updated_revision: 10,
              },
            ],
            observations: [],
            pending_observations: [],
            hypotheses: [],
            recent_revisions: [],
          }}
        />
      </QueryClientProvider>,
    );

    expect(screen.getByText("久坐后右臀疼痛")).toBeInTheDocument();

    expect(
      screen.getByRole("button", { name: "后来改善" }),
    ).toBeInTheDocument();

    expect(
      screen.getByRole("button", { name: "后来加重" }),
    ).toBeInTheDocument();

    expect(
      screen.getByRole("button", { name: "后来恢复" }),
    ).toBeInTheDocument();
  });

  it("opens current lifestyle update editor", async () => {
    const user = userEvent.setup();

    const queryClient = new QueryClient({
      defaultOptions: {
        mutations: { retry: false },
      },
    });

    render(
      <QueryClientProvider client={queryClient}>
        <BodyStateWorkbench
          snapshot={{
            user_id: "user-1",
            current_revision: 10,
            safety_state: {},
            facts: [
              {
                id: "lifestyle-1",
                kind: "lifestyle.activity",
                body_region: "",
                value: "日常活动以久坐为主",
                origin: "user_reported",
                review_state: "confirmed",
                lifecycle_state: "active",
                trend: "stable",
                updated_revision: 10,
              },
            ],
            observations: [],
            pending_observations: [],
            hypotheses: [],
            recent_revisions: [],
          }}
        />
      </QueryClientProvider>,
    );

    await user.click(
      screen.getByRole("button", {
        name: "更新现状",
      }),
    );

    expect(screen.getByDisplayValue("日常活动以久坐为主")).toBeInTheDocument();

    expect(
      screen.getByRole("button", {
        name: "取消",
      }),
    ).toBeInTheDocument();

    expect(
      screen.getByRole("button", {
        name: "保存更新",
      }),
    ).toBeInTheDocument();

    const textarea = screen.getByDisplayValue("日常活动以久坐为主");

    await user.clear(textarea);

    await user.type(textarea, "现在工作中会频繁走动");

    expect(
      screen.getByDisplayValue("现在工作中会频繁走动"),
    ).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", {
        name: "取消",
      }),
    );

    expect(
      screen.queryByDisplayValue("日常活动以久坐为主"),
    ).not.toBeInTheDocument();

    expect(
      screen.getByRole("button", {
        name: "更新现状",
      }),
    ).toBeInTheDocument();
  });

  it("saves a lifestyle current-state update", async () => {
    const updateLifestyleCurrent = vi
      .spyOn(workspaceApi, "updateLifestyleCurrent")
      .mockResolvedValue({});

    const user = userEvent.setup();

    const queryClient = new QueryClient({
      defaultOptions: {
        mutations: { retry: false },
      },
    });

    render(
      <QueryClientProvider client={queryClient}>
        <BodyStateWorkbench
          snapshot={{
            user_id: "user-1",
            current_revision: 10,
            safety_state: {},
            facts: [
              {
                id: "lifestyle-1",
                kind: "lifestyle.activity",
                body_region: "",
                value: "日常活动以久坐为主",
                details: {},
                origin: "user_reported",
                review_state: "confirmed",
                lifecycle_state: "active",
                trend: "stable",
                updated_revision: 10,
              },
            ],
            observations: [],
            pending_observations: [],
            hypotheses: [],
            recent_revisions: [],
          }}
        />
      </QueryClientProvider>,
    );

    await user.click(
      screen.getByRole("button", {
        name: "更新现状",
      }),
    );

    const textarea = screen.getByDisplayValue("日常活动以久坐为主");

    await user.clear(textarea);

    await user.type(textarea, "现在工作中会频繁走动");

    await user.click(
      screen.getByRole("button", {
        name: "保存更新",
      }),
    );

    await waitFor(() =>
      expect(updateLifestyleCurrent).toHaveBeenCalledWith(
        10,
        "activity",
        "现在工作中会频繁走动",
        {},
      ),
    );
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

  it("shows extracted symptom facts as review candidates instead of current truth", async () => {
    const review = vi.spyOn(workspaceApi, "reviewFact").mockResolvedValue({
      fact: {} as never,
    });
    const user = userEvent.setup();
    const queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    });

    render(
      <QueryClientProvider client={queryClient}>
        <BodyStateWorkbench
          snapshot={{
            user_id: "user-1",
            current_revision: 9,
            safety_state: {},
            facts: [],
            pending_facts: [
              {
                id: "candidate-1",
                kind: "discomfort",
                body_region: "右臀",
                value: "疼痛",
                origin: "ai_extracted",
                review_state: "unverified",
                lifecycle_state: "active",
                trend: "unknown",
                updated_revision: 9,
              },
            ],
            observations: [],
            pending_observations: [],
            hypotheses: [],
            recent_revisions: [],
          }}
        />
      </QueryClientProvider>,
    );

    expect(screen.getByText(/从对话中识别/)).toBeInTheDocument();
    expect(screen.getByText("右臀 · 不适 / 症状")).toBeInTheDocument();
    expect(screen.getByText("疼痛")).toBeInTheDocument();
    expect(screen.getByText(/确认前不会用于诊断/)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "确认记录" }));
    expect(review).toHaveBeenCalledWith("candidate-1", 9, "confirmed");
  });

  it("filters records by selected canonical region and preserves the canonical id when adding", async () => {
    const addFact = vi.spyOn(workspaceApi, "addFact").mockResolvedValue({
      fact: {} as never,
    });
    const user = userEvent.setup();
    const queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    });

    render(
      <QueryClientProvider client={queryClient}>
        <BodyStateWorkbench
          selectedRegionId="shoulder.right"
          snapshot={{
            user_id: "user-1",
            current_revision: 7,
            safety_state: {},
            facts: [
              {
                id: "right-shoulder",
                kind: "discomfort",
                body_region: "右肩",
                body_region_id: "shoulder.right",
                value: "抬手时右肩疼",
                origin: "user_reported",
                review_state: "confirmed",
                lifecycle_state: "active",
                trend: "stable",
                updated_revision: 7,
              },
              {
                id: "left-knee",
                kind: "discomfort",
                body_region: "左膝",
                body_region_id: "knee.left",
                value: "跑步后左膝酸",
                origin: "user_reported",
                review_state: "confirmed",
                lifecycle_state: "active",
                trend: "stable",
                updated_revision: 7,
              },
            ],
            observations: [],
            pending_observations: [],
            hypotheses: [],
            recent_revisions: [],
          }}
        />
      </QueryClientProvider>,
    );

    expect(screen.getByText("抬手时右肩疼")).toBeInTheDocument();
    expect(screen.queryByText("跑步后左膝酸")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "添加记录" }));
    await user.type(screen.getByPlaceholderText("记录内容"), "右肩今天更轻松");
    await user.click(screen.getByRole("button", { name: "保存记录" }));

    expect(addFact).toHaveBeenCalledWith(
      7,
      expect.objectContaining({
        body_region: "右肩",
        body_region_id: "shoulder.right",
        concern_key: "region:shoulder.right",
        value: "右肩今天更轻松",
      }),
    );
  });
});
