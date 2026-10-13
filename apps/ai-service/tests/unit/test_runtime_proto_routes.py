from __future__ import annotations

import copy
import json
from pathlib import Path

import pytest

from src.runtime.consultation_thread import get_consultation_manifest

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


@pytest.mark.parametrize(
    "configuration_id", ["consult-config-7feb8ca2d5bfad5a", "consult-config-83b6a1cc70477c82"]
)
def test_start_turn_route_streams_proto_oneof_with_complete_versioned_provenance(
    client, monkeypatch, configuration_id
) -> None:
    monkeypatch.setenv("ENVIRONMENT", "test")
    monkeypatch.setenv("BODYSENSE_E2E_STUB_AI", "1")

    command = copy.deepcopy(CORPUS["start_turn"])
    manifest = get_consultation_manifest(configuration_id)
    command["configuration_id"] = manifest.configuration_id

    response = client.post(
        f"/runtime/threads/{command['thread_id']}/turns",
        json=command,
    )
    assert response.status_code == 200
    assert response.headers["content-type"].startswith("application/x-ndjson")

    records = [json.loads(line) for line in response.text.splitlines() if line.strip()]
    assert len(records) >= 3
    first = records[0]
    assert first["agent_configuration"]["agent_configuration"]["id"] == manifest.configuration_id
    intake = first["agent_configuration"]["agent_configuration"]["intake"]
    assert manifest.intake is not None
    assert intake["output_schema_revision"] == manifest.intake.output_schema_revision
    assert intake["policy_revision"] == manifest.intake.policy_revision
    assert intake["generation"] == manifest.intake.generation.model_dump(mode="json")
    assert "channel" not in first
    assert "type" not in first
    assert "payload" not in first
    assert "stream_done" in records[-1]
