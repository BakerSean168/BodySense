"""Checkpoint cleanup behind the Go pruned cursor (PC-RET-001 slice 2).

Go owns chat retention. When it prunes raw messages it also asks this module to
remove the controlled copies LangGraph keeps of them: every historical
checkpoint of the consultation thread, and the ``runtime_messages`` entries of
the pruned Runs in the latest checkpoint. Go advances its pruned cursor only
after it has verified the receipt returned here.

The cleanup fails closed. It refuses a thread that is mid-step or waiting on an
interrupt (so an Ask User resume can never land on a rewritten checkpoint), a
thread whose latest checkpoint moved while cleaning, and a prune set that would
drop a kept Run's entries. Every refusal leaves the thread untouched or in a
state the next attempt can finish.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import UTC, datetime
from typing import Any, cast

from langchain_core.runnables import RunnableConfig
from langgraph.checkpoint.base import CheckpointMetadata, copy_checkpoint
from langgraph.checkpoint.base.id import uuid6
from langgraph.checkpoint.memory import InMemorySaver
from langgraph.checkpoint.postgres.aio import AsyncPostgresSaver

RUNTIME_MESSAGES = "runtime_messages"
ROOT_NAMESPACE = ""


class CheckpointCleanupError(RuntimeError):
    """A cleanup the caller must retry later; nothing was claimed as erased."""

    code = "checkpoint_cleanup_failed"


class ThreadBusyError(CheckpointCleanupError):
    code = "thread_busy"


class CheckpointConflictError(CheckpointCleanupError):
    code = "checkpoint_conflict"


class RunOrderAnomalyError(CheckpointCleanupError):
    code = "run_order_anomaly"


class UnsupportedCheckpointerError(CheckpointCleanupError):
    code = "unsupported_checkpointer"


@dataclass(frozen=True)
class CheckpointCleanupReceipt:
    thread_id: str
    pruned_run_ids: list[str]
    checkpoint_id: str | None
    removed_messages: int
    retained_messages: int
    deleted_checkpoints: int
    status: str = "cleaned"
    remaining_run_ids: list[str] = field(default_factory=list)


def tag_runtime_message(message: dict[str, Any], run_id: str | None) -> dict[str, Any]:
    """Mark the user entry that opens a Run's segment of ``runtime_messages``."""
    if run_id:
        return {**message, "run_id": run_id}
    return message


def _segment_runs(messages: list[dict[str, Any]]) -> list[str | None]:
    """Attribute every entry to the Run whose tagged user entry opened it.

    A Run (including its Ask User resume) appends one user entry tagged with its
    id and then assistant/tool entries, so an entry belongs to the closest
    tagged entry before it. Entries before any tagged entry have no owner.
    """
    owners: list[str | None] = []
    current: str | None = None
    for message in messages:
        tagged = message.get("run_id")
        if isinstance(tagged, str) and tagged:
            current = tagged
        owners.append(current)
    return owners


def trim_runtime_messages(
    messages: list[dict[str, Any]], pruned_run_ids: set[str]
) -> list[dict[str, Any]]:
    """Drop the prefix of ``messages`` that ends with the last pruned Run.

    Pruning is a prefix in Go (every message at or before the boundary), so the
    entries to drop are a prefix here too. An entry of a kept Run inside that
    prefix means the two sides disagree about order: refuse instead of guessing.
    """
    owners = _segment_runs(messages)
    cut = 0
    for index, owner in enumerate(owners):
        if owner is not None and owner in pruned_run_ids:
            cut = index + 1
    for owner in owners[:cut]:
        if owner is not None and owner not in pruned_run_ids:
            raise RunOrderAnomalyError(f"kept Run {owner} precedes a pruned Run in the thread")
    remaining = messages[cut:]
    if any(owner in pruned_run_ids for owner in owners[cut:] if owner is not None):
        raise RunOrderAnomalyError("pruned Run entries remain after trimming")
    return remaining


def _root_config(thread_id: str) -> RunnableConfig:
    return cast(
        RunnableConfig,
        {"configurable": {"thread_id": thread_id, "checkpoint_ns": ROOT_NAMESPACE}},
    )


async def _compact_postgres(saver: AsyncPostgresSaver, thread_id: str, keep_id: str) -> int:
    """Delete the checkpoints older than ``keep_id`` and their blobs in one transaction.

    Only older rows are touched: a checkpoint a concurrent turn writes after the
    check below is newer, survives, and makes the final verification fail.
    """
    from langgraph.checkpoint.postgres import _ainternal

    async with _ainternal.get_connection(saver.conn) as conn, conn.transaction():
        async with conn.cursor() as cur:
            await cur.execute(
                "SELECT pg_advisory_xact_lock(hashtextextended(%s, 0))",
                (f"bodysense.checkpoint.{thread_id}",),
            )
            await cur.execute(
                "SELECT max(checkpoint_id) AS latest FROM checkpoints "
                "WHERE thread_id = %s AND checkpoint_ns = %s",
                (thread_id, ROOT_NAMESPACE),
            )
            row = await cur.fetchone()
            latest = (row["latest"] if isinstance(row, dict) else row[0]) if row else None
            if latest != keep_id:
                raise CheckpointConflictError("thread advanced while cleaning")
            await cur.execute(
                """
WITH gone AS (
  DELETE FROM checkpoints
  WHERE thread_id = %(t)s AND checkpoint_ns = %(ns)s AND checkpoint_id < %(keep)s
  RETURNING checkpoint
), gone_versions AS (
  SELECT DISTINCT v.key AS channel, v.value AS version
  FROM gone, jsonb_each_text(gone.checkpoint -> 'channel_versions') v
), kept_versions AS (
  SELECT v.key AS channel, v.value AS version
  FROM checkpoints c, jsonb_each_text(c.checkpoint -> 'channel_versions') v
  WHERE c.thread_id = %(t)s AND c.checkpoint_ns = %(ns)s AND c.checkpoint_id >= %(keep)s
), dropped_blobs AS (
  DELETE FROM checkpoint_blobs b
  USING gone_versions g
  WHERE b.thread_id = %(t)s AND b.checkpoint_ns = %(ns)s
    AND b.channel = g.channel AND b.version = g.version
    AND NOT EXISTS (SELECT 1 FROM kept_versions k
                    WHERE k.channel = g.channel AND k.version = g.version)
)
SELECT count(*) AS deleted FROM gone""",
                {"t": thread_id, "ns": ROOT_NAMESPACE, "keep": keep_id},
            )
            row = await cur.fetchone()
            deleted = int((row["deleted"] if isinstance(row, dict) else row[0]) if row else 0)
            await cur.execute(
                "DELETE FROM checkpoint_writes "
                "WHERE thread_id = %s AND checkpoint_ns = %s AND checkpoint_id < %s",
                (thread_id, ROOT_NAMESPACE, keep_id),
            )
            await cur.execute(
                "UPDATE checkpoints SET parent_checkpoint_id = NULL "
                "WHERE thread_id = %s AND checkpoint_ns = %s AND checkpoint_id = %s",
                (thread_id, ROOT_NAMESPACE, keep_id),
            )
    return deleted


def _compact_memory(saver: InMemorySaver, thread_id: str, keep_id: str) -> int:
    root = saver.storage.get(thread_id, {}).get(ROOT_NAMESPACE, {})
    if keep_id not in root or max(root) != keep_id:
        raise CheckpointConflictError("thread advanced while cleaning")
    older = [checkpoint_id for checkpoint_id in root if checkpoint_id < keep_id]
    gone: set[tuple[str, str]] = set()
    for checkpoint_id in older:
        saved, _metadata, _parent = root.pop(checkpoint_id)
        gone.update(saver.serde.loads_typed(saved)["channel_versions"].items())
    kept: set[tuple[str, str]] = set()
    for saved, _metadata, _parent in root.values():
        kept.update(saver.serde.loads_typed(saved)["channel_versions"].items())
    saved, metadata, _parent = root[keep_id]
    root[keep_id] = (saved, metadata, None)
    for key in list(saver.writes):
        if key[0] == thread_id and key[1] == ROOT_NAMESPACE and key[2] < keep_id:
            del saver.writes[key]
    for channel, version in gone - kept:
        saver.blobs.pop((thread_id, ROOT_NAMESPACE, channel, version), None)
    return len(older)


async def _compact(saver: Any, thread_id: str, keep_id: str) -> int:
    if isinstance(saver, AsyncPostgresSaver):
        return await _compact_postgres(saver, thread_id, keep_id)
    if isinstance(saver, InMemorySaver):
        return _compact_memory(saver, thread_id, keep_id)
    raise UnsupportedCheckpointerError(f"cannot compact {type(saver).__name__}")


async def cleanup_thread_checkpoints(
    graph: Any, *, thread_id: str, pruned_run_ids: list[str]
) -> CheckpointCleanupReceipt:
    """Remove pruned Runs from the thread's checkpoints and drop its history.

    Idempotent: a repeat with the same prune set finds nothing left to trim and
    reports the single checkpoint that remains.
    """
    pruned = set(pruned_run_ids)
    saver = graph.checkpointer
    config = _root_config(thread_id)

    snapshot = await graph.aget_state(config)
    if snapshot.next or snapshot.interrupts:
        raise ThreadBusyError("thread is running or waiting on an interrupt")

    current = await saver.aget_tuple(config)
    if current is None:
        return CheckpointCleanupReceipt(
            thread_id=thread_id,
            pruned_run_ids=sorted(pruned),
            checkpoint_id=None,
            removed_messages=0,
            retained_messages=0,
            deleted_checkpoints=0,
        )
    if current.pending_writes:
        raise ThreadBusyError("thread has pending writes")

    messages = list(current.checkpoint["channel_values"].get(RUNTIME_MESSAGES) or [])
    remaining = trim_runtime_messages(messages, pruned)
    keep_id = current.checkpoint["id"]
    if len(remaining) != len(messages):
        rewritten = copy_checkpoint(current.checkpoint)
        version = saver.get_next_version(rewritten["channel_versions"].get(RUNTIME_MESSAGES), None)
        step = int((current.metadata or {}).get("step", -1)) + 1
        rewritten["id"] = str(uuid6(clock_seq=step))
        rewritten["ts"] = datetime.now(UTC).isoformat()
        rewritten["channel_values"][RUNTIME_MESSAGES] = remaining
        rewritten["channel_versions"][RUNTIME_MESSAGES] = version
        rewritten["updated_channels"] = [RUNTIME_MESSAGES]
        metadata = cast(CheckpointMetadata, {"source": "update", "step": step, "parents": {}})
        await saver.aput(current.config, rewritten, metadata, {RUNTIME_MESSAGES: version})
        keep_id = rewritten["id"]

    deleted = await _compact(saver, thread_id, keep_id)

    # Verify from storage, not from what we meant to write.
    final = await saver.aget_tuple(config)
    if final is None or final.checkpoint["id"] != keep_id:
        raise CheckpointConflictError("kept checkpoint is no longer the latest")
    final_messages = list(final.checkpoint["channel_values"].get(RUNTIME_MESSAGES) or [])
    owners = {owner for owner in _segment_runs(final_messages) if owner is not None}
    if owners & pruned:
        raise CheckpointCleanupError("pruned Run entries survived cleanup")
    history = [item async for item in saver.alist(config, limit=2)]
    if len(history) != 1:
        raise CheckpointConflictError("thread history was not compacted")

    return CheckpointCleanupReceipt(
        thread_id=thread_id,
        pruned_run_ids=sorted(pruned),
        checkpoint_id=keep_id,
        removed_messages=len(messages) - len(final_messages),
        retained_messages=len(final_messages),
        deleted_checkpoints=deleted,
        remaining_run_ids=sorted(owners),
    )
