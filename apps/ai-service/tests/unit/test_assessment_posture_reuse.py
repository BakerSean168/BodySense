"""Assessment reuses stored posture analysis as observation evidence."""

from src.prompts.assessment import format_posture_analysis_section, get_assessment_prompt

SAMPLE = {
    "has_analysis": True,
    "views": [
        {
            "view": "front",
            "analysis": {
                "overall_confidence": "high",
                "findings": [
                    {
                        "key": "uneven_shoulders",
                        "label": "高低肩倾向",
                        "severity": "mild",
                        "evidence": "右侧肩峰略高",
                    }
                ],
                "summary_markdown": "正面观轻微高低肩。",
            },
        }
    ],
    "findings": [],
    "summaries": ["正面观轻微高低肩。"],
}


def test_format_posture_section_includes_stored_findings():
    section = format_posture_analysis_section(SAMPLE)
    assert "高低肩倾向" in section
    assert "analysis_result" in section
    assert "右侧肩峰" in section
    assert "不得直接形成 Diagnosis 或 Treatment" in section


def test_format_posture_section_empty_without_analysis():
    assert format_posture_analysis_section(None) == ""
    assert format_posture_analysis_section({"has_analysis": False}) == ""


def test_assessment_prompt_is_observation_only():
    prompt = get_assessment_prompt(
        {"gender": "female", "birth_date": "1996-08-27", "age_years": 30},
        posture_analysis=SAMPLE,
    )
    assert "高低肩倾向" in prompt
    assert "不构成医疗诊断、治疗方案或运动处方" in prompt
    assert "改善建议" not in prompt
