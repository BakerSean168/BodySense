import importlib.util
from pathlib import Path
import unittest
from unittest.mock import patch

SCRIPT = Path(__file__).with_name("run-go-db-tests.py")
SPEC = importlib.util.spec_from_file_location("go_db", SCRIPT)
go_db = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(go_db)
import json


def event(package, name, action):
    return json.dumps({
        "Time": "2026-10-26T00:00:00Z",
        "Action": action,
        "Package": "github.com/bodysense/api/" + package,
        "Test": name,
    })


class GoDatabaseInventoryTests(unittest.TestCase):
    def test_inventory_requires_actual_go_functions_from_pg_test_files(self):
        inventory = go_db.discover_cases()
        self.assertGreaterEqual(sum(map(len, inventory.values())), 20)
        self.assertIn("TestBodyFactHTTPRecordReadRetryRoundTripPostgres",
                      inventory["./internal/transport/httpapi"])
        self.assertEqual(sum(len(v) for v in go_db.cases_for("pr").values()), 7)
        self.assertGreaterEqual(sum(len(v) for v in go_db.cases_for("full").values()), 20)

    def test_all_required_tests_must_pass_with_actual_package_match(self):
        selected = {"./internal/repository": {"TestBodyStateLegacyTablesAbsentPostgres"}}
        base = event("internal/repository", "TestBodyStateLegacyTablesAbsentPostgres", "run")
        done = event("internal/repository", "TestBodyStateLegacyTablesAbsentPostgres", "pass")
        ok, msg = go_db.verify_results(base + "\n" + done, selected, 0)
        self.assertTrue(ok, msg)

    def test_missing_or_skipped_or_failed_or_nonzero_is_rejected(self):
        selected = {"./internal/repository": {"TestBodyStateLegacyTablesAbsentPostgres"}}
        for status in ("skip", "fail"):
            with self.subTest(status=status):
                ok, msg = go_db.verify_results(
                    event("internal/repository", "TestBodyStateLegacyTablesAbsentPostgres", status),
                    selected, 0)
                self.assertFalse(ok)
                self.assertIn(status, msg)
        for lines in (
            "",
            event("internal/service", "TestBodyStateLegacyTablesAbsentPostgres", "pass"),
            event("internal/repository", "TestBodyStateLegacyTablesAbsentPostgres2", "pass"),
            event("internal/repository", "TestBodyStateLegacyTablesAbsentPostgres/nested", "pass"),
            "not-json-output",
        ):
            ok, _ = go_db.verify_results(lines, selected, 0)
            self.assertFalse(ok, lines)
        parent_ok = event("internal/repository", "TestBodyStateLegacyTablesAbsentPostgres", "pass")
        nested_skip = event("internal/repository", "TestBodyStateLegacyTablesAbsentPostgres/really_required", "skip")
        ok, msg = go_db.verify_results(parent_ok + "\n" + nested_skip, selected, 0)
        self.assertFalse(ok)
        self.assertIn("nested subtest skipped", msg)
        ok, msg = go_db.verify_results(
            event("internal/repository", "TestBodyStateLegacyTablesAbsentPostgres", "pass"),
            selected, 1)
        self.assertFalse(ok)
        self.assertIn("exit status 1", msg)


if __name__ == "__main__":
    unittest.main()
