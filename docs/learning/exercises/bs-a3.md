# BS-A3 · evidence retrieval, acquisition and admissibility

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **BodySense Agent Extension A3**

## Concept

`retrieved != admissible != gap resolved`: evidence acquisition is a traceable process whose semantic usefulness and authority are evaluated separately from retrieval success.

## Prerequisites

- BS-A1
- BS-A2

## BodySense target files

- `apps/ai-service/src/agents/evidence.py`
- `apps/ai-service/src/rag/external_evidence.py`
- `apps/ai-service/src/evals/diagnosis_evidence_policy.py`
- `apps/ai-service/tests/unit/test_diagnosis_evidence_acquisition.py`
- `apps/ai-service/tests/unit/test_diagnosis_evidence_evals.py`

## Prediction before reading/running

For one missing Diagnosis fact, predict the sequence `gap -> acquisition attempt -> retrieved item -> source identity -> admissibility -> gap status`. State which step can fail while the previous step still succeeds.

## Task

Trace one Diagnosis evidence gap from proposal through acquisition/source identity into admissibility policy. Distinguish retrieved evidence, admissible evidence, attempted acquisition and resolved gap; identify which layer may make each claim.

## Failure case

A retriever returns plausible text, but source identity/provenance is missing or the evidence does not actually satisfy the requested gap. Predict whether the gap closes and which durable/runtime trace records the failed attempt.

## Verification command / evidence

- `cd apps/ai-service && .venv/bin/python -m pytest tests/unit/test_diagnosis_evidence_acquisition.py tests/unit/test_diagnosis_evidence_evals.py -q`
- Record one evidence item/gap/acquisition trace and explain why retrieval alone cannot close the gap.

Passing tests is not enough for L4. Explain why each test/evidence item distinguishes acquisition success from admissibility and what observation would falsify your explanation.

## Explain-back questions

- Why is a successful vector/search hit not automatically evidence?
- Who owns source identity, admissibility and the final gap-resolution decision?
- Why must failed acquisition attempts be inspectable instead of disappearing?

## Production change

None unless a source/admissibility/gap invariant is missing; add a focused regression test before altering retrieval or policy.

## L4 acceptance

Complete only when the learner can independently trace a gap through acquisition/admissibility, design a negative case, and justify the authority boundary from code/tests without relying on an AI-generated conclusion.
