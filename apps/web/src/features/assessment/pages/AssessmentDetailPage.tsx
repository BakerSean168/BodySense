import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { MainLayout } from "@/components/layout/MainLayout";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import {
  assessmentApi,
  type AssessmentReport,
} from "../services/assessmentService";

const GRADE_LABELS: Record<string, string> = {
  A: "状态良好",
  B: "总体稳定",
  C: "值得关注",
  D: "建议优先复核",
};

const SCORE_LABELS: Record<string, string> = {
  posture: "体态",
  exercise: "运动",
  lifestyle: "生活方式",
  injury_risk: "风险信号",
  overall: "综合",
};

function summaryText(report: AssessmentReport): string {
  return typeof report.summary === "string"
    ? report.summary
    : report.summary?.text || "当前评估没有额外摘要。";
}

function observationText(value: Record<string, unknown>): string {
  const description = value.description;
  if (typeof description === "string" && description.trim()) return description;
  return JSON.stringify(value, null, 2);
}

export function AssessmentDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [report, setReport] = useState<AssessmentReport | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!id) {
      navigate("/assessment", { replace: true });
      return;
    }
    let active = true;
    assessmentApi
      .getReport(id)
      .then((value) => active && setReport(value))
      .catch(
        (reason: unknown) =>
          active &&
          setError(reason instanceof Error ? reason.message : "加载评估失败"),
      )
      .finally(() => active && setLoading(false));
    return () => {
      active = false;
    };
  }, [id, navigate]);

  const scores = useMemo(
    () => (report ? Object.entries(report.dimension_scores) : []),
    [report],
  );

  if (loading) {
    return (
      <MainLayout>
        <div className="flex min-h-[60vh] items-center justify-center text-sm text-muted-foreground">
          正在加载观察报告…
        </div>
      </MainLayout>
    );
  }

  if (error || !report) {
    return (
      <MainLayout>
        <Card className="mx-auto max-w-lg p-8 text-center">
          <h1 className="text-xl font-semibold">无法加载观察报告</h1>
          <p className="mt-3 text-sm text-muted-foreground">
            {error || "报告不存在"}
          </p>
          <Button className="mt-5" onClick={() => navigate("/assessment")}>
            返回报告列表
          </Button>
        </Card>
      </MainLayout>
    );
  }

  return (
    <MainLayout>
      <div className="space-y-6">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => navigate("/assessment")}
            >
              ← 返回观察报告
            </Button>
            <h1 className="mt-3 text-3xl font-display font-semibold">
              健康观察报告
            </h1>
            <p className="mt-2 text-sm text-muted-foreground">
              {new Date(report.created_at).toLocaleString("zh-CN")}
            </p>
          </div>
          <div className="rounded-3xl border bg-card px-8 py-5 text-center shadow-sm">
            <div className="text-5xl font-bold">{report.health_grade}</div>
            <div className="mt-1 text-xs text-muted-foreground">
              {GRADE_LABELS[report.health_grade]}
            </div>
          </div>
        </div>

        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
          {scores.map(([key, value]) => (
            <Card key={key} className="p-4">
              <p className="text-xs text-muted-foreground">
                {SCORE_LABELS[key] || key}
              </p>
              <p className="mt-2 text-2xl font-semibold">{value}</p>
            </Card>
          ))}
        </div>

        <Card className="p-6">
          <h2 className="text-lg font-semibold">评估摘要</h2>
          <p className="mt-3 text-sm leading-7 text-muted-foreground">
            {summaryText(report)}
          </p>
        </Card>

        <Card className="p-6">
          <h2 className="text-lg font-semibold">已写入 BodyState 的观察</h2>
          <p className="mt-1 text-xs text-muted-foreground">
            这些条目是可复核的
            Observation，不是医学诊断，也不会直接生成训练处方。
          </p>
          {report.observations.length === 0 ? (
            <p className="mt-4 rounded-xl border border-dashed p-4 text-sm text-muted-foreground">
              当前输入不足以形成可靠观察。
            </p>
          ) : (
            <div className="mt-4 space-y-3">
              {report.observations.map((observation, index) => (
                <div
                  key={`${observation.kind}-${observation.body_region}-${index}`}
                  className="rounded-xl border p-4"
                >
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-medium">
                      {observation.body_region || "全身"}
                    </span>
                    <span className="rounded-full bg-muted px-2 py-0.5 text-xs">
                      {observation.kind}
                    </span>
                    {observation.confidence ? (
                      <span className="text-xs text-muted-foreground">
                        置信度：{observation.confidence}
                      </span>
                    ) : null}
                  </div>
                  <p className="mt-2 whitespace-pre-wrap text-sm leading-6">
                    {observationText(observation.value)}
                  </p>
                  <p className="mt-2 text-xs text-muted-foreground">
                    依据：{observation.evidence}
                  </p>
                </div>
              ))}
            </div>
          )}
        </Card>

        <Card className="p-6">
          <h2 className="text-lg font-semibold">仍需补充的信息</h2>
          {report.information_gaps.length === 0 ? (
            <p className="mt-3 text-sm text-muted-foreground">
              当前没有明确的信息缺口。
            </p>
          ) : (
            <ul className="mt-3 list-disc space-y-2 pl-5 text-sm text-muted-foreground">
              {report.information_gaps.map((gap) => (
                <li key={gap}>{gap}</li>
              ))}
            </ul>
          )}
        </Card>

        <p className="rounded-xl border border-amber-200 bg-amber-50 p-4 text-xs leading-5 text-amber-900">
          本报告是 BodyState
          的派生观察视图，不生成训练处方或治疗方案，也不构成医疗诊断。持续疼痛、明显无力、麻木或其他严重不适应优先寻求专业医疗评估。
        </p>
      </div>
    </MainLayout>
  );
}
