import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { toast } from "sonner";
import { BodyMetricsPanel } from "./BodyMetricsPanel";
import { LifestylePanel } from "./LifestylePanel";
import { HealthHistoryPanel } from "./HealthHistoryPanel";
import { ProfileDrawer } from "./ProfileDrawer";
import { bodyMetricsService } from "../../services/bodyMetricsService";
import { lifestyleService } from "../../services/lifestyleService";
import { healthHistoryService } from "../../services/healthHistoryService";
import { useProfileStore } from "@/stores/profileStore";
import { useUploadStore } from "@/stores/uploadStore";

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), warning: vi.fn(), success: vi.fn() },
}));

vi.mock("../../services/bodyMetricsService", () => ({
  bodyMetricsService: { get: vi.fn(), update: vi.fn() },
}));

vi.mock("../../services/lifestyleService", () => ({
  lifestyleService: { get: vi.fn(), update: vi.fn() },
}));

vi.mock("../../services/healthHistoryService", () => ({
  healthHistoryService: {
    getInjuryHistory: vi.fn(),
    updateInjuryHistory: vi.fn(),
  },
}));

const metricsSnapshot = {
  current_revision: 1,
  height: { value: 172, unit: "cm" },
  weight: { value: 68, unit: "kg" },
  bmi: 23,
};

const lifestyleSection = {
  kind: "lifestyle",
  summary: "",
  details: {},
};

const lifestyleSnapshot = {
  current_revision: 1,
  activity: lifestyleSection,
  sleep: lifestyleSection,
  exercise: lifestyleSection,
  nutrition: lifestyleSection,
  substances: lifestyleSection,
  recovery: lifestyleSection,
};

const injurySnapshot = {
  current_revision: 1,
  summary: "2024 年左膝轻微拉伤",
};

describe("profile panel error feedback", () => {
  beforeEach(() => {
    vi.mocked(toast.error).mockClear();
    vi.mocked(toast.warning).mockClear();
  });

  afterEach(() => {
    act(() => {
      useProfileStore.setState({
        profile: null,
        isLoading: false,
        error: null,
      });
      useUploadStore.setState({ uploads: [] });
    });
  });

  it("keeps a body metrics load failure recoverable instead of a dead red block", async () => {
    vi.mocked(bodyMetricsService.get)
      .mockRejectedValueOnce(new Error("网络中断"))
      .mockResolvedValueOnce(metricsSnapshot);

    render(<BodyMetricsPanel />);

    expect(await screen.findByText("网络中断")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "重试" }));

    await waitFor(() => {
      expect(screen.queryByText("网络中断")).toBeNull();
    });
    expect(bodyMetricsService.get).toHaveBeenCalledTimes(2);
    expect(
      screen.getByRole("button", { name: /更新测量/ }),
    ).toBeInTheDocument();
  });

  it("reports a body metrics save failure through a toast, not the content flow", async () => {
    vi.mocked(bodyMetricsService.get).mockResolvedValue(metricsSnapshot);
    vi.mocked(bodyMetricsService.update).mockRejectedValue(
      new Error("保存冲突"),
    );

    render(<BodyMetricsPanel />);
    fireEvent.click(await screen.findByRole("button", { name: /更新测量/ }));
    fireEvent.click(screen.getByRole("button", { name: "保存测量" }));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("保存冲突");
    });
    expect(screen.queryByText("保存冲突")).toBeNull();
    // The form stays open so the user can retry without retyping.
    expect(
      screen.getByRole("button", { name: "保存测量" }),
    ).toBeInTheDocument();
  });

  it("keeps body metrics range validation inline next to the fields", async () => {
    vi.mocked(bodyMetricsService.get).mockResolvedValue(metricsSnapshot);

    render(<BodyMetricsPanel />);
    fireEvent.click(await screen.findByRole("button", { name: /更新测量/ }));
    fireEvent.change(screen.getByLabelText("身高 (cm)"), {
      target: { value: "10" },
    });
    fireEvent.click(screen.getByRole("button", { name: "保存测量" }));

    expect(
      await screen.findByText("身高必须在 50-250 cm 之间"),
    ).toBeInTheDocument();
    expect(toast.error).not.toHaveBeenCalled();
  });

  it("recovers a lifestyle load failure with 重试", async () => {
    vi.mocked(lifestyleService.get)
      .mockRejectedValueOnce(new Error("生活方式读取失败"))
      .mockResolvedValueOnce(lifestyleSnapshot);

    render(<LifestylePanel />);

    expect(await screen.findByText("生活方式读取失败")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "重试" }));

    await waitFor(() => {
      expect(screen.queryByText("生活方式读取失败")).toBeNull();
    });
    expect(lifestyleService.get).toHaveBeenCalledTimes(2);
    expect(screen.getByText("日常活动")).toBeInTheDocument();
  });

  it("reports a lifestyle save failure through a toast", async () => {
    vi.mocked(lifestyleService.get).mockResolvedValue(lifestyleSnapshot);
    vi.mocked(lifestyleService.update).mockRejectedValue(
      new Error("生活方式保存失败"),
    );

    render(<LifestylePanel />);
    fireEvent.click(
      await screen.findByRole("button", { name: /编辑生活方式/ }),
    );
    fireEvent.click(screen.getByRole("button", { name: "保存变化" }));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("生活方式保存失败");
    });
    expect(screen.queryByText("生活方式保存失败")).toBeNull();
  });

  it("recovers a health history load failure with 重试", async () => {
    vi.mocked(healthHistoryService.getInjuryHistory)
      .mockRejectedValueOnce(new Error("健康历史读取失败"))
      .mockResolvedValueOnce(injurySnapshot);

    render(<HealthHistoryPanel />);

    expect(await screen.findByText("健康历史读取失败")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "重试" }));

    await waitFor(() => {
      expect(screen.queryByText("健康历史读取失败")).toBeNull();
    });
    expect(healthHistoryService.getInjuryHistory).toHaveBeenCalledTimes(2);
    expect(screen.getByText("2024 年左膝轻微拉伤")).toBeInTheDocument();
  });

  it("reports a health history save failure through a toast", async () => {
    vi.mocked(healthHistoryService.getInjuryHistory).mockResolvedValue(
      injurySnapshot,
    );
    vi.mocked(healthHistoryService.updateInjuryHistory).mockRejectedValue(
      new Error("健康历史保存失败"),
    );

    render(<HealthHistoryPanel />);
    fireEvent.click(await screen.findByRole("button", { name: "编辑" }));
    fireEvent.click(screen.getByRole("button", { name: "保存变化" }));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("健康历史保存失败");
    });
    expect(screen.queryByText("健康历史保存失败")).toBeNull();
  });

  it("offers a profile refetch instead of a dismiss-only block", async () => {
    const fetchProfile = vi.fn().mockResolvedValue(undefined);
    useProfileStore.setState({
      profile: null,
      isLoading: false,
      error: "身体档案读取失败",
      fetchProfile,
    });
    useUploadStore.setState({ uploads: [], fetchUploads: vi.fn() });

    render(
      <MemoryRouter>
        <ProfileDrawer open onOpenChange={() => {}} />
      </MemoryRouter>,
    );

    expect(screen.getByText("身体档案读取失败")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "重试" }));

    await waitFor(() => {
      expect(fetchProfile).toHaveBeenCalled();
    });
    expect(screen.queryByRole("button", { name: "关闭" })).toBeNull();
  });

  it("reports a profile save failure through a toast and keeps editing", async () => {
    const saveProfile = vi.fn().mockRejectedValue(new Error("资料保存失败"));
    useProfileStore.setState({
      profile: {
        id: "profile-1",
        user_id: "user-1",
        display_name: "测试",
        gender: "male",
        birth_date: "1990-01-01",
        preferences: {},
        revision: 1,
        created_at: "2026-01-01T00:00:00Z",
        updated_at: "2026-01-01T00:00:00Z",
      },
      isLoading: false,
      error: null,
      fetchProfile: vi.fn().mockResolvedValue(undefined),
      saveProfile,
      updateProfile: async (data) => {
        await saveProfile({ ...data });
      },
    });
    useUploadStore.setState({ uploads: [], fetchUploads: vi.fn() });

    render(
      <MemoryRouter>
        <ProfileDrawer open onOpenChange={() => {}} />
      </MemoryRouter>,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: /编辑基本信息/ }),
    );
    fireEvent.click(screen.getByRole("button", { name: "保存" }));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("资料保存失败");
    });
    expect(screen.getByRole("button", { name: "保存" })).toBeInTheDocument();
  });
});
