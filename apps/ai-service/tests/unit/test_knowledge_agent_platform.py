"""North-Star tests for offline Knowledge Agent configuration and lineage."""

import json

import pytest
from pydantic import ValidationError

from src.ai.types import AiResponse
from src.api.routes.knowledge import IngestVideoRequestModel
from src.configuration.knowledge_agent_config import (
    get_knowledge_curator_configuration,
    get_knowledge_splitter_configuration,
)
from src.rag.ai_curator import AICurator
from src.rag.ai_splitter import LLMSplitter
from src.rag.knowledge_pack import (
    GeneratedKnowledgePack,
    KnowledgeUnitCandidate,
    SourceVideoMetadata,
    TranscriptSegment,
)
from src.rag.video_pipeline import VideoIngestionPipeline, VideoIngestionRequest

SPLITTER_ID = "knowledge-splitter-config-b14201d581dbf854"
CURATOR_ID = "knowledge-curator-config-59b2868d6fbbd12a"


class _FakeAI:
    def __init__(self, text: str) -> None:
        self.text = text
        self.requests = []

    async def generate(self, request):  # noqa: ANN001
        self.requests.append(request)
        return AiResponse(
            text=self.text,
            model="provider-model-v1",
            provider="test-provider",
        )


def _unit() -> KnowledgeUnitCandidate:
    return KnowledgeUnitCandidate(
        unit_key="forward-head-explanation-01",
        problem_slug="forward-head",
        problem_display_name="头前移",
        category="posture.forward-head",
        unit_type="explanation",
        title="头前移基础说明",
        summary="摘要",
        body_markdown="正文",
        source_start_sec=0,
        source_end_sec=5,
        evidence_segment_indices=[0],
    )


def test_knowledge_manifests_have_stable_repository_identities() -> None:
    splitter = get_knowledge_splitter_configuration(SPLITTER_ID)
    curator = get_knowledge_curator_configuration(CURATOR_ID)
    assert splitter.configuration_id == SPLITTER_ID
    assert splitter.role == "knowledge_splitter"
    assert splitter.logical_model == "bodysense-structured"
    assert curator.configuration_id == CURATOR_ID
    assert curator.role == "knowledge_curator"
    assert curator.logical_model == "bodysense-structured"


def test_ingestion_contract_requires_exact_agent_ids_when_llm_capabilities_are_enabled() -> None:
    base = {
        "source_key": "video-forward-head-test",
        "expected_content_hash": "a" * 64,
        "video_path": "sources/video.mp4",
        "problem_slug": "forward-head",
        "problem_display_name": "头前移",
        "author": "tester",
    }
    with pytest.raises(ValidationError, match="splitter_configuration_id"):
        IngestVideoRequestModel.model_validate({**base, "splitter_provider": "llm"})
    with pytest.raises(ValidationError, match="curator_configuration_id"):
        IngestVideoRequestModel.model_validate({**base, "ai_refine": True})


@pytest.mark.asyncio
async def test_llm_splitter_executes_exact_manifest_and_records_lineage() -> None:
    fake = _FakeAI(
        json.dumps(
            {
                "units": [
                    {
                        "unit_type": "explanation",
                        "title": "头前移基础解释",
                        "summary": "摘要",
                        "body_markdown": "正文",
                        "start_sec": 0,
                        "end_sec": 5,
                        "tags": ["forward-head"],
                    }
                ]
            },
            ensure_ascii=False,
        )
    )
    splitter = LLMSplitter(configuration_id=SPLITTER_ID, ai=fake)
    units = await splitter.split(
        [TranscriptSegment(segment_index=0, start_sec=0, end_sec=5, text="头前移说明")],
        "forward-head",
        "头前移",
    )
    assert len(units) == 1
    request = fake.requests[0]
    assert request.logical_model == "bodysense-structured"
    assert request.model_settings == {"temperature": 0.3, "max_tokens": 2048}
    record = splitter.execution_record
    assert record["agent_configuration"]["id"] == SPLITTER_ID
    assert record["execution_provenance"]["providers"] == ["test-provider"]
    assert record["execution_provenance"]["models"] == ["provider-model-v1"]
    assert record["execution_provenance"]["fallback"] == "none"


@pytest.mark.asyncio
async def test_curator_executes_exact_manifest_and_records_lineage() -> None:
    fake = _FakeAI(
        json.dumps(
            {
                "title": "精修后的头前移说明",
                "summary": "精修摘要",
                "body_markdown": "精修正文",
                "tags": ["forward-head", "explanation"],
                "quality_score": 0.9,
            },
            ensure_ascii=False,
        )
    )
    curator = AICurator(configuration_id=CURATOR_ID, ai=fake)
    source = SourceVideoMetadata(
        source_key="source",
        source_type="video",
        title="source",
        author="tester",
        problem_slug="forward-head",
        problem_display_name="头前移",
        original_file_path="video.mp4",
    )
    pack = GeneratedKnowledgePack(
        source=source,
        artifact_dir=".",
        transcript_segments=[],
        units=[_unit()],
        clips=[],
    )
    refined = await curator.refine_pack(pack)
    assert refined.units[0].title == "精修后的头前移说明"
    request = fake.requests[0]
    assert request.logical_model == "bodysense-structured"
    record = curator.execution_record
    assert record["agent_configuration"]["id"] == CURATOR_ID
    assert record["execution_provenance"]["call_count"] == 1
    assert record["execution_provenance"]["failed_units"] == 0


@pytest.mark.asyncio
async def test_video_pipeline_carries_splitter_lineage_into_source_metadata(
    tmp_path, monkeypatch
) -> None:
    video = tmp_path / "video.mp4"
    video.write_bytes(b"not-a-real-video")
    segments = [TranscriptSegment(segment_index=0, start_sec=0, end_sec=5, text="头前移说明")]

    class _Splitter:
        execution_record = {
            "agent_configuration": {"id": SPLITTER_ID, "role": "knowledge_splitter"},
            "execution_provenance": {"status": "executed", "logical_model": "bodysense-structured"},
        }

        async def split(self, transcript_segments, problem_slug, problem_display_name):  # noqa: ANN001
            return [_unit()]

    pipeline = VideoIngestionPipeline(data_root=tmp_path / "data")

    async def fake_transcribe(**kwargs):  # noqa: ANN003
        return segments

    monkeypatch.setattr(pipeline, "_transcribe", fake_transcribe)
    monkeypatch.setattr("src.rag.video_pipeline.get_splitter", lambda *args, **kwargs: _Splitter())
    monkeypatch.setattr("src.rag.video_pipeline.export_clips", lambda **kwargs: [])
    monkeypatch.setattr("src.rag.video_pipeline._probe_duration", lambda _path: 5.0)

    pack = await pipeline.ingest(
        VideoIngestionRequest(
            video_path=str(video),
            problem_slug="forward-head",
            problem_display_name="头前移",
            author="tester",
            source_title="source",
            splitter_provider="llm",
            splitter_configuration_id=SPLITTER_ID,
            export_clips=False,
        )
    )
    execution = pack.source.metadata["agent_execution"]
    assert execution["knowledge_splitter"]["agent_configuration"]["id"] == SPLITTER_ID
