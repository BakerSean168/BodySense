# Production Capacity and Observability Runbook

## Purpose

BodySense production currently runs on a small Alibaba ECS. This runbook defines the measured resource envelope, bounded swap policy, container ceilings, capacity/liveness status contract, conservative cleanup, and the evidence required before changing host size.

This is a protection layer, not a substitute for scaling. Limits are intentionally above the observed workload and must not be tightened from a single sample.

## Measured baseline — 2026-08-24

Read-only production observation before introducing limits:

- host RAM: approximately 1.64 GiB;
- available RAM: approximately 258 MiB;
- swap: 0;
- root filesystem: approximately 20% used;
- LiteLLM: approximately 701 MiB;
- AI service: approximately 137 MiB;
- PostgreSQL: approximately 77 MiB;
- Caddy: approximately 20 MiB;
- API: approximately 14 MiB;
- Web: approximately 5 MiB;
- Redis: approximately 3 MiB;
- all seven long-running containers: restart count 0, `OOMKilled=false`;
- no recent kernel OOM evidence was observed.

The immediate risk is therefore memory-spike failure behavior on a zero-swap host, not current disk exhaustion or a demonstrated steady-state capacity outage.

## Versioned resource envelope

| Service | Reservation | Hard limit | Baseline rationale |
| --- | ---: | ---: | --- |
| PostgreSQL | 128 MiB | 384 MiB | ~77 MiB observed; leaves several-times headroom for query/migration bursts |
| Redis | 16 MiB | 96 MiB | ~3 MiB observed |
| LiteLLM | 640 MiB | 1152 MiB | ~701 MiB observed; largest resident process and widest burst envelope |
| AI service | 160 MiB | 384 MiB | ~137 MiB observed |
| API | 48 MiB | 192 MiB | ~14 MiB observed |
| Web | 16 MiB | 96 MiB | ~5 MiB observed |
| Caddy | 32 MiB | 128 MiB | ~20 MiB observed |

The ops-only DR container retains its existing 768 MiB hard limit. Docker Compose is validated on the actual production runtime family (Compose v5 / cgroup v2).

All Compose services use bounded `json-file` logging (`10m × 3`) so container logs cannot grow without limit.

## Swap policy

`install-production-capacity.sh` creates a 2 GiB `/swapfile` only when the host has no active swap and enough disk headroom. It configures:

```text
vm.swappiness=10
```

Swap is an OOM shock absorber. It is not considered normal working capacity.

Safety behavior:

- an existing inactive `/swapfile` is never overwritten automatically;
- a nominal swap target may report up to one kernel page less usable capacity because `mkswap` reserves metadata; that formatting overhead is accepted;
- an existing unknown swap layout materially smaller than the configured target (more than one kernel page below it) is not mutated automatically;
- configured swap is persisted through `/etc/fstab`;
- deployment establishes swap before restarting memory-capped services;
- rollback does not remove swap, because removing emergency headroom during a failed deployment would increase risk.

## Capacity status contract

`production-capacity-status.sh status` records `/opt/bodysense/.capacity-state` and evaluates:

- host `MemAvailable` percentage;
- configured swap size and swap-use percentage;
- root filesystem usage;
- maximum container memory use as a percentage of its cgroup limit;
- container restart count and OOM state;
- missing/unhealthy long-running services;
- any service that unexpectedly has no memory limit;
- stale running Consultation executions when lease columns are available;
- deploy-watch timer/result;
- off-host PostgreSQL DR status when DR is enabled.

Exit contract:

- `0 = PASS`;
- `1 = WARN` and is accepted by the systemd oneshot so warnings remain observable without marking the timer failed;
- `2 = CRITICAL` and leaves the systemd unit failed/visible in journal/systemctl.

Default thresholds:

| Signal | WARN | CRITICAL |
| --- | ---: | ---: |
| host available RAM | <=15% | <=8% |
| swap used | >=25% | >=60% |
| root disk used | >=80% | >=90% |
| container memory / limit | >=80% | >=92% |
| restart count | >0 | — |
| OOMKilled | — | >0 |
| unhealthy/missing service | — | >0 |
| stale running Run | >0 | — |
| unbounded container | >0 | — |
| deploy timer inactive | — | yes |
| enabled DR status failed | — | yes |

The status timer runs every six hours with jitter.

## Conservative cleanup

The daily cleanup acquires the same deploy lock used by the production deployment transaction. If deployment is active, cleanup exits without mutation.

Eligible cleanup:

- local pre-deploy dumps older than the configured retention window;
- old runtime-bundle backup directories;
- stopped containers older than the Docker retention filter;
- dangling images only;
- old builder cache.

Explicitly forbidden:

- `docker image prune -a`;
- `docker volume prune`;
- removal of active/tagged release images;
- removal of PostgreSQL/Redis/upload/Caddy persistent volumes;
- remote OSS DR retention (owned by the bucket lifecycle policy).

## Installation / deployment behavior

Runtime bundles carry the capacity scripts and systemd units. During a coherent production deployment:

1. runtime bundle is validated and synchronized;
2. `install-production-capacity.sh --swap-only` ensures emergency swap exists;
3. normal DR/deploy gates run;
4. memory-capped Compose services are restarted on the coherent revision;
5. external health succeeds;
6. only then are capacity status/cleanup timers installed and enabled.

A failed application deployment does not activate a new set of capacity timers.

## Production verification after first cutover

Run on Alibaba after the release is active:

```bash
free -h
swapon --show
sysctl vm.swappiness
systemctl status bodysense-capacity-status.timer bodysense-capacity-cleanup.timer --no-pager
/opt/bodysense/scripts/production-capacity-status.sh status || true

docker inspect docker-litellm-gateway-1 --format '{{.HostConfig.Memory}} {{.HostConfig.MemoryReservation}}'
docker inspect docker-ai-service-1 --format '{{.HostConfig.Memory}} {{.HostConfig.MemoryReservation}}'
docker stats --no-stream
```

Acceptance requires the seven long-running services to expose non-zero cgroup memory limits, swap to meet the configured nominal size within the one-page `mkswap` metadata allowance, no OOM/restart regression, application health to remain green, and `.capacity-state` to be produced.

## Host-upgrade trigger

Do not keep tightening container ceilings to fit the current ECS. Escalate to a larger RAM class when representative observations show any of the following rather than a one-off transient:

- host available RAM remains at/below the 15% warning boundary;
- swap stays above 25% during ordinary workload or repeatedly churns under routine requests;
- LiteLLM or another core service repeatedly exceeds 80% of its configured cgroup limit under expected workload;
- any legitimate production path is OOM-killed under the current envelope;
- lowering ceilings would put a service below a validated production-shaped peak;
- normal deployment, restore drill, ingestion, or evaluation activity cannot run without sustained memory pressure.

A single short spike is evidence to inspect, not automatically a reason to resize. A repeated pressure pattern is capacity evidence.

## Validation

Repository gate:

```bash
bash scripts/validate-production-capacity.sh
bash scripts/validate-repo.sh
```

Production-shaped gate:

```bash
bash scripts/local-deploy-validate.sh
```
