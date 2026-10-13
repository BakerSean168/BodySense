import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApiRequestError } from "@/lib/api-client";
import type { BodyFactRecord } from "@/features/profile/services/bodyFactRecordsService";
import { BodyFactAmendment } from "./BodyFactAmendment";

const api = vi.hoisted(() => ({ correct: vi.fn(), withdraw: vi.fn() }));
vi.mock("@/features/profile/services/bodyFactRecordsService", () => ({
  bodyFactRecordsService: api,
}));
const id = "11111111-1111-4111-8111-111111111111";
const owner = "22222222-2222-4222-8222-222222222222";
const source = "33333333-3333-4333-8333-333333333333";
const record: BodyFactRecord = {
  id,
  user_id: owner,
  subject: "self",
  kind: "symptom",
  statement: "我昨天左肩很痛",
  body_region_ids: ["shoulder.left"],
  details: { presence: "present" },
  temporal: { precision: "unknown", original_expression: "昨天" },
  source_evidence_id: source,
  assertion_status: "asserted",
  created_revision: 1,
  recorded_at: "2026-10-13T00:00:00Z",
};
const receipt = {
  id: source,
  user_id: owner,
  revision: 2,
  revision_id: source,
  event_id: source,
  fact_ids: [id],
  committed_at: "2026-10-13T01:00:00Z",
};
function mount() {
  const onCommitted = vi.fn();
  const onRefresh = vi.fn();
  render(
    <BodyFactAmendment
      record={record}
      revision={1}
      onCommitted={onCommitted}
      onRefresh={onRefresh}
    />,
  );
  return { onCommitted, onRefresh };
}
beforeEach(() => {
  api.correct.mockReset();
  api.withdraw.mockReset();
});

describe("explicit BodyFact correction and withdrawal", () => {
  it("does not mutate a record when merely opening or cancelling withdrawal", async () => {
    mount();
    await userEvent.click(screen.getByRole("button", { name: "撤回陈述" }));
    expect(screen.getByText(/这不是隐私数据删除/)).toBeVisible();
    expect(api.withdraw).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: "取消" }));
    expect(api.withdraw).not.toHaveBeenCalled();
  });

  it("binds a confirmed withdrawal to exact target and revision", async () => {
    api.withdraw.mockResolvedValue(receipt);
    const { onCommitted } = mount();
    await userEvent.click(screen.getByRole("button", { name: "撤回陈述" }));
    await userEvent.click(screen.getByRole("button", { name: "确认撤回" }));
    expect(api.withdraw).toHaveBeenCalledWith(
      id,
      expect.objectContaining({ expected_revision: 1 }),
    );
    expect(onCommitted).toHaveBeenCalledWith(
      expect.stringContaining("不表示隐私数据已删除"),
    );
  });

  it("corrects the full user statement without carrying forward false optional claims", async () => {
    api.correct.mockResolvedValue(receipt);
    const { onCommitted } = mount();
    await userEvent.click(screen.getByRole("button", { name: "更正" }));
    expect(screen.getByRole("button", { name: "保存更正" })).toBeDisabled();
    const editor = screen.getByLabelText("更正后的完整陈述");
    await userEvent.clear(editor);
    await userEvent.type(editor, "我没有左肩痛，之前说错了");
    await userEvent.click(screen.getByRole("button", { name: "保存更正" }));
    expect(api.correct).toHaveBeenCalledWith(
      id,
      expect.objectContaining({
        expected_revision: 1,
        replacement: {
          kind: "symptom",
          statement: "我没有左肩痛，之前说错了",
          details: {},
          temporal: { precision: "unknown" },
          body_region_ids: [],
        },
      }),
    );
    expect(onCommitted).toHaveBeenCalledWith(
      expect.stringContaining("原陈述保留"),
    );
  });

  it("allows an explicit category correction without keeping the old discriminator", async () => {
    api.correct.mockResolvedValue(receipt);
    const { onCommitted } = mount();
    await userEvent.click(screen.getByRole("button", { name: "更正" }));
    await userEvent.selectOptions(
      screen.getByLabelText("更正后的类别"),
      "activity",
    );
    await userEvent.clear(screen.getByLabelText("更正后的完整陈述"));
    await userEvent.type(
      screen.getByLabelText("更正后的完整陈述"),
      "我昨天做了抬手活动",
    );
    await userEvent.click(screen.getByRole("button", { name: "保存更正" }));
    expect(api.correct).toHaveBeenCalledWith(
      id,
      expect.objectContaining({
        replacement: expect.objectContaining({
          kind: "activity",
          statement: "我昨天做了抬手活动",
          details: {},
        }),
      }),
    );
    expect(onCommitted).toHaveBeenCalled();
  });

  it("replays the identical command after uncertain transport failure", async () => {
    api.withdraw.mockRejectedValueOnce(new Error("网络中断"));
    api.withdraw.mockResolvedValueOnce(receipt);
    mount();
    await userEvent.click(screen.getByRole("button", { name: "撤回陈述" }));
    await userEvent.click(screen.getByRole("button", { name: "确认撤回" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("网络中断");
    const first = structuredClone(api.withdraw.mock.calls[0]);
    await userEvent.click(screen.getByRole("button", { name: "确认撤回" }));
    expect(api.withdraw.mock.calls[1]).toEqual(first);
  });

  it("refetches conflicts without silently repeating or reporting success", async () => {
    api.withdraw.mockRejectedValue(
      new ApiRequestError("record changed", 409, "FACT_NOT_CURRENT"),
    );
    const { onCommitted, onRefresh } = mount();
    await userEvent.click(screen.getByRole("button", { name: "撤回陈述" }));
    await userEvent.click(screen.getByRole("button", { name: "确认撤回" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "记录或档案已发生变化",
    );
    expect(onRefresh).toHaveBeenCalledTimes(1);
    expect(onCommitted).not.toHaveBeenCalled();
    expect(api.withdraw).toHaveBeenCalledTimes(1);
  });

  it("rejects a receipt for a different owner or target", async () => {
    api.withdraw.mockResolvedValue({ ...receipt, fact_ids: [source] });
    const { onCommitted } = mount();
    await userEvent.click(screen.getByRole("button", { name: "撤回陈述" }));
    await userEvent.click(screen.getByRole("button", { name: "确认撤回" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "未确认目标记录",
    );
    expect(onCommitted).not.toHaveBeenCalled();
  });
});
