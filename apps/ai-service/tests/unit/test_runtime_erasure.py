"""AS-002: erasing one asset or statement from consultation runtime checkpoints."""

from __future__ import annotations

import asyncio
import base64
import hashlib
import json
import os
from typing import Any, TypedDict
from unittest.mock import patch
from uuid import uuid4

import pytest
from fastapi.testclient import TestClient
from langgraph.checkpoint.memory import InMemorySaver
from langgraph.graph import END, START, StateGraph
from langgraph.types import Command, interrupt

from src.runtime import checkpoint_retention, erasure
from src.runtime.checkpoint_retention import UnsupportedCheckpointerError
from src.runtime.erasure import (
    ERASED_IMAGE_TEXT,
    ERASED_TEXT,
    ErasureFingerprint,
    erase_runtime_copies,
)


class TurnState(TypedDict, total=False):
    run_id: str
    current_user_message: str
    pending_user_images: list[dict[str, Any]]
    posture_analysis: dict[str, Any] | None
    body_state: dict[str, Any]
    runtime_messages: list[dict[str, Any]]


async def _prepare(state: TurnState) -> dict[str, Any]:
    blocks: list[dict[str, Any]] = [{"type": "text", "text": state["current_user_message"]}]
    for image in state.get("pending_user_images") or []:
        blocks.append({"type": "image_url", "image_url": {"url": image["data_url"]}})
    messages = list(state.get("runtime_messages", []))
    messages.append({"role": "user", "content": blocks, "run_id": state["run_id"]})
    return {"runtime_messages": messages, "pending_user_images": []}


async def _reply(state: TurnState) -> dict[str, Any]:
    text = state["current_user_message"]
    if text.startswith("ask:"):
        answer = interrupt({"question": text})
        text = f"{text} -> {answer}"
    messages = list(state.get("runtime_messages", []))
    posture = state.get("posture_analysis")
    if text.startswith("posture"):
        messages.append(
            {"role": "tool", "tool_call_id": "call-1", "content": json.dumps(posture or {})}
        )
    messages.append({"role": "assistant", "content": f"reply to {text}"})
    return {"runtime_messages": messages}


def _graph(saver: Any) -> Any:
    builder = StateGraph(TurnState)
    builder.add_node("prepare", _prepare)
    builder.add_node("reply", _reply)
    builder.add_edge(START, "prepare")
    builder.add_edge("prepare", "reply")
    builder.add_edge("reply", END)
    return builder.compile(checkpointer=saver)


def _config(thread_id: str) -> dict[str, Any]:
    return {"configurable": {"thread_id": thread_id}}


class Image:
    def __init__(self) -> None:
        self.asset_id = str(uuid4())
        raw = os.urandom(96)
        self.sha256 = hashlib.sha256(raw).hexdigest()
        self.payload = base64.b64encode(raw).decode()
        self.data_url = f"data:image/png;base64,{self.payload}"

    def ref(self) -> dict[str, Any]:
        return {"upload_id": self.asset_id, "mime_type": "image/png", "data_url": self.data_url}

    def posture(self) -> dict[str, Any]:
        return {
            "has_analysis": True,
            "views": [{"upload_id": self.asset_id, "view": "front", "analysis": {"tilt": 4}}],
            "summaries": ["posture-summary-sentinel"],
        }


async def _turn(graph: Any, thread_id: str, text: str, **extra: Any) -> None:
    await graph.ainvoke(
        {"run_id": str(uuid4()), "current_user_message": text, **extra}, config=_config(thread_id)
    )


async def _history(graph: Any, thread_id: str) -> list[Any]:
    return [item async for item in graph.checkpointer.alist(_config(thread_id))]


async def _erase(graph: Any, image: Image, threads: list[str], attached: list[str]) -> Any:
    return await erase_runtime_copies(
        graph,
        subject_id=image.asset_id,
        fingerprint=ErasureFingerprint(ids=(image.asset_id,), content_sha256=image.sha256),
        thread_ids=threads,
        attached_thread_ids=attached,
    )


def test_fingerprint_matches_only_the_asset_bytes():
    image, other = Image(), Image()
    fingerprint = ErasureFingerprint(ids=(image.asset_id,), content_sha256=image.sha256)
    assert fingerprint.matches_data_url(image.data_url)
    assert not fingerprint.matches_data_url(other.data_url)
    assert not fingerprint.matches_data_url("data:image/png;base64,***")
    assert not fingerprint.matches_data_url(image.payload)
    assert fingerprint.names(f"see {image.asset_id}")


def test_erasure_removes_the_image_and_derived_copies_and_all_history():
    async def run():
        saver = InMemorySaver()
        graph = _graph(saver)
        image, kept = Image(), Image()
        thread = str(uuid4())
        await _turn(graph, thread, "first", pending_user_images=[image.ref(), kept.ref()])
        await _turn(graph, thread, "posture?", posture_analysis=image.posture())
        await _turn(graph, thread, "later")
        assert len(await _history(graph, thread)) > 3

        receipt = await _erase(graph, image, [thread], [thread])

        assert receipt.status == "erased"
        [result] = receipt.threads
        assert result.status == "erased" and result.failure_code is None
        # image block + posture channel + posture tool result
        assert result.redacted_refs == 3
        assert result.deleted_checkpoints > 3
        history = await _history(graph, thread)
        assert [item.checkpoint["id"] for item in history] == [result.checkpoint_id]
        state = (await graph.aget_state(_config(thread))).values
        first = state["runtime_messages"][0]["content"]
        assert first[0] == {"type": "text", "text": "first"}
        assert first[1] == {"type": "text", "text": ERASED_IMAGE_TEXT}
        assert first[2] == {"type": "image_url", "image_url": {"url": kept.data_url}}
        assert state["runtime_messages"][3]["content"] == ERASED_TEXT
        assert state.get("posture_analysis") is None
        for needle in (image.asset_id, image.payload, "posture-summary-sentinel"):
            assert not await erasure._storage_mentions(saver, thread, needle)
        assert await erasure._storage_mentions(saver, thread, kept.payload)

        # A repeat is idempotent and the thread keeps working.
        again = await _erase(graph, image, [thread], [thread])
        assert again.status == "erased"
        assert (again.threads[0].redacted_refs, again.threads[0].deleted_checkpoints) == (0, 0)
        assert again.threads[0].checkpoint_id == result.checkpoint_id
        await _turn(graph, thread, "after")
        state = (await graph.aget_state(_config(thread))).values
        assert state["runtime_messages"][-1]["content"] == "reply to after"

    asyncio.run(run())


def test_unattached_threads_are_left_alone_unless_they_name_the_asset():
    async def run():
        saver = InMemorySaver()
        graph = _graph(saver)
        image = Image()
        untouched, posture_only = str(uuid4()), str(uuid4())
        await _turn(graph, untouched, "hello")
        await _turn(graph, untouched, "again")
        await _turn(graph, posture_only, "posture?", posture_analysis=image.posture())
        before = len(await _history(graph, untouched))
        never_used = str(uuid4())

        receipt = await _erase(graph, image, [untouched, posture_only, never_used], [])

        statuses = {item.thread_id: item.status for item in receipt.threads}
        assert statuses == {untouched: "absent", posture_only: "erased", never_used: "absent"}
        assert receipt.status == "erased"
        assert len(await _history(graph, untouched)) == before
        assert not await erasure._storage_mentions(saver, posture_only, image.asset_id)

    asyncio.run(run())


def test_a_waiting_thread_is_outstanding_and_untouched_while_others_erase():
    async def run():
        saver = InMemorySaver()
        graph = _graph(saver)
        image = Image()
        waiting, idle = str(uuid4()), str(uuid4())
        await _turn(graph, waiting, "first", pending_user_images=[image.ref()])
        await _turn(graph, waiting, "ask:pain?")
        await _turn(graph, idle, "photo", pending_user_images=[image.ref()])
        before = len(await _history(graph, waiting))

        receipt = await _erase(graph, image, [waiting, idle], [waiting, idle])

        assert receipt.status == "outstanding"
        by_thread = {item.thread_id: item for item in receipt.threads}
        assert by_thread[waiting].status == "outstanding"
        assert by_thread[waiting].failure_code == "thread_busy"
        assert by_thread[idle].status == "erased"
        assert len(await _history(graph, waiting)) == before
        assert len((await graph.aget_state(_config(waiting))).interrupts) == 1

        await graph.ainvoke(Command(resume="yes"), config=_config(waiting))
        retry = await _erase(graph, image, [waiting, idle], [waiting, idle])
        assert retry.status == "erased"
        assert not await erasure._storage_mentions(saver, waiting, image.payload)

    asyncio.run(run())


def test_a_turn_racing_the_erasure_wins_and_the_next_attempt_finishes():
    async def run():
        saver = InMemorySaver()
        graph = _graph(saver)
        image = Image()
        thread = str(uuid4())
        await _turn(graph, thread, "photo", pending_user_images=[image.ref()])
        original = checkpoint_retention._compact

        async def racing_compact(saver_arg: Any, thread_id: str, keep_id: str) -> int:
            await _turn(graph, thread, "racing")
            return await original(saver_arg, thread_id, keep_id)

        with patch.object(erasure, "_compact", racing_compact):
            receipt = await _erase(graph, image, [thread], [thread])
        assert receipt.status == "outstanding"
        assert receipt.threads[0].failure_code == "checkpoint_conflict"

        retry = await _erase(graph, image, [thread], [thread])
        assert retry.status == "erased"
        contents = (await graph.aget_state(_config(thread))).values["runtime_messages"]
        assert contents[-1]["content"] == "reply to racing"
        assert not await erasure._storage_mentions(saver, thread, image.payload)

    asyncio.run(run())


def test_erasure_route_validates_and_reports_per_thread(client: TestClient):
    image = Image()
    thread = str(uuid4())
    body = {
        "user_id": str(uuid4()),
        "content_sha256": image.sha256,
        "thread_ids": [],
        "attached_thread_ids": [thread],
    }
    response = client.post(f"/runtime/assets/{image.asset_id}/erasure", json=body)
    assert response.status_code == 422
    assert response.json()["detail"]["code"] == "thread_mismatch"

    bad = {**body, "thread_ids": [thread], "content_sha256": "not-a-hash"}
    assert client.post(f"/runtime/assets/{image.asset_id}/erasure", json=bad).status_code == 422

    graph = _graph(InMemorySaver())
    asyncio.run(_turn(graph, thread, "photo", pending_user_images=[image.ref()]))

    async def fixed_graph() -> Any:
        return graph

    body["thread_ids"] = [thread]
    with patch("src.api.routes.runtime.get_runtime_graph", fixed_graph):
        response = client.post(f"/runtime/assets/{image.asset_id}/erasure", json=body)
    assert response.status_code == 200
    payload = response.json()
    assert payload["status"] == "erased"
    assert payload["subject_id"] == image.asset_id
    assert payload["threads"][0]["thread_id"] == thread
    assert payload["threads"][0]["status"] == "erased"
    assert payload["threads"][0]["redacted_refs"] == 1

    async def unsupported(*_args: Any, **_kwargs: Any) -> Any:
        raise UnsupportedCheckpointerError("no saver")

    with patch("src.api.routes.runtime.erase_runtime_copies", unsupported):
        response = client.post(f"/runtime/assets/{image.asset_id}/erasure", json=body)
    assert response.status_code == 503
    assert response.json()["detail"]["code"] == "unsupported_checkpointer"


def _body_state(statement: str, fact: str, other_fact: str) -> dict[str, Any]:
    return {
        "schema": "agent-body-context-v1",
        "facts": [
            {"id": fact, "source_evidence_id": statement, "statement": "statement-sentinel 左膝痛"},
            {"id": other_fact, "source_evidence_id": str(uuid4()), "statement": "kept-fact"},
        ],
    }


def test_statement_erasure_drops_derived_records_by_id():
    async def run():
        saver = InMemorySaver()
        graph = _graph(saver)
        thread = str(uuid4())
        statement, fact, other = str(uuid4()), str(uuid4()), str(uuid4())
        await _turn(graph, thread, "my knee hurts", body_state=_body_state(statement, fact, other))
        await _turn(graph, thread, "later")
        fingerprint = ErasureFingerprint(ids=(statement, fact))
        assert not fingerprint.matches_data_url(Image().data_url)

        receipt = await erase_runtime_copies(
            graph,
            subject_id=statement,
            fingerprint=fingerprint,
            thread_ids=[thread],
            attached_thread_ids=[thread],
        )

        assert receipt.status == "erased" and receipt.subject_id == statement
        assert receipt.threads[0].redacted_refs == 1
        state = (await graph.aget_state(_config(thread))).values
        assert [f["id"] for f in state["body_state"]["facts"]] == [other]
        for needle in (statement, fact, "statement-sentinel"):
            assert not await erasure._storage_mentions(saver, thread, needle)
        # The user's own chat turn is a conversation record, not a copy.
        assert state["runtime_messages"][0]["content"][0]["text"] == "my knee hurts"

    asyncio.run(run())


def test_statement_erasure_route(client: TestClient):
    thread, statement, fact, other = (str(uuid4()) for _ in range(4))
    graph = _graph(InMemorySaver())
    asyncio.run(_turn(graph, thread, "hi", body_state=_body_state(statement, fact, other)))

    async def fixed_graph() -> Any:
        return graph

    body = {"user_id": str(uuid4()), "record_ids": [fact], "thread_ids": [thread]}
    with patch("src.api.routes.runtime.get_runtime_graph", fixed_graph):
        response = client.post(f"/runtime/statements/{statement}/erasure", json=body)
        assert response.status_code == 200
        payload = response.json()
        assert payload["subject_id"] == statement and payload["status"] == "erased"
        assert payload["threads"][0]["status"] == "erased"
        bad = {**body, "attached_thread_ids": [str(uuid4())]}
        response = client.post(f"/runtime/statements/{statement}/erasure", json=bad)
        assert response.status_code == 422


PG_URL = os.getenv("BODYSENSE_TEST_CHECKPOINT_PG_URL")


@pytest.mark.skipif(not PG_URL, reason="needs a throwaway Postgres")
def test_postgres_erasure_leaves_no_copy_anywhere():
    from langgraph.checkpoint.postgres.aio import AsyncPostgresSaver
    from psycopg.rows import dict_row
    from psycopg_pool import AsyncConnectionPool

    async def run():
        pool = AsyncConnectionPool(
            PG_URL,
            kwargs={"autocommit": True, "prepare_threshold": 0, "row_factory": dict_row},
            open=False,
        )
        await pool.open(wait=True, timeout=5)
        try:
            saver = AsyncPostgresSaver(pool)
            await saver.setup()
            graph = _graph(saver)
            image, kept = Image(), Image()
            thread, other = str(uuid4()), str(uuid4())
            await _turn(graph, thread, "photo", pending_user_images=[image.ref(), kept.ref()])
            await _turn(graph, thread, "posture?", posture_analysis=image.posture())
            await _turn(graph, thread, "later")
            await _turn(graph, other, "photo", pending_user_images=[image.ref()])

            receipt = await _erase(graph, image, [thread], [thread])

            assert receipt.status == "erased"
            assert receipt.threads[0].redacted_refs == 3
            assert len(await _history(graph, thread)) == 1
            mentions = erasure._storage_mentions
            for needle in (image.asset_id, image.payload, "posture-summary-sentinel"):
                assert not await mentions(saver, thread, needle)
            assert await mentions(saver, thread, kept.payload)
            # Superseded blobs are gone: one blob per kept channel version.
            async with pool.connection() as conn, conn.cursor() as cur:
                await cur.execute(
                    "SELECT count(*) AS n, count(DISTINCT channel) AS c FROM checkpoint_blobs "
                    "WHERE thread_id = %s",
                    (thread,),
                )
                row = await cur.fetchone()
                assert row["n"] == row["c"]
            # Another thread is never touched by a call that does not list it.
            assert await mentions(saver, other, image.payload)
            assert len(await _history(graph, other)) > 1

            await _turn(graph, thread, "pg-after")
            state = (await graph.aget_state(_config(thread))).values
            assert state["runtime_messages"][-1]["content"] == "reply to pg-after"
            again = await _erase(graph, image, [thread], [thread])
            assert again.status == "erased"
            assert again.threads[0].redacted_refs == 0
        finally:
            await pool.close()

    asyncio.run(run())
