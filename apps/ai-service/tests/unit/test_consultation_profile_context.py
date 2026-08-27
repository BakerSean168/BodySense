from src.prompts.consultation import format_profile_context


def test_profile_context_prefers_health_context_fields() -> None:
    context = format_profile_context(
        {
            "gender": "male",
            "birth_date": "1998-05-20",
            "age_years": 28,
            "age": 28,
            "activity_pattern": "久坐为主，每次连续坐 2-3 小时",
            "occupation": "程序员",
            "sleep_pattern": "轮班，平均每天睡 6-7 小时",
            "sleep_time": "23:00",
            "wake_time": "07:00",
            "exercise_type": "力量训练",
            "exercise_frequency": "3-4",
            "injury_history": "左膝旧伤",
            "self_description": "最近肩颈不舒服",
        }
    )

    assert "出生日期：1998-05-20（系统计算年龄：28岁）" in context
    assert "日常活动与工作习惯：久坐为主，每次连续坐 2-3 小时" in context
    assert "睡眠与作息：轮班，平均每天睡 6-7 小时" in context
    assert "程序员" not in context
    assert "自我描述" not in context
    assert "最近肩颈不舒服" not in context


def test_profile_context_keeps_safe_legacy_fallbacks_for_existing_profiles() -> None:
    context = format_profile_context(
        {
            "age": 30,
            "occupation": "教师",
            "sleep_time": "23:30",
            "wake_time": "07:00",
        }
    )

    assert "年龄（旧档案字段）：30岁" in context
    assert "教师" not in context
    assert "作息（旧档案字段）：23:30 入睡，07:00 起床" in context
