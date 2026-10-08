import { useState, type FormEvent } from "react";
import { Backpack, Plus, Check, LoaderCircle } from "lucide-react";
import { errorMessage } from "@/lib/api-client";
import type { BodyStateFact, BodyStateProjection } from "@/features/consultation/types/consultation";
import { useBodyStateCommand } from "@/features/workspace/hooks/useBodyStateCommand";

export const AVAILABLE_EQUIPMENT_KIND = "equipment.available";

export function availableEquipment(snapshot: BodyStateProjection | null | undefined): BodyStateFact[] {
  return (snapshot?.facts ?? []).filter((fact) => fact.kind === AVAILABLE_EQUIPMENT_KIND && fact.lifecycle_state === "active" && fact.review_state === "confirmed" && !fact.excluded_from_reasoning);
}

export function EquipmentInventory({ snapshot, canEdit }: { snapshot: BodyStateProjection | null; canEdit: boolean }) {
  const command = useBodyStateCommand();
  const [name, setName] = useState("");
  const [notes, setNotes] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const equipment = availableEquipment(snapshot);
  const add = async (event: FormEvent) => {
    event.preventDefault();
    if (!snapshot || !canEdit || !name.trim() || command.isPending) return;
    setError(null); setSuccess(null);
    if (equipment.some((fact) => fact.value.trim().toLocaleLowerCase() === name.trim().toLocaleLowerCase())) { setError("这件设备已经在当前清单中。"); return; }
    try {
      await command.mutateAsync({ type: "addFact", expectedRevision: snapshot.current_revision, fact: {
        concern_key: "resources:equipment", kind: AVAILABLE_EQUIPMENT_KIND, value: name.trim(),
        details: { notes: notes.trim(), source: "user_inventory" }, origin: "user_reported",
        review_state: "confirmed", lifecycle_state: "active", trend: "unknown", observed_at: new Date().toISOString(),
      } });
      setName(""); setNotes(""); setSuccess("设备已保存；新方案生成时会使用当前清单。");
    } catch (cause) { setError(errorMessage(cause, "设备没有保存，请重试。")); }
  };
  const remove = async (fact: BodyStateFact) => {
    if (!snapshot || !canEdit || command.isPending) return;
    setError(null); setSuccess(null);
    try {
      await command.mutateAsync({ type: "updateFactTemporal", factId: fact.id, expectedRevision: snapshot.current_revision, input: { lifecycle_state: "inactive", valid_until: new Date().toISOString() } });
      setSuccess("已标记为当前不可用，历史记录仍然保留。");
    } catch (cause) { setError(errorMessage(cause, "设备状态没有保存，请重试。")); }
  };
  return <div className="bc-equipment">
    <p className="bc-muted">维护你实际可用的器械。清单不代表器械适合当前状态，也不会自动修改已接受的方案。</p>
    {equipment.length ? <ul className="bc-equipment-list">{equipment.map((fact) => <li key={fact.id}>
      <span className="bc-equipment-symbol"><Backpack size={22} /></span><div><strong>{fact.value}</strong>{typeof fact.details?.notes === "string" && fact.details.notes && <p>{fact.details.notes}</p>}<span className="bc-status"><Check size={13} />当前可用</span></div>
      <button className="bc-button" disabled={!canEdit || command.isPending} onClick={() => void remove(fact)}>暂不可用</button>
    </li>)}</ul> : <div className="bc-empty"><Backpack size={32} /><h3>设备背包还是空的</h3><p>有弹力带、哑铃或其他器械，再添加；徒手训练不需要填满这里。</p></div>}
    <form onSubmit={(event) => void add(event)} className="bc-equipment-form">
      <label>设备名称<input value={name} onChange={(event) => setName(event.target.value)} maxLength={80} placeholder="例如：弹力带" disabled={!canEdit || command.isPending} required /></label>
      <label>规格或备注<span className="bc-optional">可选</span><input value={notes} onChange={(event) => setNotes(event.target.value)} maxLength={240} placeholder="阻力、重量或使用限制" disabled={!canEdit || command.isPending} /></label>
      <button type="submit" className="bc-button bc-primary" disabled={!canEdit || command.isPending || !name.trim()}>{command.isPending ? <LoaderCircle size={17} className="animate-spin" /> : <Plus size={17} />}添加设备</button>
    </form>
    {error && <p role="alert" className="bc-error">{error}</p>}{success && <p role="status" className="bc-status">{success}</p>}
  </div>;
}
