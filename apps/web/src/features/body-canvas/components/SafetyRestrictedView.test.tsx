import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { expect, it, vi } from "vitest";
import SafetyRestrictedView from "./SafetyRestrictedView";
const list = vi.hoisted(() => vi.fn());
vi.mock("@/features/consultation/services/safetyRestrictionService", () => ({
  safetyRestrictionKeys: { all: ["safety-test"] },
  safetyRestrictionService: { list },
}));
it("retries a failed safety read without presenting it as no active restrictions", async () => {
  list
    .mockRejectedValueOnce(new Error("HTTP 503"))
    .mockResolvedValue({ restrictions: [] });
  render(
    <QueryClientProvider client={new QueryClient()}>
      <SafetyRestrictedView onBack={vi.fn()} onAddReview={vi.fn()} />
    </QueryClientProvider>,
  );
  expect(await screen.findByRole("alert")).toBeVisible();
  expect(
    screen.queryByTestId("safety-restricted-none"),
  ).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "重新读取" }));
  expect(await screen.findByTestId("safety-restricted-none")).toBeVisible();
  expect(list).toHaveBeenCalledTimes(2);
});
