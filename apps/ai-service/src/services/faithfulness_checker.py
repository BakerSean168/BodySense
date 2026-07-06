"""RAG faithfulness checker for treatment plans.

Limitations (MVP):
- Matching uses substring comparison, which can produce false positives
  for short Chinese terms (e.g., "臀" matching "臀部", "臀桥", "臀肌").
- EXERCISE_ALIASES is a static mapping that must be manually updated
  when the knowledge base adds new exercises.
- No semantic similarity or NLP-based matching; context is not considered.

Future improvements: use jieba tokenization + embedding similarity for
more robust matching, or delegate to an LLM-based faithfulness judge.
"""

from dataclasses import dataclass, field
from typing import Any, Literal


@dataclass
class ExerciseFaithfulness:
    """Faithfulness status for a single exercise."""

    name: str
    grounded: bool
    source: str | None = None
    confidence: Literal["high", "medium", "low"] = "low"


@dataclass
class FaithfulnessResult:
    """Result of faithfulness check for a treatment plan."""

    faithful: bool
    exercises: list[ExerciseFaithfulness] = field(default_factory=list)
    ungrounded_exercises: list[str] = field(default_factory=list)

    def to_dict(self) -> dict[str, Any]:
        """Convert to dict for JSON serialization."""
        return {
            "faithful": self.faithful,
            "exercises": [
                {
                    "name": e.name,
                    "grounded": e.grounded,
                    "source": e.source,
                    "confidence": e.confidence,
                }
                for e in self.exercises
            ],
            "ungrounded_exercises": self.ungrounded_exercises,
        }


# Common exercise name aliases for fuzzy matching
EXERCISE_ALIASES: dict[str, list[str]] = {
    "胸小肌拉伸": ["胸肌拉伸", "胸小肌", "胸大肌拉伸", "门框拉伸"],
    "颈部后缩": ["收下巴", "颈后缩", "下巴后缩", "颈椎后缩"],
    "肩胛骨回缩": ["夹肩胛骨", "扩胸", "肩胛后缩", "挺胸"],
    "猫牛式": ["猫牛伸展", "脊柱屈伸", "四点跪位屈伸"],
    "臀桥": ["臀桥训练", "仰卧臀桥", "glute bridge"],
    "死虫式": ["死虫", "仰卧对侧伸展"],
    "鸟狗式": ["bird dog", "对侧伸展"],
    "平板支撑": ["plank", "平板"],
    "侧平板": ["侧桥", "side plank"],
    "蚌式开合": ["蚌式", "clamshell"],
    "泡沫轴放松": ["泡沫轴", "筋膜放松", "滚筒放松"],
}


def _normalize_exercise_name(name: str) -> str:
    """Normalize exercise name for matching."""
    # Remove common prefixes/suffixes
    name = name.strip()
    for prefix in ["做", "进行", "完成", "练习"]:
        if name.startswith(prefix):
            name = name[len(prefix) :]
    return name.strip()


def _find_matching_source(
    exercise_name: str,
    rag_results: list[dict],
) -> tuple[bool, str | None, str]:
    """
    Find if an exercise matches any RAG result.

    Returns:
        (grounded, source_title, confidence)
    """
    normalized = _normalize_exercise_name(exercise_name).lower()

    # Guard against single-character false positives in substring matching
    if len(normalized) < 2:
        return False, None, "low"

    # Check direct match in RAG results
    for result in rag_results:
        title = (result.get("title") or "").lower()
        content = (result.get("body_markdown") or result.get("content") or "").lower()
        clips = result.get("clips") or []

        # Check title match
        if normalized in title or title in normalized:
            return True, result.get("title"), "high"

        # Check content match
        if normalized in content:
            return True, result.get("title"), "medium"

        # Check clips match
        for clip in clips:
            clip_title = (clip.get("title") or "").lower()
            if normalized in clip_title or clip_title in normalized:
                return True, result.get("title"), "high"

    # Check aliases
    for canonical, aliases in EXERCISE_ALIASES.items():
        if normalized == canonical.lower() or normalized in [a.lower() for a in aliases]:
            # Found an alias, check if canonical matches RAG
            for result in rag_results:
                content = (
                    result.get("body_markdown") or result.get("content") or ""
                ).lower()
                if canonical.lower() in content:
                    return True, result.get("title"), "medium"

    return False, None, "low"


class FaithfulnessChecker:
    """Verify treatment exercises are grounded in knowledge base."""

    def check_treatment_faithfulness(
        self,
        treatment_plan: dict,
        rag_results: list[dict],
    ) -> FaithfulnessResult:
        """
        Check each exercise in treatment plan against RAG results.

        Args:
            treatment_plan: The treatment plan dict with correction_exercises.
            rag_results: RAG search results from knowledge base.

        Returns:
            FaithfulnessResult with per-exercise status.
        """
        exercises_data = treatment_plan.get("correction_exercises", [])
        if not exercises_data:
            return FaithfulnessResult(faithful=False, exercises=[])

        exercise_results: list[ExerciseFaithfulness] = []
        ungrounded: list[str] = []

        for exercise in exercises_data:
            name = exercise.get("name", "")
            if not name:
                continue

            grounded, source, confidence = _find_matching_source(name, rag_results)
            exercise_results.append(
                ExerciseFaithfulness(
                    name=name,
                    grounded=grounded,
                    source=source,
                    confidence=confidence,
                )
            )
            if not grounded:
                ungrounded.append(name)

        return FaithfulnessResult(
            faithful=len(ungrounded) == 0,
            exercises=exercise_results,
            ungrounded_exercises=ungrounded,
        )


# Singleton
_checker: FaithfulnessChecker | None = None


def get_faithfulness_checker() -> FaithfulnessChecker:
    """Get or create the default faithfulness checker."""
    global _checker
    if _checker is None:
        _checker = FaithfulnessChecker()
    return _checker
