"""PC-RET-001 slice 2: checkpoint cleanup behind the Go pruned cursor."""

from __future__ import annotations

import asyncio
import os
from typing import Any, TypedDict
from unittest.mock import patch
from uuid import uuid4

import pytest
from fastapi.testclient import TestClient
from langgraph.checkpoint.memory import InMemorySaver
from langgraph.graph import END, START, StateGraph
from langgraph.types import Command, interrupt

from src.runtime import checkpoint_retention
from src.runtime.checkpoint_retention import (
    CheckpointConflictError,
    RunOrderAnomalyError,
    ThreadBusyError,
    cleanup_thread_checkpoints,
    tag_runtime_message,
    trim_runtime_messages,
)


class TurnState(TypedDict, total=False):
    run_id: str
    current_user_message: str
    runtime_messages: list[dict[str, Any]]


async def _prepare(state: TurnState) -> dict[str, Any]:
    messages = list(state.get("runtime_messages", []))
    messages.append(
        tag_runtime_message(
            {"role": "user", "content": state["current_user_message"]}, state["run_id"]
        )
    )
    return {"runtime_messages": messages}


async def _reply(state: TurnState) -> dict[str, Any]:
    text = state["current_user_message"]
    if text.startswith("ask:"):
        answer = interrupt({"question": text})
        text = f"{text} -> {answer}"
    messages = list(state.get("runtime_messages", []))
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


async def _turn(graph: Any, thread_id: str, run_id: str, text: str) -> None:
    await graph.ainvoke({"run_id": run_id, "current_user_message": text}, config=_config(thread_id))


async def _history(graph: Any, thread_id: str) -> list[Any]:
    return [item async for item in graph.checkpointer.alist(_config(thread_id))]


def _memory_contains(saver: InMemorySaver, thread_id: str, needle: str) -> bool:
    raw = needle.encode()
    for (thread, *_rest), (_kind, blob) in saver.blobs.items():
        if thread == thread_id and raw in (blob or b""):
            return True
    for checkpoints in saver.storage.get(thread_id, {}).values():
        for (_kind, saved), (_mkind, metadata), _parent in checkpoints.values():
            if raw in saved or raw in metadata:
                return True
    return False


def test_trim_drops_the_pruned_prefix_and_keeps_later_runs():
    messages = [
        {"role": "user", "content": "legacy"},
        {"role": "user", "content": "a", "run_id": "A"},
        {"role": "assistant", "content": "ra"},
        {"role": "user", "content": "b", "run_id": "B"},
        {"role": "tool", "content": "tb"},
        {"role": "user", "content": "c", "run_id": "C"},
    ]
    assert trim_runtime_messages(messages, {"A", "B"}) == [messages[5]]
    assert trim_runtime_messages(messages, set()) == messages
    # A Run the thread never saw (it failed before reaching AI) trims nothing.
    assert trim_runtime_messages(messages, {"Z"}) == messages


def test_trim_refuses_a_kept_run_inside_the_pruned_prefix():
    messages = [
        {"role": "user", "content": "a", "run_id": "A"},
        {"role": "user", "content": "b", "run_id": "B"},
    ]
    with pytest.raises(RunOrderAnomalyError):
        trim_runtime_messages(messages, {"B"})


def test_cleanup_removes_pruned_runs_and_all_history():
    async def run():
        saver = InMemorySaver()
        graph = _graph(saver)
        thread = str(uuid4())
        runs = [str(uuid4()) for _ in range(3)]
        for index, run_id in enumerate(runs):
            await _turn(graph, thread, run_id, f"sentinel-{index}")
        assert len(await _history(graph, thread)) > 3

        receipt = await cleanup_thread_checkpoints(graph, thread_id=thread, pruned_run_ids=runs[:2])

        assert receipt.status == "cleaned"
        assert receipt.pruned_run_ids == sorted(runs[:2])
        assert receipt.removed_messages == 4
        assert receipt.retained_messages == 2
        assert receipt.remaining_run_ids == [runs[2]]
        history = await _history(graph, thread)
        assert [item.checkpoint["id"] for item in history] == [receipt.checkpoint_id]
        assert history[0].parent_config is None
        state = await graph.aget_state(_config(thread))
        assert [m["content"] for m in state.values["runtime_messages"]] == [
            "sentinel-2",
            "reply to sentinel-2",
        ]
        assert not _memory_contains(saver, thread, "sentinel-0")
        assert not _memory_contains(saver, thread, "sentinel-1")
        assert _memory_contains(saver, thread, "sentinel-2")

        # Exact retry: nothing left to trim, one checkpoint reported again.
        again = await cleanup_thread_checkpoints(graph, thread_id=thread, pruned_run_ids=runs[:2])
        assert (again.removed_messages, again.deleted_checkpoints) == (0, 0)
        assert again.checkpoint_id == receipt.checkpoint_id

        # The thread keeps working from the compacted checkpoint.
        await _turn(graph, thread, str(uuid4()), "after")
        state = await graph.aget_state(_config(thread))
        assert [m["content"] for m in state.values["runtime_messages"]] == [
            "sentinel-2",
            "reply to sentinel-2",
            "after",
            "reply to after",
        ]

    asyncio.run(run())


def test_cleanup_of_an_unknown_thread_is_an_empty_receipt():
    async def run():
        graph = _graph(InMemorySaver())
        receipt = await cleanup_thread_checkpoints(
            graph, thread_id=str(uuid4()), pruned_run_ids=[str(uuid4())]
        )
        assert receipt.checkpoint_id is None
        assert receipt.deleted_checkpoints == 0

    asyncio.run(run())


def test_waiting_interrupt_refuses_cleanup_and_resume_has_no_ghost():
    async def run():
        saver = InMemorySaver()
        graph = _graph(saver)
        thread = str(uuid4())
        first, asking = str(uuid4()), str(uuid4())
        await _turn(graph, thread, first, "old-sentinel")
        await _turn(graph, thread, asking, "ask:pain?")
        waiting = await graph.aget_state(_config(thread))
        assert len(waiting.interrupts) == 1
        before = len(await _history(graph, thread))

        with pytest.raises(ThreadBusyError):
            await cleanup_thread_checkpoints(graph, thread_id=thread, pruned_run_ids=[first])
        # Refused before touching anything: the pending interrupt survives.
        assert len(await _history(graph, thread)) == before
        assert (await graph.aget_state(_config(thread))).interrupts == waiting.interrupts

        await graph.ainvoke(Command(resume="yes"), config=_config(thread))
        await cleanup_thread_checkpoints(graph, thread_id=thread, pruned_run_ids=[first])

        state = await graph.aget_state(_config(thread))
        assert state.interrupts == () and state.next == ()
        assert [m["content"] for m in state.values["runtime_messages"]] == [
            "ask:pain?",
            "reply to ask:pain? -> yes",
        ]
        assert not _memory_contains(saver, thread, "old-sentinel")

    asyncio.run(run())


def test_a_turn_racing_the_cleanup_wins_and_cleanup_reports_conflict():
    async def run():
        saver = InMemorySaver()
        graph = _graph(saver)
        thread = str(uuid4())
        pruned = str(uuid4())
        await _turn(graph, thread, pruned, "old")
        await _turn(graph, thread, str(uuid4()), "kept")
        original_compact = checkpoint_retention._compact

        async def racing_compact(saver_arg: Any, thread_id: str, keep_id: str) -> int:
            await _turn(graph, thread, str(uuid4()), "racing")
            return await original_compact(saver_arg, thread_id, keep_id)

        with patch.object(checkpoint_retention, "_compact", racing_compact):
            with pytest.raises(CheckpointConflictError):
                await cleanup_thread_checkpoints(graph, thread_id=thread, pruned_run_ids=[pruned])

        state = await graph.aget_state(_config(thread))
        contents = [m["content"] for m in state.values["runtime_messages"]]
        assert contents[-2:] == ["racing", "reply to racing"]
        assert "kept" in contents

        # The next attempt finishes the job without losing the racing turn.
        await cleanup_thread_checkpoints(graph, thread_id=thread, pruned_run_ids=[pruned])
        state = await graph.aget_state(_config(thread))
        assert [m["content"] for m in state.values["runtime_messages"]] == [
            "kept",
            "reply to kept",
            "racing",
            "reply to racing",
        ]
        assert len(await _history(graph, thread)) == 1

    asyncio.run(run())


def test_cleanup_route_validates_identity_and_maps_refusals(client: TestClient):
    thread = str(uuid4())
    body = {"conversation_id": str(uuid4()), "user_id": str(uuid4()), "pruned_run_ids": []}
    response = client.post(f"/runtime/threads/{thread}/checkpoints/cleanup", json=body)
    assert response.status_code == 422
    assert response.json()["detail"]["code"] == "thread_mismatch"

    async def busy(*_args: Any, **_kwargs: Any) -> Any:
        raise ThreadBusyError("waiting")

    body["conversation_id"] = thread
    with patch("src.api.routes.runtime.cleanup_thread_checkpoints", busy):
        response = client.post(f"/runtime/threads/{thread}/checkpoints/cleanup", json=body)
    assert response.status_code == 409
    assert response.json()["detail"]["code"] == "thread_busy"

    graph = _graph(InMemorySaver())

    async def fixed_graph() -> Any:
        return graph

    with patch("src.api.routes.runtime.get_runtime_graph", fixed_graph):
        response = client.post(f"/runtime/threads/{thread}/checkpoints/cleanup", json=body)
    assert response.status_code == 200
    assert response.json()["status"] == "cleaned"
    assert response.json()["thread_id"] == thread


PG_URL = os.getenv("BODYSENSE_TEST_CHECKPOINT_PG_URL")


@pytest.mark.skipif(not PG_URL, reason="needs a throwaway Postgres")
def test_postgres_cleanup_leaves_no_pruned_copy_anywhere():
    from langgraph.checkpoint.postgres.aio import AsyncPostgresSaver
    from psycopg.rows import dict_row
    from psycopg_pool import AsyncConnectionPool

    async def contains(pool: Any, thread: str, needle: str) -> bool:
        async with pool.connection() as conn, conn.cursor() as cur:
            await cur.execute(
                """
SELECT (SELECT count(*) FROM checkpoint_blobs
         WHERE thread_id = %(t)s AND position(convert_to(%(n)s, 'UTF8') in blob) > 0)
     + (SELECT count(*) FROM checkpoint_writes
         WHERE thread_id = %(t)s AND position(convert_to(%(n)s, 'UTF8') in blob) > 0)
     + (SELECT count(*) FROM checkpoints
         WHERE thread_id = %(t)s AND strpos(checkpoint::text || metadata::text, %(n)s) > 0)
     AS hits""",
                {"t": thread, "n": needle},
            )
            row = await cur.fetchone()
            return bool(row["hits"])

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
            thread = str(uuid4())
            runs = [str(uuid4()) for _ in range(3)]
            for index, run_id in enumerate(runs):
                await _turn(graph, thread, run_id, f"pg-sentinel-{index}")
            other = str(uuid4())
            await _turn(graph, other, str(uuid4()), "pg-sentinel-0")

            receipt = await cleanup_thread_checkpoints(
                graph, thread_id=thread, pruned_run_ids=runs[:2]
            )

            assert receipt.removed_messages == 4
            assert len(await _history(graph, thread)) == 1
            assert not await contains(pool, thread, "pg-sentinel-0")
            assert not await contains(pool, thread, "pg-sentinel-1")
            assert await contains(pool, thread, "pg-sentinel-2")
            # Another thread is never touched.
            assert await contains(pool, other, "pg-sentinel-0")
            assert len(await _history(graph, other)) > 1

            await _turn(graph, thread, str(uuid4()), "pg-after")
            state = await graph.aget_state(_config(thread))
            assert [m["content"] for m in state.values["runtime_messages"]][-2:] == [
                "pg-after",
                "reply to pg-after",
            ]
            again = await cleanup_thread_checkpoints(
                graph, thread_id=thread, pruned_run_ids=runs[:2]
            )
            assert again.removed_messages == 0
            assert len(await _history(graph, thread)) == 1
        finally:
            await pool.close()

    asyncio.run(run())
