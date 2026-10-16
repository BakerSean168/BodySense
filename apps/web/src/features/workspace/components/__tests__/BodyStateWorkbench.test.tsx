import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { readBodyRecords } from "@/features/profile/services/bodyRecordsService";
import { bodyRecordsFixture } from "@/features/profile/services/__fixtures__/bodyRecordsFixture";
import { BodyStateWorkbench } from "../BodyStateWorkbench";

vi.mock("@/features/profile/services/bodyRecordsService", () => ({
  readBodyRecords: vi.fn(),
}));
vi.mock("@/stores/authStore", () => ({
  useAuthStore: (select: (state: { user: { id: string } }) => unknown) =>
    select({ user: { id: "owner" } }),
}));
vi.mock("@/features/body-canvas/components/BodyFactRecords", () => ({
  BodyFactRecords: () => <div>用户陈述编辑</div>,
}));
vi.mock("@/features/body-canvas/components/BodyEpisodeAssociations", () => ({
  BodyEpisodeAssociations: () => <div>Episode 关联</div>,
}));

function renderWorkbench() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <BodyStateWorkbench canEdit />
    </QueryClientProvider>,
  );
}

beforeEach(() => vi.resetAllMocks());
describe("native BodyStateWorkbench", () => {
  it("shows original observation and raw measurement without promoting it to a fact", async () => {
    const records = bodyRecordsFixture();
    records.observations = [
      {
        id: "observation",
        user_id: "owner",
        kind: "weight",
        statement: "今天称重72公斤",
        raw_value: "72",
        raw_unit: "kg",
        observed_at: { precision: "unknown" },
        source: { kind: "user_report", id: "source", version: 1 },
        assertion_status: "asserted",
        created_revision: 4,
        recorded_at: "2026-10-16T00:00:00Z",
      },
    ];
    vi.mocked(readBodyRecords).mockResolvedValue(records);
    renderWorkbench();
    expect(await screen.findByText("今天称重72公斤")).toBeInTheDocument();
    expect(screen.getByText("72 · kg")).toBeInTheDocument();
    expect(screen.queryByText("已确认记录")).not.toBeInTheDocument();
  });
  it("shows a failed read with retry instead of an empty ledger", async () => {
    vi.mocked(readBodyRecords).mockRejectedValue(new Error("读取失败"));
    renderWorkbench();
    expect(await screen.findByRole("alert")).toHaveTextContent("读取失败");
    expect(screen.getByRole("button", { name: "重试" })).toBeInTheDocument();
    expect(screen.queryByText("暂无观察记录")).not.toBeInTheDocument();
  });
});
