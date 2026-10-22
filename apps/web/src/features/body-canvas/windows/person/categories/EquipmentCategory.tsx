import {
  Backpack,
  Dumbbell,
  LoaderCircle,
  MoreHorizontal,
  Trash2,
} from "lucide-react";
import { errorMessage } from "@/lib/api-client";
import { Switch } from "@/components/ui/switch";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useEquipmentInventory } from "../hooks/useEquipmentInventory";
import { PersonFormDialog } from "../PersonFormDialog";
import { PersonEmptyState } from "./PersonEmptyState";
import { PersonDataError } from "./PersonDataError";
import { Input } from "@/components/ui/Input";
import { Textarea } from "@/components/ui/textarea";
import { IconTip } from "@/components/ui/tooltip";

/**
 * Equipment is the user's own EquipmentAsset inventory (PC-D2), read from
 * /api/v1/equipment — not BodyState. Cards grow with the data (4 columns
 * desktop / 2 mobile). Adding happens only from the window header "+", which
 * opens the add dialog; each card has an availability Switch (an unavailable
 * card is dimmed) and a "⋯" menu whose 移除 retires the item after an undo
 * toast. None of this edits an accepted plan.
 */
export function EquipmentCategory({
  canEdit,
  addOpen = false,
  onAddOpenChange,
}: {
  canEdit: boolean;
  /** Controlled by the window header "+". */
  addOpen?: boolean;
  onAddOpenChange?: (open: boolean) => void;
}) {
  const {
    inventory,
    equipment,
    pending,
    busyId,
    error,
    name,
    changeName,
    notes,
    changeNotes,
    isCreating,
    resetForm,
    add,
    changeAvailability,
    remove,
  } = useEquipmentInventory(canEdit);

  const closeAdd = () => {
    resetForm();
    onAddOpenChange?.(false);
  };
  const dialog = (
    <PersonFormDialog
      open={addOpen && canEdit}
      onOpenChange={(open) => (open ? onAddOpenChange?.(true) : closeAdd())}
      title="添加器材"
      submitLabel="添加"
      submitting={isCreating}
      canSubmit={canEdit && !pending && Boolean(name.trim())}
      error={addOpen ? error : null}
      onSubmit={() => {
        void add().then((saved) => {
          if (saved) onAddOpenChange?.(false);
        });
      }}
    >
      <label className="bc-form-dialog-field">
        名称
        <Input
          value={name}
          onChange={(event) => changeName(event.target.value)}
          maxLength={80}
          placeholder="例如：弹力带"
          disabled={isCreating}
          autoFocus
        />
      </label>
      <label className="bc-form-dialog-field">
        备注<span className="bc-optional">可选</span>
        <Textarea
          value={notes}
          onChange={(event) => changeNotes(event.target.value)}
          maxLength={240}
          rows={2}
          placeholder="阻力、重量等"
          disabled={isCreating}
        />
      </label>
    </PersonFormDialog>
  );

  if (inventory.isPending) {
    return (
      <p role="status" className="bc-person-loading">
        <LoaderCircle size={16} className="animate-spin" />
        正在读取器材
      </p>
    );
  }
  if (inventory.isError) {
    return (
      <PersonDataError
        title="器材清单暂时无法读取"
        message={errorMessage(inventory.error, "请稍后重试。")}
        onRetry={() => void inventory.refetch()}
      />
    );
  }

  const listError =
    error && !addOpen ? (
      <p role="alert" className="bc-error">
        {error}
      </p>
    ) : null;

  if (equipment.length === 0) {
    return (
      <>
        <PersonEmptyState
          icon={<Dumbbell size={24} />}
          title="还没有器材"
          message="训练会按你的器材安排"
        />
        {listError}
        {dialog}
      </>
    );
  }

  return (
    <div className="bc-person-category">
      <ul className="bc-person-equipment">
        {equipment.map((asset) => {
          const available = asset.availability === "available";
          const busy = busyId === asset.id;
          return (
            <li
              key={asset.id}
              data-available={available ? "true" : "false"}
              data-testid="equipment-card"
            >
              <div className="bc-person-equipment-head">
                <span className="bc-person-equipment-symbol" aria-hidden="true">
                  {busy ? (
                    <LoaderCircle size={20} className="animate-spin" />
                  ) : (
                    <Backpack size={20} />
                  )}
                </span>
                <Switch
                  checked={available}
                  disabled={!canEdit || pending || busy}
                  aria-label={`${asset.display_name} 可用`}
                  title={available ? "可用" : "暂不可用"}
                  onCheckedChange={(checked) =>
                    void changeAvailability(
                      asset,
                      checked ? "available" : "unavailable",
                    )
                  }
                />
                <DropdownMenu>
                  <IconTip label="更多">
                    <DropdownMenuTrigger
                      className="bc-icon-button bc-person-equipment-menu"
                      aria-label={`更多操作：${asset.display_name}`}
                      disabled={!canEdit || busy}
                    >
                      <MoreHorizontal size={17} aria-hidden="true" />
                    </DropdownMenuTrigger>
                  </IconTip>
                  <DropdownMenuContent align="end" className="bc-account-menu">
                    <DropdownMenuItem
                      variant="destructive"
                      onClick={() => remove(asset)}
                    >
                      <Trash2 />
                      移除
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
              <strong>{asset.display_name}</strong>
              {asset.notes ? <p>{asset.notes}</p> : null}
            </li>
          );
        })}
      </ul>
      {listError}
      {dialog}
    </div>
  );
}
