import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  TrainerEntryGuard,
  type TrainerEntryGuardProps,
} from "./TrainerEntryGuard";

vi.mock("@/features/training/components/TrainingExecutionPanel", () => ({
  TrainingExecutionPanel: ({ planId }: { planId: string }) => (
    <div data-testid="training-execution-panel">{planId}</div>
  ),
}));

const allowed: TrainerEntryGuardProps = {
  planId: "accepted-1",
  launchedPlanId: "accepted-1",
  accepted: true,
  canExecute: true,
  safetyBlocked: false,
  loading: false,
  error: null,
  onOpenTraining: vi.fn(),
};

describe("V2 Trainer entry admission", () => {
  it("does not mount the execution panel from a deep link or single-key shortcut alone", async () => {
    const goToPlans = vi.fn();
    render(
      <TrainerEntryGuard
        {...allowed}
        launchedPlanId={null}
        onOpenTraining={goToPlans}
      />,
    );
    expect(screen.getByTestId("trainer-entry-guard")).toBeVisible();
    expect(
      screen.queryByTestId("training-execution-panel"),
    ).not.toBeInTheDocument();
    expect(screen.getByText(/先在训练计划中选择/)).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: /查看训练计划/ }));
    expect(goToPlans).toHaveBeenCalledTimes(1);
  });

  it("rejects a draft, another selected plan, missing backend permission or an active safety hold", () => {
    const { rerender } = render(
      <TrainerEntryGuard {...allowed} accepted={false} />,
    );
    expect(screen.getByTestId("trainer-entry-guard")).toBeVisible();
    rerender(<TrainerEntryGuard {...allowed} planId="different-plan" />);
    expect(screen.getByTestId("trainer-entry-guard")).toBeVisible();
    rerender(<TrainerEntryGuard {...allowed} canExecute={false} />);
    expect(screen.getByText(/未授予训练执行权限/)).toBeVisible();
    rerender(<TrainerEntryGuard {...allowed} safetyBlocked />);
    expect(screen.getByRole("alert")).toHaveTextContent(/不允许开始或继续训练/);
    expect(
      screen.queryByTestId("training-execution-panel"),
    ).not.toBeInTheDocument();
  });

  it("fails closed on loading and transport failure", () => {
    const { rerender } = render(<TrainerEntryGuard {...allowed} loading />);
    expect(screen.getByText(/正在核对当前训练计划/)).toBeVisible();
    expect(
      screen.queryByTestId("training-execution-panel"),
    ).not.toBeInTheDocument();
    rerender(<TrainerEntryGuard {...allowed} error="计划请求超时" />);
    expect(screen.getByText(/暂时无法读取/)).toBeVisible();
    expect(
      screen.queryByTestId("training-execution-panel"),
    ).not.toBeInTheDocument();
  });

  it("only mounts execution after an exact accepted, permitted and explicit launch", () => {
    const { rerender } = render(<TrainerEntryGuard {...allowed} />);
    expect(screen.queryByTestId("trainer-entry-guard")).not.toBeInTheDocument();
    expect(screen.getByTestId("training-execution-panel")).toHaveTextContent(
      "accepted-1",
    );
    rerender(<TrainerEntryGuard {...allowed} planId="accepted-2" />);
    expect(
      screen.queryByTestId("training-execution-panel"),
    ).not.toBeInTheDocument();
    rerender(
      <TrainerEntryGuard
        {...allowed}
        planId="accepted-2"
        launchedPlanId="accepted-2"
      />,
    );
    expect(screen.getByTestId("training-execution-panel")).toHaveTextContent(
      "accepted-2",
    );
  });
});
