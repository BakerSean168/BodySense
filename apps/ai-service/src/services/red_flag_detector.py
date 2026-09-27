"""Red flag symptom detector for consultation safety.

Design note: The default/literal v1 detector intentionally remains context-free
and high recall; this preserves historical behavior. The opt-in v2 detector is
an explicit-negation-aware revision used only by the v5 Diagnosis configuration.
The opt-in v3 detector adds a small allowlisted bridge grammar for v6; it does
not broaden v2 semantics. Both revisions use conservative local rules rather
than general NLP rewriting: ambiguous phrasing remains a positive signal.
"""

import re
from dataclasses import dataclass, field
from typing import Any

RED_FLAG_DETECTOR_REVISION_LITERAL_V1 = "red-flag-detector-literal-v1"
RED_FLAG_DETECTOR_REVISION_NEGATION_AWARE_V2 = "red-flag-detector-negation-aware-v2"
RED_FLAG_DETECTOR_REVISION_NEGATION_BRIDGE_V3 = "red-flag-detector-negation-bridge-v3"
DEFAULT_RED_FLAG_DETECTOR_REVISION = RED_FLAG_DETECTOR_REVISION_LITERAL_V1

_NEGATION_CUE_RE = re.compile(
    r"(?P<cue>也没有|并没有|并无|没有|没|未见|未出现|否认|不伴有|不伴|不存在|无)\s*$"
)
_AMBIGUOUS_NEGATION_PREFIX_RE = re.compile(
    r"(?:不是|并非|非|没有|没|无|不|未|不能说|不能确认|无法确认)\s*$"
)
_NEGATION_BRIDGE_RE = re.compile(r"(?:明显的?|出现|存在|任何|再出现)?\s*$")


def _is_explicitly_negated(text: str, start: int) -> bool:
    """Return true only for a narrow, unambiguous local absence clause."""
    prefix = text[max(0, start - 24) : start]
    match = _NEGATION_CUE_RE.search(prefix)
    if match is None:
        return False
    before_cue = prefix[: match.start()]
    return _AMBIGUOUS_NEGATION_PREFIX_RE.search(before_cue) is None


def _is_explicitly_negated_with_bridge(text: str, start: int) -> bool:
    """Recognize v3's finite, safe bridge tokens without free-text gaps."""
    prefix = text[max(0, start - 32) : start]
    bridge = _NEGATION_BRIDGE_RE.search(prefix)
    if bridge is None:
        return False
    cue_prefix = prefix[: bridge.start()]
    return _is_explicitly_negated(cue_prefix, len(cue_prefix))


def _keyword_is_present(text: str, keyword: str, revision: str) -> bool:
    if revision == RED_FLAG_DETECTOR_REVISION_LITERAL_V1:
        return keyword in text
    if revision not in {
        RED_FLAG_DETECTOR_REVISION_NEGATION_AWARE_V2,
        RED_FLAG_DETECTOR_REVISION_NEGATION_BRIDGE_V3,
    }:
        raise ValueError(f"unsupported red-flag detector revision: {revision}")
    start = text.find(keyword)
    while start >= 0:
        is_negated = (
            _is_explicitly_negated_with_bridge(text, start)
            if revision == RED_FLAG_DETECTOR_REVISION_NEGATION_BRIDGE_V3
            else _is_explicitly_negated(text, start)
        )
        if not is_negated:
            return True
        start = text.find(keyword, start + len(keyword))
    return False


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
        *,
        revision: str = DEFAULT_RED_FLAG_DETECTOR_REVISION,
    ) -> RedFlagResult:
        """
        Scan extracted info and conversation for red flags.

        Args:
            extracted_info: List of extracted symptom dicts.
            conversation_text: Full conversation text to scan.

        Returns:
            RedFlagResult with detected flags.
        """
        if revision not in {
            RED_FLAG_DETECTOR_REVISION_LITERAL_V1,
            RED_FLAG_DETECTOR_REVISION_NEGATION_AWARE_V2,
            RED_FLAG_DETECTOR_REVISION_NEGATION_BRIDGE_V3,
        }:
            raise ValueError(f"unsupported red-flag detector revision: {revision}")

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

        scan_segments = [combined_text]
        if revision in {
            RED_FLAG_DETECTOR_REVISION_NEGATION_AWARE_V2,
            RED_FLAG_DETECTOR_REVISION_NEGATION_BRIDGE_V3,
        }:
            scan_segments = [conversation_text]
            scan_segments.extend(
                info.get("additional_notes", "")
                for info in extracted_info
                if info.get("additional_notes", "")
            )

        for pattern in RED_FLAG_PATTERNS:
            for keyword in pattern["keywords"]:
                if any(
                    _keyword_is_present(segment, keyword, revision)
                    for segment in scan_segments
                ):
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
        *,
        revision: str = DEFAULT_RED_FLAG_DETECTOR_REVISION,
    ) -> bool:
        """Quick check if any red flags detected."""
        return self.detect(extracted_info, conversation_text, revision=revision).has_red_flags


# Singleton
_detector: RedFlagDetector | None = None


def get_red_flag_detector() -> RedFlagDetector:
    """Get or create the default red flag detector."""
    global _detector
    if _detector is None:
        _detector = RedFlagDetector()
    return _detector
