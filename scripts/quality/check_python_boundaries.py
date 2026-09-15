#!/usr/bin/env python3
from __future__ import annotations

import argparse
import ast
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
ALLOWED_RUNTIME_PROTO_CONSUMERS = {
    "apps/ai-service/src/api/runtime_proto_adapter.py",
    "apps/ai-service/tests/unit/test_runtime_proto_contract.py",
}
RETIRED_STREAM_EVENT_AUTHORITY = "apps/ai-service/src/models/stream_event.py"
RETIRED_RUNTIME_IDENTIFIERS = {"_RUNTIME_EVENT_FIELD_BY_TYPE"}
PYDANTIC_GATEWAY_OWNER = "apps/ai-service/src/ai/gateway.py"
PYDANTIC_GATEWAY_MODULES = {
    "pydantic_ai.models.openai",
    "pydantic_ai.providers.openai",
}


def _normalize(path: str | Path) -> str:
    return Path(path).as_posix()


def _is_generated_runtime_module(module: str) -> bool:
    return "generated.runtimeproto" in module


def analyze_source(rel_path: str, source: str) -> list[str]:
    rel = _normalize(rel_path)
    if rel.startswith("apps/ai-service/src/generated/"):
        return []

    violations: list[str] = []
    if rel == RETIRED_STREAM_EVENT_AUTHORITY:
        violations.append(f"{rel}: retired generic Python runtime StreamEvent authority resurfaced")

    tree = ast.parse(source, filename=rel)
    for node in ast.walk(tree):
        if isinstance(node, ast.Name) and node.id in RETIRED_RUNTIME_IDENTIFIERS:
            violations.append(
                f"{rel}:{node.lineno}:{node.col_offset + 1}: retired generic internal runtime authority resurfaced ({node.id})"
            )

        modules: list[str] = []
        if isinstance(node, ast.Import):
            modules.extend(alias.name for alias in node.names)
        elif isinstance(node, ast.ImportFrom):
            modules.append(node.module or "")

        for module in modules:
            if _is_generated_runtime_module(module) and rel not in ALLOWED_RUNTIME_PROTO_CONSUMERS:
                violations.append(
                    f"{rel}:{node.lineno}:{node.col_offset + 1}: generated runtime Proto leaked outside the Python boundary adapter ({module})"
                )
            if module in PYDANTIC_GATEWAY_MODULES and rel != PYDANTIC_GATEWAY_OWNER:
                violations.append(
                    f"{rel}:{node.lineno}:{node.col_offset + 1}: PydanticAI OpenAI transport must be constructed only by ai/gateway.py ({module})"
                )
    return violations


def scan_python_boundaries(root: Path = REPO_ROOT) -> list[str]:
    service_root = root / "apps/ai-service"
    violations: list[str] = []
    for full in service_root.rglob("*.py"):
        rel = full.relative_to(root).as_posix()
        if "/.venv/" in f"/{rel}/" or rel.startswith("apps/ai-service/src/generated/"):
            continue
        violations.extend(analyze_source(rel, full.read_text(encoding="utf-8")))
    return violations


def main() -> int:
    parser = argparse.ArgumentParser(description="Validate Python generated-boundary architecture")
    parser.add_argument("--root", type=Path, default=REPO_ROOT)
    args = parser.parse_args()
    violations = scan_python_boundaries(args.root.resolve())
    if violations:
        print("Python architecture violations:")
        print("\n".join(violations))
        return 1
    print("PYTHON_GENERATED_BOUNDARY=PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
