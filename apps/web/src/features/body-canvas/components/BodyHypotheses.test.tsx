import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { useAuthStore } from "@/stores/authStore";
import { BodyHypotheses } from "./BodyHypotheses";

const api = vi.hoisted(() => ({
  read: vi.fn(),
  groups: vi.fn(),
  record: vi.fn(),
  correct: vi.fn(),
  withdraw: vi.fn(),
}));
vi.mock("@/features/profile/services/bodyRecordsService", () => ({
  readBodyRecords: api.read,
}));
vi.mock("@/features/profile/services/bodyHypothesisService", () => ({
  bodyHypothesisService: api,
}));
const owner = "11111111-1111-4111-8111-111111111111";
const id = "22222222-2222-4222-8222-222222222222";
const successorId = "33333333-3333-4333-8333-333333333333";
const snapshot = {
  user_id: owner,
  current_revision: 3,
  facts: [],
  observations: [],
  episodes: [],
  hypotheses: [],
};
const receipt = {
  user_id: owner,
  operation: "record",
  hypothesis_id: id,
  revision: 4,
};
function mount(canEdit = true) {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <BodyHypotheses canEdit={canEdit} />
    </QueryClientProvider>,
  );
}
beforeEach(() => {
  vi.resetAllMocks();
  useAuthStore.setState({
    user: { id: owner, email: "synthetic@example.test" },
    isAuthenticated: true,
  });
  api.read.mockResolvedValue(snapshot);
  api.groups.mockResolvedValue({
    user_id: owner,
    current_revision: 3,
    groups: [],
  });
});
it("waits for a commit receipt before success and refreshes both reads", async () => {
  let resolve!: (value: typeof receipt) => void;
  api.record.mockReturnValue(
    new Promise((r) => {
      resolve = r;
    }),
  );
  mount();
  await screen.findByText("还没有你提出的猜测。");
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "添加我的猜测" }));
  await user.type(screen.getByLabelText("我的完整猜测"), "我猜可能与久坐有关");
  await user.click(screen.getByRole("button", { name: "保存猜测" }));
  expect(screen.queryByText("你的猜测已保存。")).not.toBeInTheDocument();
  resolve(receipt);
  expect(await screen.findByText("你的猜测已保存。")).toBeVisible();
  expect(api.read.mock.calls.length).toBeGreaterThan(1);
  expect(api.groups.mock.calls.length).toBeGreaterThan(1);
  expect(api.record).toHaveBeenCalledWith(
    expect.objectContaining({
      expected_revision: 3,
      statement: "我猜可能与久坐有关",
      idempotency_key: expect.any(String),
    }),
  );
});

const hypothesis = {
  id,
  user_id: owner,
  lineage_id: id,
  statement: "我猜与久坐有关",
  source: {
    actor: "user" as const,
    kind: "direct_edit" as const,
    id,
    version: 1,
  },
  related_fact_ids: [],
  related_observation_ids: [],
  assertion_status: "asserted" as const,
  created_revision: 3,
  recorded_at: "2026-10-17T00:00:00Z",
};
async function addDraft() {
  const user = userEvent.setup();
  await screen.findByText("还没有你提出的猜测。");
  await user.click(screen.getByRole("button", { name: "添加我的猜测" }));
  await user.type(screen.getByLabelText("我的完整猜测"), "我猜与久坐有关");
  return user;
}
it("shows refreshed list after commit, then corrects and withdraws exact targets", async () => {
  api.record.mockImplementation(async () => {
    api.read.mockResolvedValue({
      ...snapshot,
      current_revision: 4,
      hypotheses: [hypothesis],
    });
    api.groups.mockResolvedValue({
      user_id: owner,
      current_revision: 4,
      groups: [],
    });
    return receipt;
  });
  mount();
  const user = await addDraft();
  await user.click(screen.getByRole("button", { name: "保存猜测" }));
  expect(await screen.findByText(hypothesis.statement)).toBeVisible();
  api.correct.mockImplementation(async () => {
    api.read.mockResolvedValue({
      ...snapshot,
      current_revision: 5,
      hypotheses: [
        {
          ...hypothesis,
          id: successorId,
          statement: "我猜与运动有关",
          supersedes_hypothesis_id: id,
        },
      ],
    });
    api.groups.mockResolvedValue({
      user_id: owner,
      current_revision: 5,
      groups: [],
    });
    return {
      ...receipt,
      operation: "correct",
      hypothesis_id: successorId,
      target_hypothesis_id: id,
      revision: 5,
    };
  });
  await user.click(screen.getByRole("button", { name: "更正猜测" }));
  await user.clear(screen.getByLabelText("我的完整猜测"));
  await user.type(screen.getByLabelText("我的完整猜测"), "我猜与运动有关");
  await user.click(screen.getByRole("button", { name: "保存猜测更正" }));
  expect(await screen.findByText("我猜与运动有关")).toBeVisible();
  expect(api.correct).toHaveBeenCalledWith(
    id,
    expect.objectContaining({
      expected_revision: 4,
      replacement: {
        statement: "我猜与运动有关",
        related_fact_ids: [],
        related_observation_ids: [],
      },
    }),
  );
  api.withdraw.mockImplementation(async () => {
    api.read.mockResolvedValue({ ...snapshot, current_revision: 6 });
    api.groups.mockResolvedValue({
      user_id: owner,
      current_revision: 6,
      groups: [],
    });
    return {
      ...receipt,
      operation: "withdraw",
      hypothesis_id: successorId,
      target_hypothesis_id: successorId,
      revision: 6,
    };
  });
  await user.click(screen.getByRole("button", { name: "撤回猜测" }));
  await user.click(screen.getByRole("button", { name: "确认撤回猜测" }));
  expect(await screen.findByText("还没有你提出的猜测。")).toBeVisible();
  expect(api.withdraw).toHaveBeenCalledWith(
    successorId,
    expect.objectContaining({ expected_revision: 5 }),
  );
});
it("retries an uncertain failure with the identical command", async () => {
  api.record
    .mockRejectedValueOnce(new Error("网络中断"))
    .mockResolvedValue(receipt);
  mount();
  const user = await addDraft();
  await user.click(screen.getByRole("button", { name: "保存猜测" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("网络中断");
  await user.click(screen.getByRole("button", { name: "保存猜测" }));
  await screen.findByText("你的猜测已保存。");
  expect(api.record.mock.calls[0][0]).toEqual(api.record.mock.calls[1][0]);
});
it.each([404, 409])(
  "closes stale edits on %s, refreshes, and requires explicit resubmission",
  async (status) => {
    const { ApiRequestError } = await import("@/lib/api-client");
    api.record.mockRejectedValue(new ApiRequestError("unavailable", status));
    mount();
    const user = await addDraft();
    await user.click(screen.getByRole("button", { name: "保存猜测" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "记录已变化或不可用",
    );
    expect(screen.queryByRole("form")).not.toBeInTheDocument();
    expect(screen.queryByText("你的猜测已保存。")).not.toBeInTheDocument();
    expect(api.read.mock.calls.length).toBeGreaterThan(1);
  },
);
it("cancels without a write", async () => {
  mount();
  const user = await addDraft();
  await user.click(screen.getByRole("button", { name: "取消" }));
  expect(api.record).not.toHaveBeenCalled();
});
it("shows groups and original fact/observation sources without clinical promotion", async () => {
  api.read.mockResolvedValue({
    ...snapshot,
    hypotheses: [hypothesis],
    facts: [{ id, statement: "肩膀不舒服" }],
    observations: [
      { id, statement: "今天称重", raw_value: "67", raw_unit: "kg" },
    ],
  });
  api.groups.mockResolvedValue({
    user_id: owner,
    current_revision: 3,
    groups: [
      {
        id,
        display_title: "相关记录",
        related_hypothesis_ids: [id],
        related_fact_ids: [id],
        related_observation_ids: [id],
        related_episode_ids: [],
      },
    ],
  });
  mount();
  expect(await screen.findByText("来源事实：肩膀不舒服")).toBeVisible();
  expect(screen.getByText("来源观察：今天称重 · 67 · kg")).toBeVisible();
  expect(screen.getByText(`用户猜测：${hypothesis.statement}`)).toBeVisible();
});
it("blocks writes on inconsistent snapshots and retries reads", async () => {
  api.groups.mockResolvedValue({
    user_id: owner,
    current_revision: 4,
    groups: [],
  });
  mount();
  await screen.findByText(/正在等待一致/);
  expect(screen.getByRole("button", { name: "添加我的猜测" })).toBeDisabled();
  api.groups.mockResolvedValue({
    user_id: owner,
    current_revision: 3,
    groups: [],
  });
  await userEvent.click(
    screen.getByRole("button", { name: "重新读取猜测与分组" }),
  );
  expect(await screen.findByText("还没有你提出的猜测。")).toBeVisible();
});
it("shows load errors with retry, never an empty state", async () => {
  api.groups.mockRejectedValueOnce(new Error("offline"));
  mount();
  expect(await screen.findByRole("alert")).toHaveTextContent("读取失败");
  expect(screen.queryByText("暂无相关记录分组。")).not.toBeInTheDocument();
  api.groups.mockResolvedValue({
    user_id: owner,
    current_revision: 3,
    groups: [],
  });
  await userEvent.click(
    screen.getByRole("button", { name: "重新读取猜测与分组" }),
  );
  expect(await screen.findByText("暂无相关记录分组。")).toBeVisible();
});
it("rejects mismatched receipt identity and does not display success", async () => {
  api.record.mockResolvedValue({ ...receipt, user_id: id });
  mount();
  const user = await addDraft();
  await user.click(screen.getByRole("button", { name: "保存猜测" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("尚未收到匹配");
  expect(screen.queryByText("你的猜测已保存。")).not.toBeInTheDocument();
});
it("does not expose write controls for a read-only window", async () => {
  api.read.mockResolvedValue({ ...snapshot, hypotheses: [hypothesis] });
  mount(false);
  expect(await screen.findByText(hypothesis.statement)).toBeVisible();
  expect(
    screen.queryByRole("button", { name: "添加我的猜测" }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "更正猜测" }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "撤回猜测" }),
  ).not.toBeInTheDocument();
});
it("keeps committed success while failed refresh blocks another write", async () => {
  api.record.mockImplementation(async () => {
    api.read.mockRejectedValue(new Error("read offline"));
    return receipt;
  });
  mount();
  const user = await addDraft();
  await user.click(screen.getByRole("button", { name: "保存猜测" }));
  expect(await screen.findByText("你的猜测已保存。")).toBeVisible();
  expect(await screen.findByRole("alert")).toHaveTextContent("读取失败");
  expect(screen.getByRole("button", { name: "添加我的猜测" })).toBeDisabled();
  expect(api.record).toHaveBeenCalledTimes(1);
});
it("drops pending results after an owner switch", async () => {
  let resolve!: (value: typeof receipt) => void;
  api.record.mockReturnValue(
    new Promise((r) => {
      resolve = r;
    }),
  );
  mount();
  const user = await addDraft();
  await user.click(screen.getByRole("button", { name: "保存猜测" }));
  api.read.mockResolvedValue({ ...snapshot, user_id: id });
  api.groups.mockResolvedValue({
    user_id: id,
    current_revision: 3,
    groups: [],
  });
  await act(async () => {
    useAuthStore.setState({ user: { id, email: "other@example.test" } });
  });
  await screen.findByText("还没有你提出的猜测。");
  await act(async () => {
    resolve(receipt);
  });
  expect(screen.queryByText("你的猜测已保存。")).not.toBeInTheDocument();
  expect(screen.queryByLabelText("我的完整猜测")).not.toBeInTheDocument();
});
it("rejects foreign group snapshots", async () => {
  api.groups.mockResolvedValue({
    user_id: id,
    current_revision: 3,
    groups: [],
  });
  mount();
  expect(await screen.findByRole("alert")).toHaveTextContent("读取失败");
  expect(screen.getByRole("button", { name: "添加我的猜测" })).toBeDisabled();
});
it("submits only the explicitly selected fact and observation links", async () => {
  api.read.mockResolvedValue({
    ...snapshot,
    facts: [{ id, statement: "肩膀不舒服" }],
    observations: [{ id, statement: "今天称重" }],
  });
  api.record.mockResolvedValue(receipt);
  mount();
  const user = await addDraft();
  await user.click(screen.getByLabelText("肩膀不舒服"));
  await user.click(screen.getByLabelText("今天称重"));
  await user.click(screen.getByRole("button", { name: "保存猜测" }));
  await screen.findByText("你的猜测已保存。");
  expect(api.record).toHaveBeenCalledWith(
    expect.objectContaining({
      related_fact_ids: [id],
      related_observation_ids: [id],
    }),
  );
});
it("rejects a correction receipt that does not identify a distinct successor", async () => {
  api.read.mockResolvedValue({ ...snapshot, hypotheses: [hypothesis] });
  api.correct.mockResolvedValue({
    ...receipt,
    operation: "correct",
    target_hypothesis_id: id,
  });
  mount();
  await screen.findByText(hypothesis.statement);
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "更正猜测" }));
  await user.clear(screen.getByLabelText("我的完整猜测"));
  await user.type(screen.getByLabelText("我的完整猜测"), "我猜与运动有关");
  await user.click(screen.getByRole("button", { name: "保存猜测更正" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("尚未收到匹配");
  expect(screen.queryByText(/猜测更正已保存/)).not.toBeInTheDocument();
});
