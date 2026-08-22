"""BodySense AI Service - FastAPI application."""

import os
from contextlib import asynccontextmanager
from pathlib import Path

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

try:
    from dotenv import load_dotenv
except ModuleNotFoundError:  # pragma: no cover - optional local convenience dependency
    load_dotenv = None

# Load environment variables from .env file
_env_paths = [
    Path(__file__).parent.parent / ".env",  # apps/ai-service/.env
    Path(__file__).parent.parent.parent / ".env",  # project root .env
]

for _env_path in _env_paths:
    if load_dotenv and _env_path.exists():
        load_dotenv(_env_path, override=True)
        break

from .api.routes import (  # noqa: E402
    assessment,
    diagnosis,
    knowledge,
    ocr,
    posture,
    runtime,
    title,
    treatment,
)
from .rag.knowledge_library import knowledge_library_lifespan  # noqa: E402
from .runtime.checkpointing import runtime_checkpointer_lifespan  # noqa: E402


@asynccontextmanager
async def app_lifespan(_: FastAPI):
    async with runtime_checkpointer_lifespan(), knowledge_library_lifespan():
        yield


app = FastAPI(
    title="BodySense AI Service",
    version="0.1.0",
    lifespan=app_lifespan,
)

# CORS middleware
_cors_origins = os.getenv("CORS_ORIGINS", "http://localhost:5173").split(",")
app.add_middleware(
    CORSMiddleware,
    allow_origins=[o.strip() for o in _cors_origins],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Include routers
app.include_router(knowledge.router)
app.include_router(ocr.router)
app.include_router(posture.router)
app.include_router(runtime.router)
app.include_router(assessment.router)
app.include_router(diagnosis.router)
app.include_router(treatment.router)
app.include_router(title.router)


@app.get("/health")
async def health():
    return {"status": "ok", "service": "bodysense-ai"}
