# Diagnosis / AI Routing Legacy Deletion Ledger

> Final status after Phase 10 (2026-08-20). Compatibility code is not considered
> retired until reachability is gone or the remaining artifact has an explicit
> release-governance reason to stay.

| Legacy surface | Replacement | Final status |
| --- | --- | --- |
| Go/Python Diagnosis `use_case="llm.json"` | immutable AgentConfiguration selected by Go | **retired Phase 2**; request field absent |
| `src/config/models.yaml` provider-routing truth | `docker/litellm/config.yaml` logical model groups | **deleted Phase 10** after all remaining callers migrated |
| `src/ai/pydantic_model.py` physical PydanticAI construction | `src/ai/gateway.py` + Diagnosis gateway adapter | **deleted Phase 10** |
| PydanticAI `FallbackModel` provider failover | LiteLLM retry/fallback | **retired repository-wide Phase 10**; no constructor/import remains |
| `src/ai/router.py` / application circuit-breaker/provider selection | LiteLLM router | **deleted Phase 10** |
| `src/ai/providers/` physical OpenAI-compatible provider clients | one internal gateway transport in `AIService` | **deleted Phase 10** |
| `AIService` provider-routing responsibility | provider-neutral logical-route gateway facade | **retired Phase 10**; class remains only to preserve business request/stream contract |
| `llm.json` / `llm.text` generic business route names | role-specific routes (`assessment.generate`, `treatment.proposal`, `knowledge.*`, `conversation.title`) | **retired repository-wide Phase 10** |
| Mimo/OpenRouter LLM credentials in `ai-service` | credentials injected only into `litellm-gateway` | **retired Phase 10**; Embedding/ASR credentials remain separate by design |
| `DIAGNOSIS_MODEL_BACKEND` migration branch | unconditional Diagnosis LiteLLM path | **deleted Phase 10** after repeated prod-like gateway/deploy validation |
| `DIAGNOSIS_AGENT_CONFIGURATION_ID` rollout compatibility alias | `DIAGNOSIS_CHAMPION_CONFIGURATION_ID` | **deleted Phase 10** |
| ad-hoc Diagnosis golden examples as release qualification | Pydantic Evals typed dataset, slices, hard gates, paired comparison | **retired as release gate Phase 4/10**; low-level red-flag unit tests remain unit tests, not promotion evidence |
| Diagnosis preloaded `rag_context` / `rag_results` | controlled EvidenceGap tool acquisition | **retired Phase 5**; Python request forbids legacy fields |
| free-form `information_gaps` driving external search | typed `EvidenceGap` + `EvidenceBudget` + `EvidenceAttempt` | **retired in v2/v3 configs**; immutable v1 tool policy remains while v1 is a real Champion/rollback/replay artifact |

## Final reachability rule

The repository has one **provider-routing** stack. Older immutable Diagnosis Agent
configurations remain because release governance needs replay/rollback identities,
but those configurations execute through the same LiteLLM gateway and therefore
do not constitute a parallel provider stack.

The v1 bare evidence-search tool may leave the serving set only after an actual
Phase-9 promotion gate advances the real environment. Historical manifest
resolution remains valid after that; no provider router needs to return.
