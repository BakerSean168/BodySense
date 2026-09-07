# Full Stack Open -> BodySense parity policy and status

> Machine-readable truth: [`ledger/full-stack-open.json`](./ledger/full-stack-open.json)
> Generated status: [`views/coverage-status.md`](./views/coverage-status.md)
> Source policy: [`SOURCES.md`](./SOURCES.md)

## What is currently proven

The pinned repository snapshot `0711aef8a451c4458263e5587ccda85f08fd7a96` contains **262 distinct numbered exercises for Parts 0-11** under the audit counting rule.

For Parts 0-7, all **158** numbered exercises now have independent ledger records with:

- exact source number/title/file/line;
- a newly written concise learning objective;
- `DIRECT` BodySense mapping;
- BodySense target surface;
- task family and evidence plan;
- prerequisite relation;
- separate lifecycle/mastery state.

This fixes the earlier range-only mapping. In particular, source `1.13` is explicitly the immutable vote-like state update objective and `1.14` is explicitly the maximum/highest-value derivation objective; the extra debugging rep is no longer substituted under a source exercise number.

`MAPPED` still does **not** mean the exercise is ready to teach. Only records with an exercise card may become `EXERCISE_READY`.

## Source-currentness boundary

| Part | Current status |
|---|---|
| 0-7 | current-site snapshot indexed + semantically mapped |
| 8-11 | historical snapshot exercise IDs indexed; current MOOC content still unverified |
| 12-14 | current MOOC source explicitly unverified in this environment |

Therefore this repository does **not** claim current Full Stack Open 0-14 parity yet.

## Exercise numbers and concepts are different inventories

Numbered exercises are not enough to prove concept coverage. The ledger therefore also stores source section headings and separate concept records.

The first explicit concept repairs cover previously audited gaps:

- Part 2 asynchronous runtime/non-blocking I/O;
- Promise pending/fulfilled/rejected/chaining/error propagation;
- Effect lifecycle/dependencies/cleanup;
- Part 7 `useMemo`, `React.memo`, `useCallback` and reference stability;
- SQL injection / parameterized queries;
- XSS and untrusted rendering;
- dependency auditing / lockfile / supply-chain risk;
- broken authentication vs broken access control and server-side authorization.

The snapshot currently has **411 indexed level-3 source section headings**. That index makes omissions inspectable, but it is not itself a claim that every paragraph-level teaching idea has been semantically decomposed. The baseline explicitly marks concept semantic parity as partial until that audit is complete.

## BodySense translation rule

The source engineering objective is preserved; source-specific implementation technology is translated only where appropriate.

Examples:

```text
Node/Express       -> Go/Gin transport/runtime comparison + direct HTTP semantics
Mongo/Mongoose     -> PostgreSQL/GORM persistence + explicit NoSQL comparison when relevant
Redux legacy path  -> compare; current Zustand/Query/Context ownership is direct
GraphQL            -> compare against current REST/SSE until current Part 8 source is verified
```

No production framework/library is changed merely to make the repository look like the source course.

## Exercise-ready reps

The first FSO cards currently ready for actual L4 learning are generated/listed in [`views/coverage-status.md`](./views/coverage-status.md). More cards are promoted from `MAPPED` only after target files, failure cases and verification evidence are concrete.
