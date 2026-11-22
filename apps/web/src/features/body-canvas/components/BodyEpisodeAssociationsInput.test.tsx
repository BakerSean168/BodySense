import { beforeEach, describe, expect, it, vi } from "vitest";

import { render, screen, waitFor } from "@testing-library/react";

import userEvent from "@testing-library/user-event";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import { useAuthStore } from "@/stores/authStore";

import { BodyEpisodeAssociations } from "./BodyEpisodeAssociations";

import { selectOption } from "@/test/selectOption";

const api = vi.hoisted(() => ({
  facts: { list: vi.fn(), record: vi.fn() },
  episode: { list: vi.fn(), create: vi.fn(), linkFact: vi.fn() },
}));

vi.mock("@/features/profile/services/bodyFactRecordsService", () => ({
  bodyFactRecordsService: api.facts,
}));

vi.mock("@/features/profile/services/bodyEpisodeService", () => ({
  bodyEpisodeService: api.episode,
}));

const owner = "11111111-1111-4111-8111-111111111111";

const positiveID = "22222222-2222-4222-8222-222222222222";

const negativeID = "33333333-3333-4333-8333-333333333333";

const episodeID = "44444444-4444-4444-8444-444444444444";

const fact = (id = positiveID, presence = "present") => ({
  id,
  user_id: owner,
  kind: "symptom",
  subject: "self",
  assertion_status: "asserted",
  statement: presence === "present" ? "我右肩痛" : "我现在没有肩痛",
  body_region_ids: [],
  temporal: { precision: "unknown" },
  details: { symptom_type: "肩痛", presence },
  source_evidence_id: id,
  created_revision: 3,
  recorded_at: "2026-10-13T01:00:00Z",
});

const factPage = (revision = 3) => ({
  user_id: owner,
  current_revision: revision,
  facts: [fact(), fact(negativeID, "absent")],
  has_more: false,
});

const episode = () => ({
  id: episodeID,
  user_id: owner,
  symptom_type: "肩痛",
  body_region_ids: [],
  laterality: "unknown",
  identity_status: "current",
  created_revision: 3,
  created_at: "2026-10-13T01:00:00Z",
  links: [],
  projection: {
    state: "unknown",
    reason: "insufficient_evidence",
    basis_fact_ids: [],
    based_on_revision: 3,
  },
});

const episodePage = (
  revision = 3,
  episodes: ReturnType<typeof episode>[] = [],
) => ({
  user_id: owner,
  current_revision: revision,
  episodes,
  has_more: false,
});

function mount(canEdit = true) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return {
    client,
    ...render(
      <QueryClientProvider client={client}>
        <BodyEpisodeAssociations canEdit={canEdit} />
      </QueryClientProvider>,
    ),
  };
}

async function openEditor() {
  await userEvent.click(screen.getByRole("button", { name: "查看与关联经历" }));
  await screen.findByTestId("body-episode-editor");
  await userEvent.click(screen.getByRole("combobox", { name: "本人症状陈述" }));
  await screen.findByRole("option", { name: "我右肩痛" });
  await userEvent.keyboard("{Escape}");
  await waitFor(() => expect(screen.queryByRole("listbox")).toBeNull());
}

beforeEach(() => {
  api.facts.list.mockReset();
  api.facts.record.mockReset();
  api.episode.list.mockReset();
  api.episode.create.mockReset();
  api.episode.linkFact.mockReset();
  api.facts.list.mockResolvedValue(factPage());
  api.episode.list.mockResolvedValue(episodePage());
  useAuthStore.setState({
    user: { id: owner, email: "test@example.test" },
    isAuthenticated: true,
  });
});

describe("Person symptom input to Episode source", () => {
  beforeEach(() => {
    api.facts.list.mockResolvedValue(factPage());
    api.episode.list.mockResolvedValue(episodePage());
  });

  it("saves a direct explicitly typed positive symptom without auto-creating an Episode", async () => {
    const newID = "88888888-8888-4888-8888-888888888888";
    api.facts.record.mockResolvedValue({
      id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
      user_id: owner,
      revision: 4,
      revision_id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
      event_id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
      fact_ids: [newID],
      committed_at: "2026-10-13T01:00:00Z",
    });
    mount();
    await openEditor();
    await userEvent.click(
      screen.getByRole("button", { name: "补充可关联的本人症状" }),
    );
    await userEvent.type(
      screen.getByLabelText("本人完整症状陈述"),
      "我昨晚右肩疼痛",
    );
    await userEvent.type(screen.getByLabelText("症状类型"), "右肩疼痛");
    await selectOption(screen.getByRole("combobox", { name: "侧别" }), "right");
    await userEvent.click(
      screen.getByRole("button", { name: "先保存症状陈述" }),
    );
    expect(api.facts.record).toHaveBeenCalledWith(
      expect.objectContaining({
        idempotency_key: expect.any(String),
        expected_revision: 3,
        facts: [
          expect.objectContaining({
            kind: "symptom",
            statement: "我昨晚右肩疼痛",
            body_region_ids: [],
            temporal: { precision: "unknown" },
            details: {
              symptom_type: "右肩疼痛",
              presence: "present",
              laterality: "right",
            },
          }),
        ],
      }),
    );
    expect(await screen.findByText(/本人症状陈述已保存/)).toBeVisible();
    expect(api.episode.create).not.toHaveBeenCalled();
  });

  it("retains exact record idempotency after an uncertain write and collapse", async () => {
    api.facts.record
      .mockRejectedValueOnce(new Error("网络断开，保存结果未确认"))
      .mockResolvedValueOnce({
        id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
        user_id: owner,
        revision: 4,
        fact_ids: [positiveID],
      });
    mount();
    await openEditor();
    await userEvent.click(
      screen.getByRole("button", { name: "补充可关联的本人症状" }),
    );
    await userEvent.type(screen.getByLabelText("本人完整症状陈述"), "我肩痛");
    await userEvent.type(screen.getByLabelText("症状类型"), "肩痛");
    await userEvent.click(
      screen.getByRole("button", { name: "先保存症状陈述" }),
    );
    expect(await screen.findByText(/网络断开，保存结果未确认/)).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "收起经历" }));
    await userEvent.click(
      screen.getByRole("button", { name: "查看与关联经历" }),
    );
    await userEvent.click(
      screen.getByRole("button", { name: "重试原症状记录" }),
    );
    expect(await screen.findByText(/本人症状陈述已保存/)).toBeVisible();
    expect(api.facts.record).toHaveBeenCalledTimes(2);
    expect(api.facts.record.mock.calls[1][0]).toEqual(
      api.facts.record.mock.calls[0][0],
    );
    expect(api.episode.create).not.toHaveBeenCalled();
  });

  it("preserves the negative statement without assigning a recovery status", async () => {
    api.facts.record.mockResolvedValue({
      id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
      user_id: owner,
      revision: 4,
      fact_ids: [negativeID],
    });
    mount();
    await openEditor();
    await userEvent.click(
      screen.getByRole("button", { name: "补充可关联的本人症状" }),
    );
    await userEvent.type(
      screen.getByLabelText("本人完整症状陈述"),
      "今天我没有肩痛",
    );
    await userEvent.type(screen.getByLabelText("症状类型"), "肩痛");
    await selectOption(
      screen.getByRole("combobox", { name: "当前陈述是否存在症状" }),
      "absent",
    );
    await userEvent.click(
      screen.getByRole("button", { name: "先保存症状陈述" }),
    );
    expect(api.facts.record).toHaveBeenCalledWith(
      expect.objectContaining({
        facts: [
          expect.objectContaining({
            details: expect.objectContaining({ presence: "absent" }),
          }),
        ],
      }),
    );
    expect(api.episode.create).not.toHaveBeenCalled();
    expect(await screen.findByText(/本人症状陈述已保存/)).toBeVisible();
  });

  it("does not submit a new symptom in read-only Person mode", async () => {
    mount(false);
    await userEvent.click(
      screen.getByRole("button", { name: "查看与关联经历" }),
    );
    await screen.findByTestId("body-episode-editor");
    expect(
      screen.queryByRole("button", { name: "补充可关联的本人症状" }),
    ).not.toBeInTheDocument();
    expect(api.facts.record).not.toHaveBeenCalled();
  });
});

describe("Episode read and ambiguous-write safety regressions", () => {
  it("never labels an unavailable initial Episode query as an empty health history", async () => {
    api.episode.list.mockRejectedValueOnce(new Error("来源暂时不可读取"));
    mount();
    await userEvent.click(
      screen.getByRole("button", { name: "查看与关联经历" }),
    );
    expect(await screen.findByText(/资料读取失败，请刷新重试/)).toBeVisible();
    expect(screen.queryByText(/没有症状经历/)).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "确认新建经历" }),
    ).not.toBeInTheDocument();
    api.episode.list.mockResolvedValue(episodePage());
    await userEvent.click(
      screen.getByRole("button", { name: "刷新症状经历和身体记录" }),
    );
    expect(await screen.findByText(/还没有症状经历/)).toBeVisible();
  });

  it("blocks new Fact or Episode commands when a cached read refresh fails", async () => {
    mount();
    await openEditor();
    await selectOption(
      screen.getByRole("combobox", { name: "本人症状陈述" }),
      positiveID,
    );
    api.episode.list.mockRejectedValueOnce(new Error("失去来源连接"));
    await userEvent.click(
      screen.getByRole("button", { name: "刷新症状经历和身体记录" }),
    );
    expect(await screen.findByText(/资料读取失败，请刷新重试/)).toBeVisible();
    expect(screen.queryByText(/还没有症状经历/)).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "确认新建经历" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "补充可关联的本人症状" }),
    ).not.toBeInTheDocument();
    expect(api.episode.create).not.toHaveBeenCalled();
    expect(api.facts.record).not.toHaveBeenCalled();
  });

  it("holds original Episode command identity against switching to another target after an unknown result", async () => {
    api.episode.create.mockRejectedValueOnce(new Error("提交状态未确认"));
    mount();
    await openEditor();
    await selectOption(
      screen.getByRole("combobox", { name: "本人症状陈述" }),
      positiveID,
    );
    await userEvent.click(screen.getByRole("button", { name: "确认新建经历" }));
    expect(await screen.findByText(/提交状态未确认/)).toBeVisible();
    expect(screen.getByRole("combobox", { name: "操作" })).toBeDisabled();
    expect(
      screen.getByRole("combobox", { name: "本人症状陈述" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "补充可关联的本人症状" }),
    ).toBeDisabled();
    expect(screen.getByRole("button", { name: "重试原请求" })).toBeEnabled();
    expect(api.episode.create).toHaveBeenCalledTimes(1);
  });

  it("does not allow editing the source Fact draft or new Episode creation with an uncertain Fact command", async () => {
    api.facts.record.mockRejectedValueOnce(new Error("症状提交状态未确认"));
    mount();
    await openEditor();
    await userEvent.click(
      screen.getByRole("button", { name: "补充可关联的本人症状" }),
    );
    await userEvent.type(
      screen.getByLabelText("本人完整症状陈述"),
      "我右肩疼痛",
    );
    await userEvent.type(screen.getByLabelText("症状类型"), "肩痛");
    await userEvent.click(
      screen.getByRole("button", { name: "先保存症状陈述" }),
    );
    expect(await screen.findByText(/症状提交状态未确认/)).toBeVisible();
    expect(screen.getByLabelText("本人完整症状陈述")).toBeDisabled();
    expect(screen.getByLabelText("症状类型")).toBeDisabled();
    expect(
      screen.getByRole("combobox", { name: "当前陈述是否存在症状" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "重试原症状记录" }),
    ).toBeEnabled();
    expect(screen.getByRole("button", { name: "确认新建经历" })).toBeDisabled();
    expect(api.episode.create).not.toHaveBeenCalled();
  });
});
