"""RAG (Retrieval Augmented Generation) module for BodySense AI."""

from .curated_source import (
    build_curated_pack,
    collect_evidence_segment_indices,
    collect_transcript_excerpt,
    load_generated_pack,
)
from .embedding import EmbeddingGenerator, get_embedding_generator
from .knowledge_base import KnowledgeBase, KnowledgeEntryData, get_knowledge_base
from .knowledge_pack import (
    GeneratedKnowledgePack,
    KnowledgeClipCandidate,
    KnowledgeUnitCandidate,
    SourceVideoMetadata,
    TranscriptSegment,
    format_timestamp_range,
    format_seconds,
    slugify,
)
from .knowledge_library import KnowledgeLibrary, SearchResult, get_knowledge_library
from .reranker import Reranker, get_reranker
from .retriever import RetrievalResult, SemanticRetriever, get_semantic_retriever
from .video_pipeline import VideoIngestionPipeline, VideoIngestionRequest

__all__ = [
    "build_curated_pack",
    "collect_evidence_segment_indices",
    "collect_transcript_excerpt",
    "load_generated_pack",
    # Embedding
    "EmbeddingGenerator",
    "get_embedding_generator",
    # Retriever
    "SemanticRetriever",
    "get_semantic_retriever",
    "RetrievalResult",
    # Reranker
    "Reranker",
    "get_reranker",
    # Knowledge Base
    "KnowledgeBase",
    "KnowledgeEntryData",
    "get_knowledge_base",
    "GeneratedKnowledgePack",
    "KnowledgeClipCandidate",
    "KnowledgeUnitCandidate",
    "SourceVideoMetadata",
    "TranscriptSegment",
    "format_timestamp_range",
    "format_seconds",
    "slugify",
    "KnowledgeLibrary",
    "SearchResult",
    "get_knowledge_library",
    "VideoIngestionPipeline",
    "VideoIngestionRequest",
]
