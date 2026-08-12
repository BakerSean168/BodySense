"""Characterization tests for the public diagnosis HTTP contract.

These tests intentionally exercise only the FastAPI adapter boundary. The
underlying diagnosis implementation is replaced with a fake so the contract
stays stable while the service is later migrated to PydanticAI.
"""


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


def test_analyze_diagnosis_preserves_http_request_and_response_contract(client, monkeypatch):
    expected = {
        "diagnoses": [
            {
                "name": "头前伸倾向",
                "confidence": "中",
                "severity": "轻度",
                "basis": "久坐后颈肩酸胀",
                "typical_symptoms": "颈肩酸胀",
            }
        ],
        "citations": [{"title": "头前伸自测"}],
        "governance": {
            "verdict": "accepted",
            "kind": "diagnosis",
            "reasons": [],
            "issues": [],
        },
    }
    fake = _FakeDiagnosisService(result=expected)
    monkeypatch.setattr(
        "src.api.routes.diagnosis.get_diagnosis_service",
        lambda: fake,
    )

    payload = {
        "extracted_info": [{"body_part": "颈椎", "symptom_type": "酸胀"}],
        "profile": {"age": 30, "occupation": "程序员"},
        "conversation_summary": "久坐后颈肩酸胀",
        "rag_context": "## 知识库\n头前伸相关资料",
        "rag_results": [{"title": "头前伸自测"}],
        "use_case": "llm.json",
    }

    response = client.post("/api/diagnosis/analyze", json=payload)

    assert response.status_code == 200
    assert response.json() == expected
    assert fake.captured == payload


def test_analyze_diagnosis_maps_domain_validation_error_to_422(client, monkeypatch):
    fake = _FakeDiagnosisService(error=ValueError("Invalid diagnosis response schema"))
    monkeypatch.setattr(
        "src.api.routes.diagnosis.get_diagnosis_service",
        lambda: fake,
    )

    response = client.post(
        "/api/diagnosis/analyze",
        json={
            "extracted_info": [],
            "profile": {},
            "use_case": "llm.json",
        },
    )

    assert response.status_code == 422
    assert response.json() == {"detail": "Invalid diagnosis response schema"}


def test_analyze_diagnosis_preserves_rejected_governance_body(client, monkeypatch):
    expected = {
        "governance": {
            "verdict": "rejected",
            "kind": "diagnosis",
            "reasons": ["unsafe clinical claim"],
            "issues": [{"policy": "red_flag"}],
        },
        "safety_fallback": (
            "本次结果未通过安全审查，请补充信息或寻求专业评估。"
        ),
    }
    fake = _FakeDiagnosisService(result=expected)
    monkeypatch.setattr(
        "src.api.routes.diagnosis.get_diagnosis_service",
        lambda: fake,
    )

    response = client.post(
        "/api/diagnosis/analyze",
        json={
            "extracted_info": [],
            "profile": {},
            "use_case": "llm.json",
        },
    )

    assert response.status_code == 200
    assert response.json() == expected
