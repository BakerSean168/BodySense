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
const mockRequestErasure = vi.fn();
vi.mock("@/features/profile/services/assetErasureService", () => ({
  assetErasureService: {
    request: (id: string, key: string) => mockRequestErasure(id, key),
    receipt: vi.fn(),
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
  ProfileDrawer: ({ open }: { open: boolean }) => (
    <div data-testid="profile-drawer" data-open={open ? "true" : "false"} />
  ),
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
      can_generate_training_plan: true,
      can_start_training: true,
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
  mockRequestErasure.mockReset();
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

/** Penpot category menu: the header trigger opens an ARIA listbox. */
async function chooseCategory(label: string) {
  await userEvent.click(screen.getByRole("button", { name: /^档案分类/ }));
  const list = await screen.findByRole("listbox", { name: "档案分类" });
  await userEvent.click(
    within(list).getByRole("option", { name: new RegExp(`^${label}`) }),
  );
}

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
    await waitFor(() =>
      expect(
        within(trainingWindow).getByText("弹力带侧走"),
      ).toBeInTheDocument(),
    );
    expect(within(trainingWindow).getByText("热身")).toBeInTheDocument();
    expect(within(trainingWindow).getByText("主训练")).toBeInTheDocument();
    expect(
      within(trainingWindow).getAllByText("3 组 × 12 次").length,
    ).toBeGreaterThan(0);
    const selector = screen.getByRole("combobox", { name: "选择训练计划" });
    expect(selector).toHaveValue("plan-2");
    expect(
      screen.getByRole("option", { name: "肩胛稳定训练" }),
    ).toBeInTheDocument();
    await userEvent.selectOptions(selector, "plan-1");
    await waitFor(() =>
      expect(localStorage.getItem("bodysense.training.selectedPlanId")).toBe(
        "plan-1",
      ),
    );
    expect(
      within(trainingWindow).getByTestId("training-gate"),
    ).toHaveTextContent("计划已暂停");
    expect(
      within(trainingWindow).getByRole("button", { name: "开始训练" }),
    ).toBeDisabled();
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

  it("opens the Person window with five categories and the identity popover", async () => {
    renderPage();
    await openDock("个人档案");
    const person = await screen.findByTestId("person-window");
    // Penpot Person header: "nickname / <category picker>" replaces the tabs.
    const picker = screen.getByRole("button", { name: "档案分类 身体记录" });
    expect(picker).toHaveAttribute("aria-haspopup", "listbox");
    expect(picker).toHaveAttribute("aria-expanded", "false");
    expect(picker).toHaveAttribute("data-value", "records");
    // The picker label is the only place the category name appears.
    expect(screen.queryAllByRole("heading", { name: "身体记录" })).toHaveLength(
      0,
    );
    expect(person.querySelector(".bc-person-popover")).toBeNull();
    await userEvent.click(
      screen.getByRole("button", { name: "synthetic@example.test" }),
    );
    // Penpot account menu: 个人资料 opens the identity popover.
    await userEvent.click(
      await screen.findByRole("menuitem", { name: "个人资料" }),
    );
    const popover = await screen.findByRole("dialog", { name: "个人资料" });
    // Minimal optional Profile only: no body measurements and no revision.
    expect(popover.textContent).not.toContain("BMI");
    expect(popover.textContent).toContain("资料缺失不会由 AI 猜测补全");
    expect(screen.getByLabelText("昵称")).toHaveValue("");
    expect(screen.getByLabelText("自报性别")).toHaveValue("");
    expect(screen.getByLabelText("显示时区")).toHaveDisplayValue(/^跟随设备/);
    expect(screen.getByRole("button", { name: "保存" })).toBeDisabled();
    // Files, source removal and erasure stay reachable in the 身体档案 drawer,
    // which loads only once it is first opened.
    expect(screen.queryByTestId("profile-drawer")).toBeNull();
    await userEvent.click(
      within(popover).getByRole("button", { name: "身体档案与文件" }),
    );
    expect(screen.queryByRole("dialog", { name: "个人资料" })).toBeNull();
    expect(await screen.findByTestId("profile-drawer")).toHaveAttribute(
      "data-open",
      "true",
    );
  });

  it("switches Person categories from the Penpot icon menu as an accessible listbox", async () => {
    const user = userEvent.setup();
    useUploadStore.setState({
      uploads: [
        {
          id: "r1",
          file_type: "report",
          original_name: "体检.pdf",
          file_size: 1000,
          mime_type: "application/pdf",
          ocr_status: "pending",
          ocr_result: null,
          analysis_status: "none",
          source_availability: "available",
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        },
      ],
      hasFetched: true,
    } as never);
    renderPage();
    await openDock("个人档案");
    const person = await screen.findByTestId("person-window");
    const trigger = screen.getByRole("button", { name: "档案分类 身体记录" });
    trigger.focus();
    await user.keyboard("{ArrowDown}");
    const list = await screen.findByRole("listbox", { name: "档案分类" });
    expect(trigger).toHaveAttribute("aria-expanded", "true");
    expect(trigger).toHaveAttribute("aria-controls", list.id);
    expect(list).toHaveFocus();
    const options = within(list).getAllByRole("option");
    expect(
      options.map(
        (o) => o.querySelector(".bc-category-option-label")?.textContent,
      ),
    ).toEqual(["身体记录", "器材", "媒体", "报告", "生活习惯"]);
    expect(options[0]).toHaveAttribute("aria-selected", "true");
    expect(list).toHaveAttribute("aria-activedescendant", options[0].id);
    // Counts only for data already read: uploads yes, records never guessed.
    expect(options[2]).toHaveTextContent("0 个");
    expect(options[3]).toHaveTextContent("1 份");
    expect(options[0].querySelector(".bc-category-option-count")).toBeNull();
    await user.keyboard("{ArrowDown}{ArrowDown}");
    expect(list).toHaveAttribute("aria-activedescendant", options[2].id);
    await user.keyboard("{End}");
    expect(list).toHaveAttribute("aria-activedescendant", options[4].id);
    await user.keyboard("{Home}{ArrowDown}{ArrowDown}{ArrowDown}");
    // Escape closes without changing the category or closing the window.
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(trigger).toHaveFocus();
    expect(trigger).toHaveAttribute("data-value", "records");
    expect(person).toBeInTheDocument();
    // Enter on the active option chooses it and returns focus.
    await user.keyboard("{Enter}");
    await screen.findByRole("listbox", { name: "档案分类" });
    await user.keyboard("{ArrowDown}{ArrowDown}{ArrowDown}{Enter}");
    expect(screen.queryByRole("listbox")).toBeNull();
    const reports = screen.getByRole("button", { name: "档案分类 报告" });
    expect(reports).toHaveFocus();
    expect(await screen.findByTestId("person-report")).toBeInTheDocument();
    // Pointer: an outside press closes it; clicking an option chooses it.
    await user.click(reports);
    await screen.findByRole("listbox", { name: "档案分类" });
    await user.click(document.body);
    expect(screen.queryByRole("listbox")).toBeNull();
    await chooseCategory("器材");
    expect(
      screen.getByRole("button", { name: "档案分类 器材" }),
    ).toHaveAttribute("data-value", "equipment");
  });

  it("uploads a report from the Penpot Reports empty state and offers retry on failure", async () => {
    const user = userEvent.setup();
    const uploadFile = vi
      .fn()
      .mockRejectedValueOnce(new Error("network"))
      .mockResolvedValueOnce(undefined);
    useUploadStore.setState({ uploadFile });
    renderPage();
    await openDock("个人档案");
    await screen.findByTestId("person-window");
    await chooseCategory("报告");
    const add = await screen.findByRole("button", { name: "上传报告" });
    const input =
      document.querySelector<HTMLInputElement>('input[type="file"]')!;
    expect(input).toHaveAttribute("accept", "image/*,application/pdf");
    const file = new File(["%PDF"], "体检.pdf", { type: "application/pdf" });
    expect(add).toBeEnabled();
    await user.upload(input, file);
    expect(uploadFile).toHaveBeenCalledWith(file, "report");
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("体检.pdf");
    await user.click(within(alert).getByRole("button", { name: "重试" }));
    await waitFor(() => expect(uploadFile).toHaveBeenCalledTimes(2));
    expect(uploadFile).toHaveBeenLastCalledWith(file, "report");
  });

  it("shows Person Media as a Penpot thumbnail grid grouped by month", async () => {
    const at = (month: number, day: number) =>
      new Date(new Date().getFullYear(), month, day, 10).toISOString();
    const base = {
      file_type: "consultation_photo",
      file_size: 2_100_000,
      mime_type: "image/jpeg",
      ocr_status: "pending",
      ocr_result: null,
      analysis_status: "none",
      source_availability: "available",
      updated_at: at(0, 1),
    };
    const uploads = [
      { ...base, id: "m1", original_name: "侧面.jpg", created_at: at(1, 3) },
      {
        ...base,
        id: "m2",
        original_name: "深蹲.mp4",
        mime_type: "video/mp4",
        created_at: at(2, 9),
      },
      {
        ...base,
        id: "m3",
        original_name: "正面.jpg",
        analysis_status: "processing",
        created_at: at(2, 8),
      },
      {
        ...base,
        id: "m4",
        original_name: "背面.jpg",
        analysis_status: "failed",
        created_at: at(2, 7),
      },
      {
        ...base,
        id: "r1",
        file_type: "report",
        original_name: "体检.pdf",
        created_at: at(2, 6),
      },
    ];
    useUploadStore.setState({ uploads } as never);
    renderPage();
    await openDock("个人档案");
    await screen.findByTestId("person-window");
    await chooseCategory("媒体");
    expect(await screen.findByText("4 个")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "上传照片" })).toBeEnabled();
    const months = within(screen.getByTestId("person-window")).getAllByRole(
      "heading",
      { level: 3 },
    );
    expect(months.map((h) => h.textContent)).toEqual(["3月", "2月"]);
    const march = within(
      screen.getByRole("region", { name: "3月" }),
    ).getAllByTestId("person-media-tile");
    expect(march.map((tile) => tile.dataset.state)).toEqual([
      "ready",
      "processing",
      "failed",
    ]);
    expect(march[0]).toHaveAttribute("data-kind", "video");
    expect(within(march[0]).getByText("视频")).toBeInTheDocument();
    expect(within(march[1]).getByText("处理中…")).toBeInTheDocument();
    expect(within(march[2]).getByText("处理失败")).toBeInTheDocument();
    expect(within(march[0]).getByText("2.0 MB · 3月9日")).toBeInTheDocument();
    // Reports stay out of Media; the manifest has no duration, so none shows.
    expect(screen.queryByText("体检.pdf")).not.toBeInTheDocument();
    expect(screen.queryByText(/\d:\d\d/)).not.toBeInTheDocument();
  });

  it("collapses Person Media to the first rows behind 查看全部", async () => {
    const uploads = Array.from({ length: 10 }, (_, i) => ({
      id: `m${i}`,
      file_type: "consultation_photo",
      original_name: `照片${i}.jpg`,
      file_size: 1_000_000,
      mime_type: "image/jpeg",
      ocr_status: "pending",
      ocr_result: null,
      analysis_status: "none",
      source_availability: "available",
      created_at: new Date(Date.now() - i * 86_400_000).toISOString(),
      updated_at: new Date().toISOString(),
    }));
    useUploadStore.setState({ uploads } as never);
    renderPage();
    await openDock("个人档案");
    await screen.findByTestId("person-window");
    await chooseCategory("媒体");
    expect(await screen.findAllByTestId("person-media-tile")).toHaveLength(8);
    const more = screen.getByRole("button", { name: "查看全部 10 个" });
    expect(more).toHaveAttribute("aria-expanded", "false");
    await userEvent.click(more);
    expect(screen.getAllByTestId("person-media-tile")).toHaveLength(10);
    expect(screen.queryByRole("button", { name: /查看全部/ })).toBeNull();
  });

  it("removes a report original in Person after the Penpot scope and confirm steps", async () => {
    const user = userEvent.setup();
    const report = {
      id: "33333333-3333-4333-8333-333333333333",
      file_type: "report",
      original_name: "体检报告_2026-09.pdf",
      file_size: 2_400_000,
      mime_type: "application/pdf",
      ocr_status: "completed",
      ocr_result: null,
      analysis_status: "none",
      source_availability: "available",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };
    const deleteUpload = vi.fn(async (id: string) => {
      useUploadStore.setState((state) => ({
        uploads: state.uploads.map((u) =>
          u.id === id
            ? { ...u, source_availability: "source_unavailable" as const }
            : u,
        ),
      }));
    });
    useUploadStore.setState({ uploads: [report], deleteUpload } as never);
    renderPage();
    await openDock("个人档案");
    await screen.findByTestId("person-window");
    await chooseCategory("报告");
    const card = await screen.findByTestId("person-report");
    expect(card).toHaveTextContent("PDF · 2.3 MB · 今天上传");
    expect(within(card).getByText("可用")).toBeInTheDocument();
    expect(within(card).getByText("完成")).toBeInTheDocument();
    await user.click(
      within(card).getByRole("button", { name: "管理“体检报告_2026-09.pdf”" }),
    );
    await user.click(await screen.findByRole("menuitem", { name: "删除…" }));

    const scene = await screen.findByTestId("person-asset-deletion");
    expect(scene).toHaveAttribute("data-scene", "scope");
    const source = within(scene).getByRole("radio", { name: /仅移除原件/ });
    const erase = within(scene).getByRole("radio", {
      name: /文件及相关健康数据/,
    });
    expect(source).toHaveAttribute("aria-checked", "true");
    // Arrow keys move the radio selection.
    source.focus();
    await user.keyboard("{ArrowDown}");
    expect(erase).toHaveAttribute("aria-checked", "true");
    await user.keyboard("{ArrowUp}");
    expect(source).toHaveAttribute("aria-checked", "true");
    // Nothing is removed before the confirm step.
    await user.click(within(scene).getByRole("button", { name: "仅移除原件" }));
    expect(deleteUpload).not.toHaveBeenCalled();
    expect(scene).toHaveAttribute("data-scene", "confirm-source");
    await user.click(
      within(scene).getByRole("button", { name: "确认移除原件" }),
    );
    expect(deleteUpload).toHaveBeenCalledWith(report.id);
    await waitFor(() =>
      expect(scene).toHaveAttribute("data-scene", "source-removed"),
    );
    expect(
      within(scene).getByText("历史证据保留，原件不可再核验"),
    ).toBeVisible();
    expect(mockRequestErasure).not.toHaveBeenCalled();
    await user.click(within(scene).getByRole("button", { name: "返回报告" }));
    const after = await screen.findByTestId("person-report");
    expect(within(after).getByText("原件不可用")).toBeInTheDocument();
    expect(after).toHaveTextContent("PDF · 原件已移除");
  });

  it("erases a report in Person and shows done only once every target verified", async () => {
    const user = userEvent.setup();
    const report = {
      id: "44444444-4444-4444-8444-444444444444",
      file_type: "report",
      original_name: "示例报告.png",
      file_size: 300_000,
      mime_type: "image/png",
      ocr_status: "processing",
      ocr_result: null,
      analysis_status: "none",
      source_availability: "available",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };
    const names = [
      "source_objects",
      "derived_jobs",
      "posture_analyses",
      "diagnosis_records",
      "document_extractions",
      "assessment_reports",
      "runtime_checkpoints",
      "asset_manifest",
    ];
    const receipt = (status: string, busy: boolean) => ({
      request_id: "abcdef12-0000-4000-8000-000000000001",
      asset_id: report.id,
      status,
      inventory_revision: "v1",
      requested_at: new Date().toISOString(),
      ...(status === "erased" ? { erased_at: new Date().toISOString() } : {}),
      targets: names.map((target) => ({
        target,
        state: busy && target === "runtime_checkpoints" ? "failed" : "verified",
        erased_count: 1,
        outstanding_count: busy && target === "runtime_checkpoints" ? 1 : 0,
        ...(busy && target === "runtime_checkpoints"
          ? { failure_code: "runtime_thread_busy" }
          : {}),
      })),
      outstanding_targets: busy ? ["runtime_checkpoints"] : [],
      reviews: [],
    });
    mockRequestErasure
      .mockReset()
      .mockResolvedValueOnce(receipt("outstanding", true))
      .mockResolvedValueOnce(receipt("erased", false));
    const forgetUpload = vi.fn((id: string) =>
      useUploadStore.setState((state) => ({
        uploads: state.uploads.filter((u) => u.id !== id),
      })),
    );
    useUploadStore.setState({ uploads: [report], forgetUpload } as never);
    renderPage();
    await openDock("个人档案");
    await screen.findByTestId("person-window");
    await chooseCategory("报告");
    const card = await screen.findByTestId("person-report");
    expect(within(card).getByText("处理中")).toBeInTheDocument();
    await user.click(
      within(card).getByRole("button", { name: "管理“示例报告.png”" }),
    );
    await user.click(await screen.findByRole("menuitem", { name: "删除…" }));
    const scene = await screen.findByTestId("person-asset-deletion");
    await user.click(
      within(scene).getByRole("radio", { name: /文件及相关健康数据/ }),
    );
    await user.click(within(scene).getByRole("button", { name: "完整清理" }));
    expect(scene).toHaveAttribute("data-scene", "confirm-erase");
    expect(within(scene).getByText("确认后立即停用该来源")).toBeVisible();
    expect(mockRequestErasure).not.toHaveBeenCalled();
    await user.click(within(scene).getByRole("button", { name: "确认清理" }));
    await waitFor(() =>
      expect(scene).toHaveAttribute("data-phase", "outstanding"),
    );
    expect(within(scene).getByText("部分副本仍待清理")).toBeVisible();
    expect(within(scene).getByText("未完成")).toBeVisible();
    expect(
      within(scene).getByText("对话正在进行，空闲后会自动重试"),
    ).toBeVisible();
    expect(within(scene).queryByText("清理已完成")).toBeNull();
    const key = mockRequestErasure.mock.calls[0][1];
    await user.click(within(scene).getByRole("button", { name: "重试未完成" }));
    await waitFor(() => expect(scene).toHaveAttribute("data-phase", "erased"));
    // The retry continues the same erasure (same key).
    expect(mockRequestErasure).toHaveBeenLastCalledWith(report.id, key);
    expect(within(scene).getByText("全部范围已核验")).toBeVisible();
    expect(within(scene).getAllByText("已核验")).toHaveLength(3);
    expect(within(scene).getByText(/回执 ABCDEF12/)).toBeVisible();
    await user.click(within(scene).getByRole("button", { name: "返回报告" }));
    expect(forgetUpload).toHaveBeenCalledWith(report.id);
    expect(await screen.findByText("还没有报告")).toBeInTheDocument();
  });

  it("shows a centred equipment empty state with a single add action", async () => {
    renderPage();
    await openDock("个人档案");
    await screen.findByTestId("person-window");
    await chooseCategory("器材");
    // Penpot "Person / Equipment · Empty": title, one quiet line, round add.
    expect(await screen.findByText("还没有器材")).toBeInTheDocument();
    expect(
      screen.getByText("加上家里或健身房能用的器材，训练会按它来安排"),
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
    await chooseCategory("器材");
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
    await chooseCategory("生活习惯");
    await waitFor(() =>
      expect(screen.getByText("每晚 7 小时")).toBeInTheDocument(),
    );
    expect(screen.getByText(/发生：未说明 · 记录/)).toBeVisible();
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
    await userEvent.keyboard("m");
    expect(await screen.findByTestId("person-window")).toBeVisible();
    await userEvent.keyboard("{Escape}");
    await userEvent.keyboard("t");
    expect(await screen.findByTestId("trainer-entry-guard")).toBeVisible();
    expect(
      screen.queryByTestId("training-execution-panel"),
    ).not.toBeInTheDocument();
  });

  it("resolves legacy ?view= aliases to the same business object", async () => {
    // `state` was a pre-window panel name; existing links must not silently
    // fall back to the bare body stage.
    renderPage("/consultation/conv-1?view=state");
    expect(await screen.findByTestId("person-window")).toBeInTheDocument();
    expect(screen.queryByTestId("training-window")).not.toBeInTheDocument();
  });

  it("surfaces the Chat window when a question is asked from a canvas surface", async () => {
    const user = userEvent.setup();
    renderPage();
    // Ask from the region card: the canvas sets spatial context and must also
    // open Chat, otherwise the question has nowhere to go.
    // Penpot Home has no "开始一次提问" button: "/" asks from the canvas.
    expect(screen.queryByRole("button", { name: "开始一次提问" })).toBeNull();
    await user.keyboard("/");
    expect(await screen.findByTestId("assistant-chat-panel")).toBeVisible();
    expect(screen.getByTestId("assistant-chat-panel")).toBeInTheDocument();
  });

  it("rehomes theme and logout into the Person header account menu", async () => {
    const user = userEvent.setup();
    renderPage();
    // No app bar: no top-level user menu or theme toggle on Home.
    expect(screen.queryByRole("button", { name: "打开用户菜单" })).toBeNull();
    expect(screen.queryByRole("button", { name: "切换深色" })).toBeNull();
    await openDock("个人档案");
    await screen.findByTestId("person-window");
    await user.click(
      screen.getByRole("button", { name: "synthetic@example.test" }),
    );
    await user.click(await screen.findByRole("menuitem", { name: "设置" }));
    const settings = await screen.findByRole("dialog", { name: "设置" });
    const light = within(settings).getByRole("radio", { name: "浅色" });
    expect(light).toHaveAttribute("aria-checked", "true");
    await user.click(within(settings).getByRole("radio", { name: "深色" }));
    expect(screen.getByTestId("body-canvas-workspace")).toHaveClass("bc-dark");
    // Arrow keys move the radio selection (roving tabindex).
    within(settings).getByRole("radio", { name: "深色" }).focus();
    await user.keyboard("{ArrowRight}");
    expect(
      within(settings).getByRole("radio", { name: "跟随系统" }),
    ).toHaveAttribute("aria-checked", "true");
    await user.click(within(settings).getByRole("radio", { name: "浅色" }));
    expect(screen.getByTestId("body-canvas-workspace")).not.toHaveClass(
      "bc-dark",
    );
    await user.click(within(settings).getByRole("button", { name: "关闭" }));
    expect(screen.queryByRole("dialog", { name: "设置" })).toBeNull();

    await user.click(
      screen.getByRole("button", { name: "synthetic@example.test" }),
    );
    await user.click(await screen.findByRole("menuitem", { name: "退出登录" }));
    const confirm = await screen.findByRole("dialog", { name: "退出登录" });
    expect(confirm).toHaveTextContent("确定要退出当前账号吗？");
    await user.click(within(confirm).getByRole("button", { name: "取消" }));
    expect(screen.queryByRole("dialog", { name: "退出登录" })).toBeNull();
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
