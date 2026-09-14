from __future__ import annotations

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[4]
CORPUS = json.loads(
    (ROOT / "contracts/internal/agent-runtime/fixtures/runtime.v1.json").read_text()
)


def test_start_turn_route_rejects_thread_path_mismatch_before_runtime(client) -> None:
    response = client.post(
        "/runtime/threads/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa/turns",
        json=CORPUS["start_turn"],
    )
    assert response.status_code == 422
    assert response.json()["detail"] == "invalid private runtime command"


def test_resume_route_rejects_interrupt_path_mismatch_before_runtime(client) -> None:
    command = CORPUS["resume_interrupt"]
    response = client.post(
        f"/runtime/threads/{command['thread_id']}/interrupts/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa/resume",
        json=command,
    )
    assert response.status_code == 422
    assert response.json()["detail"] == "invalid private runtime command"
