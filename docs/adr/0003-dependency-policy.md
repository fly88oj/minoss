# ADR 0003: Dependency policy and stdlib preference

Date: 2026-10-13 · Status: accepted

## Context

~95 direct dependencies at fork time; seven of them duplicated the
standard library or platform features one-to-one.

## Decision

- Prefer the standard library. Replaced at fork time: json-iterator
  (stdlib-compatible mode everywhere), buger/jsonparser (JWT claims),
  mitchellh/go-homedir, pkg/errors (only errors.New sites remained),
  go.uber.org/atomic (typed sync/atomic + small Duration shim),
  mousetrap (Explorer.exe guard), yaml.v2 (v3 already in graph).
- New direct dependencies require justification in the PR; new `minio/*`
  pins must be immutable (tag or SHA pseudo-version is fine *because
  go.sum pins the content*; branch-following pins are not).
- Deliberate exceptions kept for performance: klauspost/compress family,
  minio/simd family via s3select (now removed), internal ringbuffer/bpool.

## Consequences

- go.mod direct count dropped by 13 at fork time; future additions are
  reviewable against this policy.
