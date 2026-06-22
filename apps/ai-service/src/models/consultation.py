"""Data models for consultation chat."""

from dataclasses import dataclass, field
from typing import Any


@dataclass
class SymptomInfo:
    """Extracted symptom information."""

    body_part: str
    symptom_type: str | None = None
    duration: str | None = None
    trigger: str | None = None
    relief: str | None = None
    severity: str | None = None
    additional_notes: str | None = None


@dataclass
class ExtractedInfo:
    """Container for all extracted information from a session."""

    symptoms: list[SymptomInfo] = field(default_factory=list)

    def add_symptom(self, info: dict[str, Any]) -> None:
        """Add or update symptom info for a body part."""
        body_part = info.get("body_part", "")
        # Check if we already have info for this body part
        for existing in self.symptoms:
            if existing.body_part == body_part:
                # Update non-empty fields
                for key, value in info.items():
                    if value and hasattr(existing, key):
                        setattr(existing, key, value)
                return
        # New body part
        fields = SymptomInfo.__dataclass_fields__
        filtered = {k: v for k, v in info.items() if k in fields}
        self.symptoms.append(SymptomInfo(**filtered))

    def to_dict(self) -> list[dict[str, Any]]:
        """Convert to list of dicts for JSON serialization."""
        return [
            {k: v for k, v in vars(s).items() if v is not None}
            for s in self.symptoms
        ]

    @classmethod
    def from_dict(cls, data: list[dict[str, Any]]) -> "ExtractedInfo":
        """Create from list of dicts."""
        info = cls()
        fields = SymptomInfo.__dataclass_fields__
        for item in data:
            filtered = {k: v for k, v in item.items() if k in fields}
            info.symptoms.append(SymptomInfo(**filtered))
        return info


@dataclass
class ChatContext:
    """Context for a chat session."""

    session_id: str
    user_id: str
    profile: dict[str, Any] = field(default_factory=dict)
    extracted_info: ExtractedInfo = field(default_factory=ExtractedInfo)
    messages: list[dict[str, Any]] = field(default_factory=list)
