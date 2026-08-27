import { useEffect, useMemo, useState } from "react";
import { PencilLine } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { errorMessage } from "@/lib/api-client";
import {
  lifestyleService,
  type LifestyleCandidate,
  type LifestyleSectionKey,
  type LifestyleSnapshot,
} from "../../services/lifestyleService";

const sections: Array<{
  key: LifestyleSectionKey;
  label: string;
  hint: string;
  placeholder: string;
}> = [
  {
    key: "activity",
    label: "日常活动",
    hint: "久坐、久站、走动、搬抬、重复动作、轮班等身体使用模式。",
    placeholder: "例如：工作日久坐为主，每次连续坐 2-3 小时；每天步行约 40 分钟。",
  },
  {
    key: "sleep",
    label: "睡眠与作息",
    hint: "规律性、轮班、睡眠时长、夜醒或长期缺觉。",
    placeholder: "例如：夜班和白班交替，平均睡 6-7 小时，换班时容易睡不够。",
  },
  {
    key: "exercise",
    label: "运动",
    hint: "主要运动形式、频率与近期训练负荷。",
    placeholder: "例如：力量训练每周 3 次，周末偶尔跑步 5 公里。",
  },
  {
    key: "nutrition",
    label: "饮食节律",
    hint: "吃饭是否规律、夜宵、节食等与健康判断有关的节律信息。",
    placeholder: "例如：通常三餐规律；夜班时会在凌晨加一餐。",
  },
  {
    key: "substances",
    label: "酒精 / 烟草 / 咖啡因",
    hint: "只记录对身体状态可能有意义的摄入习惯，不做精细饮食追踪。",
    placeholder: "例如：应酬时每周饮酒 1-2 次；每天咖啡约 2 杯；不吸烟。",
  },
  {
    key: "recovery",
    label: "恢复与压力",
    hint: "长期压力、恢复感受、工作节奏等可能影响身体恢复的背景。",
    placeholder: "例如：最近工作压力偏高，工作日恢复感一般，周末会明显好一些。",
  },
];

const sectionLabelByKind: Record<string, string> = Object.fromEntries(
  sections.map((section) => [`lifestyle.${section.key}`, section.label]),
);

type Draft = Record<LifestyleSectionKey, string>;

function emptyDraft(): Draft {
  return {
    activity: "",
    sleep: "",
    exercise: "",
    nutrition: "",
    substances: "",
    recovery: "",
  };
}

export function LifestylePanel() {
  const [snapshot, setSnapshot] = useState<LifestyleSnapshot | null>(null);
  const [draft, setDraft] = useState<Draft>(emptyDraft());
  const [editing, setEditing] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [reviewingId, setReviewingId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const syncSnapshot = (data: LifestyleSnapshot) => {
    setSnapshot(data);
    setDraft(
      Object.fromEntries(
        sections.map(({ key }) => [key, data[key].summary || ""]),
      ) as Draft,
    );
  };

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await lifestyleService.get();
      syncSnapshot(data);
    } catch (cause) {
      setError(errorMessage(cause, "生活方式加载失败"));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const hasAny = useMemo(
    () => sections.some(({ key }) => Boolean(snapshot?.[key].summary)),
    [snapshot],
  );

  const save = async () => {
    if (!snapshot) return;
    setSaving(true);
    setError(null);
    try {
      const next = await lifestyleService.update({
        expected_revision: snapshot.current_revision,
        ...Object.fromEntries(
          sections.map(({ key }) => [
            key,
            { summary: draft[key].trim(), details: snapshot[key].details ?? {} },
          ]),
        ),
      });
      syncSnapshot(next);
      setEditing(false);
    } catch (cause) {
      setError(errorMessage(cause, "生活方式保存失败"));
    } finally {
      setSaving(false);
    }
  };

  const reviewCandidate = async (candidate: LifestyleCandidate, action: "accept" | "reject") => {
    if (!snapshot) return;
    setReviewingId(candidate.fact_id);
    setError(null);
    try {
      const next =
        action === "accept"
          ? await lifestyleService.acceptCandidate(candidate.fact_id, snapshot.current_revision)
          : await lifestyleService.rejectCandidate(candidate.fact_id, snapshot.current_revision);
      syncSnapshot(next);
    } catch (cause) {
      setError(errorMessage(cause, action === "accept" ? "确认更新失败" : "忽略更新失败"));
    } finally {
      setReviewingId(null);
    }
  };

  if (loading) {
    return <div className="py-12 text-center text-sm text-muted-foreground">正在读取生活方式…</div>;
  }

  return (
    <div className="space-y-5">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h2 className="text-lg font-semibold text-foreground">生活方式</h2>
          <p className="mt-1 max-w-2xl text-xs leading-5 text-muted-foreground">
            这里展示当前已确认的 BodyState 生活方式。您直接编辑会立即形成长期记录；对话中由 AI
            整理出的变化会先作为待确认候选保存，只有您确认后才会替换当前状态并进入推理。
          </p>
        </div>
        {!editing ? (
          <Button size="sm" variant="outline" onClick={() => setEditing(true)}>
            <PencilLine className="h-4 w-4" />
            编辑生活方式
          </Button>
        ) : null}
      </div>

      {error ? (
        <div className="rounded-xl border border-destructive/20 bg-destructive/10 p-3 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      {snapshot?.pending_updates?.length ? (
        <div className="rounded-2xl border border-primary/20 bg-primary/5 p-4">
          <div className="text-sm font-semibold text-foreground">对话中识别到的待确认更新</div>
          <p className="mt-1 text-xs leading-5 text-muted-foreground">
            BodySense 从对话中整理出了这些生活方式信息。它们已经持久保存，但目前不会进入健康推理，也不会覆盖当前长期记录。
          </p>
          <div className="mt-3 space-y-3">
            {snapshot.pending_updates.map((candidate) => (
              <div key={candidate.fact_id} className="rounded-xl border border-border bg-background p-3">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div className="min-w-0 flex-1">
                    <div className="text-xs font-semibold text-primary">
                      {sectionLabelByKind[candidate.kind] || candidate.kind}
                    </div>
                    <p className="mt-1 whitespace-pre-wrap text-sm leading-6 text-foreground">
                      {candidate.summary}
                    </p>
                    <p className="mt-1 text-[11px] text-muted-foreground">
                      来自对话整理 · {new Date(candidate.created_at).toLocaleString()}
                    </p>
                  </div>
                  <div className="flex shrink-0 gap-2">
                    <Button
                      size="sm"
                      variant="ghost"
                      disabled={reviewingId === candidate.fact_id}
                      onClick={() => void reviewCandidate(candidate, "reject")}
                    >
                      忽略
                    </Button>
                    <Button
                      size="sm"
                      isLoading={reviewingId === candidate.fact_id}
                      onClick={() => void reviewCandidate(candidate, "accept")}
                    >
                      确认为当前状态
                    </Button>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      ) : null}

      {editing ? (
        <div className="space-y-4">
          {sections.map((section) => (
            <div key={section.key} className="rounded-2xl border border-border p-4">
              <label className="text-sm font-semibold text-foreground" htmlFor={`lifestyle-${section.key}`}>
                {section.label}
              </label>
              <p className="mt-1 text-xs text-muted-foreground">{section.hint}</p>
              <textarea
                id={`lifestyle-${section.key}`}
                rows={3}
                value={draft[section.key]}
                onChange={(event) =>
                  setDraft((current) => ({ ...current, [section.key]: event.target.value }))
                }
                placeholder={section.placeholder}
                className="mt-3 w-full rounded-xl border border-border bg-background px-3 py-2 text-sm outline-none focus:border-primary"
              />
            </div>
          ))}
          <div className="flex justify-end gap-2">
            <Button variant="ghost" disabled={saving} onClick={() => setEditing(false)}>
              取消
            </Button>
            <Button isLoading={saving} onClick={() => void save()}>
              保存变化
            </Button>
          </div>
        </div>
      ) : (
        <div className="grid gap-3 sm:grid-cols-2">
          {sections.map((section) => {
            const value = snapshot?.[section.key];
            return (
              <div key={section.key} className="rounded-2xl border border-border bg-muted/20 p-4">
                <div className="text-sm font-semibold text-foreground">{section.label}</div>
                <p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-muted-foreground">
                  {value?.summary || "尚未记录"}
                </p>
                {value?.valid_from ? (
                  <p className="mt-3 text-[11px] text-muted-foreground/70">
                    当前状态自 {new Date(value.valid_from).toLocaleDateString()} 起记录
                  </p>
                ) : null}
              </div>
            );
          })}
          {!hasAny ? (
            <p className="sm:col-span-2 text-xs text-muted-foreground">
              可以先留空；后续直接在对话里自然描述，BodySense 也可以逐步补全这里。
            </p>
          ) : null}
        </div>
      )}
    </div>
  );
}
