import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useAuthStore } from "@/stores/authStore";
import { ApiRequestError } from "@/lib/api-client";
import { BodyFactRecords } from "./BodyFactRecords";

const api = vi.hoisted(() => ({ list: vi.fn(), record: vi.fn() }));
vi.mock("@/features/profile/services/bodyFactRecordsService", () => ({
  bodyFactRecordsService: api,
}));
const owner = "11111111-1111-4111-8111-111111111111";
const factId = "22222222-2222-4222-8222-222222222222";
const fact = (kind = "symptom", statement = "我昨天肩膀不舒服") => ({
  id: factId,
  user_id: owner,
  kind,
  statement,
  body_region_ids: [],
  temporal: { precision: "unknown" },
  details: {},
  subject: "self",
  source_evidence_id: factId,
  assertion_status: "asserted",
  created_revision: 3,
  recorded_at: "2026-10-13T00:00:00Z",
});
const page = (facts: ReturnType<typeof fact>[] = []) => ({
  user_id: owner,
  current_revision: 3,
  facts,
  has_more: false,
});
const receipt = {
  id: factId,
  user_id: owner,
  revision: 4,
  revision_id: factId,
  event_id: factId,
  fact_ids: [factId],
  committed_at: "2026-10-13T01:00:00Z",
};
function mount(habitsOnly = false) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return {
    client,
    ...render(
      <QueryClientProvider client={client}>
        <BodyFactRecords habitsOnly={habitsOnly} canEdit />
      </QueryClientProvider>,
    ),
  };
}
beforeEach(() => {
  api.list.mockReset();
  api.record.mockReset();
  api.list.mockResolvedValue(page());
  useAuthStore.setState({
    user: { id: owner, email: "synthetic@example.test" },
    isAuthenticated: true,
  });
});

describe("BodyFact Records and Habits", () => {
  it("consumes the bounded V2 API projection rather than an invented complete clinical VM", async () => {
    mount();
    await screen.findByText(/还没有身体记录/);
    expect(screen.getByTestId("body-fact-records")).toHaveAttribute(
      "data-vm-contract",
      "bodyfact-ledger-page/v1",
    );
    expect(screen.getByTestId("body-fact-records")).toHaveAttribute(
      "data-vm-state",
      "empty",
    );
  });

  it("renders a genuine empty state without technical revisions or clinical confirmation", async () => {
    mount();
    expect(await screen.findByText(/还没有身体记录/)).toBeVisible();
    expect(
      screen.queryByText(/revision|修订号|临床确诊/),
    ).not.toBeInTheDocument();
  });

  it("reads habit assertions from the shared ledger, not the old summary slots", async () => {
    api.list.mockResolvedValue(
      page([
        fact("symptom", "我肩膀不舒服"),
        {
          ...fact("lifestyle", "我每晚睡七小时"),
          id: "33333333-3333-4333-8333-333333333333",
        },
      ]),
    );
    mount(true);
    expect(await screen.findByText("我每晚睡七小时")).toBeVisible();
    expect(screen.queryByText("我肩膀不舒服")).not.toBeInTheDocument();
    expect(screen.getByText("发生时间：未说明")).toBeVisible();
  });

  it("does not call a partially scanned Habits history truly empty", async () => {
    api.list.mockResolvedValueOnce({
      ...page([fact("symptom", "昨天肩痛")]),
      has_more: true,
      next_before_revision: 3,
    });
    api.list.mockResolvedValueOnce(
      page([
        {
          ...fact("lifestyle", "一周前每天散步"),
          id: "33333333-3333-4333-8333-333333333333",
          created_revision: 2,
        },
      ]),
    );
    mount(true);
    expect(
      await screen.findByText(/当前加载的记录中没有生活习惯/),
    ).toBeVisible();
    expect(screen.queryByText(/尚未记录生活习惯/)).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "读取更早记录" }));
    expect(await screen.findByText("一周前每天散步")).toBeVisible();
  });

  it("does not mix historical records from different revisions after a concurrent edit", async () => {
    api.list.mockResolvedValueOnce({
      ...page([fact("symptom", "原有肩部陈述")]),
      has_more: true,
      next_before_revision: 3,
    });
    api.list.mockResolvedValueOnce({
      ...page([
        {
          ...fact("activity", "已过时的历史记录"),
          id: "44444444-4444-4444-8444-444444444444",
        },
      ]),
      current_revision: 4,
    });
    mount();
    expect(await screen.findByText("原有肩部陈述")).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "读取更早记录" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "翻页期间身体记录发生了变化",
    );
    expect(screen.queryByText("已过时的历史记录")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /补充记录/ })).toBeDisabled();
  });

  it("shows read failure with retry, rather than an empty or fabricated record", async () => {
    api.list.mockRejectedValueOnce(new Error("offline"));
    mount();
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "身体记录暂时无法读取",
    );
    expect(screen.queryByText(/还没有身体记录/)).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "重新读取" }));
    expect(await screen.findByText(/还没有身体记录/)).toBeVisible();
  });

  it("preserves the exact command across an uncertain transport failure", async () => {
    api.record.mockRejectedValueOnce(new Error("连接中断"));
    api.record.mockResolvedValueOnce(receipt);
    mount();
    await screen.findByText(/还没有身体记录/);
    await userEvent.click(screen.getByRole("button", { name: /补充记录/ }));
    await userEvent.type(screen.getByLabelText("你的描述"), "我昨天肩膀不舒服");
    await userEvent.click(screen.getByRole("button", { name: "保存记录" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("连接中断");
    const original = structuredClone(api.record.mock.calls[0][0]);
    expect(original.expected_revision).toBe(3);
    expect(original.facts[0].temporal).toEqual({ precision: "unknown" });
    expect(screen.queryByText("已保存你的陈述。")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "保存记录" }));
    expect(await screen.findByText("已保存你的陈述。")).toBeVisible();
    expect(api.record.mock.calls[1][0]).toEqual(original);
  });

  it("keeps a confirmed commit successful when the following refresh fails", async () => {
    api.record.mockResolvedValue(receipt);
    mount();
    await screen.findByText(/还没有身体记录/);
    await userEvent.click(screen.getByRole("button", { name: /补充记录/ }));
    await userEvent.type(screen.getByLabelText("你的描述"), "我今天散步");
    api.list.mockRejectedValue(new Error("read unavailable"));
    await userEvent.click(screen.getByRole("button", { name: "保存记录" }));
    expect(await screen.findByText("已保存你的陈述。")).toBeVisible();
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "记录已保存，但列表刷新失败",
    );
    expect(api.record).toHaveBeenCalledTimes(1);
  });

  it("refetches after a conflict without silently resubmitting", async () => {
    api.record.mockRejectedValue(
      new ApiRequestError("stale", 409, "REVISION_CONFLICT"),
    );
    mount();
    await screen.findByText(/还没有身体记录/);
    await userEvent.click(screen.getByRole("button", { name: /补充记录/ }));
    await userEvent.type(screen.getByLabelText("你的描述"), "我今天散步");
    await userEvent.click(screen.getByRole("button", { name: "保存记录" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "记录已发生变化",
    );
    await waitFor(() => expect(api.list).toHaveBeenCalledTimes(2));
    expect(api.record).toHaveBeenCalledTimes(1);
  });

  it("does not reuse another user's cached facts or query without an identity", async () => {
    useAuthStore.setState({ user: null });
    mount();
    expect(screen.getByText("请登录后查看个人记录。")).toBeVisible();
    expect(api.list).not.toHaveBeenCalled();
  });

  it("follows the server cursor and retains every row from both pages", async () => {
    api.list.mockResolvedValueOnce({
      ...page([fact()]),
      has_more: true,
      next_before_revision: 3,
    });
    api.list.mockResolvedValueOnce(
      page([
        {
          ...fact("activity", "我前天散步"),
          id: "33333333-3333-4333-8333-333333333333",
          created_revision: 2,
        },
      ]),
    );
    mount();
    await screen.findByText("我昨天肩膀不舒服");
    await userEvent.click(screen.getByRole("button", { name: "读取更早记录" }));
    expect(await screen.findByText("我前天散步")).toBeVisible();
    expect(screen.getByText("我昨天肩膀不舒服")).toBeVisible();
    expect(api.list).toHaveBeenLastCalledWith(3);
  });
});
