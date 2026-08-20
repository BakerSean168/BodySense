# Diagnosis Legacy AI Routing Deletion Ledger

> Rule: compatibility code is temporary and must have a named deletion condition.

| Legacy surface | Current role | Target owner/replacement | Deletion condition | Status |
| --- | --- | --- | --- | --- |
| Go `DiagnosisRequest.UseCase` / `"llm.json"` | selects Python model route | AgentConfiguration deployment pointer | Go/Python contract carries configuration identity; no Diagnosis caller sends `llm.json` | retired from Diagnosis in Phase 2; configuration identity lands in Phase 3 |
| `apps/ai-service/src/config/models.yaml` Diagnosis route | physical provider candidate order/settings | LiteLLM `bodysense-diagnosis` model group + AgentConfiguration generation settings | gateway cutover green and non-Diagnosis consumers migrated or split | queued |
| `apps/ai-service/src/ai/pydantic_model.py` | constructs physical PydanticAI models and `FallbackModel` | single gateway-backed PydanticAI model adapter | Diagnosis and all remaining typed Agents have no physical-provider construction dependency | queued |
| PydanticAI `FallbackModel` | application-level provider failover | LiteLLM retry/fallback | gateway fallback characterization green | retired from Diagnosis in Phase 2; legacy non-Diagnosis callers remain |
| provider credentials in `ai-service` | provider authentication | `litellm-gateway` only | compose/env validation proves AI service has only internal gateway credential | migration bridge: gateway owns new route; remove from AI service in Phase 2 cutover |
| legacy `AIService` / `ModelRouter` provider policy | general application-level provider routing | gateway transport + role-specific typed Agents | repository-wide callers migrated; non-Agent utility calls have explicit gateway client | queued |
| ad-hoc Diagnosis golden tests as release qualification | scattered deterministic examples | Pydantic Evals dataset/evaluators/qualification report | equivalent regression cases represented in eval suites | queued |
| Diagnosis `rag_context` / `rag_results` request fields | allowed preloaded retrieval to bypass acquisition policy | no preloaded Diagnosis RAG; controlled tool acquisition only | Go has no caller and Python rejects the legacy fields | retired in Phase 5 |
| free-form `information_gaps` as acquisition control | v1 Champion still exposes legacy bare `search_evidence`; v2 Challenger uses typed gaps | typed `EvidenceGap` + `EvidenceBudget` + attempt trace | final promoted Diagnosis configuration no longer exposes the v1 tool and replay can still resolve v1 | v2 retired this control in Phase 5; v1 compatibility remains until Phase 9/10 |

| `DIAGNOSIS_MODEL_BACKEND` legacy migration branch | protects old Watchtower runtime from image-before-Compose cutover | LiteLLM-only Diagnosis boundary | all deployed Compose/runtime files set `litellm` and deployment smoke proves gateway before next AI image rollout | active migration bridge; delete in Phase 10 |

This ledger is updated as each migration PR closes a deletion condition. A surface is not considered retired while reachable callers remain.
