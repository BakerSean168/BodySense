import { bodyRecordsFixture } from "@/features/profile/services/__fixtures__/bodyRecordsFixture";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useAuthStore } from "@/stores/authStore";
import { useUploadStore } from "@/stores/uploadStore";

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
const mockListBodyFacts = vi.fn();
const mockReadBodyRecords = vi.fn();
vi.mock("@/features/profile/services/bodyRecordsService", () => ({
  readBodyRecords: () => mockReadBodyRecords(),
}));
vi.mock("@/features/profile/services/bodyFactRecordsService", () => ({
  bodyFactRecordsService: { list: () => mockListBodyFacts(), record: vi.fn() },
}));
vi.mock("@/features/profile/services/bodyEpisodeService", () => ({
  bodyEpisodeService: { list: async () => ({ user_id: "11111111-1111-4111-8111-111111111111", current_revision: 1, episodes: [], has_more: false }) },
}));
const mockListEquipment = vi.fn();
const mockUpdateEquipment = vi.fn();
vi.mock("@/features/profile/services/equipmentService", () => ({
  equipmentKeys: { all: ["equipment"] },
  equipmentService: {
    list: () => mockListEquipment(),
    create: vi.fn(),
    update: (id: string, input: unknown) => mockUpdateEquipment(id, input),
  },
  availabilityChange: (
    asset: { revision: number; display_name: string },
    availability: string,
  ) => ({
    expected_revision: asset.revision,
    display_name: asset.display_name,
    availability,
  }),
}));

vi.mock("../../hooks/useConversationsQuery", () => ({
  useConversationsQuery: () => mockUseConversationsQuery(),
}));
vi.mock("../../hooks/useConsultationThreadQuery", () => ({
  useConsultationThreadQuery: (conversationId: string | null) =>
    mockUseConsultationThreadQuery(conversationId),
}));
vi.mock("@/features/workspace/hooks/useHealthWorkspaceQuery", () => ({
  useHealthWorkspaceQuery: () => mockUseHealthWorkspaceQuery(),
}));
vi.mock("@/features/training/services/trainingService", () => ({
  trainingApi: { listPlans: () => mockListTrainingPlans() },
}));
vi.mock("@/features/training/components/TrainingExecutionPanel", () => ({
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

function makeThread(
  overrides: Partial<ConsultationThread> = {},
): ConsultationThread {
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

function makeWorkspace(
  overrides: Partial<HealthWorkspace> = {},
): HealthWorkspace {
  return {
    generated_at: "2026-10-11T00:00:00Z",
    profile_ready: true,
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
  const dock = screen.getByRole("navigation", { name: "身体工作区导航" });
  // Diagnosis gains a confirmed-count suffix; the Dock module identity stays
  // stable and its name must not depend on an invented clinical item count.
  return userEvent.click(
    within(dock).getByRole("button", {
      name: new RegExp(`^${name}(?:，.*)?$`),
    }),
  );
}

beforeEach(() => {
  mockReadBodyRecords.mockReset();
  mockReadBodyRecords.mockResolvedValue(bodyRecordsFixture());
  mockUseConversationsQuery.mockReset();
  mockUseConsultationThreadQuery.mockReset();
  mockUseHealthWorkspaceQuery.mockReset();
  mockListTrainingPlans.mockReset();
  mockGetBodyMetrics.mockReset();
  mockGetLifestyle.mockReset();
  mockListEquipment.mockReset();
  mockListEquipment.mockResolvedValue({ items: [] });
  mockUpdateEquipment.mockReset();
  const owner = "11111111-1111-4111-8111-111111111111";
  useAuthStore.setState({
    user: { id: owner, email: "synthetic@example.test" },
    isAuthenticated: true,
  });
  useUploadStore.setState({
    uploads: [],
    fetchUploads: vi.fn().mockResolvedValue(undefined),
  });
  mockListBodyFacts.mockReset();
  localStorage.removeItem("bodysense.training.selectedPlanId");
  mockListBodyFacts.mockResolvedValue({
    user_id: owner,
    current_revision: 1,
    has_more: false,
    facts: [
      {
        id: "22222222-2222-4222-8222-222222222222",
        user_id: owner,
        kind: "lifestyle",
        statement: "每晚 7 小时",
        body_region_ids: [],
        temporal: { precision: "unknown" },
        details: {},
        subject: "self",
        assertion_status: "asserted",
        source_evidence_id: "22222222-2222-4222-8222-222222222222",
        created_revision: 1,
        recorded_at: "2026-10-13T00:00:00Z",
      },
    ],
  });
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
    expect(
      screen.getByRole("heading", { name: "我的身体" }),
    ).toBeInTheDocument();
  });

  it("opens the Chat window from the Dock and closes it again", async () => {
    renderPage();
    await openDock("对话");
    expect(await screen.findByTestId("assistant-chat-panel")).toBeVisible();
    await openDock("对话");
    expect(screen.getByTestId("assistant-chat-panel")).not.toBeVisible();
  });

  it("opens the V2 Diagnosis selector toolbar with a single Close control", async () => {
    renderPage();
    await openDock("综合诊断");
    expect(await screen.findByTestId("diagnosis-window")).toBeVisible();
    expect(
      screen.getByRole("combobox", { name: "选择已确认病情" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "关闭综合诊断" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "综合诊断", level: 2 }),
    ).not.toBeInTheDocument();
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
          candidate_assessments: [
            { candidate_id: "cand-1", state: "confirmed" },
          ],
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
    expect(
      screen.getByRole("button", { name: "综合诊断，1 项已确认病情" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("combobox", { name: "选择已确认病情" }),
    ).toHaveValue("cand-1");
    expect(
      screen.getByRole("heading", { name: "上交叉综合征倾向" }),
    ).toBeVisible();
    expect(
      screen.getByText("长期久坐使肩胛稳定肌群持续紧张"),
    ).toBeInTheDocument();
    expect(screen.getByText("你已确认的分析结果")).toBeInTheDocument();
    expect(screen.getByText("夜间痛是否加重")).toBeInTheDocument();
    expect(
      screen.getByText(/还没有针对这条病情的已接受处理方案/),
    ).toBeInTheDocument();
  });

  it("shows a real empty state instead of a demo plan when the user has none", async () => {
    renderPage();
    await openDock("训练计划");
    await screen.findByTestId("training-window");
    await waitFor(() =>
      expect(screen.getByText("还没有训练计划")).toBeInTheDocument(),
    );
  });

  it("renders an accepted plan with real planId and dosage, excluding unaccepted alternatives", async () => {
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
        training_plan: {
          id: "plan-1",
          status: "active",
          goal: "肩胛稳定训练",
          duration_weeks: 6,
          current_week: 1,
          phases: [],
          created_at: "2026-10-11T00:00:00Z",
        },
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

    // The V2 header includes the accepted plan only; an unaccepted paused
    // plan cannot silently be selected as the current executable plan.
    expect(trainingWindow.querySelector(".bc-plan-selector")).toBeNull();
    const selector = screen.getByRole("combobox", { name: "选择训练计划" });
    expect(selector).toHaveValue("plan-1");
    expect(
      screen.getByRole("option", { name: "肩胛稳定训练" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("option", { name: "臀腿力量训练" }),
    ).not.toBeInTheDocument();
    await waitFor(() =>
      expect(localStorage.getItem("bodysense.training.selectedPlanId")).toBe(
        "plan-1",
      ),
    );
  });

  it("preserves one V2 Training window when Dock closes and reopens it", async () => {
    renderPage();
    await openDock("训练计划");
    const panel = await screen.findByTestId("training-window");
    expect(panel).toBeVisible();
    for (let i = 0; i < 3; i++) {
      await openDock("训练计划");
      expect(panel).not.toBeVisible();
      await openDock("训练计划");
      expect(panel).toBeVisible();
      expect(screen.getAllByTestId("training-window")).toHaveLength(1);
      expect(screen.getByTestId("training-window")).toBe(panel);
    }
    expect(document.querySelectorAll(".bc-plan-selector")).toHaveLength(0);
  });

  it("switches Training to Diagnosis without destroying the hidden Training window", async () => {
    renderPage();
    await openDock("训练计划");
    const training = await screen.findByTestId("training-window");
    await openDock("综合诊断");
    expect(await screen.findByTestId("diagnosis-window")).toBeVisible();
    expect(training).not.toBeVisible();
    await openDock("训练计划");
    expect(training).toBeVisible();
    expect(screen.getByTestId("diagnosis-window")).not.toBeVisible();
    expect(screen.getByTestId("training-window")).toBe(training);
  });

  it("keeps the chat mounted when its window closes so in-flight state survives", async () => {
    renderPage();
    await openDock("对话");
    expect(await screen.findByTestId("assistant-chat-panel")).toBeVisible();
    await openDock("对话");
    // Still mounted, just hidden: closing a window must not discard the task.
    expect(screen.getByTestId("assistant-chat-panel")).not.toBeVisible();
  });

  it("restores a hidden Diagnosis window using its Dock entry without recreating the task", async () => {
    renderPage();
    await openDock("综合诊断");
    const diagnosis = await screen.findByTestId("diagnosis-window");
    await openDock("对话");
    expect(diagnosis).not.toBeVisible();
    expect(await screen.findByTestId("assistant-chat-panel")).toBeVisible();
    await openDock("综合诊断");
    expect(diagnosis).toBeVisible();
    expect(screen.getByTestId("diagnosis-window")).toBe(diagnosis);
    expect(screen.getAllByTestId("diagnosis-window")).toHaveLength(1);
  });

  it("keeps at most one V2 business window visible across repeated Dock switches", async () => {
    renderPage();
    await openDock("综合诊断");
    await screen.findByTestId("diagnosis-window");
    await openDock("训练计划");
    await screen.findByTestId("training-window");
    await openDock("对话");
    expect(screen.getByTestId("training-window")).not.toBeVisible();
    expect(screen.getByTestId("diagnosis-window")).not.toBeVisible();
    expect(screen.getByTestId("assistant-chat-panel")).toBeVisible();
    expect(screen.getByRole("button", { name: "对话" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByRole("button", { name: "训练计划" })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
    expect(
      screen.queryByRole("button", { name: /恢复综合诊断|恢复训练计划/ }),
    ).not.toBeInTheDocument();
  });

  it("opens the Person window with five categories and a compact identity popover", async () => {
    renderPage();
    await openDock("个人档案");
    const person = await screen.findByTestId("person-window");
    for (const label of ["身体记录", "装备", "影像资料", "医学报告", "习性"]) {
      expect(screen.getByRole("button", { name: label })).toBeInTheDocument();
    }
    // The tab label is the only place the category name appears.
    expect(screen.queryAllByRole("heading", { name: "身体记录" })).toHaveLength(
      0,
    );
    expect(person.querySelector(".bc-person-popover")).toBeNull();
    await userEvent.click(
      screen.getByRole("button", { name: "synthetic@example.test" }),
    );
    const popover = await screen.findByRole("dialog", { name: "基础资料" });
    expect(popover.textContent).toContain("BMI —");
    expect(
      screen.getByRole("button", { name: "编辑基础资料" }),
    ).toBeInTheDocument();
  });

  it("shows a centred equipment empty state with a single add action", async () => {
    renderPage();
    await openDock("个人档案");
    await screen.findByTestId("person-window");
    await userEvent.click(screen.getByRole("button", { name: "装备" }));
    expect(
      await screen.findByText("添加个人器材，帮助 AI 更好规划训练项目"),
    ).toBeInTheDocument();
    expect(mockListEquipment).toHaveBeenCalled();
    expect(
      screen.getByRole("button", { name: /添加个人器材/ }),
    ).toBeInTheDocument();
    // No fixed slot ceiling is displayed.
    expect(screen.queryByText(/\/\s*12/)).not.toBeInTheDocument();
  });

  it("lists EquipmentAsset items and marks one unavailable without touching BodyState", async () => {
    const asset = {
      id: "eq-1",
      user_id: "11111111-1111-4111-8111-111111111111",
      display_name: "弹力带",
      notes: "中等阻力",
      access_kind: "owned",
      availability: "available",
      valid_from: null,
      valid_until: null,
      revision: 2,
      created_at: "2026-10-15T08:00:00Z",
      updated_at: "2026-10-15T08:00:00Z",
    };
    mockListEquipment.mockResolvedValue({ items: [asset] });
    mockUpdateEquipment.mockResolvedValue({
      ...asset,
      availability: "unavailable",
      revision: 3,
    });
    renderPage();
    await openDock("个人档案");
    await screen.findByTestId("person-window");
    await userEvent.click(screen.getByRole("button", { name: "装备" }));
    expect(await screen.findByText("弹力带")).toBeInTheDocument();
    expect(screen.getByText("中等阻力")).toBeInTheDocument();
    expect(screen.getByText("当前可用")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "标记暂不可用" }));
    await waitFor(() =>
      expect(mockUpdateEquipment).toHaveBeenCalledWith("eq-1", {
        expected_revision: 2,
        display_name: "弹力带",
        availability: "unavailable",
      }),
    );
  });

  it("renders real habit values and marks unrecorded ones as missing", async () => {
    renderPage();
    await openDock("个人档案");
    await screen.findByTestId("person-window");
    await userEvent.click(screen.getByRole("button", { name: "习性" }));
    await waitFor(() =>
      expect(screen.getByText("每晚 7 小时")).toBeInTheDocument(),
    );
    expect(screen.getByText("发生时间：未说明")).toBeVisible();
    expect(mockListBodyFacts).toHaveBeenCalled();
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

  it("never mounts a Trainer execution from a deep link without explicit Start", async () => {
    renderPage("/consultation/conv-1?view=trainer");
    expect(await screen.findByTestId("trainer-entry-guard")).toBeVisible();
    expect(
      screen.queryByTestId("training-execution-panel"),
    ).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /查看训练计划/ }));
    expect(await screen.findByTestId("training-window")).toBeVisible();
    expect(screen.getByTestId("trainer-window")).not.toBeVisible();
  });

  it("maps Person and Trainer shortcut keys without treating them as a data-write command", async () => {
    renderPage();
    await userEvent.keyboard("i");
    expect(await screen.findByTestId("person-window")).toBeVisible();
    await userEvent.keyboard("{Escape}");
    await userEvent.keyboard("t");
    expect(await screen.findByTestId("trainer-entry-guard")).toBeVisible();
    expect(
      screen.queryByTestId("training-execution-panel"),
    ).not.toBeInTheDocument();
  });

  it("resolves legacy ?view= aliases to the same business object", async () => {
    // `state` and `treatment` were the pre-window panel names; existing links
    // must not silently fall back to the bare body stage.
    renderPage("/consultation/conv-1?view=state");
    expect(await screen.findByTestId("person-window")).toBeInTheDocument();
    expect(screen.queryByTestId("training-window")).not.toBeInTheDocument();
  });

  it("surfaces the Chat window when a question is asked from a canvas surface", async () => {
    const user = userEvent.setup();
    renderPage();
    // Ask from the region card: the canvas sets spatial context and must also
    // open Chat, otherwise the question has nowhere to go.
    await user.click(screen.getByRole("button", { name: "开始一次提问" }));
    expect(await screen.findByTestId("assistant-chat-panel")).toBeVisible();
    expect(screen.getByTestId("assistant-chat-panel")).toBeInTheDocument();
  });
});

it("retries failed native body reads from the canvas sync banner", async () => {
  mockReadBodyRecords.mockRejectedValueOnce(new Error("unavailable"));
  mockUseHealthWorkspaceQuery.mockReturnValue({ data: makeWorkspace(), isFetching: false, isError: false, refetch: vi.fn() });
  renderPage();
  const alert = await screen.findByRole("alert");
  expect(alert).toHaveTextContent("身体信息暂时无法同步");
  await userEvent.click(within(alert).getByRole("button", { name: "重试" }));
  await waitFor(() => expect(mockReadBodyRecords).toHaveBeenCalledTimes(2));
  await waitFor(() => expect(screen.queryByRole("alert")).not.toBeInTheDocument());
});
