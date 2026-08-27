from src.prompts.consultation import format_profile_context


def test_profile_context_contains_stable_identity_only() -> None:
    context = format_profile_context(
        {
            "gender": "male",
            "birth_date": "1998-05-20",
            "age_years": 28,
            # Even if a stale caller sends mutable health fields, Profile context
            # must not duplicate BodyState truth.
            "activity_pattern": "久坐为主",
            "sleep_pattern": "轮班",
            "exercise_type": "力量训练",
            "injury_history": "左膝旧伤",
            "weight_kg": 72,
            "occupation": "程序员",
        }
    )

    assert "性别：male" in context
    assert "出生日期：1998-05-20（系统计算年龄：28岁）" in context
    assert "久坐" not in context
    assert "轮班" not in context
    assert "力量训练" not in context
    assert "左膝旧伤" not in context
    assert "72" not in context
    assert "程序员" not in context


def test_empty_stable_profile_is_explicit() -> None:
    assert format_profile_context({}) == "（用户尚未填写稳定身份信息）"
