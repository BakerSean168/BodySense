import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { PrivacyPanel } from "./PrivacyPanel";
import { privacyApi } from "../../services/privacyService";

vi.mock("../../services/privacyService", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../services/privacyService")>();
  return {
    ...actual,
    privacyApi: {
      getErasurePlan: vi.fn(),
      requestErasure: vi.fn(),
    },
  };
});

const plan = {
  destructive: true as const,
  confirmation_phrase: "DELETE ALL BODY DATA",
  counts: [
    { name: "account", count: 1 },
    { name: "uploads", count: 3 },
  ],
  retained_audit: ["anonymous erasure request status/timestamps"],
};

describe("PrivacyPanel", () => {
  beforeEach(() => {
    vi.mocked(privacyApi.getErasurePlan).mockReset();
    vi.mocked(privacyApi.requestErasure).mockReset();
  });

  it("distinguishes chat deletion from full longitudinal health-data erasure", () => {
    render(<PrivacyPanel onErasureAccepted={vi.fn()} />);
    expect(
      screen.getByText(/删除会话只会清除聊天历史和对应分享/),
    ).toBeInTheDocument();
    expect(screen.getByText(/BodyState、诊断分析、治疗方案/)).toBeInTheDocument();
  });

  it("requires a dry-run and exact confirmation phrase before accepting erasure", async () => {
    vi.mocked(privacyApi.getErasurePlan).mockResolvedValue(plan);
    vi.mocked(privacyApi.requestErasure).mockResolvedValue({
      request_id: "request-1",
      status: "completed",
      message: "accepted",
    });
    const accepted = vi.fn();
    render(<PrivacyPanel onErasureAccepted={accepted} />);

    fireEvent.click(screen.getByRole("button", { name: /删除全部数据$/ }));

    expect(await screen.findByText(/当前 dry-run 识别到 4 条直接归属记录/)).toBeInTheDocument();
    expect(privacyApi.getErasurePlan).toHaveBeenCalledTimes(1);

    const submit = screen.getByRole("button", { name: "确认并永久删除" });
    expect(submit).toBeDisabled();

    fireEvent.change(screen.getByLabelText(/输入 DELETE ALL BODY DATA 以确认/), {
      target: { value: "DELETE ALL BODY DATA" },
    });
    expect(submit).toBeEnabled();
    fireEvent.click(submit);

    await waitFor(() => {
      expect(privacyApi.requestErasure).toHaveBeenCalledWith("DELETE ALL BODY DATA");
      expect(accepted).toHaveBeenCalledTimes(1);
    });
  });
});
