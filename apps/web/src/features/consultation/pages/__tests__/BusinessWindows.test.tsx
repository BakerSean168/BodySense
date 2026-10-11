import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

vi.mock("@/lib/clientDiagnostics", () => ({
  createClientDiagnosticId: () => "consultation-test-session",
  reportClientDiagnostic: vi.fn(),
}));

const mockUseConversationsQuery = vi.fn();
const mockUseConsultationThreadQuery = vi.fn();
const mockUseHealthWorkspaceQuery = vi.fn();
const mockListTrainingPlans = vi.fn();
const mockGetBodyMetrics = vi.fn();
const mockGetLifestyle = vi.fn();

vi.mock("../../hooks/useConversationsQuery", () => ({
  useConversationsQuery: () => mockUseConversationsQuery(),
}));
vi.mock("../../hooks/useConsultationThreadQuery", () => ({
  useConsultationThreadQuery: (conversationId: string | null) =>
    mockUseConsultationThreadQuery(conversationId),
}));
vi.mock("@/features/workspace", () => ({
  useHealthWorkspaceQuery: () => mockUseHealthWorkspaceQuery(),
  workspaceKeys: { all: ["workspace"] },
}));
vi.mock("@/features/training", () => ({
  trainingApi: { listPlans: () => mockListTrainingPlans() },
  TrainingExecutionPanel: ({ planId }: { planId: string }) => (
    <div data-testid="training-execution-panel">{planId}</div>
  ),
}));
vi.mock("@/features/profile/services/bodyMetricsService", () => ({
  bodyMetricsService: { get: () => mockGetBodyMetrics() },
}));
vi.mock("@/features/profile/services/lifestyleService", () => ({
  lifestyleService: { get: () => mockGetLifestyle() },
}));
vi.mock("@/features/profile/components/profile/ProfileDrawer", () => ({
  ProfileDrawer: () => <div data-testid="profile-drawer" />,
}));
vi.mock("@/features/body-explorer", () => ({
  useBodyExplorerWorkspace: () => ({
    selectedRegionId: null,
    selectRegion: vi.fn(),
    semanticBridge: {},
  }),
}));
vi.mock("@/features/body-explorer/components/BodyExplorer", () => ({
  BodyExplorer: () => <div data-testid="body-explorer" />,
}));
vi.mock("../../components/AssistantChatPanel", () => ({
  AssistantChatPanel: () => <div data-testid="assistant-chat-panel">chat</div>,
}));

import { ConsultationPage } from "../ConsultationPage";
import type {
  ConsultationThread,
  Conversation,
} from "../../types/consultation";
import type { HealthWorkspace } from "@/features/workspace/types/workspace";

function makeConversation(overrides: Partial<Conversation> = {}): Conversation {
  return {
    id: "conv-1",
    title: "肩颈咨询",
    title_status: "generated",
    status: "active",
    pinned: false,
    pinned_at: null,
    default_model: null,
    last_message_at: null,
    message_count: 0,
    metadata: {},
    created_at: "2026-07-06T00:00:00Z",
    updated_at: "2026-07-06T00:00:00Z",
    ...overrides,
  };
}

function makeThread(overrides: Partial<ConsultationThread> = {}): ConsultationThread {
  const conversation = makeConversation(overrides.conversation);
  return {
    conversation,
    conversation_id: conversation.id,
    phase: "collecting",
    extracted_info: [],
    diagnosis: null,
    pending_interactions: [],
    interaction_history: [],
    created_at: "2026-07-06T00:00:00Z",
    updated_at: "2026-07-06T00:00:00Z",
    ended_at: null,
    messages: [],
    tool_calls: [],
    ...overrides,
  };
}

function makeWorkspace(overrides: Partial<HealthWorkspace> = {}): HealthWorkspace {
  return {
    generated_at: "2026-10-11T00:00:00Z",
    profile_ready: true,
    body_state: {
      current_revision: 2,
      safety_state: {},
      facts: [],
      observations: [],
      pending_facts: [],
      pending_observations: [],
      hypotheses: [],
      recent_revisions: [],
    },
    treatment_revisions: [],
    recent_outcomes: [],
    trends: [],
    capabilities: {
      can_continue_consultation: true,
      can_edit_body_state: true,
      can_request_diagnosis: true,
      can_review_diagnosis: true,
      can_generate_treatment: true,
      can_accept_treatment: true,
      can_execute_treatment: true,
      can_record_outcome: true,
      can_review_treatment: true,
      requires_safety_review: false,
      requires_diagnosis_review: false,
      requires_treatment_review: false,
    },
    actions: [],
    ...overrides,
  };
}

function renderPage(path = "/consultation/conv-1") {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/consultation/:id" element={<ConsultationPage />} />
          <Route path="/consultation" element={<ConsultationPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function openDock(name: string) {
  return userEvent.click(screen.getByRole("button", { name }));
}

beforeEach(() => {
  mockUseConversationsQuery.mockReset();
  mockUseConsultationThreadQuery.mockReset();
  mockUseHealthWorkspaceQuery.mockReset();
  mockListTrainingPlans.mockReset();
  mockGetBodyMetrics.mockReset();
  mockGetLifestyle.mockReset();
  mockUseConversationsQuery.mockReturnValue({
    data: [makeConversation()],
    isPending: false,
  });
  mockUseConsultationThreadQuery.mockReturnValue({
    data: makeThread(),
    isPending: false,
    isFetching: false,
    isError: false,
  });
  mockUseHealthWorkspaceQuery.mockReturnValue({
    data: makeWorkspace(),
    isPending: false,
    isFetching: false,
    isError: false,
  });
  mockListTrainingPlans.mockResolvedValue([]);
  mockGetBodyMetrics.mockResolvedValue({ current_revision: 1 });
  mockGetLifestyle.mockResolvedValue({
    current_revision: 1,
    activity: { kind: "activity", summary: "", details: {} },
    sleep: { kind: "sleep", summary: "每晚 7 小时", details: {} },
    exercise: { kind: "exercise", summary: "", details: {} },
    nutrition: { kind: "nutrition", summary: "", details: {} },
    substances: { kind: "substances", summary: "", details: {} },
    recovery: { kind: "recovery", summary: "", details: {} },
    pending_updates: [],
  });
});

describe("body-first business windows", () => {
  it("starts body-first with no business window open", () => {
    renderPage();
    expect(screen.getByTestId("body-canvas-workspace")).toBeInTheDocument();
    expect(screen.queryByTestId("diagnosis-window")).not.toBeInTheDocument();
    expect(screen.queryByTestId("training-window")).not.toBeInTheDocument();
    expect(screen.queryByTestId("person-window")).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "我的身体" })).toBeInTheDocument();
  });

  it("opens the Chat window from the Dock and closes it again", async () => {
    renderPage();
    await openDock("对话");
    expect(await screen.findByTestId("assistant-chat-panel")).toBeVisible();
    await openDock("对话");
    expect(screen.getByTestId("assistant-chat-panel")).not.toBeVisible();
  });

  it("opens the Diagnosis window with a single level-1 title and Close", async () => {
    renderPage();
    await openDock("综合诊断");
    const window = await screen.findByTestId("diagnosis-window");
    expect(window).toBeInTheDocument();
    expect(screen.getAllByRole("heading", { name: "综合诊断", level: 2 })).toHaveLength(1);
    // The window has no duplicated body title and no technical version UI.
    expect(screen.queryByText(/修订|版本号|revision/i)).not.toBeInTheDocument();
  });

  it("never shows an unconfirmed candidate as a confirmed condition (DX-06)", async () => {
    mockUseHealthWorkspaceQuery.mockReturnValue({
      data: makeWorkspace({
        diagnosis: {
          analysis_id: "analysis-1",
          body_state_revision: 2,
          status: "completed",
          scope: "comprehensive",
          summary: "",
          candidates: [
            {
              candidate_id: "cand-1",
              name: "上交叉综合征倾向",
              confidence: "高",
              basis: "久坐后肩颈酸胀",
            },
          ],
          citations: [],
          freshness: {
            analysis_id: "analysis-1",
            state: "fresh",
            evaluated_against_revision: 2,
            reasons: [],
            checked_at: "2026-10-11T00:00:00Z",
          },
          candidate_assessments: [],
          created_at: "2026-10-11T00:00:00Z",
        },
      }),
      isPending: false,
      isFetching: false,
      isError: false,
    });
    renderPage();
    await openDock("综合诊断");
    await screen.findByTestId("diagnosis-window");
    expect(screen.getByText("还没有已确认的病情")).toBeInTheDocument();
    expect(screen.getByText(/1 条分析提案尚未确认/)).toBeInTheDocument();
  });

  it("lists a confirmed condition with cause, handling boundary and certainty", async () => {
    mockUseHealthWorkspaceQuery.mockReturnValue({
      data: makeWorkspace({
        diagnosis: {
          analysis_id: "analysis-1",
          body_state_revision: 2,
          status: "completed",
          scope: "comprehensive",
          summary: "",
          candidates: [
            {
              candidate_id: "cand-1",
              name: "上交叉综合征倾向",
              confidence: "高",
              basis: "久坐后肩颈酸胀",
              reasoning_summary: "长期久坐使肩胛稳定肌群持续紧张",
              missing_information: ["夜间痛是否加重"],
            },
          ],
          citations: [],
          freshness: {
            analysis_id: "analysis-1",
            state: "fresh",
            evaluated_against_revision: 2,
            reasons: [],
            checked_at: "2026-10-11T00:00:00Z",
          },
          candidate_assessments: [{ candidate_id: "cand-1", state: "confirmed" }],
          created_at: "2026-10-11T00:00:00Z",
        },
      }),
      isPending: false,
      isFetching: false,
      isError: false,
    });
    renderPage();
    await openDock("综合诊断");
    await screen.findByTestId("diagnosis-window");
    expect(screen.getByText("已确认病情 · 1 项")).toBeInTheDocument();
    expect(screen.getByText("上交叉综合征倾向")).toBeInTheDocument();
    expect(screen.getByText("长期久坐使肩胛稳定肌群持续紧张")).toBeInTheDocument();
    expect(screen.getByText("你已确认的分析结果")).toBeInTheDocument();
    expect(screen.getByText("夜间痛是否加重")).toBeInTheDocument();
    expect(screen.getByText(/还没有针对这条病情的已接受处理方案/)).toBeInTheDocument();
  });

  it("shows a real empty state instead of a demo plan when the user has none", async () => {
    renderPage();
    await openDock("训练计划");
    await screen.findByTestId("training-window");
    await waitFor(() =>
      expect(screen.getByText("还没有训练计划")).toBeInTheDocument(),
    );
  });

  it("renders the real plan with planId, dosage and stages, and picks by planId", async () => {
    const user = userEvent.setup();
    mockListTrainingPlans.mockResolvedValue([
      {
        id: "plan-1",
        status: "active",
        goal: "肩胛稳定训练",
        duration_weeks: 6,
        current_week: 1,
        treatment_revision_id: "rev-1",
        phases: [],
        created_at: "2026-10-11T00:00:00Z",
      },
      {
        id: "plan-2",
        status: "paused",
        goal: "臀腿力量训练",
        duration_weeks: 4,
        current_week: 1,
        phases: [],
        created_at: "2026-10-11T00:00:00Z",
      },
    ]);
    mockUseHealthWorkspaceQuery.mockReturnValue({
      data: makeWorkspace({
        treatment_revisions: [
          {
            id: "rev-1",
            treatment_id: "treatment-1",
            revision: 1,
            acceptance_state: "accepted",
            lifecycle_state: "active",
            source_body_state_revision: 2,
            source_diagnosis_analysis_id: "analysis-1",
            goal: "肩胛稳定训练",
            duration_weeks: 6,
            plan: {
              summary: "",
              goal: "肩胛稳定训练",
              duration_weeks: 6,
              interventions: [],
              daily_habits: [],
              expected_timeline: "",
              warning_signs: [],
              review_triggers: [],
              safety_notes: [],
            },
            user_constraints: {},
            evidence_ids: [],
            governance: {},
            change_reason: "",
            created_at: "2026-10-11T00:00:00Z",
            interventions: [
              {
                id: "int-warm",
                treatment_id: "treatment-1",
                treatment_revision_id: "rev-1",
                kind: "mobility",
                title: "胸椎活动",
                description: "",
                prescription: { sets: 2, reps: 10 },
                position: 0,
                status: "active",
                created_at: "2026-10-11T00:00:00Z",
                updated_at: "2026-10-11T00:00:00Z",
              },
              {
                id: "int-main",
                treatment_id: "treatment-1",
                treatment_revision_id: "rev-1",
                kind: "exercise",
                title: "靠墙肩胛滑动",
                description: "",
                prescription: { sets: 3, reps: 10, frequency: "每周 3 次" },
                position: 1,
                status: "active",
                created_at: "2026-10-11T00:00:00Z",
                updated_at: "2026-10-11T00:00:00Z",
              },
            ],
          },
        ],
      }),
      isPending: false,
      isFetching: false,
      isError: false,
    });
    renderPage();
    await openDock("训练计划");
    const trainingWindow = await screen.findByTestId("training-window");

    // The selected plan is a real planId from the list, not a static example.
    await waitFor(() =>
      expect(screen.getByText("靠墙肩胛滑动")).toBeInTheDocument(),
    );
    expect(screen.getByText("胸椎活动")).toBeInTheDocument();
    expect(screen.getByText("2 组 · 10 次")).toBeInTheDocument();
    expect(screen.getByText("3 组 · 10 次")).toBeInTheDocument();
    expect(screen.getByText(/每周 3 次/)).toBeInTheDocument();
    expect(screen.getByText("热身与活动度")).toBeInTheDocument();
    expect(screen.getByText("正式训练")).toBeInTheDocument();

    // The picker switches plans by their real identity.
    await user.click(trainingWindow.querySelector(".bc-plan-selector-trigger")!);
    await user.click(screen.getByRole("option", { name: /臀腿力量训练/ }));
    await waitFor(() =>
      expect(localStorage.getItem("bodysense.training.selectedPlanId")).toBe(
        "plan-2",
      ),
    );
  });

  it("keeps exactly one Training window when plans switch repeatedly (V5.5 Dock exclusion)", async () => {
    const user = userEvent.setup();
    mockListTrainingPlans.mockResolvedValue([
      {
        id: "plan-1",
        status: "active",
        goal: "肩胛稳定训练",
        duration_weeks: 6,
        current_week: 1,
        treatment_revision_id: "rev-1",
        phases: [],
        created_at: "2026-10-11T00:00:00Z",
      },
      {
        id: "plan-2",
        status: "paused",
        goal: "臀腿力量训练",
        duration_weeks: 4,
        current_week: 1,
        phases: [],
        created_at: "2026-10-11T00:00:00Z",
      },
    ]);
    renderPage();
    await openDock("训练计划");
    const window = await screen.findByTestId("training-window");
    await waitFor(() =>
      expect(window.querySelectorAll(".bc-plan-selector-trigger")).toHaveLength(1),
    );

    for (const title of ["臀腿力量训练", "肩胛稳定训练"]) {
      await user.click(window.querySelector(".bc-plan-selector-trigger")!);
      await user.click(screen.getByRole("option", { name: new RegExp(title) }));
      await waitFor(() =>
        expect(window.querySelector(".bc-plan-selector-name")?.textContent).toBe(title),
      );
    }
    // Only one Training window exists, and no residual variant is left behind.
    expect(screen.getAllByTestId("training-window")).toHaveLength(1);
  });

  it("closes the Training window when another window opens on demand", async () => {
    const user = userEvent.setup();
    renderPage();
    await openDock("训练计划");
    await screen.findByTestId("training-window");
    await openDock("综合诊断");
    await screen.findByTestId("diagnosis-window");
    // Close Training explicitly, mirroring the prototype's mutual exclusion.
    await user.click(screen.getByRole("button", { name: "关闭训练计划" }));
    // Closing keeps the window mounted but hidden; only one window is visible.
    expect(screen.getByTestId("training-window")).not.toBeVisible();
    expect(screen.getByTestId("diagnosis-window")).toBeVisible();
  });

  it("keeps the chat mounted when its window closes so in-flight state survives", async () => {
    renderPage();
    await openDock("对话");
    expect(await screen.findByTestId("assistant-chat-panel")).toBeVisible();
    await openDock("对话");
    // Still mounted, just hidden: closing a window must not discard the task.
    expect(screen.getByTestId("assistant-chat-panel")).not.toBeVisible();
  });

  it("folds and restores a window without losing its identity", async () => {
    renderPage();
    await openDock("综合诊断");
    await screen.findByTestId("diagnosis-window");
    await userEvent.click(screen.getByRole("button", { name: "最小化综合诊断" }));
    // Folded away but still open and recoverable.
    expect(screen.getByTestId("diagnosis-window")).not.toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "恢复综合诊断" }));
    expect(screen.getByTestId("diagnosis-window")).toBeVisible();
    // Exactly one window identity survived the round trip.
    expect(screen.getAllByTestId("diagnosis-window")).toHaveLength(1);
  });

  it("keeps at most one business window visible when several are opened", async () => {
    renderPage();
    await openDock("综合诊断");
    await openDock("训练计划");
    await openDock("对话");
    // Dock/Open/Active are mutually exclusive: the previously visible windows are
    // folded, not destroyed, so they cannot overlap or steal focus.
    expect(screen.getByTestId("training-window")).not.toBeVisible();
    expect(screen.getByTestId("diagnosis-window")).not.toBeVisible();
    expect(screen.getByTestId("assistant-chat-panel")).toBeVisible();
    expect(
      screen.getByRole("button", { name: "恢复综合诊断" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "恢复训练计划" })).toBeInTheDocument();
  });

  it("opens the Person window with five categories and a compact identity popover", async () => {
    renderPage();
    await openDock("个人档案");
    const person = await screen.findByTestId("person-window");
    for (const label of ["身体记录", "装备", "影像资料", "医学报告", "习性"]) {
      expect(screen.getByRole("button", { name: label })).toBeInTheDocument();
    }
    // The tab label is the only place the category name appears.
    expect(
      screen.queryAllByRole("heading", { name: "身体记录" }),
    ).toHaveLength(0);
    expect(person.querySelector(".bc-person-popover")).toBeNull();
    await userEvent.click(screen.getByRole("button", { name: /我的档案|未设置昵称/ }));
    const popover = await screen.findByRole("dialog", { name: "基础资料" });
    expect(popover.textContent).toContain("BMI —");
    expect(screen.getByRole("button", { name: "编辑基础资料" })).toBeInTheDocument();
  });

  it("shows a centred equipment empty state with a single add action", async () => {
    renderPage();
    await openDock("个人档案");
    await screen.findByTestId("person-window");
    await userEvent.click(screen.getByRole("button", { name: "装备" }));
    expect(
      screen.getByText("添加个人器材，帮助 AI 更好规划训练项目"),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /添加个人器材/ })).toBeInTheDocument();
    // No fixed slot ceiling is displayed.
    expect(screen.queryByText(/\/\s*12/)).not.toBeInTheDocument();
  });

  it("renders real habit values and marks unrecorded ones as missing", async () => {
    renderPage();
    await openDock("个人档案");
    await screen.findByTestId("person-window");
    await userEvent.click(screen.getByRole("button", { name: "习性" }));
    await waitFor(() =>
      expect(screen.getByText("每晚 7 小时")).toBeInTheDocument(),
    );
    expect(screen.getAllByText("未记录").length).toBeGreaterThan(0);
  });

  it("closes the topmost window on Escape instead of clearing the body selection", async () => {
    renderPage();
    await openDock("综合诊断");
    await screen.findByTestId("diagnosis-window");
    await userEvent.keyboard("{Escape}");
    expect(screen.getByTestId("diagnosis-window")).not.toBeVisible();
    // Escape retreats one layer; the body stage and its selection survive.
    expect(screen.getByRole("heading", { name: "我的身体" })).toBeVisible();
  });

  it("opens a deep-linked window from the route", async () => {
    renderPage("/consultation/conv-1?view=training");
    expect(await screen.findByTestId("training-window")).toBeInTheDocument();
  });
});
