# ADR 0002: Fork release and update independence

Date: 2026-10-13 · Status: accepted

## Context

Upstream release tooling pushed artifacts to dl.min.io, minisigned with
MinIO's key, and CI depended on MinIO-corp runners (the `mint` pool),
secrets (BOT_PAT) and images. A community fork cannot ship through that
path, and `git describe` reported upstream versions because the fork
inherited 523 upstream tags.

## Decision

- Releases are **GitHub Releases on tag push** (`.github/workflows/release.yml`):
  a 15-target cross-compile matrix with sha256 checksums, plus multi-arch
  container images pushed to GHCR (`ghcr.io/fly88oj/minoss`).
- Release Dockerfiles are **self-built multi-stage images**; nothing is
  downloaded from dl.min.io, and the MinIO minisign pubkey is gone.
- The 523 inherited upstream tags were deleted from the fork; fork tags
  keep the `RELEASE.YYYY-MM-DDTHH-MM-SSZ` shape so the ecosystem (mc,
  dashboards) keeps parsing versions.
- The committed private test CA (`root.key`/`root.cert` in
  `.github/workflows/`) was removed; nothing referenced it.
- Test scripts build `mc` from source (pinned release tag) instead of
  downloading from dl.min.io.
- The `upstream` remote is configured locally for cherry-picking security
  fixes while upstream still publishes any.

## Consequences

- Upgrades arrive via package managers / release artifacts, never via an
  in-process self-update (see ADR 0001).
- CI runs entirely on stock GitHub-hosted runners.
