"""Prompts and view metadata for posture photo analysis.

Phase 1 is pure-VLM: the model must describe only what is observable and must
NOT emit any numeric angles (metrics are cleared by governance regardless, but
the prompt reinforces the rule). Findings are constrained per view so the model
cannot guess sagittal problems from a frontal photo, etc.
"""

from __future__ import annotations

# Human-readable Chinese label for each view.
VIEW_LABEL: dict[str, str] = {
    "front": "正面",
    "side": "侧面",
    "back": "背面",
}

# Findings that are physically judgeable from each view. Prompt + governance
# both enforce this allow-list so the model never guesses across views.
VIEW_ALLOWED_KEYS: dict[str, list[str]] = {
    "side": [
        "forward_head",  # 头前移
        "rounded_shoulders",  # 圆肩
        "kyphosis",  # 驼背
        "pelvic_anterior_tilt",  # 骨盆前倾
        "knee_hyperextension",  # 膝超伸
    ],
    "front": [
        "shoulder_tilt",  # 高低肩
        "pelvic_lateral_tilt",  # 骨盆侧倾
        "head_tilt",  # 头侧倾
        "knee_valgus_varus",  # 膝内外翻
    ],
    "back": [
        "shoulder_tilt",  # 高低肩
        "scapular_asymmetry",  # 肩胛不对称
        "spinal_lateral_deviation",  # 脊柱侧弯倾向
        "pelvic_lateral_tilt",  # 骨盆侧倾
    ],
}

# Label lookup used to normalize/repair finding labels.
KEY_LABELS: dict[str, str] = {
    "forward_head": "头前移倾向",
    "rounded_shoulders": "圆肩倾向",
    "kyphosis": "驼背倾向",
    "pelvic_anterior_tilt": "骨盆前倾倾向",
    "knee_hyperextension": "膝超伸倾向",
    "shoulder_tilt": "高低肩倾向",
    "pelvic_lateral_tilt": "骨盆侧倾倾向",
    "head_tilt": "头侧倾倾向",
    "knee_valgus_varus": "膝内外翻倾向",
    "scapular_asymmetry": "肩胛不对称",
    "spinal_lateral_deviation": "脊柱侧弯倾向",
}

DEFAULT_DISCLAIMER = (
    "本分析基于照片视觉判断，仅供参考，不构成医疗诊断。"
    "如有明显不适或疑似结构性问题，请及时就医由专业人员评估。"
)


def _allowed_keys_block(view: str) -> str:
    keys = VIEW_ALLOWED_KEYS.get(view, [])
    lines = [f"- {k}（{KEY_LABELS.get(k, k)}）" for k in keys]
    return "\n".join(lines)


def build_posture_system_prompt(view: str) -> str:
    """Build the system prompt for a specific view."""
    view_label = VIEW_LABEL.get(view, view)
    allowed = _allowed_keys_block(view)
    header = (
        f"你是一位专业的体态评估助手。用户提供了一张{view_label}站姿照片，"
        "请仅基于**照片中可观察到的现象**进行体态评估。"
    )
    return f"""{header}

## 铁律（必须遵守）
1. 只描述照片中可见的现象，使用"倾向""疑似"等表述，**绝不做确诊**。
2. 本次为「{view_label}视角」，**只能输出下列 finding.key**，跨视角的项目一律不得输出：
{allowed}
3. **严禁输出任何角度或数值**（本阶段 metric 必须为 null）。不要编造"颅椎角 48 度"这类具体度数。
4. 若照片中人体不清晰或无法判断，输出空的 findings 列表并在 summary 中说明。
5. 若观察到**明显不对称、疑似脊柱侧弯、双肩高度差异显著**等情况，在 red_flags 中提示并建议就医。

## 输出格式（严格输出以下 JSON，不要包含多余文字）
{{
  "schema_version": 1,
  "view": "{view}",
  "overall_confidence": "high | medium | low",
  "findings": [
    {{
      "key": "上面允许的 key 之一",
      "label": "简短中文标签",
      "severity": "mild | moderate | marked",
      "confidence": "high | medium | low",
      "evidence": "照片中可观察到的依据",
      "metric": null
    }}
  ],
  "red_flags": [
    {{ "category": "severe_asymmetry", "message": "……建议就医评估" }}
  ],
  "summary_markdown": "用通俗中文总结主要发现，末尾附上免责声明。",
  "disclaimer": "{DEFAULT_DISCLAIMER}"
}}
"""
