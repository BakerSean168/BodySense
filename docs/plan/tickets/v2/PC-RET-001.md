# PC-RET-001 · Implement three-cursor retention and controlled pruning

Status: IN PROGRESS · lane C (Hark parallel lanes, 2026-10-17), worktree `bodysense-pcret001`, base `1abcc9f16`. Phase 2. SF cross-reference: SF-70.
**Owner role:** Chat Retention Owner. **Dependencies:** `PC-001`.
**Exclusive resources:** `conversation-schema`, `privacy-erasure`.

## Current remaining scope · 2026-10-16

**GitHub:** [#307](https://github.com/BakerSean168/BodySense/issues/307) (epic [#301](https://github.com/BakerSean168/BodySense/issues/301)) · Wave 2 · size M · blocked by: PC-001 #302 · blocks: AS-002 #314, QA-001 #316, REL-001 #317.

Code-verified on main `85b9e68a6`. Big-bang policy (Master §0): edit `000001_vnext_baseline` in place, no numbered migrations, no rollback work. The steps further below are historical decomposition; deliver only this scope.

**Missing:**

- `conversations` 新增 `processed_through_message_id`, `context_start_message_id`, `pruned_through_message_id`, `retention_revision`；有序不变量 `pruned ≤ min(processed, context_start)`（Go 校验 + DB CHECK 可表达部分）
- Context assembly 从 `context_start` 读（Go 构建，Python 只收）
- Go prune job（经 `jobs`）：仅删除 `< min(processed, context_start)` 的 `messages` / `thread_projection_*`，且被 `statement_evidence` / AskUser source 引用的源文本必须保留（或先确认 evidence 自带冻结副本）
- Python checkpoint cleanup endpoint（按 pruned cursor），Go 调用并校验回执

**Slices (one PR each):**

1. api: cursors + ordering invariants + context window read — baseline 列 + 单调前进命令（expected revision）；测试：cursor 不能倒退/跳过未处理消息、并发推进只有一个成功、context 读取只返回 context_start 之后
2. api+ai: prune job + checkpoint cleanup — 测试：evidence 引用的消息不被删除、interrupted Run resume 不出现 ghost、prune 与新消息并发不丢消息、Python cleanup 失败 → job 重试且 cursor 不前进

## Goal and why now

Implement three-cursor retention and controlled pruning. This is an execution slice under the approved BodySense V2 Master; it does not approve clinical decisions or establish any readiness gate by itself.

## Scoped paths / ownership

- `apps/api/internal/repository/** (retention-specific only)`
- `apps/api/internal/service/** (retention-specific only)`
- `apps/ai-service/src/** (checkpoint cleanup only)`

**Out of scope:** Other owners' dirty worktrees, unrelated source files, direct production changes, automatic clinical authorization, and any unapproved legacy contract restoration. Public OpenAPI, generated clients and migrations require a serialized designated owner before mutation.

## Ordered implementation

1. Reuse committed DS-PC-01 fail-closed model/adversarial specifications.
2. Implement processed/context/pruned cursor ordering and minimum source preservation.
3. Prune raw messages only after durable adopted evidence, outbox, checkpoint/cache/index and linked source references qualify.
4. Test interrupted resume, rollback, changed policy, controlled copy and concurrent new messages.

## Verification commands and negative cases

- node --test scripts/plan/v2-chat-retention-model.test.mjs scripts/plan/v2-chat-retention-adversarial.test.mjs
- Native PG18 -race + Python checkpoint negative tests

## Acceptance evidence

- No cursor skip, unsupported source loss, ghost resume or false erasure receipt.
- Explicit cross-domain evidence and deletion scope proof.

**Required handoff:** execution identity/model; actual cwd/branch; input and output exact SHA; changed file list; tests with exit codes and logs; uncommitted diff disclosure; code review; pending gates/owner. Only an Integrator may accept a new shared schema/contract into the integration candidate.

**Protected invariants:** owner identity and source provenance; commit receipt before success; revision + idempotency; source deletion distinct from erasure; Go-owned medical authority and fail-closed Safety; one writer per checkout. No invented Diagnosis Q-17 TTL, clinical thresholds, or old BodyConcern.

**Scheduling constraint:** even when DAG dependencies pass, do not run in parallel with any other ticket sharing `conversation-schema` or any other listed exclusive resource. Ticket does not authorize T3 to use canonical root as a shared writer.
