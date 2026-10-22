import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { EquipmentCategory } from "./EquipmentCategory";

const service = vi.hoisted(() => ({
  list: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
}));

vi.mock("@/features/profile/services/equipmentService", () => ({
  equipmentKeys: { all: ["equipment"] },
  equipmentService: service,
  availabilityChange: vi.fn(),
}));

function mount() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <EquipmentCategory canEdit />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  service.list.mockReset();
  service.create.mockReset();
  service.update.mockReset();
  service.list.mockResolvedValue({ items: [] });
  service.create.mockResolvedValue({ id: "demo-asset" });
});

describe("EquipmentCategory", () => {
  it("adds equipment using the extracted domain hook", async () => {
    const user = userEvent.setup();
    mount();
    await screen.findByText("还没有器材");
    await user.click(screen.getByRole("button", { name: "添加个人器材" }));
    await user.type(screen.getByPlaceholderText("例如：弹力带"), "弹力带");
    await user.type(
      screen.getByPlaceholderText("阻力、重量或使用限制"),
      "轻阻力",
    );
    await user.click(screen.getByRole("button", { name: "添加设备" }));
    await waitFor(() =>
      expect(service.create.mock.calls[0]?.[0]).toEqual(
        expect.objectContaining({
          display_name: "弹力带",
          notes: "轻阻力",
          availability: "available",
        }),
      ),
    );
  });

  it("shows a recoverable category error when the equipment read fails", async () => {
    service.list.mockReset();
    service.list
      .mockRejectedValueOnce(new Error("暂时不可用"))
      .mockResolvedValue({ items: [] });

    const user = userEvent.setup();
    mount();
    await screen.findByText("器材清单暂时无法读取");
    await user.click(screen.getByRole("button", { name: "重新读取" }));
    await screen.findByText("还没有器材");
    expect(service.list).toHaveBeenCalledTimes(2);
  });
});
