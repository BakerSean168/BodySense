import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useUploadStore } from "@/stores/uploadStore";
import type { UserUpload } from "@/features/profile/types/upload.types";
import type { AssetErasureReceipt } from "@/features/profile/services/assetErasureService";
import { PersonAssetDeletion } from "./PersonAssetDeletion";

const erasure = vi.hoisted(() => ({ request: vi.fn() }));
vi.mock("@/features/profile/services/assetErasureService", () => ({
  assetErasureService: { request: erasure.request },
}));

const original = useUploadStore.getState();
const id = "11111111-1111-4111-8111-111111111111";
const receiptId = "22222222-2222-4222-8222-222222222222";

const upload = {
  id,
  file_type: "report",
  original_name: "测试报告.pdf",
  file_size: 22000,
  mime_type: "application/pdf",
  source_availability: "available",
  ocr_result: null,
  ocr_status: "completed",
  analysis_status: "completed",
  created_at: "2026-10-21T10:00:00Z",
  updated_at: "2026-10-21T10:00:00Z",
} satisfies UserUpload;

function makeReceipt(
  status: "running" | "erased",
  assetId = id,
): AssetErasureReceipt {
  const erased = status === "erased";
  return {
    request_id: receiptId,
    asset_id: assetId,
    status,
    inventory_revision: "storybook-test-revision",
    requested_at: "2026-10-21T10:00:00Z",
    ...(erased ? { erased_at: "2026-10-21T10:02:00Z" } : {}),
    targets: [
      "source_objects",
      "asset_manifest",
      "derived_jobs",
      "runtime_checkpoints",
    ].map((target) => ({
      target: target as
        | "source_objects"
        | "asset_manifest"
        | "derived_jobs"
        | "runtime_checkpoints",
      state: erased ? ("verified" as const) : ("pending" as const),
      erased_count: erased ? 1 : 0,
      outstanding_count: erased ? 0 : 1,
    })),
    outstanding_targets: [],
    reviews: [],
  };
}

function mount(removeSource = vi.fn(async () => {})) {
  const onBack = vi.fn();
  const onErased = vi.fn();
  act(() => {
    useUploadStore.setState({ uploads: [upload], deleteUpload: removeSource });
  });
  render(
    <PersonAssetDeletion
      upload={upload}
      backLabel="返回媒体"
      onBack={onBack}
      onErased={onErased}
    />,
  );
  return { onBack, onErased, removeSource };
}

beforeEach(() => erasure.request.mockReset());
afterEach(() => {
  act(() => useUploadStore.setState(original));
});

describe("PersonAssetDeletion two-step confirmation", () => {
  it("offers keyboard accessible choices and exits safely without deleting", async () => {
    const user = userEvent.setup();
    const { onBack, onErased, removeSource } = mount();
    const group = screen.getByRole("radiogroup", { name: "删除范围" });
    expect(within(group).getAllByRole("radio")).toHaveLength(2);
    await user.keyboard("{Escape}");
    expect(onBack).toHaveBeenCalledOnce();
    expect(onErased).not.toHaveBeenCalled();
    expect(removeSource).not.toHaveBeenCalled();
    expect(erasure.request).not.toHaveBeenCalled();
  });

  it("removes only the original after explicit confirmation", async () => {
    const user = userEvent.setup();
    const { onErased, removeSource } = mount();
    await user.click(screen.getByRole("button", { name: "仅移除原件" }));
    expect(screen.getByTestId("person-asset-deletion")).toHaveAttribute(
      "data-scene",
      "confirm-source",
    );
    expect(removeSource).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "确认移除原件" }));
    expect(removeSource).toHaveBeenCalledExactlyOnceWith(id);
    expect(
      await screen.findByText("历史证据保留，原件不可再核验"),
    ).toBeVisible();
    expect(erasure.request).not.toHaveBeenCalled();
    expect(onErased).not.toHaveBeenCalled();
  });

  it("retries the same idempotency key and never claims erasure before a verified receipt", async () => {
    erasure.request
      .mockRejectedValueOnce(new Error("server temporarily unavailable"))
      .mockResolvedValueOnce(makeReceipt("running"))
      .mockResolvedValueOnce(makeReceipt("erased"));

    const user = userEvent.setup();
    const { onErased } = mount();
    await user.click(screen.getByRole("button", { name: "完整清理" }));
    expect(screen.getByTestId("person-asset-deletion")).toHaveAttribute(
      "data-scene",
      "confirm-erase",
    );
    expect(erasure.request).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "确认清理" }));
    expect(await screen.findByRole("alert")).toBeVisible();
    expect(onErased).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "确认清理" }));
    expect(await screen.findByText("正在逐项清理并核验")).toBeVisible();
    expect(screen.queryByText("全部范围已核验")).toBeNull();
    expect(onErased).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "再次检查" }));
    expect(await screen.findByText("全部范围已核验")).toBeVisible();
    expect(onErased).not.toHaveBeenCalled();
    expect(erasure.request).toHaveBeenCalledTimes(3);

    const calls = erasure.request.mock.calls;
    expect(
      calls.every((call) => call[0] === id && call[1] === calls[0][1]),
    ).toBe(true);

    await user.click(screen.getByRole("button", { name: "返回媒体" }));
    expect(onErased).toHaveBeenCalledExactlyOnceWith(id);
  });

  it("rejects a receipt that refers to another asset", async () => {
    erasure.request.mockResolvedValue(
      makeReceipt("erased", "33333333-3333-4333-8333-333333333333"),
    );
    const user = userEvent.setup();
    const { onErased } = mount();
    await user.click(screen.getByRole("button", { name: "完整清理" }));
    await user.click(screen.getByRole("button", { name: "确认清理" }));
    expect(await screen.findByRole("alert")).toBeVisible();
    expect(screen.queryByText("全部范围已核验")).toBeNull();
    expect(onErased).not.toHaveBeenCalled();
  });
});
