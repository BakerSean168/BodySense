import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { EquipmentCategory } from "./EquipmentCategory";

const service = vi.hoisted(() => ({
  list: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
}));
const toast = vi.hoisted(() => vi.fn());

vi.mock("@/features/profile/services/equipmentService", () => ({
  equipmentKeys: { all: ["equipment"] },
  equipmentService: service,
  availabilityChange: (asset: { revision: number }, availability: string) => ({
    expected_revision: asset.revision,
    availability,
  }),
}));
vi.mock("sonner", () => ({ toast }));

const band = {
  id: "band-1",
  display_name: "弹力带",
  notes: "50 磅",
  availability: "available",
  access_kind: "owned",
  revision: 2,
  valid_from: null,
  valid_until: null,
};

/** Stands in for the Person window header "+". */
function WithHeaderAdd() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button type="button" onClick={() => setOpen(true)}>
        添加器材
      </button>
      <EquipmentCategory canEdit addOpen={open} onAddOpenChange={setOpen} />
    </>
  );
}

function mount() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <WithHeaderAdd />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  service.list.mockReset();
  service.create.mockReset();
  service.update.mockReset();
  toast.mockReset();
  service.list.mockResolvedValue({ items: [] });
  service.create.mockResolvedValue({ id: "demo-asset" });
  service.update.mockResolvedValue({});
});

describe("EquipmentCategory", () => {
  it("has no add button of its own and adds through the header dialog", async () => {
    const user = userEvent.setup();
    mount();
    await screen.findByText("还没有器材");
    expect(screen.getAllByRole("button")).toHaveLength(1);
    await user.click(screen.getByRole("button", { name: "添加器材" }));
    const dialog = await screen.findByRole("dialog", { name: "添加器材" });
    expect(dialog).toBeVisible();
    await user.type(screen.getByPlaceholderText("例如：弹力带"), "弹力带");
    await user.type(screen.getByPlaceholderText("阻力、重量等"), "轻阻力");
    await user.click(screen.getByRole("button", { name: "添加" }));
    await waitFor(() =>
      expect(service.create.mock.calls[0]?.[0]).toEqual(
        expect.objectContaining({
          display_name: "弹力带",
          notes: "轻阻力",
          availability: "available",
        }),
      ),
    );
    await waitFor(() =>
      expect(
        screen.queryByRole("dialog", { name: "添加器材" }),
      ).not.toBeInTheDocument(),
    );
  });

  it("toggles availability with a Switch and dims the card", async () => {
    service.list.mockResolvedValue({
      items: [{ ...band, availability: "unavailable" }],
    });
    const user = userEvent.setup();
    mount();
    const toggle = await screen.findByRole("switch", { name: "弹力带 可用" });
    expect(toggle).not.toBeChecked();
    expect(screen.getByTestId("equipment-card")).toHaveAttribute(
      "data-available",
      "false",
    );
    expect(screen.queryByText(/当前可用|暂不可用|已登记/)).toBeNull();
    await user.click(toggle);
    await waitFor(() =>
      expect(service.update).toHaveBeenCalledWith("band-1", {
        expected_revision: 2,
        availability: "available",
      }),
    );
  });

  it("removes from the ⋯ menu right away and retires only after the undo window", async () => {
    service.list.mockResolvedValue({ items: [band] });
    const user = userEvent.setup();
    mount();
    await screen.findByText("弹力带");
    await user.click(screen.getByRole("button", { name: "更多操作：弹力带" }));
    await user.click(await screen.findByRole("menuitem", { name: "移除" }));
    await waitFor(() => expect(screen.queryByText("弹力带")).toBeNull());
    expect(service.update).not.toHaveBeenCalled();
    const [message, options] = toast.mock.calls[0] as [
      string,
      {
        action: { label: string; onClick: () => void };
        onAutoClose: () => void;
      },
    ];
    expect(message).toBe("已移除“弹力带”");
    expect(options.action.label).toBe("撤销");

    act(() => options.action.onClick());
    expect(await screen.findByText("弹力带")).toBeVisible();
    act(() => options.onAutoClose());
    expect(service.update).not.toHaveBeenCalled();
  });

  it("commits the retirement when the toast closes without undo", async () => {
    service.list.mockResolvedValue({ items: [band] });
    const user = userEvent.setup();
    mount();
    await screen.findByText("弹力带");
    await user.click(screen.getByRole("button", { name: "更多操作：弹力带" }));
    await user.click(await screen.findByRole("menuitem", { name: "移除" }));
    const options = toast.mock.calls[0]?.[1] as { onAutoClose: () => void };
    act(() => options.onAutoClose());
    await waitFor(() =>
      expect(service.update).toHaveBeenCalledWith("band-1", {
        expected_revision: 2,
        availability: "retired",
      }),
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
