"""RAG (Retrieval Augmented Generation) module for BodySense AI."""

from .ai_curator import AICurator
from .asr import ASRProvider, get_asr_provider
from .curated_source import (
    build_curated_pack,
    collect_evidence_segment_indices,
    collect_transcript_excerpt,
    load_generated_pack,
)
from .embedding import EmbeddingGenerator, get_embedding_generator
from .knowledge_library import KnowledgeLibrary, SearchResult, get_knowledge_library
from .knowledge_pack import (
    GeneratedKnowledgePack,
    KnowledgeClipCandidate,
    KnowledgeUnitCandidate,
    SourceVideoMetadata,
    TranscriptSegment,
    format_seconds,
    format_timestamp_range,
    slugify,
)
from .splitter import (
    CLIP_WORTHY_TYPES,
    HeuristicSplitter,
    Splitter,
    build_knowledge_units,
    classify_text,
    get_splitter,
)
from .video_pipeline import VideoIngestionPipeline, VideoIngestionRequest

__all__ = [
    # AI Curator
    "AICurator",
    # ASR
    "ASRProvider",
    "get_asr_provider",
    # Curated source
    "build_curated_pack",
    "collect_evidence_segment_indices",
    "collect_transcript_excerpt",
    "load_generated_pack",
    # Embedding
    "EmbeddingGenerator",
    "get_embedding_generator",
    # Knowledge Library (unified RAG system)
    "KnowledgeLibrary",
    "SearchResult",
    "get_knowledge_library",
    # Knowledge Pack
    "GeneratedKnowledgePack",
    "KnowledgeClipCandidate",
    "KnowledgeUnitCandidate",
    "SourceVideoMetadata",
    "TranscriptSegment",
    "format_timestamp_range",
    "format_seconds",
    "slugify",
    # Splitter
    "Splitter",
    "HeuristicSplitter",
    "get_splitter",
    "build_knowledge_units",
    "classify_text",
    "CLIP_WORTHY_TYPES",
    # Video Pipeline
    "VideoIngestionPipeline",
    "VideoIngestionRequest",
]
