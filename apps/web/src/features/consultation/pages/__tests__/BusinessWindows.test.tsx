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

const mockUseConversationQuery = vi.fn();
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
  bodyEpisodeService: {
    list: async () => ({
      user_id: "11111111-1111-4111-8111-111111111111",
      current_revision: 1,
      episodes: [],
      has_more: false,
    }),
  },
}));
const mockNativeDiagnosis = vi.fn();
const mockLatestDiagnosisRun = vi.fn();
const mockDiagnosisRun = vi.fn();
vi.mock("../../services/nativeDiagnosisService", () => ({
  nativeDiagnosisService: {
    current: () => mockNativeDiagnosis(),
    latestRun: () => mockLatestDiagnosisRun(),
    run: (runId: string) => mockDiagnosisRun(runId),
    decide: vi.fn(),
    settle: vi.fn(),
  },
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

vi.mock("../../hooks/useConversationQuery", () => ({
  useConversationQuery: () => mockUseConversationQuery(),
}));
vi.mock("../../hooks/useConsultationThreadQuery", () => ({
  useConsultationThreadQuery: (conversationId: string | null) =>
    mockUseConsultationThreadQuery(conversationId),
}));
vi.mock("@/features/workspace/hooks/useHealthWorkspaceQuery", () => ({
  useHealthWorkspaceQuery: () => mockUseHealthWorkspaceQuery(),
}));
const mockListPlanProposals = vi.fn();
vi.mock("@/features/training/services/trainingService", () => ({
  trainingApi: {
    listPlanRecords: () => mockListTrainingPlans(),
    listProposals: () => mockListPlanProposals(),
    acceptProposal: vi.fn(),
    rejectProposal: vi.fn(),
  },
  newPlanDecisionIdentity: () => ({ decisionId: "d", idempotencyKey: "k" }),
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

import { makePlan } from "@/features/body-canvas/model/__fixtures__/trainingPlanFixtures";
import {
  RUN_ID,
  makeCondition,
  makeDiagnosis,
  makeRun,
} from "@/features/body-canvas/model/__fixtures__/nativeDiagnosisFixtures";
import { ConsultationPage } from "../ConsultationPage";
import type {
  ConsultationThread,
  Conversation,
} from "../../types/consultation";
import type { HealthWorkspace } from "@/features/workspace/types/workspace";

function makeConversation(overrides: Partial<Conversation> = {}): Conversation {
  return {
    id: "conv-1",
    status: "active",
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
    recent_outcomes: [],
    trends: [],
    capabilities: {
      can_continue_consultation: true,
      can_edit_body_state: true,
      can_request_diagnosis: true,
      can_review_diagnosis: true,
      can_generate_treatment: true,
      can_execute_treatment: true,
      can_record_outcome: true,
      requires_safety_review: false,
      requires_diagnosis_review: false,
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
  mockUseConversationQuery.mockReset();
  mockUseConsultationThreadQuery.mockReset();
  mockUseHealthWorkspaceQuery.mockReset();
  mockListTrainingPlans.mockReset();
  mockGetBodyMetrics.mockReset();
  mockGetLifestyle.mockReset();
  mockListEquipment.mockReset();
  mockListEquipment.mockResolvedValue({ items: [] });
  mockNativeDiagnosis.mockReset();
  mockNativeDiagnosis.mockResolvedValue(makeDiagnosis());
  mockLatestDiagnosisRun.mockReset();
  mockLatestDiagnosisRun.mockResolvedValue(null);
  mockDiagnosisRun.mockReset();
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
  mockUseConversationQuery.mockReturnValue({
    data: { conversationId: "conv-1" },
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
  mockListPlanProposals.mockReset();
  mockListPlanProposals.mockResolvedValue([]);
  mockGetBodyMetrics.mockResolvedValue({ current_revision: 1 });
  mockGetLifestyle.mockResolvedValue({
    current_revision: 1,
    activity: { kind: "activity", summary: "", details: {} },
    sleep: { kind: "sleep", summary: "每晚 7 小时", details: {} },
    exercise: { kind: "exercise", summary: "", details: {} },
    nutrition: { kind: "nutrition", summary: "", details: {} },
    substances: { kind: "substances", summary: "", details: {} },
    recovery: { kind: "recovery", summary: "", details: {} },
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
    // Penpot Diagnosis / Empty: title only until a condition is adopted.
    expect(
      screen.queryByRole("combobox", { name: "选择已确认病情" }),
    ).toBeNull();
    expect(await screen.findByTestId("diagnosis-empty")).toBeVisible();
    expect(
      screen.getByRole("button", { name: "关闭综合诊断" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "综合诊断", level: 2 }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/修订|版本号|revision/i)).not.toBeInTheDocument();
  });

  it("never shows an unconfirmed proposal as a confirmed condition (DX-06)", async () => {
    mockLatestDiagnosisRun.mockResolvedValue({
      runId: RUN_ID,
      trigger: "new_analysis",
      basedOnBodyStateRevision: 5,
      basedOnDiagnosisRevision: 2,
      startedAt: "2026-10-18T03:00:00Z",
      state: "open",
    });
    mockDiagnosisRun.mockResolvedValue(makeRun());
    renderPage();
    await openDock("综合诊断");
    await screen.findByTestId("diagnosis-window");
    expect(await screen.findByTestId("diagnosis-proposals")).toBeVisible();
    // Penpot Confirm first: one proposal at a time, never a condition heading.
    expect(screen.getAllByText("待你决定")).toHaveLength(1);
    expect(screen.queryByRole("heading", { name: "上交叉综合征" })).toBeNull();
    expect(screen.getByTestId("diagnosis-mode")).toHaveTextContent("正式分析");
    expect(
      screen.queryByRole("combobox", { name: "选择已确认病情" }),
    ).toBeNull();
  });

  it("lists an adopted condition with cause, handling boundary and certainty", async () => {
    mockNativeDiagnosis.mockResolvedValue(
      makeDiagnosis({ conditionEntries: [makeCondition()] }),
    );
    renderPage();
    await openDock("综合诊断");
    await screen.findByTestId("diagnosis-window");
    expect(
      await screen.findByRole("button", { name: "综合诊断，1 项已确认病情" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("combobox", { name: "选择已确认病情" }),
    ).toHaveValue(makeCondition().conditionId);
    expect(screen.getByRole("heading", { name: "肩峰下撞击" })).toBeVisible();
    expect(screen.getByText("肩胛控制不足")).toBeInTheDocument();
    expect(screen.getByText("你已确认的分析结果")).toBeInTheDocument();
    expect(screen.getByText("是否做过影像检查")).toBeInTheDocument();
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

  it("renders the user's V2 plan records with real stages and every plan in the picker", async () => {
    mockListTrainingPlans.mockResolvedValue([
      makePlan({ planId: "plan-1", title: "肩胛稳定训练", status: "paused" }),
      makePlan({ planId: "plan-2", title: "臀腿力量训练" }),
    ]);
    renderPage();
    await openDock("训练计划");
    const trainingWindow = await screen.findByTestId("training-window");

    // The active plan wins over the first one; content is the real revision.
    await waitFor(() => expect(within(trainingWindow).getByText("弹力带侧走")).toBeInTheDocument());
    expect(within(trainingWindow).getByText("热身")).toBeInTheDocument();
    expect(within(trainingWindow).getByText("主训练")).toBeInTheDocument();
    expect(within(trainingWindow).getAllByText("3 组 × 12 次").length).toBeGreaterThan(0);
    const selector = screen.getByRole("combobox", { name: "选择训练计划" });
    expect(selector).toHaveValue("plan-2");
    expect(screen.getByRole("option", { name: "肩胛稳定训练" })).toBeInTheDocument();
    await userEvent.selectOptions(selector, "plan-1");
    await waitFor(() =>
      expect(localStorage.getItem("bodysense.training.selectedPlanId")).toBe("plan-1"),
    );
    expect(within(trainingWindow).getByTestId("training-gate")).toHaveTextContent("计划已暂停");
    expect(within(trainingWindow).getByRole("button", { name: "开始训练" })).toBeDisabled();
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
  mockUseHealthWorkspaceQuery.mockReturnValue({
    data: makeWorkspace(),
    isFetching: false,
    isError: false,
    refetch: vi.fn(),
  });
  renderPage();
  const alert = await screen.findByRole("alert");
  expect(alert).toHaveTextContent("身体信息暂时无法同步");
  await userEvent.click(within(alert).getByRole("button", { name: "重试" }));
  await waitFor(() => expect(mockReadBodyRecords).toHaveBeenCalledTimes(2));
  await waitFor(() =>
    expect(screen.queryByRole("alert")).not.toBeInTheDocument(),
  );
});
