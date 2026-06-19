"""BodySense AI Service - FastAPI application."""

from fastapi import FastAPI

app = FastAPI(
    title="BodySense AI Service",
    version="0.1.0",
)


@app.get("/health")
async def health():
    return {"status": "ok", "service": "bodysense-ai"}
