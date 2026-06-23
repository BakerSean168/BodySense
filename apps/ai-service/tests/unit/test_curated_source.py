"""Tests for curated source helpers."""

from src.rag.curated_source import collect_evidence_segment_indices
from src.rag.knowledge_pack import TranscriptSegment


def test_collect_evidence_segment_indices():
    segments = [
        TranscriptSegment(segment_index=0, start_sec=0.0, end_sec=1.0, text="a"),
        TranscriptSegment(segment_index=1, start_sec=1.0, end_sec=2.0, text="b"),
        TranscriptSegment(segment_index=2, start_sec=2.0, end_sec=3.0, text="c"),
    ]

    indices = collect_evidence_segment_indices(segments, 0.5, 2.1)

    assert indices == [0, 1, 2]
