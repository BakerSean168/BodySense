#!/usr/bin/env python3
from __future__ import annotations

import importlib.util
import unittest
from pathlib import Path

MODULE_PATH = Path(__file__).with_name("check_python_boundaries.py")
SPEC = importlib.util.spec_from_file_location("check_python_boundaries", MODULE_PATH)
assert SPEC is not None and SPEC.loader is not None
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class PythonBoundaryPolicyTests(unittest.TestCase):
    def test_runtime_proto_is_allowed_in_boundary_adapter(self) -> None:
        violations = MODULE.analyze_source(
            "apps/ai-service/src/api/runtime_proto_adapter.py",
            "from ..generated.runtimeproto.bodysense.runtime.v1 import runtime_pb2\n",
        )
        self.assertEqual(violations, [])

    def test_runtime_proto_fails_when_it_leaks_into_runtime_domain(self) -> None:
        violations = MODULE.analyze_source(
            "apps/ai-service/src/runtime/leaky_runtime.py",
            "from src.generated.runtimeproto.bodysense.runtime.v1 import runtime_pb2\n",
        )
        self.assertEqual(len(violations), 1)
        self.assertIn("generated runtime Proto leaked outside the Python boundary adapter", violations[0])

    def test_retired_stream_event_authority_cannot_reappear(self) -> None:
        violations = MODULE.analyze_source(
            "apps/ai-service/src/models/stream_event.py",
            "class StreamEvent: pass\n",
        )
        self.assertEqual(len(violations), 1)
        self.assertIn("retired generic Python runtime StreamEvent authority resurfaced", violations[0])

    def test_retired_runtime_event_map_identifier_cannot_reappear(self) -> None:
        violations = MODULE.analyze_source(
            "apps/ai-service/src/runtime/leaky_runtime.py",
            "_RUNTIME_EVENT_FIELD_BY_TYPE = {}\n",
        )
        self.assertEqual(len(violations), 1)
        self.assertIn("retired generic internal runtime authority resurfaced", violations[0])

    def test_pydanticai_openai_transport_is_owned_by_shared_gateway(self) -> None:
        owner = MODULE.analyze_source(
            "apps/ai-service/src/ai/gateway.py",
            "from pydantic_ai.providers.openai import OpenAIProvider\n",
        )
        self.assertEqual(owner, [])

        leaked = MODULE.analyze_source(
            "apps/ai-service/src/ai/role_specific_gateway.py",
            "from pydantic_ai.providers.openai import OpenAIProvider\n",
        )
        self.assertEqual(len(leaked), 1)
        self.assertIn("PydanticAI OpenAI transport must be constructed only by ai/gateway.py", leaked[0])

    def test_non_generated_import_is_unrestricted(self) -> None:
        violations = MODULE.analyze_source(
            "apps/ai-service/src/runtime/normal.py",
            "from pydantic import BaseModel\n",
        )
        self.assertEqual(violations, [])


if __name__ == "__main__":
    unittest.main()
