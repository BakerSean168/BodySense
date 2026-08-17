/**
 * useConsultationSessionQuery —— 用 TanStack Query 拉取单个会话详情。
 *
 * ===== 值得学的点 =====
 * - `enabled: !!conversationId` 让“还没有选中会话”时不发请求（条件查询）。
 * - queryKey 在“有 id”和“空”两种状态下用不同 key，避免缓存串台；这是
 *   “外部数据”而非组件内部状态，由查询层统一管理获取与失效。
 * - staleTime / refetchOn* 控制重新获取频率，避免无谓请求与抖动。
 *
 * 深入笔记（Thought Forest 文件名）：
 * - tanstack-query-suspense-integration.md
 * - react-effects-and-external-systems.md
 */

import { useQuery } from "@tanstack/react-query";
import { consultationApi } from "../services/consultationService";
import { consultationKeys } from "../services/consultationQueryKeys";

export function useConsultationSessionQuery(conversationId: string | null) {
  return useQuery({
    queryKey: conversationId
      ? consultationKeys.session(conversationId)
      : consultationKeys.sessionEmpty(),
    queryFn: () => consultationApi.getConsultation(conversationId!),
    enabled: !!conversationId,
    staleTime: 60_000,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    refetchOnMount: false,
  });
}
