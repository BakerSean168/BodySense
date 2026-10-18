import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useAuthStore } from "@/stores/authStore";
import { PersonRecordsView } from "./PersonRecordsView";

const owner = "11111111-1111-4111-8111-111111111111";
const factId = "22222222-2222-4222-8222-222222222222";
const episodeId = "33333333-3333-4333-8333-333333333333";
const hypothesisId = "44444444-4444-4444-8444-444444444444";
const groupId = "55555555-5555-4555-8555-555555555555";

const fact = {
  id: factId,
  user_id: owner,
  kind: "symptom",
  subject: "self",
  statement: "昨天有头痛。",
  body_region_ids: [],
  temporal: { precision: "unknown", original_expression: "昨天" },
  details: { symptom_type: "头痛", presence: "present" },
  source_evidence_id: factId,
  assertion_status: "asserted",
  created_revision: 1,
  recorded_at: "2026-10-14T01:12:00Z",
};
const episode = {
  id: episodeId,
  user_id: owner,
  symptom_type: "头痛",
  body_region_ids: [],
  laterality: "unknown",
  identity_status: "current",
  created_revision: 2,
  created_at: "2026-10-14T01:13:00Z",
  links: [
    {
      fact_id: factId,
      role: "observation",
      association_method: "explicit",
      is_primary: true,
      linked_revision: 2,
    },
  ],
  projection: {
    state: "unknown",
    reason: "insufficient_evidence",
    basis_fact_ids: [factId],
    based_on_revision: 2,
  },
};
const records = {
  user_id: owner,
  current_revision: 3,
  facts: [fact],
  observations: [],
  episodes: [episode],
  hypotheses: [
    {
      id: hypothesisId,
      user_id: owner,
      lineage_id: hypothesisId,
      statement: "我猜和睡得晚有关。",
      related_fact_ids: [factId],
      related_observation_ids: [],
      recorded_at: "2026-10-14T01:14:00Z",
    },
  ],
};
const groups = {
  user_id: owner,
  current_revision: 3,
  groups: [
    {
      id: groupId,
      display_title: "我猜和睡得晚有关。",
      related_hypothesis_ids: [hypothesisId],
      related_fact_ids: [factId],
      related_observation_ids: [],
      related_episode_ids: [episodeId],
    },
  ],
};

const api = vi.hoisted(() => ({
  list: vi.fn(),
  records: vi.fn(),
  groups: vi.fn(),
}));
vi.mock("@/features/profile/services/bodyFactRecordsService", () => ({
  bodyFactRecordsService: { list: api.list },
}));
vi.mock("@/features/profile/services/bodyRecordsService", () => ({
  readBodyRecords: api.records,
}));
vi.mock("@/features/profile/services/bodyHypothesisService", () => ({
  bodyHypothesisService: { groups: api.groups },
}));
// The Episode editor has its own reads; this suite covers navigation only.
vi.mock("./BodyEpisodeAssociations", () => ({
  BodyEpisodeEditor: () => <div data-testid="episode-editor-stub" />,
}));

function mount() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <PersonRecordsView canEdit now={new Date("2026-10-14T04:00:00Z")} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  api.list.mockResolvedValue({
    user_id: owner,
    current_revision: 3,
    facts: [fact],
    has_more: false,
  });
  api.records.mockResolvedValue(records);
  api.groups.mockResolvedValue(groups);
  useAuthStore.setState({
    user: { id: owner, email: "synthetic@example.test" },
    isAuthenticated: true,
  });
});

describe("Person Records navigation (Penpot Records / Body Episode)", () => {
  it("offers related Episodes, hypotheses and observations from one snapshot", async () => {
    mount();
    const related = await screen.findByRole("navigation", { name: "相关记录" });
    expect(
      within(related).getByRole("button", { name: "经历 1 段" }),
    ).toBeVisible();
    expect(
      within(related).getByRole("button", { name: "我的猜测 1 条" }),
    ).toBeVisible();
    expect(
      within(related).getByRole("button", { name: "观察 0 条" }),
    ).toBeVisible();
  });

  it("opens a statement, follows its Episode and shows gaps and derived groups", async () => {
    mount();
    await userEvent.click(
      await screen.findByRole("button", { name: "查看记录：昨天有头痛。" }),
    );
    const detail = screen.getByTestId("body-fact-detail");
    expect(
      within(detail).getByRole("heading", { name: "身体感受" }),
    ).toBeVisible();
    await userEvent.click(
      within(detail).getByRole("button", { name: "头痛经历" }),
    );
    const scene = await screen.findByTestId("person-episode");
    expect(
      within(scene).getByRole("heading", { name: "头痛经历" }),
    ).toBeVisible();
    expect(within(scene).getByText("是否持续 · 未知")).toHaveAttribute(
      "title",
      "持续状态尚未确认",
    );
    expect(within(scene).getByText("1 条陈述")).toBeVisible();
    expect(within(scene).getByText("“昨天有头痛。”")).toBeVisible();
    const gaps = within(scene).getByRole("region", { name: "信息缺口" });
    expect(gaps).toHaveTextContent("严重程度");
    expect(gaps).toHaveTextContent("现在是否还在持续");
    expect(
      await within(scene).findByRole("button", { name: "我猜和睡得晚有关。" }),
    ).toBeVisible();
    await userEvent.click(
      within(scene).getByRole("button", { name: "补充说明" }),
    );
    expect(
      await screen.findByTestId("episode-editor-stub"),
    ).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "返回记录" }));
    expect(await screen.findByText("已加载 1 条")).toBeVisible();
  });

  it("does not show groups from a different revision than the records snapshot", async () => {
    api.groups.mockResolvedValue({ ...groups, current_revision: 2 });
    mount();
    await userEvent.click(
      await screen.findByRole("button", { name: /经历 1 段/ }),
    );
    await userEvent.click(
      await screen.findByRole("button", { name: /头痛经历/ }),
    );
    const scene = await screen.findByTestId("person-episode");
    await screen.findByText("“昨天有头痛。”");
    expect(within(scene).queryByRole("region", { name: "相关" })).toBeNull();
    expect(
      within(scene).getByRole("button", { name: "返回经历" }),
    ).toBeVisible();
  });
});
