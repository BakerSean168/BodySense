"""Red flag symptom detector for consultation safety.

Design note: This detector uses a conservative (high-recall) strategy.
Keyword matching without context analysis means phrases like "我之前摔倒过，
但现在好多了" will still trigger the "trauma" flag due to the "摔倒" keyword.
This is intentional — in a health consultation context, false negatives
(missing a real red flag) are far more costly than false positives (showing
an unnecessary safety warning). Users can dismiss false-positive warnings.

Future improvement: use NLP context analysis or an LLM-based classifier
to reduce false positives while maintaining high recall.
"""

from dataclasses import dataclass, field
from typing import Any


@dataclass
class RedFlag:
    """A detected red flag symptom."""

    category: str
    message: str
    matched_text: str
    source: str  # "extracted_info" or "conversation"


@dataclass
class RedFlagResult:
    """Result of red flag detection."""

    has_red_flags: bool
    flags: list[RedFlag] = field(default_factory=list)

    def to_dict(self) -> dict[str, Any]:
        """Convert to dict for JSON serialization."""
        return {
            "has_red_flags": self.has_red_flags,
            "flags": [
                {
                    "category": f.category,
                    "message": f.message,
                    "matched_text": f.matched_text,
                    "source": f.source,
                }
                for f in self.flags
            ],
        }


# Red flag patterns organized by category
RED_FLAG_PATTERNS: list[dict] = [
    # Severe pain
    {
        "category": "severe_pain",
        "keywords": ["剧烈疼痛", "严重疼痛", "无法忍受", "疼得睡不着", "疼痛难忍"],
        "message": "您描述的疼痛程度较为严重，建议尽快就医进行专业评估。",
    },
    # Radiating pain / nerve compression
    {
        "category": "radiating_pain",
        "keywords": [
            "放射痛",
            "放射到手臂",
            "放射到腿部",
            "串到手臂",
            "串到腿",
            "触电感",
        ],
        "message": "疼痛放射可能提示神经受压，建议就医进行影像学检查。",
    },
    # Neurological symptoms
    {
        "category": "numbness",
        "keywords": ["麻木无力", "肢体麻木", "手指麻木", "脚趾麻木", "肌肉萎缩"],
        "message": "肢体麻木或无力需要专业神经功能评估，建议就医检查。",
    },
    {
        "category": "neurological",
        "keywords": ["头晕", "视物模糊", "吞咽困难", "走路不稳", "踩棉花感"],
        "message": "您描述的症状可能涉及神经系统，建议尽快就医。",
    },
    # Trauma
    {
        "category": "trauma",
        "keywords": ["外伤", "摔倒", "撞击", "骨折", "扭伤后肿胀", "车祸"],
        "message": "外伤后需要专业评估，建议就医进行影像检查排除骨折等损伤。",
    },
    # Progressive worsening
    {
        "category": "worsening",
        "keywords": [
            "持续加重",
            "越来越严重",
            "无法缓解",
            "休息也不缓解",
            "越来越疼",
        ],
        "message": "症状持续加重需要专业评估，建议就医查明原因。",
    },
    # Infection signs
    {
        "category": "infection",
        "keywords": ["发热", "红肿热痛", "局部红肿", "感染"],
        "message": "局部红肿热痛或发热可能提示感染，建议就医处理。",
    },
    # Systemic symptoms
    {
        "category": "systemic",
        "keywords": ["体重下降", "夜间盗汗", "不明原因消瘦"],
        "message": "全身性症状需要全面检查，建议尽快就医。",
    },
]

# Severity keywords for extracted info
SEVERITY_RED_FLAGS = ["重度"]


class RedFlagDetector:
    """Detect red flag symptoms that require immediate medical attention."""

    def detect(
        self,
        extracted_info: list[dict],
        conversation_text: str = "",
    ) -> RedFlagResult:
        """
        Scan extracted info and conversation for red flags.

        Args:
            extracted_info: List of extracted symptom dicts.
            conversation_text: Full conversation text to scan.

        Returns:
            RedFlagResult with detected flags.
        """
        flags: list[RedFlag] = []

        # Check extracted info for severity red flags
        for info in extracted_info:
            severity = info.get("severity", "")
            body_part = info.get("body_part", "")
            if severity in SEVERITY_RED_FLAGS:
                flags.append(
                    RedFlag(
                        category="severe_symptom",
                        message=f"{body_part}的{severity}症状需要专业评估，建议就医。",
                        matched_text=f"{body_part}：{severity}",
                        source="extracted_info",
                    )
                )

        # Check conversation text against patterns
        combined_text = conversation_text
        # Also include extracted info notes
        for info in extracted_info:
            notes = info.get("additional_notes", "")
            if notes:
                combined_text += " " + notes

        for pattern in RED_FLAG_PATTERNS:
            for keyword in pattern["keywords"]:
                if keyword in combined_text:
                    flags.append(
                        RedFlag(
                            category=pattern["category"],
                            message=pattern["message"],
                            matched_text=keyword,
                            source="conversation",
                        )
                    )
                    break  # One flag per category

        # Deduplicate by category
        seen_categories: set[str] = set()
        unique_flags: list[RedFlag] = []
        for flag in flags:
            if flag.category not in seen_categories:
                seen_categories.add(flag.category)
                unique_flags.append(flag)

        return RedFlagResult(
            has_red_flags=len(unique_flags) > 0,
            flags=unique_flags,
        )

    def is_red_flag(
        self,
        extracted_info: list[dict],
        conversation_text: str = "",
    ) -> bool:
        """Quick check if any red flags detected."""
        return self.detect(extracted_info, conversation_text).has_red_flags


# Singleton
_detector: RedFlagDetector | None = None


def get_red_flag_detector() -> RedFlagDetector:
    """Get or create the default red flag detector."""
    global _detector
    if _detector is None:
        _detector = RedFlagDetector()
    return _detector
