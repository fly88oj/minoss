# Architecture views: the `cmd/` → `internal/` extraction

Generated 2026-10-13 from [`extraction.model.json`](extraction.model.json) by
[`generate.py`](generate.py). The JSON model is the source of truth; the `.dot`
files and [`cmd-internal-extraction.drawio`](cmd-internal-extraction.drawio) are
renderings of it.

Audience: whoever picks up the fork improvement program (see
[../backlog.md](../backlog.md)). Scope: **current state** plus explicitly
labelled target/proposed nodes. Level: L3 components/modules and L4 symbols.

Reading order: `extraction-boundary` first (which subsystems have left `cmd`,
which have not), then `iam-seam-blockers` (what still holds IAM inside `cmd`).

## How the numbers were obtained

| Claim | Method |
|---|---|
| `cmd` = 286 non-test files, 175,519 lines (245,108 with tests) | `ls cmd/*.go \| grep -vc _test`; `wc -l` over non-test vs all |
| `cmd` imports 59 `internal/*` packages; **no `internal/*` imports `cmd`** | `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./cmd ./internal/...` |
| IAM move scope = 7,043 lines in four files | `wc -l cmd/iam*.go` (2,559 + 3,071 + 912 + 501) |
| `IAMStorageAPI` = 27 methods | read the interface at `cmd/iam-store.go:591` |
| IAM reads ~75+ `cmd` symbols (a floor: the compiler stops early) | compile probe: the four files copied into a scratch package inside this module and compiled; the undefined-symbol list **is** the seam |
| 31 files use `globalIAMSys`; 29 reach through the unexported `.store` | `grep -l globalIAMSys cmd/*.go`, `grep -c "globalIAMSys\.store\." cmd/site-replication.go cmd/admin-handlers-users.go` (23 + 6) |
| IAM ↔ site-replication is bidirectional | `cmd/iam.go:2072,2168` (`IAMChangeHook`), `cmd/iam-store.go:3039` (`isEnabled`) |
| site replication ≈ 8,980 lines incl. codecs | `wc -l cmd/site-replication*.go` (excluding `_test`) |
| 21 metrics v3 collector paths | `internal/metrics/v3.go:36-63` |

Confidence follows the plugin's evidence model: **high** = measured in this
checkout (every node/edge above is), **medium** = several partial signals and no
direct confirmation (the `siterep → internal/iam` ordering edge, since it is a
design consequence rather than an existing import).

## What the two views say

**Extraction boundary.** Three edges answer the question. `internal/metrics`
shows the pattern works: `metrics.Deps` is ~30 narrow injected hooks plus value
fields, wired once in `cmd/metrics-v3.go`, and the package holds no `cmd`
globals. IAM is partway: `internal/auth` now owns the JWT claim names and
`internal/iam` now owns the 18 error sentinels, but all four implementation
files remain in `cmd`. The three red edges are the blockers:
1. IAM reads ~75+ symbols out of `cmd` (compile-probe floor).
2. `cmd` reads IAM back through `globalIAMSys.store`, an **unexported** field,
   from 29 sites — which alone makes the package move impossible.
3. `ObjectLayer` cannot be implemented outside `cmd` at all: `Walk` takes
   `chan<- itemOrErr[ObjectInfo]`, and both element types are unexported. A
   consumer needs a narrower seam, not the 44-method interface.

**IAM seam blockers.** The same surface sorted by disposition: what moves with
the package, what must be **injected** (`Deps` hooks), what must stay a *shared
identity* rather than being redeclared (`errConfigNotFound`,
`errServerNotInitialized`, `errInvalidArgument` — `cmd/api-errors.go` switches
on their identity, so a second definition silently converts 4xx into 500), what
the standard library can just replace, and the two nodes that need a new seam
rather than an injection.

## Known limitations

- The `~75+ cmd symbols` count is what the compiler reported before it stopped,
  so it is a floor; the backlog records the inventory the two probes produced.
- Line counts are of the current tree, not of net movement after the extraction.
- `siterep → internal/iam` is drawn as ordering intent, not an existing edge.
- Top-level package names only; nothing here claims runtime topology or request
  paths. For the metrics scrape paths that matter operationally, the Helm chart
  still points at removed v2 endpoints (recorded in `../backlog.md`).
- No confidence below **medium** appears in either view; anything that was a
  guess went into the backlog as an open question instead.
