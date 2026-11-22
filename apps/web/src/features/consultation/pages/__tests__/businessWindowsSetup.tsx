/** Shared mocks, factories and per-test setup for the BusinessWindows suites.
 * Importing this module registers the service mocks and the common
 * beforeEach baseline for the importing test file. */
import { bodyRecordsFixture } from "@/features/profile/services/__fixtures__/bodyRecordsFixture";
import { beforeEach, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useAuthStore } from "@/stores/authStore";
import { useUploadStore } from "@/stores/uploadStore";

vi.mock("@/lib/clientDiagnostics", () => ({
  createClientDiagnosticId: () => "consultation-test-session",
  reportClientDiagnostic: vi.fn(),
}));

export const mockUseConversationQuery = vi.fn();
export const mockUseConsultationThreadQuery = vi.fn();
export const mockUseHealthWorkspaceQuery = vi.fn();
export const mockListTrainingPlans = vi.fn();
export const mockGetBodyMetrics = vi.fn();
export const mockGetLifestyle = vi.fn();
export const mockListBodyFacts = vi.fn();
export const mockReadBodyRecords = vi.fn();
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
export const mockNativeDiagnosis = vi.fn();
export const mockLatestDiagnosisRun = vi.fn();
export const mockDiagnosisRun = vi.fn();
vi.mock("../../services/nativeDiagnosisService", () => ({
  nativeDiagnosisService: {
    current: () => mockNativeDiagnosis(),
    latestRun: () => mockLatestDiagnosisRun(),
    run: (runId: string) => mockDiagnosisRun(runId),
    decide: vi.fn(),
    settle: vi.fn(),
  },
}));
export const mockRequestErasure = vi.fn();
vi.mock("@/features/profile/services/assetErasureService", () => ({
  assetErasureService: {
    request: (id: string, key: string) => mockRequestErasure(id, key),
    receipt: vi.fn(),
  },
}));
export const mockListEquipment = vi.fn();
export const mockUpdateEquipment = vi.fn();
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
export const mockListPlanProposals = vi.fn();
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

import { makeDiagnosis } from "@/features/body-canvas/model/__fixtures__/nativeDiagnosisFixtures";
import { ConsultationPage } from "../ConsultationPage";
import type {
  ConsultationThread,
  Conversation,
} from "../../types/consultation";
import type { HealthWorkspace } from "@/features/workspace/types/workspace";

export function makeConversation(
  overrides: Partial<Conversation> = {},
): Conversation {
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

export function makeThread(
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

export function makeWorkspace(
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

export function renderPage(path = "/consultation/conv-1") {
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

export function openDock(name: string) {
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
export async function chooseCategory(label: string) {
  await userEvent.click(screen.getByRole("button", { name: /^档案分类/ }));
  const list = await screen.findByRole("listbox", { name: "档案分类" });
  await userEvent.click(
    within(list).getByRole("option", { name: new RegExp(`^${label}`) }),
  );
}
