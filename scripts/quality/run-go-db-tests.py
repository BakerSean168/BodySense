#!/usr/bin/env python3
"""Run real PostgreSQL-backed Go tests and reject silent integration-test skips.

This is *not* an all-stack web E2E runner. It runs Go packages directly against
the PG18 schema after the migration validator has prepared the test database.
"""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys

API_ROOT = Path(__file__).resolve().parents[2] / "apps" / "api"
DATABASE_ENV = "BODYSENSE_INTEGRATION_DATABASE_URL"
GO_TEST_NAME = re.compile(r"^\s*func\s+(Test[A-Za-z0-9_]+)\s*\(", re.M)
PR_CASES = {
    "./internal/consultation": (
        "TestInteractionResumeSameRunAndAtomicMilestonesPostgres",
    ),
    "./internal/repository": (
        "TestBodyStateLegacyTablesAbsentPostgres",
        "TestRunLeaseConcurrentReconciliationPostgres",
        "TestConversationSingletonDatabaseRejectsSecondConversation",
    ),
    "./internal/service": (
        "TestOnboardingContextCommitsProfileAndV2LedgerPostgres",
        "TestChatRetentionPruneJobRetriesFailedCleanupPostgres",
    ),
    "./internal/transport/httpapi": (
        "TestBodyFactHTTPRecordReadRetryRoundTripPostgres",
    ),
}


def discover_cases() -> dict[str, set[str]]:
    """Return all Go tests in files explicitly requiring the PG integration DSN."""
    cases: dict[str, set[str]] = {}
    for file in sorted(API_ROOT.rglob("*_test.go")):
        contents = file.read_text(encoding="utf8")
        if DATABASE_ENV not in contents:
            continue
        names = set(GO_TEST_NAME.findall(contents)) - {"TestMain"}
        if not names:
            raise ValueError(f"{file} reads {DATABASE_ENV} but contains no test")
        package = "./" + file.parent.relative_to(API_ROOT).as_posix()
        cases.setdefault(package, set()).update(names)
    return cases


def cases_for(profile: str) -> dict[str, set[str]]:
    inventory = discover_cases()
    if not inventory or sum(map(len, inventory.values())) < 20:
        raise ValueError("Unexpectedly empty PG test inventory; fail closed")
    if profile == "full":
        return inventory
    if profile != "pr":
        raise ValueError(f"Unknown profile {profile}")
    selected = {name: set(tests) for name, tests in PR_CASES.items()}
    for package, tests in selected.items():
        if not tests <= inventory.get(package, set()):
            raise ValueError(f"Missing required PR PostgreSQL test(s): {package}: {tests - inventory.get(package, set())}")
    return selected


def verify_results(lines: str, selected: dict[str, set[str]], returncode: int) -> tuple[bool, str]:
    expected = {(package.removeprefix("./"), name) for package, names in selected.items() for name in names}
    observed: dict[tuple[str, str], str] = {}
    failures: list[str] = []
    parsed = 0
    for line in lines.splitlines():
        if not line.strip():
            continue
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            failures.append(f"Invalid Go -json output: {line[:150]}")
            continue
        parsed += 1
        name = event.get("Test", "")
        if not name:
            continue
        package = event.get("Package", "")
        if "/" in name:
            # A top-level Go Test can be "pass" while a required nested
            # assertion was skipped. Preserve fail-closed semantics.
            for pkg, test in expected:
                if name.startswith(test + "/") and package.endswith("/" + pkg) and event.get("Action") == "skip":
                    failures.append(f"{pkg}/{name} nested subtest skipped")
            continue
        for pkg, test in expected:
            if test == name and package.endswith("/" + pkg):
                if event.get("Action") in {"pass", "fail", "skip"}:
                    observed[(pkg, test)] = event["Action"]
    for key in sorted(expected):
        action = observed.get(key, "missing")
        if action != "pass":
            failures.append(f"{key[0]}/{key[1]} status={action}, expected=pass")
    if returncode:
        failures.append(f"go test exit status {returncode}")
    if parsed == 0:
        failures.append("No parsed Go -json events")
    return not failures, "\n".join(failures)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--profile", choices=["pr", "full"], required=True)
    parser.add_argument("--list", action="store_true")
    args = parser.parse_args()
    selected = cases_for(args.profile)
    total = sum(map(len, selected.values()))
    if args.list:
        print(json.dumps({"profile": args.profile, "count": total, "packages": {k: sorted(v) for k,v in selected.items()}}, indent=2))
        return 0
    if not os.environ.get(DATABASE_ENV):
        print(f"{DATABASE_ENV} missing; refusing to treat skipped integration tests as pass", file=sys.stderr)
        return 2
    names = sorted({name for tests in selected.values() for name in tests})
    names_regex = "^(" + "|".join(re.escape(n) for n in names) + ")$"
    command = ["go", "test", "-json", "-count=1", "-p=1", "-parallel=1", "-timeout=4m", "-run", names_regex, *sorted(selected)]
    print(f"GO_DB_INTEGRATION profile={args.profile} expected={total} packages={len(selected)}", flush=True)
    result = subprocess.run(command, cwd=API_ROOT, capture_output=True, text=True, check=False, timeout=840)
    ok, detail = verify_results(result.stdout, selected, result.returncode)
    if not ok:
        print(detail, file=sys.stderr)
        print(result.stderr[-12000:], file=sys.stderr)
        for line in result.stdout.splitlines():
            try:
                event = json.loads(line)
            except json.JSONDecodeError:
                continue
            if event.get("Action") in {"fail", "skip"}:
                print(json.dumps(event), file=sys.stderr)
        return 1
    print(f"GO_DB_INTEGRATION=PASS profile={args.profile} tests={total}")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (ValueError, subprocess.TimeoutExpired) as err:
        print(f"GO_DB_INTEGRATION=FAIL {err}", file=sys.stderr)
        raise SystemExit(2)
