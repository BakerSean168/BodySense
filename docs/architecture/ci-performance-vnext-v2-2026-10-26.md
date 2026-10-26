# BodySense CI Performance Evidence

Source: GitHub Actions job start/completion timestamps from the linked runs.
Values exclude GitHub queue time from the PR gate, but queue is recorded separately.
Runner-minutes are observed summed job durations, **not billed minutes**.
Nearest-rank P95 with small samples is provisional. All five change classes need at least three completed representative runs.

| Change class | Successful PR samples | Gate P50 | Gate P95 | Sampling ready |
| --- | ---: | ---: | ---: | --- |
| root | 4 | 215.0s | 273.0s | yes |
| web | 0 | — | — | no |
| server | 0 | — | — | no |
| ai | 0 | — | — | no |
| contracts | 0 | — | — | no |

Target: ordinary PR **P95 <= 300s**. Evidence status: **NOT YET CERTIFIED**.

| Run | Event | Class | Source SHA | Gate | Wall | Queue | Sum runner-min | Longest job |
| --- | --- | --- | --- | ---: | ---: | ---: | ---: | --- |
| [40309858820](https://github.com/BakerSean168/BodySense/actions/runs/40309858820) | pull_request | root | 68cb289b02 | 273s | 273s | 81s | 18.33 | Test Web (Browser) (239s) |
| [40311620486](https://github.com/BakerSean168/BodySense/actions/runs/40311620486) | pull_request | root | 59409cdef3 | 215s | 215s | 108s | 17.8 | Test Server (PostgreSQL baseline) (172s) |
| [40316155651](https://github.com/BakerSean168/BodySense/actions/runs/40316155651) | pull_request | root | f90f776480 | 212s | 212s | 3s | 18.13 | Test Server (PostgreSQL baseline) (175s) |
| [40317701029](https://github.com/BakerSean168/BodySense/actions/runs/40317701029) | pull_request | root | cbd2e14ba3 | 223s | 223s | 4s | 18.08 | Test Server (PostgreSQL baseline) (175s) |

Historical B8 reference **excluded from the new P95 sample**: [40157990561](https://github.com/BakerSean168/BodySense/actions/runs/40157990561) — gate 510s, observed runner-minutes 33.88; longest job Full quality child (validate-repo).
The historic run's validation scope and job graph differ, so these figures are directional rather than a strictly controlled A/B.

Incomplete/cancelled/failed runs are not included in P50/P95.
These root/CI runs are stacked and may have different cache warmness, base branches and parallel runner availability; they are not a controlled T3 Code A/B comparison.
Do not switch branch protection or merge the final integration PR based on this report alone.
