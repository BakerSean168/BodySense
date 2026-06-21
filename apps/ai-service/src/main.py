"""BodySense AI Service - FastAPI application."""

from fastapi import FastAPI

from .api.routes import knowledge

app = FastAPI(
    title="BodySense AI Service",
    version="0.1.0",
)

# Include routers
app.include_router(knowledge.router)


@app.get("/health")
async def health():
    return {"status": "ok", "service": "bodysense-ai"}
