"""Minimal internal runtime for governed health-document extraction.

This process intentionally does not initialize Agent checkpointing, RAG, the
knowledge library, or LLM providers. Go JobRuntime remains the sole durable job
owner; this app is only a bounded HTTP execution surface for document workers.
"""

from __future__ import annotations

from fastapi import FastAPI

from .api.routes.ocr import router as ocr_router

app = FastAPI(
    title="BodySense Health Document Runtime",
    version="1.0.0",
    docs_url=None,
    redoc_url=None,
    openapi_url=None,
)


@app.get("/health")
async def health() -> dict[str, str]:
    return {"status": "ok", "service": "bodysense-health-document"}


app.include_router(ocr_router)
