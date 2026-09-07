# TECH SCHOOL Backend Master Class -> BodySense parity policy and status

> Machine-readable truth: [`ledger/techschool-backend.json`](./ledger/techschool-backend.json)
> Generated status: [`views/coverage-status.md`](./views/coverage-status.md)
> Source policy: [`SOURCES.md`](./SOURCES.md)

## Proven source coverage

The public `techschool/simplebank` README is pinned at:

```text
97f000fe58ad01a0774179ffa8884ac7784cf263
```

At that commit, Backend Master Class lectures **#0 through #77** are present. The ledger contains all 78 lecture IDs, exact public titles, README line locations and public video URLs.

All 78 have a title-level BodySense mapping using `DIRECT` or `COMPARE`.

That statement is intentionally narrower than “all TECH SCHOOL knowledge is 100% reproduced”: the paid/video-internal teaching sequence, demonstrations and sub-objectives were not exhaustively transcribed/audited. Every ledger record therefore carries source authority `VERIFIED_PUBLIC_README_TITLE_ONLY`.

## Translation policy

`DIRECT` examples:

- PostgreSQL schema/migrations;
- transaction boundaries;
- row locks/deadlock/isolation;
- Gin REST semantics;
- validation/database error translation;
- bcrypt/JWT/session authority;
- Docker/CI/release;
- durable background jobs;
- graceful shutdown and CORS.

`COMPARE` examples:

- sqlc vs current GORM/repository boundary;
- PASETO vs JWT access token + opaque refresh credential + Redis session authority;
- gRPC/protobuf vs current Go <-> Python HTTP/typed JSON boundaries;
- Asynq vs BodySense durable JobRuntime;
- AWS EKS/RDS-specific operations vs current BodySense deployment.

A comparison lecture still requires verification evidence (for example a small spike, failure analysis, protocol/schema comparison or production trace) before learner mastery can be recorded.

## Exercise readiness

Title-level mapping is `MAPPED`, not `EXERCISE_READY`.

The high-value backend focus nodes include:

- `BS-TECH-06` transaction boundary;
- `BS-TECH-07` lock/deadlock reasoning;
- `BS-TECH-09` isolation/read phenomena;
- `BS-TECH-11` REST boundary;
- `BS-TECH-16` database error classification;
- `BS-TECH-37` refresh-token race/replay;
- `BS-TECH-54` durable job lifecycle.

Their prerequisite closure is also promoted to `EXERCISE_READY`, so the actual ready count is larger than this focus list. Every ready card has concrete BodySense files, prediction, failure case, verification evidence and an L4 gate. The generated coverage view is authoritative for the current count.
