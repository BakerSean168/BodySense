from __future__ import annotations

from pathlib import Path
import csv
import tiktoken

ENC = tiktoken.get_encoding("o200k_base")
ROOT = Path("/home/dev/projects")
COMMON = ROOT / "bodysense-contract-codegen-common"
ORVAL = ROOT / "bodysense-contract-codegen-orval"
HEY = ROOT / "bodysense-contract-codegen-heyapi"
J1 = ROOT / "bodysense-contract-codegen-jsonschema"
PROTO = ROOT / "bodysense-contract-codegen-proto"
OPENAPI = ROOT / "bodysense-contract-codegen-openapi-shared"


def files_under(path: Path, suffixes: tuple[str, ...] = (".ts", ".go", ".py", ".js", ".cjs", ".mjs", ".yaml", ".yml", ".json", ".proto")) -> list[Path]:
    return sorted(p for p in path.rglob("*") if p.is_file() and p.suffix in suffixes and "__pycache__" not in p.parts)


def stat(paths: list[Path]) -> tuple[int, int, int]:
    seen: set[Path] = set()
    chars = tokens = 0
    count = 0
    for p in paths:
        p = p.resolve()
        if p in seen or not p.exists() or not p.is_file():
            continue
        seen.add(p)
        text = p.read_text(errors="ignore")
        chars += len(text)
        tokens += len(ENC.encode(text))
        count += 1
    return count, chars, tokens

spec = COMMON / "experiments/contract-codegen/common/spec/bodysense-spike.openapi.yaml"

rows: list[dict[str, object]] = []

def add(scope: str, candidate: str, canonical: list[Path], handwritten: list[Path], generated: list[Path], note: str):
    cf, cc, ct = stat(canonical)
    hf, hc, ht = stat(handwritten)
    gf, gc, gt = stat(generated)
    rows.append({
        "scope": scope,
        "candidate": candidate,
        "canonical_files": cf,
        "canonical_tokens": ct,
        "handwritten_files": hf,
        "handwritten_tokens": ht,
        "generated_files": gf,
        "generated_tokens": gt,
        "context_tokens_generated_visible": ct + ht + gt,
        "context_tokens_generated_excluded": ct + ht,
        "generated_visible_multiplier": round((ct + ht + gt) / max(1, ct + ht), 2),
        "note": note,
    })

add(
    "REST R1/R2",
    "B0 handwritten",
    [],
    [
        COMMON / "apps/api/internal/dto/health_workspace.go",
        COMMON / "apps/api/internal/dto/body_state.go",
        COMMON / "apps/web/src/features/workspace/types/workspace.ts",
        COMMON / "apps/web/src/features/workspace/api/workspaceApi.ts",
        COMMON / "apps/web/src/lib/api-client.ts",
    ],
    [],
    "Current Go DTO + TS DTO/client + shared error helper; domain/service internals excluded.",
)
add(
    "REST R1/R2",
    "O1 OpenAPI + Orval",
    [spec],
    [
        ORVAL / "experiments/contract-codegen/orval/candidate/orval.production.config.ts",
        OPENAPI / "experiments/contract-codegen/openapi-shared/oapi-codegen-strict.yaml",
        COMMON / "apps/web/src/lib/api-client.ts",
    ],
    files_under(ORVAL / "experiments/contract-codegen/orval/gen/fetch-validated-mini-force-success")
    + [OPENAPI / "experiments/contract-codegen/openapi-shared/gen/go-strict/spike.gen.go", OPENAPI / "experiments/contract-codegen/openapi-shared/gen/python/models.py"],
    "Production-shaped Mini validated fetch + Go strict server + Python model; TanStack adapter intentionally handwritten/small.",
)
add(
    "REST R1/R2",
    "O2 OpenAPI + Hey API",
    [spec],
    [
        HEY / "experiments/contract-codegen/heyapi/openapi-ts.config.ts",
        OPENAPI / "experiments/contract-codegen/openapi-shared/oapi-codegen-strict.yaml",
        COMMON / "apps/web/src/lib/api-client.ts",
    ],
    files_under(HEY / "experiments/contract-codegen/heyapi/gen")
    + [OPENAPI / "experiments/contract-codegen/openapi-shared/gen/go-strict/spike.gen.go", OPENAPI / "experiments/contract-codegen/openapi-shared/gen/python/models.py"],
    "Generated SDK + Zod + TanStack; runtime parse/strictness still needs explicit boundary adapter.",
)

add(
    "Public StreamEvent",
    "B0 handwritten/parity",
    [],
    [
        COMMON / "packages/contracts/src/stream-events.ts",
        COMMON / "packages/contracts/src/stream-event-parser.ts",
        COMMON / "packages/contracts/schemas/stream-event.v1.schema.json",
        COMMON / "apps/api/internal/dto/stream_event.go",
        COMMON / "apps/ai-service/src/models/stream_event.py",
    ],
    [],
    "Current five-definition surface before fixtures/tests.",
)
add(
    "Public StreamEvent",
    "J1 JSON Schema-first",
    [J1 / "experiments/contract-codegen/jsonschema/candidate/stream-event.v1.schema.json"],
    [J1 / "experiments/contract-codegen/jsonschema/compile-validator.mjs"],
    [
        J1 / "experiments/contract-codegen/jsonschema/gen/stream-event-fixed.d.ts",
        J1 / "experiments/contract-codegen/jsonschema/gen/validate-stream-event-fixed.cjs",
    ],
    "Fixed schema is canonical; generated TS + Ajv standalone should be excluded from normal Agent context.",
)

add(
    "Internal Go-Python runtime",
    "B0 HTTP/NDJSON handwritten",
    [],
    [
        COMMON / "apps/api/internal/service/ai_client.go",
        COMMON / "apps/ai-service/src/api/routes/runtime.py",
        COMMON / "apps/ai-service/src/models/stream_event.py",
    ],
    [],
    "Current cross-language request/stream boundary.",
)
add(
    "Internal Go-Python runtime",
    "P1 Proto/Buf internal",
    [PROTO / "experiments/contract-codegen/proto/proto/bodysense/contracts/v1/consultation.proto"],
    [PROTO / "experiments/contract-codegen/proto/buf.gen.yaml", PROTO / "experiments/contract-codegen/proto/buf.yaml"],
    files_under(PROTO / "experiments/contract-codegen/proto/gen/go") + files_under(PROTO / "experiments/contract-codegen/proto/gen/python"),
    "Primary internal scope; TS/browser generation intentionally excluded from this row.",
)
add(
    "Internal Go-Python runtime",
    "P1 Proto/Buf full 3-language",
    [PROTO / "experiments/contract-codegen/proto/proto/bodysense/contracts/v1/consultation.proto"],
    [PROTO / "experiments/contract-codegen/proto/buf.gen.yaml", PROTO / "experiments/contract-codegen/proto/buf.yaml"],
    files_under(PROTO / "experiments/contract-codegen/proto/gen/go") + files_under(PROTO / "experiments/contract-codegen/proto/gen/python") + files_under(PROTO / "experiments/contract-codegen/proto/gen/ts"),
    "Control showing generated-context tax if every language output is indexed/read by the Agent.",
)

out = COMMON / "experiments/contract-codegen/common/metrics/ai-context-proxy.tsv"
with out.open("w", newline="") as f:
    w = csv.DictWriter(f, fieldnames=list(rows[0].keys()), delimiter="\t")
    w.writeheader(); w.writerows(rows)

for r in rows:
    print(r)
