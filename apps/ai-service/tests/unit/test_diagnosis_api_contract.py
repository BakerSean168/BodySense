"""HTTP contract tests for the BodyState-pinned Diagnosis adapter."""


class _FakeDiagnosisService:
    def __init__(self, result=None, error: Exception | None = None):
        self.result = result
        self.error = error
        self.captured = None

    async def generate_diagnosis(self, **kwargs):
        self.captured = kwargs
        if self.error is not None:
            raise self.error
        return self.result


def _payload() -> dict:
    return {
        "user_id": "user-1",
        "body_state_revision": 12,
        "configuration_id": "diag-config-test",
        "body_state": {
            "current_revision": 12,
            "facts": [{"id": "fact-1", "kind": "discomfort", "value": "颈肩酸胀"}],
            "observations": [],
        },
        "relevant_history": [{"revision": 11, "change_type": "fact.temporal_changed"}],
        "profile": {"gender": "female", "birth_date": "1996-08-27", "age_years": 30},
    }


def test_analyze_diagnosis_preserves_body_state_request_and_response_contract(client, monkeypatch):
    expected = {
        "status": "completed",
        "candidates": [{"name": "颈肩姿势负荷相关模式", "confidence": "中"}],
        "governance": {"verdict": "accepted", "kind": "diagnosis", "reasons": [], "issues": []},
    }
    fake = _FakeDiagnosisService(result=expected)
    monkeypatch.setattr("src.api.routes.diagnosis.get_diagnosis_service", lambda: fake)

    payload = _payload()
    response = client.post("/api/diagnosis/analyze", json=payload)

    assert response.status_code == 200
    assert response.json() == expected
    assert fake.captured == payload


def test_analyze_diagnosis_applies_optional_defaults(client, monkeypatch):
    fake = _FakeDiagnosisService(result={"status": "insufficient_information", "candidates": []})
    monkeypatch.setattr("src.api.routes.diagnosis.get_diagnosis_service", lambda: fake)
    payload = {
        "body_state_revision": 12,
        "configuration_id": "diag-config-test",
        "body_state": {"current_revision": 12, "facts": [], "observations": []},
    }

    response = client.post("/api/diagnosis/analyze", json=payload)

    assert response.status_code == 200
    assert fake.captured == {
        "user_id": "",
        "body_state_revision": 12,
        "configuration_id": "diag-config-test",
        "body_state": payload["body_state"],
        "relevant_history": [],
        "profile": {},
    }


def test_analyze_diagnosis_rejects_missing_durable_revision_at_http_boundary(client):
    response = client.post(
        "/api/diagnosis/analyze",
        json={"body_state": {"current_revision": 0, "facts": [], "observations": []}},
    )
    assert response.status_code == 422


def test_analyze_diagnosis_maps_domain_validation_error_to_422(client, monkeypatch):
    fake = _FakeDiagnosisService(
        error=ValueError("body_state_revision does not match body_state.current_revision")
    )
    monkeypatch.setattr("src.api.routes.diagnosis.get_diagnosis_service", lambda: fake)

    response = client.post("/api/diagnosis/analyze", json=_payload())

    assert response.status_code == 422
    assert response.json() == {
        "detail": "body_state_revision does not match body_state.current_revision"
    }


def test_analyze_diagnosis_preserves_rejected_governance_body(client, monkeypatch):
    expected = {
        "governance": {
            "verdict": "rejected",
            "kind": "diagnosis",
            "reasons": ["unsafe clinical claim"],
            "issues": [{"policy": "red_flag"}],
        },
        "safety_fallback": "本次结果未通过安全审查，请补充信息或寻求专业评估。",
    }
    fake = _FakeDiagnosisService(result=expected)
    monkeypatch.setattr("src.api.routes.diagnosis.get_diagnosis_service", lambda: fake)

    response = client.post("/api/diagnosis/analyze", json=_payload())

    assert response.status_code == 200
    assert response.json() == expected


def test_analyze_diagnosis_rejects_legacy_preloaded_rag_bypass(client):
    payload = _payload()
    payload["rag_context"] = "legacy preloaded evidence"
    payload["rag_results"] = [{"evidence_id": "legacy"}]

    response = client.post("/api/diagnosis/analyze", json=payload)

    assert response.status_code == 422
    detail = response.json()["detail"]
    assert {item["loc"][-1] for item in detail} == {"rag_context", "rag_results"}
