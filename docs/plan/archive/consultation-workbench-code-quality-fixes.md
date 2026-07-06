# 咨询工作台 Code Quality 优化计划

**日期**: 2026-06-25
**状态**: 全部 7 批修复完成（含测试质量），review agent 已启动
**来源**: 代码质量审查（Python AI 服务、Go 后端、前端、测试共 30 个文件）

## 问题总览

| 严重度 | 数量 | 说明 |
|--------|------|------|
| Critical | 10 | 功能正确性、数据安全、安全漏洞 |
| Warning | 24 | 可靠性、性能、代码质量 |
| Info | 13 | 死代码、命名、可改进项 |

## 第一批：功能正确性（Critical 前端 + Python）

### C1. `useMemo` 内调用 state setter — 无限重渲染
- **文件**: `AssistantChatPanel.tsx:104,119,129`
- **问题**: `setCitations`、`setRedFlags`、`setKnowledgeGaps` 在 `useMemo` 回调内执行，触发无限渲染循环
- **修复**: 迁移到 `useEffect`，依赖 `thread.messages`

### C2. `handleSend` 未实际发送消息
- **文件**: `AssistantChatPanel.tsx:180-188`
- **问题**: 只清空本地 state，未调用 runtime 的 `sendMessage`
- **修复**: 接入 assistant-ui runtime 的 send 机制

### C6. Phase 名称不匹配 — 静默死逻辑
- **文件**: `consultation_graph.py:307` vs `agent_workflow.py:103`
- **问题**: graph 返回 `"ready_for_analysis"`，workflow 检查 `"analysis_ready"`
- **修复**: 统一为 `"ready_for_analysis"`

## 第二批：数据安全（Critical Go）

### C3. `AppendMessage` 无事务保护 — 并发丢失更新
- **文件**: `consultation_service.go:71-101`
- **问题**: Read-modify-write 无锁无事务
- **修复**: 使用 PostgreSQL `jsonb_set` 或 `SELECT ... FOR UPDATE`

### C4. `CreateSession` / `UpdatePhase` 吞没 DB 错误
- **文件**: `consultation_service.go:38-41,121-129`
- **问题**: DB 错误时静默 fallthrough
- **修复**: 显式返回 error

### C5. `http.DefaultClient` 无超时
- **文件**: `consultation_handler.go:239`、`diagnosis_handler.go:110,218`
- **问题**: AI 服务挂起时 goroutine 永不返回
- **修复**: 注入带超时的 `http.Client`

### W12. `json.Marshal` 错误被丢弃
- **文件**: `diagnosis_handler.go:96,203`
- **修复**: 检查 error 并返回 500

### W10. DB 500 错误被报告为 404
- **文件**: `consultation_handler.go` 多处
- **修复**: 分别处理 error 和 nil

### W11. Phase rank 碰撞 `""` 和 `"collecting"` 同为 0
- **文件**: `consultation_phase.go:7-8`
- **修复**: `""` rank 设为 -1

### W8. post-stream 持久化错误全部丢弃
- **文件**: `consultation_handler.go:205,329,335,339`
- **修复**: 至少记录日志

### W9. `io.ReadAll` 无大小限制
- **文件**: `diagnosis_handler.go:117,224`
- **修复**: 使用 `io.LimitReader`

### W13. UTF-8 截断按字节切分
- **文件**: `knowledge_helper.go:128-129`
- **修复**: 使用 rune-aware 截断

## 第三批：Python 可靠性

### C7. SSE 流无异常处理
- **文件**: `api/routes/chat.py:47-54`
- **修复**: 加 try/except + error SSE 事件

### C8. 诊断路由泄露内部错误详情
- **文件**: `api/routes/diagnosis.py:49,68`
- **修复**: 替换为通用错误消息

### W14. 重复知识库搜索
- **文件**: `consultation_graph.py:450-457`
- **修复**: `execute_search_knowledge` 返回 raw results

### W15. `except Exception: pass` 吞没所有异常
- **文件**: `consultation_graph.py:576,646`
- **修复**: 记录日志 + 使用具体异常类型

### W17. `to_dict()` 返回 list 而非 dict
- **文件**: `models/consultation.py:42`
- **修复**: 重命名为 `to_list()` / `from_list()`

### W18. 诊断/方案 RAG 序列化逻辑重复
- **文件**: `consultation_graph.py:555-575,624-644`
- **修复**: 提取共享 helper

### W19. 空运动列表 `faithful=True`
- **文件**: `faithfulness_checker.py:134`
- **修复**: 空列表返回 `faithful=False`

### W20. `_determine_phase` 过于宽松
- **文件**: `consultation_graph.py:304-306`
- **修复**: 要求 `body_part` + 至少一个 detail

## 第四批：测试质量 ✅

### C9. useChatSSE 测试假阳性 ✅
- **文件**: `useChatSSE.test.ts:43-97`
- **修复**: 导入真实 `dispatchSSEData` / `processSSELine` 函数，移除手动复制

### C10. chat_service 测试测错模块 ✅
- **文件**: `test_chat_service.py:8-9,213-269`
- **修复**: 移除 graph 内部函数测试（已在 test_consultation_graph.py 覆盖），仅保留 ChatService.stream_chat 公共 API 测试

### W21. 多轮 tool loop 仅测单轮
- **文件**: `test_consultation_graph.py`
- **状态**: 保留为后续改进（需要 mock 复杂的多轮 tool call 链路）

### W22. Red flag 测试不验证 message/source ✅
- **文件**: `test_red_flag_detector.py`
- **修复**: 增加 `message` 非空断言和 `source` 字段断言（conversation / extracted_info）

### W24. 意图分类缺少边界情况
- **文件**: `test_agent_workflow.py`
- **状态**: 保留为后续改进

## 第五批：死代码清理 ✅

### I1. `indexOfNewline` → `strings.IndexByte` ✅
### I2. `CreateSessionRequest` DTO 未使用 ✅
### I3. `CompleteSession` 从未被调用 ✅
### I4. `conversation_summary` 始终为空
### I5. Python 死代码：`merge_extracted_info`、`should_generate_treatment`（保留，有测试覆盖）
### I6. `extract_info` graph 节点是 no-op（保留，作为未来扩展检查点）
### I8. `build_rag_context` 可能未使用
### W3. `ChatPanel` 组件疑似死代码 ✅
### W4. `ChatContent` 4 个未使用 props ✅
### I9. `ChatMessage` 接口重复定义

## 第六批：前端可靠性 ✅

### W1. SSE 回调闭包过期
- **文件**: `useChatSSE.ts:143-219`
- **状态**: 保留为后续改进（当前用法中回调稳定性足够）

### W2. 组件卸载时未 abort fetch ✅
- **文件**: `useChatSSE.ts`
- **修复**: 添加 useEffect cleanup 调用 abortControllerRef.current?.abort()

### W5. 连续两次 `setSession` 浪费渲染 ✅
- **文件**: `ConsultationPage.tsx:148-160`
- **修复**: 移除中间的 diagnosis_confirmed phase 更新，合并为一次 setSession

### W6. `isSessionFetching` 不反映真实状态 ✅
- **文件**: `ConsultationPage.tsx:47`
- **修复**: 替换为 `isSessionReady`（`!isLoading && session?.id === id`）

### W7. 会话加载无 AbortController
- **文件**: `ConsultationPage.tsx:49-89`
- **状态**: 保留为后续改进

### I10. 会话列表 markup 重复
### I11. 移动端 drawer 缺少 ARIA

## 修复汇总

| 批次 | 范围 | 修复项数 | 状态 |
|------|------|----------|------|
| 第一批 | 前端功能正确性 | 4 | ✅ |
| 第二批 | Go 数据安全 | 8 | ✅ |
| 第三批 | Python 可靠性 | 8 | ✅ |
| 第四批 | 测试质量 | 3 | ✅ |
| 第五批 | 死代码清理 | 4 | ✅ |
| 第六批 | 前端可靠性 | 3 | ✅ |

**已修复**: C1-C10, W2-W6, W8-W13, W14-W15, W17-W20, W22, I1-I3, W3-W4
**保留为后续**: W1, W7, W21, W24, I4-I9, I10-I11

## 第七批：二次审查修复 ✅

| # | 严重度 | 问题 | 修复 |
|---|--------|------|------|
| 1 | Medium | `setKnowledgeGaps` 中 `sort()` 直接修改 prev state | 使用 `[...prev].sort()` 创建新数组 |
| 2 | Medium | 导航切换会话时 isLoading 未重置 | useEffect 开头设置 `setIsLoading(true)` |
| 3 | Low | `chat.py` exception handler 内 import logging | 移到模块级别 |
| 4 | Low | `chat.py` 错误消息泄露异常类名 | 替换为通用消息 |
| 5 | Low | Go `fmt.Fprintf(os.Stderr)` 不一致 | 统一为 `log.Printf` |
| 6 | Low | diagnosis_handler UpdatePhase 错误被丢弃 | 添加日志记录 |
| 7 | Low | AppendMessage 缺少错误包装 | 添加 `fmt.Errorf` 包装 |

## 验证结果

- `pnpm exec tsc --noEmit` ✅
- `npx vitest run` — 52 tests passed ✅
- `go vet ./...` ✅
- `go test ./...` ✅
- `uv run ruff check .` ✅
- `uv run pytest` — 152 tests passed ✅
