import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { EpisodicNotesCategory } from "./EpisodicNotesCategory";

const service = vi.hoisted(() => ({
  list: vi.fn(),
  delete: vi.fn(),
}));

vi.mock("@/features/profile/services/episodicNotesService", () => ({
  episodicNotesKeys: { all: ["episodic-notes"] },
  episodicNotesService: service,
}));

const note = {
  id: "note-1",
  contentText: "用户偏好早上训练。",
  fromSeq: 3,
  toSeq: 8,
  status: "shadow" as const,
  generatorVersion: "episodic-generator-v1",
  retentionPolicyVersion: "episodic-retention-v1",
  createdAt: "2026-11-05T12:00:00",
  updatedAt: "2026-11-05T12:00:00",
};

function mount() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <EpisodicNotesCategory />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  service.list.mockReset();
  service.delete.mockReset();
  service.list.mockResolvedValue({ notes: [] });
  service.delete.mockResolvedValue({
    noteId: note.id,
    deleted: true,
    message: "episodic note removed",
  });
});

describe("EpisodicNotesCategory", () => {
  it("shows content, date, source range and non-serving status", async () => {
    service.list.mockResolvedValue({ notes: [note] });
    mount();
    expect(await screen.findByText(note.contentText)).toBeVisible();
    expect(screen.getByText("2026/11/05 · 对话第 3–8 条")).toBeVisible();
    expect(screen.getByText("仅保存，不参与对话")).toBeVisible();
    expect(screen.queryByTestId("person-add")).toBeNull();
  });

  it("uses an honest empty state that is not a clinical or erasure claim", async () => {
    mount();
    expect(await screen.findByText("还没有记忆")).toBeVisible();
    expect(
      screen.getByText(
        "对话摘要会出现在这里；它们是对话记忆，不是身体记录，也不会用于诊断。",
      ),
    ).toBeVisible();
  });

  it("surfaces a read failure and retries", async () => {
    service.list.mockRejectedValueOnce(new Error("读取失败"));
    const user = userEvent.setup();
    mount();
    expect(await screen.findByText("记忆暂时无法读取")).toBeVisible();
    expect(screen.getByText("读取失败")).toBeVisible();
    service.list.mockResolvedValueOnce({ notes: [] });
    await user.click(screen.getByRole("button", { name: "重新读取" }));
    expect(await screen.findByText("还没有记忆")).toBeVisible();
  });

  it("confirms the exact permanent-deletion consequences and removes the row", async () => {
    service.list.mockResolvedValue({ notes: [note] });
    const user = userEvent.setup();
    mount();
    await screen.findByText(note.contentText);
    await user.click(
      screen.getByRole("button", { name: /^删除记忆：2026\/11\/05/ }),
    );
    const dialog = await screen.findByRole("dialog", {
      name: "删除这条记忆？",
    });
    expect(dialog).toBeVisible();
    expect(
      screen.getByText(
        "删除后这条记忆会被永久移除，且不会被重新生成。这不会删除身体记录等临床事实，也不是账号数据清除。删除不可撤销。",
      ),
    ).toBeVisible();
    await user.click(screen.getByRole("button", { name: "永久删除" }));
    await waitFor(() => expect(service.delete).toHaveBeenCalledWith(note.id));
    await waitFor(() =>
      expect(screen.queryByText(note.contentText)).toBeNull(),
    );
    expect(
      screen.queryByRole("dialog", { name: "删除这条记忆？" }),
    ).not.toBeInTheDocument();
  });

  it("keeps the row when the server rejects the deletion", async () => {
    service.list.mockResolvedValue({ notes: [note] });
    service.delete.mockRejectedValueOnce(new Error("删除失败"));
    const user = userEvent.setup();
    mount();
    await screen.findByText(note.contentText);
    await user.click(
      screen.getByRole("button", { name: /^删除记忆：2026\/11\/05/ }),
    );
    await user.click(await screen.findByRole("button", { name: "永久删除" }));
    expect(await screen.findByText("删除失败")).toBeVisible();
    expect(screen.getByText(note.contentText)).toBeVisible();
  });
});
