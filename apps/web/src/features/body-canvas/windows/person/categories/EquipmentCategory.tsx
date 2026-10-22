import { Backpack, Check, Dumbbell, LoaderCircle, Plus } from "lucide-react";
import { errorMessage } from "@/lib/api-client";
import { useEquipmentInventory } from "../hooks/useEquipmentInventory";
import { PersonEmptyState } from "./PersonEmptyState";
import { PersonDataError } from "./PersonDataError";

/**
 * Equipment is the user's own EquipmentAsset inventory (PC-D2), read from
 * /api/v1/equipment — not BodyState. The category has no fixed slot ceiling:
 * cards grow with the data (4 columns desktop / 2 columns mobile). Marking an
 * item unavailable or removing it never edits an accepted plan, and the list
 * never asserts that an item is suitable for the user's current condition.
 */
export function EquipmentCategory({
  canEdit,
  onAddEquipment,
}: {
  canEdit: boolean;
  /** Opens the inline add form; there is no separate selector route yet. */
  onAddEquipment?: () => void;
}) {
  const {
    inventory,
    equipment,
    pending,
    busyId,
    error,
    adding,
    setAdding,
    name,
    changeName,
    notes,
    changeNotes,
    isCreating,
    resetForm,
    add,
    changeAvailability,
  } = useEquipmentInventory(canEdit);
  const addForm = (
    <form
      className="bc-equipment-form bc-person-equipment-form"
      onSubmit={(event) => {
        event.preventDefault();
        void add();
      }}
    >
      <label>
        设备名称
        <input
          value={name}
          onChange={(event) => {
            changeName(event.target.value);
          }}
          maxLength={80}
          placeholder="例如：弹力带"
          disabled={!canEdit || pending}
          autoFocus
        />
      </label>
      <label>
        规格或备注<span className="bc-optional">可选</span>
        <input
          value={notes}
          onChange={(event) => {
            changeNotes(event.target.value);
          }}
          maxLength={240}
          placeholder="阻力、重量或使用限制"
          disabled={!canEdit || pending}
        />
      </label>
      <button
        type="submit"
        className="bc-button bc-primary"
        disabled={!canEdit || pending || !name.trim()}
      >
        {isCreating ? (
          <LoaderCircle size={17} className="animate-spin" />
        ) : (
          <Plus size={17} />
        )}
        添加设备
      </button>
      <button
        type="button"
        className="bc-button"
        disabled={pending}
        onClick={resetForm}
      >
        取消
      </button>
    </form>
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

  if (equipment.length === 0 && !adding) {
    return (
      <>
        <PersonEmptyState
          icon={<Dumbbell size={24} />}
          title="还没有器材"
          message="加上家里或健身房能用的器材，训练会按它来安排"
          actionLabel="添加个人器材"
          onAction={() => {
            setAdding(true);
            onAddEquipment?.();
          }}
          disabled={!canEdit}
        />
        {error ? (
          <p role="alert" className="bc-error">
            {error}
          </p>
        ) : null}
      </>
    );
  }

  return (
    <div className="bc-person-category">
      {equipment.length === 0 ? null : (
        <>
          <p className="bc-person-count">已登记 {equipment.length} 件器材</p>
          <ul className="bc-person-equipment">
            {equipment.map((asset) => {
              const available = asset.availability === "available";
              const busy = busyId === asset.id;
              return (
                <li key={asset.id}>
                  <span
                    className="bc-person-equipment-symbol"
                    aria-hidden="true"
                  >
                    <Backpack size={20} />
                  </span>
                  <strong>{asset.display_name}</strong>
                  {asset.notes ? <p>{asset.notes}</p> : null}
                  <span className="bc-status">
                    {available ? <Check size={13} /> : null}
                    {available ? "当前可用" : "暂不可用"}
                  </span>
                  <button
                    type="button"
                    className="bc-text-button"
                    disabled={!canEdit || pending || busy}
                    onClick={() =>
                      void changeAvailability(
                        asset,
                        available ? "unavailable" : "available",
                      )
                    }
                  >
                    {busy ? (
                      <LoaderCircle size={14} className="animate-spin" />
                    ) : null}
                    {available ? "标记暂不可用" : "恢复可用"}
                  </button>
                  <button
                    type="button"
                    className="bc-text-button"
                    disabled={!canEdit || pending || busy}
                    onClick={() => void changeAvailability(asset, "retired")}
                  >
                    移除
                  </button>
                </li>
              );
            })}
          </ul>
        </>
      )}
      {adding ? (
        addForm
      ) : (
        <button
          type="button"
          className="bc-button"
          onClick={() => {
            setAdding(true);
            onAddEquipment?.();
          }}
          disabled={!canEdit}
        >
          <Plus size={16} />
          添加个人器材
        </button>
      )}
      {error ? (
        <p role="alert" className="bc-error">
          {error}
        </p>
      ) : null}
    </div>
  );
}
