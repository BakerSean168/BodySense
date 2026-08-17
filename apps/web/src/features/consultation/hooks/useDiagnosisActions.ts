import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { workspaceKeys } from "@/features/workspace";
import { errorMessage } from "@/lib/api-client";
import { consultationApi } from "../services/consultationService";
import { consultationKeys } from "../services/consultationQueryKeys";
import { diagnosisHistoryQueryOptions } from "../services/consultationQueryOptions";
import type {
  BodyStateSnapshot,
  ConsultationThread,
  DiagnosisAnalysis,
  DiagnosisCandidateAssessmentState,
} from "../types/consultation";

interface UseDiagnosisActionsOptions {
  conversationId: string | null;
  bodyState: BodyStateSnapshot | null;
  analysis: DiagnosisAnalysis | null;
}

export function useDiagnosisActions({
  conversationId,
  bodyState,
  analysis,
}: UseDiagnosisActionsOptions) {
  const queryClient = useQueryClient();
  const [error, setError] = useState<string | null>(null);
  const historyQuery = useQuery(diagnosisHistoryQueryOptions(20));

  const analyzeMutation = useMutation({
    mutationKey: ["consultation", "diagnosis", "analyze"],
    mutationFn: () => {
      if (!conversationId) throw new Error("No active conversation");
      return consultationApi.analyzeDiagnosis(conversationId);
    },
    onMutate: () => setError(null),
    onSuccess: (result) => {
      if (conversationId) {
        queryClient.setQueryData<ConsultationThread>(
          consultationKeys.thread(conversationId),
          (old) =>
            old
              ? {
                  ...old,
                  diagnosis: result,
                  phase:
                    result.status === "completed" || result.status === "partial"
                      ? "analysis_ready"
                      : old.phase,
                }
              : old,
        );
      }
      void queryClient.invalidateQueries({
        queryKey: consultationKeys.diagnosisHistoryAll(),
      });
      void queryClient.invalidateQueries({ queryKey: workspaceKeys.all });
    },
    onError: (mutationError) =>
      setError(errorMessage(mutationError, "生成可能性分析失败，请稍后重试")),
  });

  const assessmentMutation = useMutation({
    mutationKey: ["consultation", "diagnosis", "assessment"],
    mutationFn: async (
      items: Array<{
        candidate_id: string;
        state: DiagnosisCandidateAssessmentState;
      }>,
    ) => {
      if (!analysis?.analysis_id)
        throw new Error("No durable diagnosis analysis");
      await consultationApi.assessDiagnosisCandidates(
        analysis.analysis_id,
        items,
      );
    },
    onMutate: () => setError(null),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: consultationKeys.diagnosisHistoryAll(),
      });
      void queryClient.invalidateQueries({ queryKey: workspaceKeys.all });
      toast.success("已保存你对这些可能性的判断");
    },
    onError: (mutationError) =>
      setError(errorMessage(mutationError, "保存判断失败，请稍后重试")),
  });

  const canAnalyze = Boolean(
    conversationId &&
    ((bodyState?.facts.length ?? 0) > 0 ||
      (bodyState?.observations.length ?? 0) > 0),
  );

  return {
    error,
    history: historyQuery.data?.analyses ?? [],
    historyQuery,
    canAnalyze,
    analyze: analyzeMutation.mutate,
    isAnalyzing: analyzeMutation.isPending,
    saveAssessments: assessmentMutation.mutateAsync,
    isSavingAssessments: assessmentMutation.isPending,
  };
}
