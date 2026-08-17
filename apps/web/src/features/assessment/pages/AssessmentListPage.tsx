import { useState, useEffect } from "react";
import { useNavigate } from "react-router";
import {
  assessmentApi,
  type AssessmentReport,
} from "../services/assessmentService";
import { MainLayout } from "@/components/layout/MainLayout";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { WorkspaceSoftGuard } from "@/features/workspace";

const GRADE_COLORS: Record<string, string> = {
  A: "from-primary-700 to-primary-500 text-[#FBFBFA] shadow-primary-700/20",
  B: "from-[#709a83] to-[#9ebdad] text-[#FBFBFA] shadow-[#709a83]/20",
  C: "from-[#CD7B67] to-[#E5A899] text-white shadow-[#CD7B67]/20",
  D: "from-[#B65E49] to-[#CD7B67] text-[#FBFBFA] shadow-[#B65E49]/20",
};

const GRADE_LABELS: Record<string, string> = {
  A: "优秀",
  B: "良好",
  C: "一般",
  D: "需改善",
};

export function AssessmentListPage() {
  const navigate = useNavigate();
  const [reports, setReports] = useState<AssessmentReport[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isGenerating, setIsGenerating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadReports();
  }, []);

  const loadReports = async () => {
    try {
      const data = await assessmentApi.listReports();
      setReports(data.reports);
    } catch {
      setError("Failed to load reports");
    } finally {
      setIsLoading(false);
    }
  };

  const handleGenerate = async () => {
    setIsGenerating(true);
    setError(null);

    try {
      const report = await assessmentApi.generate();
      navigate(`/assessment/${report.id}`);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to generate report",
      );
    } finally {
      setIsGenerating(false);
    }
  };

  if (isLoading) {
    return (
      <MainLayout>
        <div className="flex items-center justify-center min-h-[60vh]">
          <div className="text-center">
            <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-600 mx-auto"></div>
            <p className="mt-4 text-[#709a83] font-semibold">加载中...</p>
          </div>
        </div>
      </MainLayout>
    );
  }

  return (
    <MainLayout>
      <WorkspaceSoftGuard route="assessment">
        <div className="w-full space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-500">
          {/* Header */}
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-6 bg-[#F7F5F0] p-6 rounded-[28px] border border-[#E5E3DF] relative overflow-hidden">
            {/* Subtle morphing blob in page header */}
            <div className="absolute top-0 right-0 w-32 h-32 bg-[#9ebdad]/10 rounded-full mix-blend-multiply filter blur-2xl opacity-75 translate-x-1/2 -translate-y-1/2"></div>

            <div className="relative z-10">
              <h1 className="text-2xl font-display font-semibold text-[#1A221E] tracking-tight">
                Observation 评估报告
              </h1>
              <p className="text-[#4A554E] font-medium mt-1">
                从身体档案和体态资料生成待审核观察；确认后才进入长期 BodyState。
              </p>
            </div>
            <Button
              onClick={handleGenerate}
              isLoading={isGenerating}
              size="lg"
              className="shrink-0 relative z-10 bg-[#CD7B67] hover:bg-[#B65E49] text-white border-none shadow-sm shadow-[#CD7B67]/15"
            >
              {isGenerating ? "正在生成 (Generating)..." : "生成观察评估"}
            </Button>
          </div>

          {error && (
            <div className="rounded-2xl bg-red-50 p-4 border border-red-100">
              <p className="text-sm font-semibold text-red-800">{error}</p>
            </div>
          )}

          {/* Content */}
          {reports.length === 0 ? (
            <Card className="p-12 text-center border-dashed border-2 border-[#D6D3CD] bg-[#F7F5F0]/30">
              <div className="w-20 h-20 bg-[#f1f5f2] border border-[#c5d7cc]/30 rounded-full flex items-center justify-center mx-auto mb-6">
                <svg
                  className="w-10 h-10 text-primary-700"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={1.5}
                    d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
                  />
                </svg>
              </div>
              <h3 className="text-lg font-display font-semibold text-[#1A221E] mb-2">
                暂无评估报告
              </h3>
              <p className="text-[#4A554E] font-medium max-w-sm mx-auto mb-6 leading-relaxed">
                点击上方“生成观察评估”按钮，基于身体档案和体态资料生成第一份待审核
                Observation 报告。
              </p>
              <Button
                variant="outline"
                onClick={handleGenerate}
                isLoading={isGenerating}
              >
                创建观察评估
              </Button>
            </Card>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              {reports.map((report) => (
                <Card
                  key={report.id}
                  onClick={() => navigate(`/assessment/${report.id}`)}
                  className="group cursor-pointer overflow-hidden relative"
                >
                  <div className="p-6">
                    <div className="flex items-start justify-between mb-5">
                      <div
                        className={`flex h-16 w-16 items-center justify-center rounded-2xl bg-gradient-to-br text-2xl font-display font-bold shadow-md ${GRADE_COLORS[report.health_grade] || "from-slate-400 to-slate-500 text-white"}`}
                      >
                        {report.health_grade}
                      </div>
                      <div className="text-right">
                        <span className="inline-block px-3 py-1 bg-[#f1f5f2] border border-[#c5d7cc]/30 text-primary-900 rounded-full text-xs font-bold">
                          评分: {report.dimension_scores?.overall || "-"}
                        </span>
                      </div>
                    </div>

                    <div>
                      <h3 className="font-display font-semibold text-[#1A221E] text-lg mb-2">
                        {GRADE_LABELS[report.health_grade] || "未知状态"}
                      </h3>
                      <p className="text-xs font-semibold text-[#709a83] uppercase tracking-wider mb-2">
                        评估日期
                      </p>
                      <p className="text-sm text-[#4A554E] font-medium flex items-center gap-1.5">
                        <svg
                          className="w-4 h-4 text-[#709a83]"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth={2}
                            d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"
                          />
                        </svg>
                        {new Date(report.created_at).toLocaleDateString(
                          "zh-CN",
                          { year: "numeric", month: "long", day: "numeric" },
                        )}
                      </p>
                    </div>
                  </div>
                  <div className="h-1 w-full bg-gradient-to-r from-transparent via-accent-terracotta to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
                </Card>
              ))}
            </div>
          )}

          {/* Disclaimer */}
          <div className="mt-8 rounded-[24px] bg-[#B65E49]/5 border border-[#B65E49]/10 p-6 flex gap-4 items-start shadow-sm shadow-[#B65E49]/2">
            <div className="p-2.5 bg-[#B65E49]/10 text-[#B65E49] rounded-xl shrink-0">
              <svg
                className="w-5.5 h-5.5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                />
              </svg>
            </div>
            <div>
              <h4 className="font-display font-semibold text-[#B65E49] mb-1">
                重要声明
              </h4>
              <p className="text-sm text-[#B65E49] font-medium leading-relaxed">
                本Observation 评估报告均由 AI
                基于您的姿势及身体档案数据生成，属于参考性建议，并不构成任何正式的医学诊断或处方。如果您正经历慢性的关节疼痛、肌肉损伤或任何身体不适，强烈建议前往专业医疗机构进行科学诊断与治疗。
              </p>
            </div>
          </div>
        </div>
      </WorkspaceSoftGuard>
    </MainLayout>
  );
}
