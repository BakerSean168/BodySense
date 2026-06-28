import pytest

from src.services.diagnosis_service import DiagnosisService


class _FakeAiResponse:
    """Minimal stand-in for AiResponse with just the .text attribute."""

    def __init__(self, text: str):
        self.text = text


class _FakeAIService:
    """Fake AIService that returns pre-configured JSON text."""

    def __init__(self, text: str):
        self._text = text

    async def generate(self, req):
        return _FakeAiResponse(self._text)


@pytest.mark.asyncio
async def test_generate_diagnosis_validates_response_schema(monkeypatch):
    monkeypatch.setattr(
        "src.services.diagnosis_service.AIService",
        lambda: _FakeAIService(
            """
            {
              "diagnoses": [
                {
                  "name": "头前伸倾向",
                  "confidence": "中",
                  "severity": "轻度",
                  "basis": "用户描述颈肩酸胀且久坐后明显。",
                  "typical_symptoms": "颈肩酸胀、头颈前移、久坐后不适。",
                  "differential": "需与急性颈椎神经症状区分。"
                }
              ]
            }
            """
        ),
    )

    result = await DiagnosisService().generate_diagnosis(
        extracted_info=[{"body_part": "颈椎", "symptom_type": "酸胀"}],
        profile={},
    )

    assert result["diagnoses"][0]["name"] == "头前伸倾向"
    assert result["diagnoses"][0]["confidence"] == "中"


@pytest.mark.asyncio
async def test_generate_diagnosis_rejects_invalid_schema(monkeypatch):
    monkeypatch.setattr(
        "src.services.diagnosis_service.AIService",
        lambda: _FakeAIService('{"diagnoses": [{"name": "头前伸倾向"}]}'),
    )

    with pytest.raises(ValueError, match="Invalid diagnosis response schema"):
        await DiagnosisService().generate_diagnosis(
            extracted_info=[{"body_part": "颈椎", "symptom_type": "酸胀"}],
            profile={},
        )


@pytest.mark.asyncio
async def test_generate_treatment_validates_response_schema(monkeypatch):
    monkeypatch.setattr(
        "src.services.diagnosis_service.AIService",
        lambda: _FakeAIService(
            """
            {
              "treatment_plan": {
                "goal": "缓解颈肩酸胀并改善头前伸",
                "duration_weeks": 4,
                "correction_exercises": [
                  {
                    "name": "胸小肌拉伸",
                    "description": "靠门框进行温和拉伸。",
                    "sets": "2组",
                    "reps": "每次30秒",
                    "notes": "避免耸肩。"
                  }
                ],
                "daily_habits": ["每45分钟起身活动"],
                "expected_timeline": "2-4周观察酸胀变化。",
                "warning_signs": ["出现放射痛或麻木无力时及时就医"]
              }
            }
            """
        ),
    )

    result = await DiagnosisService().generate_treatment(
        confirmed_diagnosis={"name": "头前伸倾向", "severity": "轻度"},
        extracted_info=[{"body_part": "颈椎", "symptom_type": "酸胀"}],
        profile={},
    )

    plan = result["treatment_plan"]
    assert plan["goal"] == "缓解颈肩酸胀并改善头前伸"
    assert plan["correction_exercises"][0]["name"] == "胸小肌拉伸"


# --- Diagnosis integration with red flags ---


DIAGNOSIS_JSON = """
{
  "diagnoses": [
    {
      "name": "头前伸倾向",
      "confidence": "中",
      "severity": "轻度",
      "basis": "颈肩酸胀",
      "typical_symptoms": "颈肩酸胀",
      "differential": "需区分"
    }
  ]
}
"""


@pytest.mark.asyncio
async def test_generate_diagnosis_includes_red_flags_when_detected(monkeypatch):
    monkeypatch.setattr(
        "src.services.diagnosis_service.AIService",
        lambda: _FakeAIService(DIAGNOSIS_JSON),
    )

    result = await DiagnosisService().generate_diagnosis(
        extracted_info=[{"body_part": "颈椎", "symptom_type": "酸胀"}],
        profile={},
        conversation_summary="剧烈疼痛，手指麻木无力",
    )

    assert "red_flags" in result
    assert result["red_flags"]["has_red_flags"] is True
    assert len(result["red_flags"]["flags"]) > 0


@pytest.mark.asyncio
async def test_generate_diagnosis_no_red_flags_for_mild(monkeypatch):
    monkeypatch.setattr(
        "src.services.diagnosis_service.AIService",
        lambda: _FakeAIService(DIAGNOSIS_JSON),
    )

    result = await DiagnosisService().generate_diagnosis(
        extracted_info=[{"body_part": "颈椎", "symptom_type": "酸胀", "severity": "轻度"}],
        profile={},
        conversation_summary="肩膀有点酸",
    )

    assert "red_flags" not in result


# --- Treatment integration with faithfulness ---


TREATMENT_JSON = """
{
  "treatment_plan": {
    "goal": "缓解颈肩酸胀",
    "duration_weeks": 4,
    "correction_exercises": [
      {
        "name": "胸小肌拉伸",
        "description": "靠门框拉伸。",
        "sets": "2组",
        "reps": "30秒",
        "notes": ""
      }
    ],
    "daily_habits": ["每45分钟起身"],
    "expected_timeline": "2-4周",
    "warning_signs": ["放射痛就医"]
  }
}
"""


@pytest.mark.asyncio
async def test_generate_treatment_includes_faithfulness_with_rag(monkeypatch):
    monkeypatch.setattr(
        "src.services.diagnosis_service.AIService",
        lambda: _FakeAIService(TREATMENT_JSON),
    )

    rag_results = [
        {"title": "胸小肌拉伸方法", "summary": "拉伸", "content": ""}
    ]

    result = await DiagnosisService().generate_treatment(
        confirmed_diagnosis={"name": "头前伸倾向", "severity": "轻度"},
        extracted_info=[{"body_part": "颈椎", "symptom_type": "酸胀"}],
        profile={},
        rag_results=rag_results,
    )

    assert "faithfulness" in result
    assert result["faithfulness"]["faithful"] is True
    assert "citations" in result


@pytest.mark.asyncio
async def test_generate_treatment_no_faithfulness_without_rag(monkeypatch):
    monkeypatch.setattr(
        "src.services.diagnosis_service.AIService",
        lambda: _FakeAIService(TREATMENT_JSON),
    )

    result = await DiagnosisService().generate_treatment(
        confirmed_diagnosis={"name": "头前伸倾向", "severity": "轻度"},
        extracted_info=[{"body_part": "颈椎", "symptom_type": "酸胀"}],
        profile={},
    )

    assert "faithfulness" not in result
    assert "citations" not in result
