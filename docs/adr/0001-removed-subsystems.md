# ADR 0001: Removed subsystems and the bar for bringing anything back

Date: 2026-10-13 · Status: accepted

## Context

The fork starts from the final community snapshot of upstream MinIO. The
baseline audit (`docs/fork/2026-10-13-quality-baseline-and-plan.md`) found
~24k lines of code pinning ~45 dependencies for features whose upstream
ecosystems are dead or commercial.

## Decision

Removed outright (breaking changes, each in its own commit):

| Subsystem | Why | Commit |
|---|---|---|
| In-place self-update (`minio update`, ServerUpdate admin API) | hardcoded to dl.min.io — would overwrite a minoss install with upstream MinIO | a907f19fe |
| S3 Select (`internal/s3select`) | AWS deprecated the service; new buckets can't use it since 2024; private SQL engine + 4 exclusive deps | 42ee30e5a |
| Object Lambda | AWS retired S3 Object Lambda in 2024 | 86cada680 |
| Prometheus metrics v1 + v2 endpoints | three exposition generations coexisted; v3 is complete and remains | 86cada680 |
| SUBNET registration + callhome | phones home to subnet.min.io, MinIO's commercial portal | d56daada8 |
| NSQ + NATS Streaming (STAN) event targets | NSQ ecosystem unmaintained since ~2021; STAN EOL'd (core NATS + JetStream stay) | fd292ed3a |

Glacier restore requests with `type=SELECT` are rejected with a clear
error; standard restore is unchanged. `jstream` (generic JSON streaming
parser) was kept and moved to `internal/jstream`.

## Consequences

- Monitoring setups must scrape `/minio/metrics/v3/*` (mc: `mc admin
  prometheus` generates the current config).
- Config referencing removed notify subsystems is ignored on load.
- Reintroducing any of these requires a new ADR overturning this one.
