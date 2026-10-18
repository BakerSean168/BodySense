"""Erasure of one asset's copies from consultation runtime state (AS-002 slice 2).

Go owns asset erasure. Its ``runtime_checkpoints`` target asks this module to
remove every copy of one asset that LangGraph keeps for the user's
consultation threads, and it marks the target verified only on the receipt
returned here. A thread can hold an asset in three ways:

* the inline image of a turn that attached it (a ``data:image/...`` URL in
  ``runtime_messages`` or in ``pending_user_images``), recognised by the
  SHA-256 of its decoded bytes, which is the asset's ``content_sha256``;
* derived data that names it (the posture summary Go prefetches into
  ``posture_analysis``, a ``get_posture_analysis`` tool result, an evidence
  ref), recognised by the asset id;
* any older checkpoint, pending write or blob that still carries either.

For each thread that holds a copy (or that Go says the asset was attached
to), the latest checkpoint is rewritten without the copies, every older
checkpoint, write and superseded blob is deleted, and the result is verified
from storage: the latest checkpoint carries no copy, it is the only one, and
no stored byte of the thread names the asset. Assistant replies the model
wrote about an image are conversation records (chat retention), not copies.

Like checkpoint retention, erasure fails closed: a thread that is mid-step or
waiting on an Ask User interrupt is left untouched and reported outstanding,
and a thread whose latest checkpoint moved while erasing is reported as a
conflict for the next attempt to finish.
"""

from __future__ import annotations

import base64
import binascii
import hashlib
from dataclasses import dataclass, field
from datetime import UTC, datetime
from typing import Any, cast

from langgraph.checkpoint.base import CheckpointMetadata, copy_checkpoint
from langgraph.checkpoint.base.id import uuid6
from langgraph.checkpoint.memory import InMemorySaver
from langgraph.checkpoint.postgres.aio import AsyncPostgresSaver

from .checkpoint_retention import (
    ROOT_NAMESPACE,
    CheckpointCleanupError,
    CheckpointConflictError,
    ThreadBusyError,
    UnsupportedCheckpointerError,
    _compact,
    _root_config,
)

ERASED_IMAGE_TEXT = "（用户上传的这张照片已按用户要求删除）"
ERASED_TEXT = "（此处引用的资料已按用户要求删除）"
# Go rebuilds this summary from the posture runs on every turn. Its findings
# and summaries are flattened without attribution, so a summary that names
# the asset anywhere is dropped whole rather than partially redacted.
POSTURE_CHANNEL = "posture_analysis"

THREAD_ABSENT = "absent"
THREAD_ERASED = "erased"
THREAD_OUTSTANDING = "outstanding"


@dataclass
class AssetFingerprint:
    """Recognises one asset: its id anywhere, its bytes inside a data URL."""

    asset_id: str
    content_sha256: str
    _decoded: dict[str, bool] = field(default_factory=dict, repr=False)

    def matches_data_url(self, value: str) -> bool:
        if not value.startswith("data:"):
            return False
        known = self._decoded.get(value)
        if known is not None:
            return known
        header, sep, payload = value.partition(",")
        match = False
        if sep and header.endswith(";base64"):
            try:
                raw = base64.b64decode(payload, validate=True)
            except (binascii.Error, ValueError):
                raw = None
            match = raw is not None and hashlib.sha256(raw).hexdigest() == self.content_sha256
        self._decoded[value] = match
        return match

    def names(self, value: str) -> bool:
        return self.asset_id in value or self.matches_data_url(value)


_DROP = object()


class _Redactor:
    """Rebuilds a channel value without the asset; counts what it removed."""

    def __init__(self, fingerprint: AssetFingerprint) -> None:
        self.fingerprint = fingerprint
        self.removed = 0

    def channel(self, name: str, value: Any) -> Any:
        if name == POSTURE_CHANNEL and value is not None:
            probe = _Redactor(self.fingerprint)
            if probe.value(value) is not value:
                self.removed += max(probe.removed, 1)
                return None
            return value
        redacted = self.value(value)
        return None if redacted is _DROP else redacted

    def value(self, value: Any) -> Any:
        if isinstance(value, str):
            if self.fingerprint.names(value):
                self.removed += 1
                return ERASED_TEXT
            return value
        if isinstance(value, list | tuple):
            items = [self.value(item) for item in value]
            if all(new is old for new, old in zip(items, value, strict=True)):
                return value
            return type(value)(item for item in items if item is not _DROP)
        if isinstance(value, dict):
            return self._mapping(value)
        return value

    def _mapping(self, value: dict[Any, Any]) -> Any:
        image = value.get("image_url") if value.get("type") == "image_url" else None
        url = image.get("url") if isinstance(image, dict) else image
        if isinstance(url, str) and self.fingerprint.matches_data_url(url):
            self.removed += 1
            return {"type": "text", "text": ERASED_IMAGE_TEXT}
        # A record that points at the asset by id or carries its bytes
        # (an image ref, a posture view, an evidence entry) goes as a whole:
        # keeping its siblings would keep the derived content.
        for key, item in value.items():
            if (isinstance(key, str) and self.fingerprint.asset_id in key) or (
                isinstance(item, str)
                and (item == self.fingerprint.asset_id or self.fingerprint.matches_data_url(item))
            ):
                self.removed += 1
                return _DROP
        out: dict[Any, Any] = {}
        changed = False
        for key, item in value.items():
            new = self.value(item)
            changed = changed or new is not item
            if new is not _DROP:
                out[key] = new
        return out if changed else value


@dataclass(frozen=True)
class ThreadErasureResult:
    thread_id: str
    status: str
    checkpoint_id: str | None = None
    redacted_refs: int = 0
    deleted_checkpoints: int = 0
    failure_code: str | None = None


@dataclass(frozen=True)
class AssetRuntimeErasureReceipt:
    asset_id: str
    threads: list[ThreadErasureResult]

    @property
    def status(self) -> str:
        if any(item.status == THREAD_OUTSTANDING for item in self.threads):
            return THREAD_OUTSTANDING
        return THREAD_ERASED


_PG_MENTIONS_SQL = """
SELECT (SELECT count(*) FROM checkpoint_blobs
         WHERE thread_id = %(t)s AND position(convert_to(%(n)s, 'UTF8') in blob) > 0)
     + (SELECT count(*) FROM checkpoint_writes
         WHERE thread_id = %(t)s AND position(convert_to(%(n)s, 'UTF8') in blob) > 0)
     + (SELECT count(*) FROM checkpoints
         WHERE thread_id = %(t)s AND strpos(checkpoint::text || metadata::text, %(n)s) > 0)
     AS hits"""


# Versions are zero-padded "<counter>.<hash>" strings, so byte order is
# version order.
_PG_PURGE_SUPERSEDED_SQL = """
DELETE FROM checkpoint_blobs b
USING checkpoints k
WHERE k.thread_id = %(t)s AND k.checkpoint_ns = %(ns)s AND k.checkpoint_id = %(keep)s
  AND b.thread_id = %(t)s AND b.checkpoint_ns = %(ns)s
  AND (b.version COLLATE "C") < ((k.checkpoint -> 'channel_versions' ->> b.channel) COLLATE "C")
"""


def _scalar(row: Any, key: str) -> Any:
    if row is None:
        return None
    return row[key] if isinstance(row, dict) else row[0]


def _bytes_in(value: Any, needle: bytes) -> bool:
    if isinstance(value, bytes | bytearray | memoryview):
        return needle in bytes(value)
    if isinstance(value, str):
        return needle.decode() in value
    if isinstance(value, tuple | list):
        return any(_bytes_in(item, needle) for item in value)
    if isinstance(value, dict):
        return any(_bytes_in(item, needle) for item in value.values())
    return False


async def _storage_mentions(saver: Any, thread_id: str, needle: str) -> bool:
    """Whether any stored byte of the thread (every namespace) names ``needle``."""
    if isinstance(saver, AsyncPostgresSaver):
        from langgraph.checkpoint.postgres import _ainternal

        async with _ainternal.get_connection(saver.conn) as conn, conn.cursor() as cur:
            await cur.execute(_PG_MENTIONS_SQL, {"t": thread_id, "n": needle})
            return bool(_scalar(await cur.fetchone(), "hits"))
    if isinstance(saver, InMemorySaver):
        raw = needle.encode()
        if _bytes_in(saver.storage.get(thread_id, {}), raw):
            return True
        if any(key[0] == thread_id and _bytes_in(v, raw) for key, v in saver.blobs.items()):
            return True
        return any(key[0] == thread_id and _bytes_in(v, raw) for key, v in saver.writes.items())
    raise UnsupportedCheckpointerError(f"cannot inspect {type(saver).__name__}")


async def _purge_superseded_blobs(saver: Any, thread_id: str, keep_id: str) -> None:
    """Delete root blobs older than the kept checkpoint's version of their channel.

    After compaction nothing references them; a turn racing this writes newer
    versions only, so the version bound never touches live state.
    """
    if isinstance(saver, AsyncPostgresSaver):
        from langgraph.checkpoint.postgres import _ainternal

        async with _ainternal.get_connection(saver.conn) as conn, conn.cursor() as cur:
            await cur.execute(
                _PG_PURGE_SUPERSEDED_SQL, {"t": thread_id, "ns": ROOT_NAMESPACE, "keep": keep_id}
            )
        return
    if isinstance(saver, InMemorySaver):
        root = saver.storage.get(thread_id, {}).get(ROOT_NAMESPACE, {})
        if keep_id not in root:
            raise CheckpointConflictError("kept checkpoint disappeared while erasing")
        kept = saver.serde.loads_typed(root[keep_id][0])["channel_versions"]
        for key in list(saver.blobs):
            thread, namespace, channel, version = key
            if thread != thread_id or namespace != ROOT_NAMESPACE or channel not in kept:
                continue
            if str(version) < str(kept[channel]):
                del saver.blobs[key]
        return
    raise UnsupportedCheckpointerError(f"cannot purge {type(saver).__name__}")


def _count_refs(values: dict[str, Any], fingerprint: AssetFingerprint) -> int:
    probe = _Redactor(fingerprint)
    for name, value in values.items():
        probe.channel(name, value)
    return probe.removed


async def erase_asset_from_thread(
    graph: Any, *, thread_id: str, fingerprint: AssetFingerprint, attached: bool
) -> ThreadErasureResult:
    """Remove one asset from one consultation thread and verify from storage.

    Idempotent: a repeat finds nothing to redact, one checkpoint left and no
    stored mention, and reports the thread erased again.
    """
    saver = graph.checkpointer
    config = _root_config(thread_id)
    mentioned = await _storage_mentions(saver, thread_id, fingerprint.asset_id)
    current = await saver.aget_tuple(config)
    if current is None and not mentioned:
        return ThreadErasureResult(thread_id=thread_id, status=THREAD_ABSENT)
    if not attached and not mentioned:
        # Images only enter a thread through an attachment Go recorded, so a
        # thread that was never attached and never names the asset holds none.
        return ThreadErasureResult(thread_id=thread_id, status=THREAD_ABSENT)

    snapshot = await graph.aget_state(config)
    if snapshot.next or snapshot.interrupts:
        raise ThreadBusyError("thread is running or waiting on an interrupt")
    if current is None:
        raise CheckpointCleanupError("asset copy outside the root checkpoint namespace")
    if current.pending_writes:
        raise ThreadBusyError("thread has pending writes")

    redactor = _Redactor(fingerprint)
    values = current.checkpoint["channel_values"]
    changed = {}
    for name, value in values.items():
        new = redactor.channel(name, value)
        if new is not value:
            changed[name] = new

    keep_id = current.checkpoint["id"]
    if changed:
        rewritten = copy_checkpoint(current.checkpoint)
        step = int((current.metadata or {}).get("step", -1)) + 1
        rewritten["id"] = str(uuid6(clock_seq=step))
        rewritten["ts"] = datetime.now(UTC).isoformat()
        new_versions = {}
        for name, value in changed.items():
            version = saver.get_next_version(rewritten["channel_versions"].get(name), None)
            rewritten["channel_values"][name] = value
            rewritten["channel_versions"][name] = version
            new_versions[name] = version
        rewritten["updated_channels"] = sorted(changed)
        metadata = cast(CheckpointMetadata, {"source": "update", "step": step, "parents": {}})
        await saver.aput(current.config, rewritten, metadata, new_versions)
        keep_id = rewritten["id"]

    deleted = await _compact(saver, thread_id, keep_id)
    await _purge_superseded_blobs(saver, thread_id, keep_id)

    # Verify from storage, not from what we meant to write.
    final = await saver.aget_tuple(config)
    if final is None or final.checkpoint["id"] != keep_id:
        raise CheckpointConflictError("kept checkpoint is no longer the latest")
    if final.pending_writes:
        raise CheckpointConflictError("thread received writes while erasing")
    if _count_refs(final.checkpoint["channel_values"], fingerprint):
        raise CheckpointCleanupError("asset copy survived the rewrite")
    history = [item async for item in saver.alist(config, limit=2)]
    if len(history) != 1:
        raise CheckpointConflictError("thread history was not compacted")
    if await _storage_mentions(saver, thread_id, fingerprint.asset_id):
        raise CheckpointCleanupError("stored checkpoint data still names the asset")

    return ThreadErasureResult(
        thread_id=thread_id,
        status=THREAD_ERASED,
        checkpoint_id=keep_id,
        redacted_refs=redactor.removed,
        deleted_checkpoints=deleted,
    )


async def erase_asset_runtime_copies(
    graph: Any,
    *,
    asset_id: str,
    content_sha256: str,
    thread_ids: list[str],
    attached_thread_ids: list[str],
) -> AssetRuntimeErasureReceipt:
    """Erase one asset from every listed thread; one thread's refusal never
    hides another's result. An unsupported checkpointer fails the whole call."""
    fingerprint = AssetFingerprint(asset_id=asset_id, content_sha256=content_sha256)
    attached = set(attached_thread_ids)
    results: list[ThreadErasureResult] = []
    for thread_id in dict.fromkeys(thread_ids):
        try:
            result = await erase_asset_from_thread(
                graph, thread_id=thread_id, fingerprint=fingerprint, attached=thread_id in attached
            )
        except UnsupportedCheckpointerError:
            raise
        except CheckpointCleanupError as exc:
            result = ThreadErasureResult(
                thread_id=thread_id, status=THREAD_OUTSTANDING, failure_code=exc.code
            )
        results.append(result)
    return AssetRuntimeErasureReceipt(asset_id=asset_id, threads=results)
