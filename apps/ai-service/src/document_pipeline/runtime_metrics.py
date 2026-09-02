"""Runtime/resource summaries for production-shaped document benchmarks."""

from __future__ import annotations

import math
import resource
from pathlib import Path

from .contracts import FixtureBenchmarkResult, RuntimeSummary

_CGROUP_ROOT = Path("/sys/fs/cgroup")


def _rss_mb(value_kib: int | float) -> float:
    # Linux getrusage reports ru_maxrss in KiB.
    return float(value_kib) / 1024.0


def _memory_limit_mb() -> float | None:
    path = _CGROUP_ROOT / "memory.max"
    if not path.is_file():
        return None
    value = path.read_text(encoding="utf-8").strip()
    if value == "max":
        return None
    try:
        return int(value) / (1024 * 1024)
    except ValueError:
        return None


def _memory_peak_mb() -> float | None:
    path = _CGROUP_ROOT / "memory.peak"
    if not path.is_file():
        return None
    value = path.read_text(encoding="utf-8").strip()
    try:
        return int(value) / (1024 * 1024)
    except ValueError:
        return None


def _memory_events() -> dict[str, int]:
    path = _CGROUP_ROOT / "memory.events"
    if not path.is_file():
        return {}
    events: dict[str, int] = {}
    for line in path.read_text(encoding="utf-8").splitlines():
        fields = line.split()
        if len(fields) != 2:
            continue
        try:
            events[fields[0]] = int(fields[1])
        except ValueError:
            continue
    return events


def _swap_peak_mb() -> float | None:
    path = _CGROUP_ROOT / "memory.swap.peak"
    if not path.is_file():
        return None
    value = path.read_text(encoding="utf-8").strip()
    try:
        return int(value) / (1024 * 1024)
    except ValueError:
        return None


def _cpu_limit() -> float | None:
    path = _CGROUP_ROOT / "cpu.max"
    if not path.is_file():
        return None
    fields = path.read_text(encoding="utf-8").strip().split()
    if len(fields) != 2 or fields[0] == "max":
        return None
    try:
        quota, period = int(fields[0]), int(fields[1])
    except ValueError:
        return None
    if quota <= 0 or period <= 0:
        return None
    return quota / period


def _p95(values: list[float]) -> float:
    if not values:
        return 0.0
    ordered = sorted(values)
    index = max(0, math.ceil(len(ordered) * 0.95) - 1)
    return ordered[index]


def summarize_runtime(results: list[FixtureBenchmarkResult]) -> RuntimeSummary:
    elapsed = [result.elapsed_ms for result in results]
    self_usage = resource.getrusage(resource.RUSAGE_SELF)
    child_usage = resource.getrusage(resource.RUSAGE_CHILDREN)
    memory_events = _memory_events()
    return RuntimeSummary(
        total_elapsed_ms=sum(elapsed),
        mean_fixture_ms=sum(elapsed) / len(elapsed) if elapsed else 0.0,
        p95_fixture_ms=_p95(elapsed),
        peak_self_rss_mb=_rss_mb(self_usage.ru_maxrss),
        peak_child_rss_mb=_rss_mb(child_usage.ru_maxrss),
        cgroup_memory_limit_mb=_memory_limit_mb(),
        cgroup_memory_peak_mb=_memory_peak_mb(),
        cgroup_memory_events_max=memory_events.get("max"),
        cgroup_memory_events_oom=memory_events.get("oom"),
        cgroup_memory_events_oom_kill=memory_events.get("oom_kill"),
        cgroup_swap_peak_mb=_swap_peak_mb(),
        cgroup_cpu_limit=_cpu_limit(),
    )
