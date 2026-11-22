import { describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useUploadStore } from "@/stores/uploadStore";
import {
  chooseCategory,
  mockListBodyFacts,
  mockListEquipment,
  mockRequestErasure,
  mockUpdateEquipment,
  openDock,
  renderPage,
} from "./businessWindowsSetup";

describe("body-first business windows: Person data", () => {
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
    // Only the original has a knowable count before erasure.
    expect(within(scene).getByText("1 份")).toBeVisible();
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

  it("shows a centred equipment empty state and adds only from the header", async () => {
    renderPage();
    await openDock("个人档案");
    await screen.findByTestId("person-window");
    await chooseCategory("器材");
    // Penpot "Person / Equipment · Empty": icon, title and one short line.
    expect(await screen.findByText("还没有器材")).toBeInTheDocument();
    expect(mockListEquipment).toHaveBeenCalled();
    const region = screen.getByRole("region", { name: "器材清单" });
    expect(within(region).queryAllByRole("button")).toHaveLength(0);
    await userEvent.click(screen.getByRole("button", { name: "添加器材" }));
    expect(
      await screen.findByRole("dialog", { name: "添加器材" }),
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
    const available = screen.getByRole("switch", { name: "弹力带 可用" });
    expect(available).toBeChecked();
    await userEvent.click(available);
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
});
