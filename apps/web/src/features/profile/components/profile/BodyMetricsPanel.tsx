import { useEffect, useState } from "react";
import { PencilLine } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { errorMessage } from "@/lib/api-client";
import {
  bodyMetricsService,
  type BodyMetricsSnapshot,
} from "../../services/bodyMetricsService";

function metricLabel(value?: { value: number; unit: string }) {
  return value ? `${value.value} ${value.unit}` : "尚未记录";
}

export function BodyMetricsPanel() {
  const [snapshot, setSnapshot] = useState<BodyMetricsSnapshot | null>(null);
  const [height, setHeight] = useState("");
  const [weight, setWeight] = useState("");
  const [editing, setEditing] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await bodyMetricsService.get();
      setSnapshot(data);
      setHeight(data.height?.value.toString() || "");
      setWeight(data.weight?.value.toString() || "");
    } catch (cause) {
      setError(errorMessage(cause, "身体测量加载失败"));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const save = async () => {
    if (!snapshot) return;
    const heightValue = Number(height);
    const weightValue = Number(weight);
    if (!Number.isFinite(heightValue) || heightValue < 50 || heightValue > 250) {
      setError("身高必须在 50-250 cm 之间");
      return;
    }
    if (!Number.isFinite(weightValue) || weightValue < 20 || weightValue > 300) {
      setError("体重必须在 20-300 kg 之间");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const next = await bodyMetricsService.update({
        expected_revision: snapshot.current_revision,
        height_cm: heightValue,
        weight_kg: weightValue,
      });
      setSnapshot(next);
      setEditing(false);
    } catch (cause) {
      setError(errorMessage(cause, "身体测量保存失败"));
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return <div className="py-10 text-center text-sm text-muted-foreground">正在读取身体测量…</div>;
  }

  return (
    <div className="space-y-4 border-t border-border pt-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h2 className="text-lg font-semibold text-foreground">身体测量</h2>
          <p className="mt-1 text-xs leading-5 text-muted-foreground">
            身高、体重属于 BodyState Observation，而不是身份字段。更新后会保留上一条测量记录，BMI 由当前测量即时计算。
          </p>
        </div>
        {!editing ? (
          <Button size="sm" variant="outline" onClick={() => setEditing(true)}>
            <PencilLine className="h-4 w-4" />
            更新测量
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
          <div className="grid gap-4 sm:grid-cols-2">
            <label className="text-sm font-medium text-foreground">
              身高 (cm)
              <input
                type="number"
                min={50}
                max={250}
                step={0.1}
                value={height}
                onChange={(event) => setHeight(event.target.value)}
                className="mt-2 w-full rounded-xl border border-border bg-background px-3 py-2.5 outline-none focus:border-primary"
              />
            </label>
            <label className="text-sm font-medium text-foreground">
              体重 (kg)
              <input
                type="number"
                min={20}
                max={300}
                step={0.1}
                value={weight}
                onChange={(event) => setWeight(event.target.value)}
                className="mt-2 w-full rounded-xl border border-border bg-background px-3 py-2.5 outline-none focus:border-primary"
              />
            </label>
          </div>
          <div className="mt-4 flex justify-end gap-2">
            <Button variant="ghost" disabled={saving} onClick={() => setEditing(false)}>
              取消
            </Button>
            <Button isLoading={saving} onClick={() => void save()}>
              保存测量
            </Button>
          </div>
        </div>
      ) : (
        <div className="grid gap-3 sm:grid-cols-3">
          <div className="rounded-2xl border border-border bg-muted/20 p-4">
            <div className="text-xs text-muted-foreground">身高</div>
            <div className="mt-2 text-base font-semibold text-foreground">{metricLabel(snapshot?.height)}</div>
          </div>
          <div className="rounded-2xl border border-border bg-muted/20 p-4">
            <div className="text-xs text-muted-foreground">体重</div>
            <div className="mt-2 text-base font-semibold text-foreground">{metricLabel(snapshot?.weight)}</div>
          </div>
          <div className="rounded-2xl border border-border bg-muted/20 p-4">
            <div className="text-xs text-muted-foreground">BMI</div>
            <div className="mt-2 text-base font-semibold text-foreground">
              {snapshot?.bmi != null ? snapshot.bmi.toFixed(1) : "暂无"}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
