"""Knowledge base API routes."""

from typing import Optional

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from ...rag import KnowledgeEntryData, get_knowledge_base

router = APIRouter(prefix="/api/knowledge", tags=["knowledge"])


# Request/Response Models


class AddEntryRequest(BaseModel):
    """Request to add a knowledge entry."""

    category: str = Field(..., min_length=1, max_length=100, description="Category of the entry")
    title: str = Field(..., min_length=1, max_length=500, description="Title of the entry")
    content: str = Field(..., min_length=1, description="Content of the entry")
    source_video: Optional[str] = Field(None, max_length=500, description="Source video URL")
    source_timestamp: Optional[str] = Field(None, max_length=50, description="Timestamp in video")


class AddEntryResponse(BaseModel):
    """Response after adding a knowledge entry."""

    id: int
    message: str = "Entry added successfully"


class SearchRequest(BaseModel):
    """Request to search knowledge base."""

    query: str = Field(..., min_length=1, description="Search query")
    top_k: int = Field(10, ge=1, le=100, description="Number of candidates to retrieve")
    top_n: int = Field(3, ge=1, le=20, description="Number of final results")
    category: Optional[str] = Field(None, description="Filter by category")


class SearchResultItem(BaseModel):
    """A single search result."""

    id: int
    category: str
    title: str
    content: str
    similarity: float
    source_video: Optional[str] = None
    source_timestamp: Optional[str] = None


class SearchResponse(BaseModel):
    """Response from search."""

    results: list[SearchResultItem]
    total: int


class EntryResponse(BaseModel):
    """Response for a single entry."""

    id: int
    category: str
    title: str
    content: str
    source_video: Optional[str] = None
    source_timestamp: Optional[str] = None


class DeleteResponse(BaseModel):
    """Response after deleting an entry."""

    message: str = "Entry deleted successfully"


class StatsResponse(BaseModel):
    """Knowledge base statistics."""

    total_entries: int


# Routes


@router.post("/entries", response_model=AddEntryResponse)
async def add_entry(request: AddEntryRequest):
    """Add a new knowledge entry with auto-generated embedding."""
    try:
        kb = get_knowledge_base()
        entry_data = KnowledgeEntryData(
            category=request.category,
            title=request.title,
            content=request.content,
            source_video=request.source_video,
            source_timestamp=request.source_timestamp,
        )
        entry_id = await kb.add_entry(entry_data)
        return AddEntryResponse(id=entry_id)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@router.post("/search", response_model=SearchResponse)
async def search_knowledge(request: SearchRequest):
    """Search knowledge base using semantic search."""
    try:
        kb = get_knowledge_base()
        results = await kb.search(
            query=request.query,
            top_k=request.top_k,
            top_n=request.top_n,
            category=request.category,
        )

        items = [
            SearchResultItem(
                id=r.id,
                category=r.category,
                title=r.title,
                content=r.content,
                similarity=r.similarity,
                source_video=r.source_video,
                source_timestamp=r.source_timestamp,
            )
            for r in results
        ]

        return SearchResponse(results=items, total=len(items))
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@router.get("/entries/{entry_id}", response_model=EntryResponse)
async def get_entry(entry_id: int):
    """Get a knowledge entry by ID."""
    try:
        kb = get_knowledge_base()
        entry = await kb.get_entry(entry_id)

        if entry is None:
            raise HTTPException(status_code=404, detail="Entry not found")

        return EntryResponse(
            id=entry.id,
            category=entry.category,
            title=entry.title,
            content=entry.content,
            source_video=entry.source_video,
            source_timestamp=entry.source_timestamp,
        )
    except HTTPException:
        raise
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@router.delete("/entries/{entry_id}", response_model=DeleteResponse)
async def delete_entry(entry_id: int):
    """Delete a knowledge entry."""
    try:
        kb = get_knowledge_base()
        deleted = await kb.delete_entry(entry_id)

        if not deleted:
            raise HTTPException(status_code=404, detail="Entry not found")

        return DeleteResponse()
    except HTTPException:
        raise
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@router.get("/stats", response_model=StatsResponse)
async def get_stats():
    """Get knowledge base statistics."""
    try:
        kb = get_knowledge_base()
        total = await kb.count()
        return StatsResponse(total_entries=total)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
