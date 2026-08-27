import { useEffect, useState } from "react";
import { PencilLine } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { errorMessage } from "@/lib/api-client";
import {
  healthHistoryService,
  type InjuryHistorySnapshot,
} from "../../services/healthHistoryService";

export function HealthHistoryPanel() {
  const [snapshot, setSnapshot] = useState<InjuryHistorySnapshot | null>(null);
  const [draft, setDraft] = useState("");
  const [editing, setEditing] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void healthHistoryService
      .getInjuryHistory()
      .then((data) => {
        if (cancelled) return;
        setSnapshot(data);
        setDraft(data.summary || "");
      })
      .catch((cause) => {
        if (!cancelled) setError(errorMessage(cause, "健康历史加载失败"));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const save = async () => {
    if (!snapshot) return;
    setSaving(true);
    setError(null);
    try {
      const next = await healthHistoryService.updateInjuryHistory({
        expected_revision: snapshot.current_revision,
        summary: draft.trim(),
      });
      setSnapshot(next);
      setDraft(next.summary || "");
      setEditing(false);
    } catch (cause) {
      setError(errorMessage(cause, "健康历史保存失败"));
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return <div className="py-12 text-center text-sm text-muted-foreground">正在读取健康历史…</div>;
  }

  return (
    <div className="space-y-5">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h2 className="text-lg font-semibold text-foreground">健康历史</h2>
          <p className="mt-1 max-w-2xl text-xs leading-5 text-muted-foreground">
            这里保留对长期身体判断有意义的既往信息。当前先维护重要伤病与手术摘要；它属于 BodyState
            健康历史，而不是静态身份 Profile。
          </p>
        </div>
        {!editing ? (
          <Button size="sm" variant="outline" onClick={() => setEditing(true)}>
            <PencilLine className="h-4 w-4" />
            编辑
          </Button>
        ) : null}
      </div>

      {error ? (
        <div className="rounded-xl border border-destructive/20 bg-destructive/10 p-3 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      {editing ? (
        <div className="rounded-2xl border border-border p-4">
          <label htmlFor="injury-history" className="text-sm font-semibold text-foreground">
            既往伤病 / 手术史
          </label>
          <p className="mt-1 text-xs text-muted-foreground">
            记录重要、可能影响当前评估或训练建议的历史即可；不需要把短暂小磕碰全部写进来。
          </p>
          <textarea
            id="injury-history"
            rows={5}
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            placeholder="例如：2024 年左膝前交叉韧带轻微拉伤，保守恢复；目前偶尔深蹲后酸胀。"
            className="mt-3 w-full rounded-xl border border-border bg-background px-3 py-2 text-sm outline-none focus:border-primary"
          />
          <div className="mt-4 flex justify-end gap-2">
            <Button variant="ghost" disabled={saving} onClick={() => setEditing(false)}>
              取消
            </Button>
            <Button isLoading={saving} onClick={() => void save()}>
              保存变化
            </Button>
          </div>
        </div>
      ) : (
        <div className="rounded-2xl border border-border bg-muted/20 p-4">
          <div className="text-sm font-semibold text-foreground">既往伤病 / 手术史</div>
          <p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-muted-foreground">
            {snapshot?.summary || "尚未记录重要伤病或手术史"}
          </p>
          {snapshot?.valid_from ? (
            <p className="mt-3 text-[11px] text-muted-foreground/70">
              当前摘要自 {new Date(snapshot.valid_from).toLocaleDateString()} 起记录
            </p>
          ) : null}
        </div>
      )}
    </div>
  );
}
