"""Unit tests for the automatic video knowledge pipeline."""

from src.rag.knowledge_pack import TranscriptSegment, format_timestamp_range, slugify
from src.rag.video_pipeline import VideoIngestionPipeline


def test_slugify():
    assert slugify("凯圣王 Forward Head 01") == "凯圣王-forward-head-01"
    assert slugify("  ") == "item"


def test_format_timestamp_range():
    assert format_timestamp_range(12, 35) == "00:12-00:35"
    assert format_timestamp_range(65, None) == "01:05"
    assert format_timestamp_range(None, None) is None


def test_parse_transcript_jsonl(tmp_path):
    transcript_path = tmp_path / "transcript.raw.jsonl"
    transcript_path.write_text(
        "\n".join(
            [
                '{"start":0,"end":1500,"text":"大家好"}',
                '{"start":1500,"end":3600,"text":"今天看头前移怎么自测"}',
            ]
        ),
        encoding="utf-8",
    )
    pipeline = VideoIngestionPipeline(data_root=tmp_path)

    segments = pipeline._parse_transcript_jsonl(transcript_path)

    assert len(segments) == 2
    assert segments[1].text == "今天看头前移怎么自测"
    assert segments[1].timestamp == "00:02-00:04"


def test_build_units_groups_and_classifies(tmp_path):
    pipeline = VideoIngestionPipeline(data_root=tmp_path)
    transcript_segments = [
        TranscriptSegment(
            segment_index=0,
            start_sec=0.0,
            end_sec=4.0,
            text="今天看头前移怎么自测",
        ),
        TranscriptSegment(
            segment_index=1,
            start_sec=4.0,
            end_sec=9.0,
            text="在自然放松的状态观察耳垂和肩峰位置",
        ),
        TranscriptSegment(
            segment_index=2,
            start_sec=12.0,
            end_sec=18.0,
            text="接下来做一个下巴回收训练动作",
        ),
    ]

    units = pipeline._build_units(
        transcript_segments=transcript_segments,
        problem_slug="forward-head-posture",
        problem_display_name="头前移",
    )

    assert len(units) >= 2
    assert units[0].unit_type == "self_check"
    assert any(unit.unit_type == "exercise" for unit in units)


def test_normalize_chunks_preserves_short_ranges(tmp_path):
    pipeline = VideoIngestionPipeline(data_root=tmp_path)

    chunks = pipeline._normalize_chunks([(0.0, 0.7), (0.7, 1.8)])

    assert chunks == [(0.0, 0.7), (0.7, 1.8)]


def test_normalize_chunks_caps_long_segments(tmp_path):
    pipeline = VideoIngestionPipeline(data_root=tmp_path)

    chunks = pipeline._normalize_chunks([(0.0, 42.0)])

    assert chunks == [(0.0, 18.0), (18.0, 36.0), (36.0, 42.0)]
