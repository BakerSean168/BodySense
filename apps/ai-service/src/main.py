"""BodySense AI Service - FastAPI application."""

from pathlib import Path

from dotenv import load_dotenv
from fastapi import FastAPI

# Load environment variables from .env file
_env_paths = [
    Path(__file__).parent.parent / ".env",  # apps/ai-service/.env
    Path(__file__).parent.parent.parent / ".env",  # project root .env
]

for _env_path in _env_paths:
    if _env_path.exists():
        load_dotenv(_env_path, override=True)
        break

from .api.routes import assessment, chat, diagnosis, knowledge, ocr, reassessment  # noqa: E402

app = FastAPI(
    title="BodySense AI Service",
    version="0.1.0",
)

# Include routers
app.include_router(knowledge.router)
app.include_router(ocr.router)
app.include_router(chat.router)
app.include_router(assessment.router)
app.include_router(diagnosis.router)
app.include_router(reassessment.router)


@app.get("/health")
async def health():
    return {"status": "ok", "service": "bodysense-ai"}
