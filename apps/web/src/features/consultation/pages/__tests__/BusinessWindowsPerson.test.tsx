import { describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useUploadStore } from "@/stores/uploadStore";
import {
  chooseCategory,
  mockRequestErasure,
  openDock,
  renderPage,
} from "./businessWindowsSetup";

describe("body-first business windows: Person", () => {
  it("opens the Person window with five categories and the identity popover", async () => {
    renderPage();
    await openDock("个人档案");
    // First test to open Person: its lazy chunk loads cold, so allow longer.
    const person = await screen.findByTestId(
      "person-window",
      {},
      { timeout: 5000 },
    );
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
    // No nickname: the header shows the account email's local part (Penpot
    // draws a name), with the full address only in the title.
    const identity = screen.getByRole("button", { name: "synthetic" });
    expect(identity).toHaveAttribute("data-name-source", "account");
    expect(identity).not.toHaveTextContent("@");
    expect(identity).toHaveAttribute(
      "title",
      "账户菜单 · synthetic@example.test",
    );
    await userEvent.click(identity);
    // Penpot account menu: 个人资料 opens the identity popover.
    await userEvent.click(
      await screen.findByRole("menuitem", { name: "个人资料" }),
    );
    const popover = await screen.findByRole("dialog", { name: "个人资料" });
    // Minimal optional Profile only: no body measurements and no revision.
    expect(popover.textContent).not.toContain("BMI");
    expect(popover.textContent).not.toContain("AI 猜测");
    expect(screen.getByLabelText("昵称")).toHaveValue("");
    expect(screen.getByLabelText("昵称")).toHaveAttribute(
      "placeholder",
      "synthetic",
    );
    expect(screen.getByRole("combobox", { name: "自报性别" })).toHaveAttribute(
      "data-value",
      "",
    );
    expect(
      screen.getByRole("combobox", { name: "显示时区" }),
    ).toHaveTextContent(/^跟随设备/);
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
    // Counts only for data already read: uploads, and the fully loaded
    // body-fact ledger (records = every fact, habits = lifestyle facts).
    expect(options[2]).toHaveTextContent("0 个");
    expect(options[3]).toHaveTextContent("1 份");
    await waitFor(() => expect(options[0]).toHaveTextContent("1 条"));
    expect(options[4]).toHaveTextContent("1 项");
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

  it("uploads a report from the header + and offers retry on failure", async () => {
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
    expect(
      await screen.findByRole("button", { name: "上传照片" }),
    ).toBeEnabled();
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
    // Penpot footer: the other scope is the quiet left action.
    expect(
      within(scene).getByRole("button", { name: "完整清理" }),
    ).toBeInTheDocument();
    expect(within(scene).getByRole("button", { name: "取消" })).toBeVisible();
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
});
