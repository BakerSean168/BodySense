import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "@/stores/authStore";
import { HealthDocumentReviewPanel } from "./HealthDocumentReviewPanel";
import type { HealthDocumentReviewContext } from "../../types/health-document-review.types";

const uploadId = "11111111-1111-4111-8111-111111111111";
const runId = "22222222-2222-4222-8222-222222222222";

function context(
  overrides: Partial<
    HealthDocumentReviewContext["review_candidates"][number]
  > = {},
): HealthDocumentReviewContext {
  return {
    extraction_run_id: runId,
    upload_id: uploadId,
    review_candidates: [
      {
        indicator_index: 0,
        indicator_id: "vitamin-d",
        candidate: {
          indicator_index: 0,
          indicator_id: "vitamin-d",
          name: "25-羟维生素D",
          value: 17.8,
          unit: "ng/mL",
          reference_range: "30-100",
          evidence_admissibility: {
            status: "needs_review",
            policy_revision: "ocr-indicator-admissibility-v1",
            reason_codes: ["source_needs_review"],
          },
          source_refs: ["page:2:block:7"],
          source_regions: [
            {
              source_ref: "page:2:block:7",
              page_number: 2,
              bbox: [10, 20, 30, 40],
            },
          ],
        },
        history: [],
        ...overrides,
      },
    ],
  };
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function setAuthenticated() {
  act(() => {
    useAuthStore.setState({
      accessToken: "review-token",
      isAuthenticated: true,
      hasHydrated: true,
      isAuthResolved: true,
    });
  });
}

beforeEach(() => {
  setAuthenticated();
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  act(() => {
    useAuthStore.setState({ accessToken: null, isAuthenticated: false });
  });
});

describe("HealthDocumentReviewPanel", () => {
  it("shows server-grounded page/region context and does not rely on color for state", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(context()));
    vi.stubGlobal("fetch", fetchMock);

    render(<HealthDocumentReviewPanel uploadId={uploadId} />);

    expect(await screen.findByText("人工复核")).toBeInTheDocument();
    expect(
      screen.getByText(/来源：第 2 页 · 区域 10, 20, 30, 40/),
    ).toBeInTheDocument();
    expect(screen.getByText("状态：待确认")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "确认无误" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "修改" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "不是这个指标" })).toBeEnabled();
    expect(screen.getByText(/不代表医学诊断/)).toBeInTheDocument();
  });

  it("fails a historical upload with no persisted current run closed and understandably", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(jsonResponse({ error: "not found" }, 404)),
    );

    render(<HealthDocumentReviewPanel uploadId={uploadId} />);

    expect(
      await screen.findByText(/历史识别结果，缺少可验证的提取运行与来源定位/),
    ).toBeInTheDocument();
    expect(screen.getByText(/不会作为已复核证据使用/)).toBeInTheDocument();
  });

  it("confirms the exact server-owned run/candidate/source refs and refreshes effective state", async () => {
    const confirmed = context({
      effective_review: {
        id: "review-confirm",
        extraction_run_id: runId,
        upload_id: uploadId,
        indicator_index: 0,
        indicator_id: "vitamin-d",
        action: "confirm",
        reviewer_user_id: "33333333-3333-4333-8333-333333333333",
        created_at: "2026-09-03T10:00:00Z",
        idempotency_key: "idem-confirm",
      },
      history: [
        {
          id: "review-confirm",
          extraction_run_id: runId,
          upload_id: uploadId,
          indicator_index: 0,
          indicator_id: "vitamin-d",
          action: "confirm",
          reviewer_user_id: "33333333-3333-4333-8333-333333333333",
          created_at: "2026-09-03T10:00:00Z",
          idempotency_key: "idem-confirm",
        },
      ],
    });
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse(context()))
      .mockResolvedValueOnce(
        jsonResponse(confirmed.review_candidates[0].effective_review, 201),
      )
      .mockResolvedValueOnce(jsonResponse(confirmed));
    vi.stubGlobal("fetch", fetchMock);

    render(<HealthDocumentReviewPanel uploadId={uploadId} />);
    await userEvent.click(
      await screen.findByRole("button", { name: "确认无误" }),
    );

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3));
    const [url, options] = fetchMock.mock.calls[1] as [string, RequestInit];
    expect(url).toContain(`/uploads/${uploadId}/extractions/${runId}/reviews`);
    const body = JSON.parse(String(options.body));
    expect(body).toMatchObject({
      indicator_index: 0,
      indicator_id: "vitamin-d",
      action: "confirm",
      source_refs: ["page:2:block:7"],
    });
    expect(typeof body.idempotency_key).toBe("string");
    expect(await screen.findByText("状态：已确认")).toBeInTheDocument();
    expect(screen.getByText(/查看复核历史（1）/)).toBeInTheDocument();
  });

  it("validates correction locally then appends corrected evidence without overwriting the machine value", async () => {
    const corrected = context({
      effective_review: {
        id: "review-correct",
        extraction_run_id: runId,
        upload_id: uploadId,
        indicator_index: 0,
        indicator_id: "vitamin-d",
        action: "correct",
        reviewed_payload: {
          indicator_id: "vitamin-d",
          name: "25-羟维生素D",
          value: "18.2",
          unit: "ng/mL",
          reference_range: "30-100",
        },
        reviewer_user_id: "33333333-3333-4333-8333-333333333333",
        created_at: "2026-09-03T10:01:00Z",
        idempotency_key: "idem-correct",
      },
      history: [],
    });
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse(context()))
      .mockResolvedValueOnce(
        jsonResponse(corrected.review_candidates[0].effective_review, 201),
      )
      .mockResolvedValueOnce(jsonResponse(corrected));
    vi.stubGlobal("fetch", fetchMock);

    render(<HealthDocumentReviewPanel uploadId={uploadId} />);
    await userEvent.click(await screen.findByRole("button", { name: "修改" }));
    const value = screen.getByLabelText("25-羟维生素D 修正值");
    fireEvent.change(value, { target: { value: "" } });
    await userEvent.click(screen.getByRole("button", { name: "保存修正" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "请输入修正后的指标值",
    );
    expect(fetchMock).toHaveBeenCalledTimes(1);

    fireEvent.change(value, { target: { value: "18.2" } });
    await userEvent.click(screen.getByRole("button", { name: "保存修正" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3));
    const body = JSON.parse(
      String((fetchMock.mock.calls[1]?.[1] as RequestInit).body),
    );
    expect(body.action).toBe("correct");
    expect(body.reviewed_payload).toMatchObject({
      indicator_id: "vitamin-d",
      value: "18.2",
      unit: "ng/mL",
      reference_range: "30-100",
    });
    expect(screen.getAllByText(/机器识别：17.8 ng\/mL/).length).toBeGreaterThan(
      0,
    );
    expect(await screen.findByText("状态：已修正")).toBeInTheDocument();
  });

  it("rejects a candidate through the append-only action and preserves a visible history", async () => {
    const rejected = context({
      effective_review: {
        id: "review-reject",
        extraction_run_id: runId,
        upload_id: uploadId,
        indicator_index: 0,
        indicator_id: "vitamin-d",
        action: "reject",
        reviewer_user_id: "33333333-3333-4333-8333-333333333333",
        created_at: "2026-09-03T10:02:00Z",
        idempotency_key: "idem-reject",
      },
      history: [
        {
          id: "review-old",
          extraction_run_id: runId,
          upload_id: uploadId,
          indicator_index: 0,
          indicator_id: "vitamin-d",
          action: "confirm",
          reviewer_user_id: "33333333-3333-4333-8333-333333333333",
          created_at: "2026-09-03T10:01:00Z",
          idempotency_key: "idem-old",
        },
        {
          id: "review-reject",
          extraction_run_id: runId,
          upload_id: uploadId,
          indicator_index: 0,
          indicator_id: "vitamin-d",
          action: "reject",
          reviewer_user_id: "33333333-3333-4333-8333-333333333333",
          created_at: "2026-09-03T10:02:00Z",
          idempotency_key: "idem-reject",
        },
      ],
    });
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse(context()))
      .mockResolvedValueOnce(
        jsonResponse(rejected.review_candidates[0].effective_review, 201),
      )
      .mockResolvedValueOnce(jsonResponse(rejected));
    vi.stubGlobal("fetch", fetchMock);

    render(<HealthDocumentReviewPanel uploadId={uploadId} />);
    await userEvent.click(
      await screen.findByRole("button", { name: "不是这个指标" }),
    );

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3));
    const body = JSON.parse(
      String((fetchMock.mock.calls[1]?.[1] as RequestInit).body),
    );
    expect(body.action).toBe("reject");
    expect(await screen.findByText("状态：已排除")).toBeInTheDocument();
    expect(screen.getByText(/查看复核历史（2）/)).toBeInTheDocument();
  });

  it("opens the authenticated original source at the reported PDF page", async () => {
    const pdf = new Blob(["pdf"], { type: "application/pdf" });
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse(context()))
      .mockResolvedValueOnce(
        new Response(pdf, {
          status: 200,
          headers: { "Content-Type": "application/pdf" },
        }),
      );
    const open = vi.fn(() => ({}) as Window);
    vi.stubGlobal("fetch", fetchMock);
    vi.stubGlobal("open", open);
    Object.defineProperty(URL, "createObjectURL", {
      configurable: true,
      value: vi.fn(() => "blob:report"),
    });
    Object.defineProperty(URL, "revokeObjectURL", {
      configurable: true,
      value: vi.fn(),
    });

    render(<HealthDocumentReviewPanel uploadId={uploadId} />);
    await userEvent.click(
      await screen.findByRole("button", { name: "查看原始位置" }),
    );

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    expect(open).toHaveBeenCalledWith(
      "blob:report#page=2",
      "_blank",
      "noopener,noreferrer",
    );
  });
});
