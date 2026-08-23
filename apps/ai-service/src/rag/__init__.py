"""RAG (Retrieval Augmented Generation) module for BodySense AI."""

from .ai_curator import AICurator
from .asr import ASRProvider, get_asr_provider
from .claim_review import (
    ClaimReviewManifest,
    ReviewedKnowledgeSnapshot,
    load_claim_review,
)
from .curated_source import (
    build_curated_pack,
    collect_evidence_segment_indices,
    collect_transcript_excerpt,
    load_generated_pack,
)
from .embedding import EmbeddingGenerator, get_embedding_generator
from .external_evidence import (
    apply_external_evidence_review,
    build_claim_admissibility,
    load_external_evidence_review,
    resolve_external_reference,
)
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
from .thought_forest_snapshot import (
    ThoughtForestHealthSnapshot,
    load_thought_forest_snapshot,
)
from .thought_forest_snapshot import (
    build_generated_packs as build_thought_forest_packs,
)
from .video_pipeline import VideoIngestionPipeline, VideoIngestionRequest

__all__ = [
    # AI Curator
    "AICurator",
    # Claim review
    "ClaimReviewManifest",
    "ReviewedKnowledgeSnapshot",
    "load_claim_review",
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
    # External evidence
    "apply_external_evidence_review",
    "build_claim_admissibility",
    "load_external_evidence_review",
    "resolve_external_reference",
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
    # Thought Forest snapshot
    "ThoughtForestHealthSnapshot",
    "build_thought_forest_packs",
    "load_thought_forest_snapshot",
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
