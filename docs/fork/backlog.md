# minoss maintenance backlog

Ordered follow-up work from the 2026-10-13 baseline (see
`2026-10-13-quality-baseline-and-plan.md`). Each item is scoped so it can
land as one reviewable change with tests green throughout.

## P2 continuation: cmd/ → internal/ extraction

Extraction order (lowest risk first). Each extraction: move the subsystem
into `internal/<name>`, replace its global reads with constructor-injected
dependencies, delete the globals, add parallel tests.

1. ~~**metrics-v3 → `internal/metrics`**~~ — **DONE (2026-10-13)**: all 22
   files moved; `metrics.Deps` (30 narrow hooks + value fields) replaces the
   20 global reads and all cmd-type dependencies; cmd wiring lives in
   `cmd/metrics-v3.go`; 8 parallel `t.Parallel()` tests in
   `internal/metrics/metrics_test.go`. Pattern to follow for the next
   extractions.
2. **IAM → `internal/iam`** — in progress, resumed here rather than finished
   in one pass. Move scope: `cmd/iam.go` (2,559), `cmd/iam-store.go` (3,071),
   `cmd/iam-object-store.go` (912), `cmd/iam-etcd-store.go` (501) = 7,043 lines.
   Copying the four files into a package probe yields **~75 undefined symbols**,
   i.e. the surface IAM reaches into `cmd` for; 303 top-level declarations of
   which **85 are referenced from outside** (31 files touch `globalIAMSys`).
   The symbol count is what the compiler reported before it stopped, so treat it
   as a floor, not a census.

   - **Done**: the JWT claim keys (now `internal/auth/claims.go`, which also
     collapsed a duplicate `"sa-policy"` declared in both `cmd/utils.go`'s
     `iamPolicyClaimNameSA()` and `internal/auth/credentials.go`) and the 18
     IAM-owned error sentinels (`69ca2d5f0`, now `internal/iam/errors.go`,
     referenced by identity — no alias vars). Those two removed the highest-
     frequency reads (`errNoSuchPolicy` 43, `errNoSuchUser` 30,
     `errNoSuchGroup` 19, `errIAMActionNotAllowed` 15).
   - **Mapping coverage of the sentinels** (measured; do not re-guess): 9 map
     through `toAPIErrorCode` and are pinned by `cmd/api-errors_test.go`; 8 map
     through `toAdminAPIErr` and are pinned by `cmd/admin-handler-utils_test.go`;
     1 (`ErrNoSuchAccount`) is deliberately unmapped. Three originally had **no
     code at all** — `ErrNoSuchTempAccount`, `ErrNoSuchAccount`,
     `ErrGroupDisabled` — so a client hitting them got a generic 500. Resolved: `ErrGroupDisabled`
     now maps to `ErrAdminGroupDisabled` (a code that was already defined and
     simply never wired up) and `ErrNoSuchTempAccount` to
     `ErrAdminNoSuchAccessKey` (the convention the account-info handler already
     applied by hand). `ErrNoSuchAccount` deliberately stays unmapped - it is
     translated inside `IAMSys.getAccountWithClaims`' two callers and cannot
     escape the subsystem, so a code would be dead weight; when the logic moves
     into `internal/iam` it can likely become unexported.
   - **Hard blockers, in order.** (a) `globalIAMSys.store` is an unexported
     field reached from **29 cmd sites** (site-replication.go 23,
     admin-handlers-users.go 6) calling `loadMappedPolicies`/`loadUsers`/
     `GetPolicyDoc`/`GetMappedPolicy`/`listPolicyDocs`. Either promote narrow
     pass-through methods onto `IAMSys` (preferred — it is the facade already)
     or export the store; leaving it unexported makes the package move
     impossible. (b) **The object dependency is deeper than "use 5 methods of
     `ObjectLayer`".** The four files do not reach the object layer directly for
     config items: they call the cmd-resident helpers `saveConfig`,
     `readConfigWithMetadata`, `deleteConfig` and `checkConfig`
     (`cmd/config-common.go:32-100`), which take the **unexported `objectIO`**
     interface (`cmd/data-usage-cache.go:871`) and build `ObjectOptions`,
     `NewPutObjReader` and `hash.NewReader` internally. So the seam is a
     `ConfigObjectStore` abstraction (get/put/delete/stat/list) implemented by
     an adapter in `cmd` over `objectIO` — and 10/3/4/1 non-IAM files share those
     same helpers, so they cannot move with IAM either. Separately,
     `ObjectLayer.Walk` cannot be satisfied outside `cmd` at all: it takes
     `chan<- itemOrErr[ObjectInfo]`, both unexported.
     `iamPolicyClaimNameOpenID` (`cmd/utils.go:885`) belongs in this group too —
     it reads `globalIAMSys.OpenIDConfig`, so it becomes a method on `IAMSys`
     rather than a hook. (c) `errConfigNotFound`, `errServerNotInitialized`,
     `errInvalidArgument` are shared with 19/20/34 non-IAM `cmd` files: inject as
     `Deps` **fields**, never redeclare — `cmd/api-errors.go` switches on their
     identity and a second value silently degrades 4xx to 500.
   - **Deps surface** for the rest: ~25 injected hooks (`NotifyIAM` fan-out
     adapter, `SiteReplicateIAM`/`SiteReplicationEnabled` — the site-replication
     callbacks, `ActiveCred`, `KMS`, `RootCAs`, `ServerConfigKV`, `SiteRegion`,
     `TokenSigningKey`, `ExtractClaims`, `AuthN/AuthZPlugin` getters+installers,
     `TraceBootstrap`, `NewTransport`/`ConnSettings`/`UserAgent`,
     `StatConfigItem`, `IsRetriableInitErr`) + value fields (`Ctx`,
     `MetaBucket`, `ConfigPrefix`, `IsDistErasure`, `SiteReplicatorAccessKey`,
     `ErrConfigNotFound`…).
   - **Move with the package** (IAM-owned, no cmd readers): `iamLogIf`/
     `iamLogOnceIf`/`iamLogEvent` (2 cmd readers, export), the etcd key helpers
     `readKeyEtcd`/`saveKeyEtcd`/`deleteKeyEtcd` (note `defaultContextTimeout`
     is *declared* in iam-etcd-store.go and also read by `cmd/etcd.go`, `cmd/healthcheck-handler.go`
     and `cmd/iam.go:541` — leave a cmd copy), the storage-layout constants
     (`iamConfigPrefix` and siblings, referenced only from the four files).
   - **Stdlib**: `UTCNow`→`time.Now().UTC()`, `SlashSeparator`→`"/"`.
     `pathJoin` is **not** `path.Join` — it retains a trailing separator and
     turns a trailing empty element into `x/`; it is safe to swap only where the
     last element is a constant filename (the 7 `iam-store.go` call sites are).
     `pathClean` is `path.Clean` that maps `"."`→`""`: needs a local copy.
   - **Interface split**: `IAMStorageAPI` is 27 methods whose `lock()/rlock()`
     return the store's `iamCache` — the cache protocol leaks. Split by
     capability: locker, config read/write, doc load/save, entity load/save,
     and keep `iamStorageWatcher` separate.
   - **Lock reduction** (perf plan L2): reads take 2-4 `rlock()`s per request
     through `IAMStoreSys`. `cmd/iam-store.go` makes 21 `rlock()` and 27
     `lock()` calls (22 `rlock()` across all four move-scope files) — count
     them, do not trust a line-number citation, they drift with every commit. A
     snapshot/atomic-read pattern is the candidate, same shape as the
     `globalAPIConfig` request-pool fix in `84650d52c`.
   - **Test safety net first**: direct IAM coverage is thin (`iam-object-store_test.go`
     53 lines, `iam-etcd-store_test.go` 40; the behavioral `TestSuiteIAM` needs
     live LDAP/OpenID). Concrete gaps measured this session, all of which make a
     mistake in this area silent:
     * no cmd test ever sends SSE-C headers to a multipart part upload (0 hits),
       so both `PutObjectPartHandler` and `CopyObjectPartHandler` have an
       unexercised `isEncrypted` branch — which is exactly where the SSE-C
       compatibility regression of `5dd147251` lived;
     * no test sends `x-minio-source-replication-request`, and none asserts
       `crypto.ErrIncompatibleEncryptionMethod` reaches a client;
     * `cmd/user-provider-utils.go` and `cmd/ftp-server-driver.go` have no test
       file at all.
     Pinned since: the sentinels' API-code mapping (two test files) and the
     claim wire values (`internal/auth/claims_test.go`). Write seam-level
     behavior tests (policy resolution, service-account expiry, group
     membership) **before** moving logic — this is
     authorization code and the move must not be a big bang.
3. **site-replication → `internal/siterep`** — the biggest single win for
   `cmd/` shrinkage. Re-measured (do not trust the prior "6,284 / 122 / 26"
   line — it was one file, not the subsystem): `cmd/site-replication.go` is
   6,284 lines / 122 funcs, of which 79 are methods on `*SiteReplicationSys`
   (struct at cmd/site-replication.go:200); the full move scope (that file +
   `-metrics.go` + `-metrics_gen.go` + `-utils.go` + `-utils_gen.go`) is
   8,980 lines / 180 funcs (`wc -l`, `grep -cE '^func '` on the five non-test files). `globalSiteReplicationSys` is referenced from 26
   cmd files (cmd/globals.go:317 is the decl).

   - **Deps surface** (globals read in move scope; classify per
     `internal/metrics/deps.go`):
     - **Injected hooks** — `globalIAMSys` (69 reads), `globalNotificationSys`
       (6: ReloadSiteReplicationConfig :315, GetClusterSiteMetrics :6073),
       `globalBucketMetadataSys` (47: `Update` x22, `Delete` x6,
       `GetReplicationConfig`), `globalBucketTargetSys` (26: List/SetTarget/
       isOffline), `globalReplicationPool` (6: resyncer start/status/cancel
       :5841-5958), `globalDeploymentID` (45; a `func() string` — keep a hook
       or boot value), `globalLeaderLock.GetLock` (:4258), `globalBucketObjectLockSys`.
     - **Value fields** — `globalActiveCred` auth.Credentials (:3896),
       `globalSite` config.Site (:680), `globalBootTime`,
       `globalRemoteTargetTransport` http.RoundTripper, `globalDNSConfig`
       dns.Store (Put/Delete :1171-1179 — already an internal type), `Ctx`
       (`GlobalContext` x6), `LogIf` (`replLogIf` x21, mirrors metrics).
     - **Move with the package** (self-owned, not deps) — `SiteReplicationSys`
       + `srState`, `SRError`, `getAdminClient` (:2608, self-contained helper),
       `siteReplicatorCred` (`globalSiteReplicatorCred` x4), `siteResyncMetrics`
       (`globalSiteResyncMetrics` x5) + both `_gen.go` codecs.
     - The 10 `globalSiteReplicationSys.*` self-refs (:716, :729, :3362,
       utils.go) collapse onto the receiver.
   - **ObjectLayer seam** — `newObjectLayerFn()` x11 (e.g. listBuckets :1852,
     syncToAllPeers :1864) plus the `Init(ctx, objAPI)` param: define a narrow
     interface (ListBuckets / GetBucketInfo / config read+write used by
     loadFromDisk/saveToDisk). `globalObjAPI` is NOT read directly.
   - **Exported boundary for cmd** — 34 exported methods are called in
     (`…Sys.[A-Z]` distinct: Init, isEnabled, IAMChangeHook x21,
     BucketMetaHook x10, GetClusterInfo, the 20 `Peer*Handler`s,
     AddPeerClusters, SiteReplicationStatus…); rewrite count = **97 consumer
     call sites across 23 files** (26 minus globals.go, site-replication.go,
     site-replication-utils.go). Shared value types that must stay exported:
     `SRStatus`, `SRError`, `SRStats`, `SiteResyncStatus`; wire types are
     already `madmin.*`.
   - **Import cycle: IAM must land first (item 2).** Coupling is bidirectional
     inside cmd today: site-rep → `globalIAMSys` x69 including IAM *internals*
     (`.store.loadMappedPolicies`/`loadUsers`/`GetUser`/`listPolicyDocs`,
     `.LDAPConfig.*`, `.OpenIDConfig.*`) and IAM → site-rep
     (iam.go:2072,2168 `IAMChangeHook`; iam-store.go:3039 `isEnabled()` - line numbers
cite the symbol, they drift with every commit).
     Two packages make that a real cycle. Seam: extract IAM first with a
     narrow exported interface + a `Replicator` callback iface that cmd wires
     at bootstrap (invert the hook, do not import siterep from iam). **Open
     question:** whether `internal/iam` exposes the `.store.*`/LDAP/OpenID
     surface or siterep needs a facade over it.
   - **Peer / grid RPC** — measured **0** direct `internode.*` / `grid.*`
     calls in move scope. Peers are reached over REST via `madmin.AdminClient`
     built in `getAdminClient` (protocol-boundary, moves with package) and via
     `globalNotificationSys` (internode fan-out — an injected iface). This
     extraction does NOT touch the deferred REST→grid migration; keep it out.
   - **Slicing (additive; keep the 12 marshal + 1 unit test green each
     commit):**
     1. Pure leaf logic (no globals): `getMissingSiteNames`, peer-set diffing,
        policy/IDP name mapping → `internal/siterep` + table tests.
     2. Metrics + resync layer: move `-metrics.go`/`-metrics_gen.go`/
        `-utils.go`/`-utils_gen.go` (2,695 lines, 12 codec tests carried);
        `siteReplicatorCred`/`siteResyncMetrics` become package state,
        `isEnabled`/`GetClusterInfo` injected as predicates.
     3. Peer-client boundary: `getAdminClient` + admin-client orchestration.
     4. Core `SiteReplicationSys` + `Deps` wiring in a new `cmd/siterep-deps.go`
        (mirror cmd/metrics-v3.go) + full method move; cmd keeps a package
        instance.
     5. Delete `globalSiteReplicationSys` from cmd/globals.go, rewrite the 97
        consumer call sites.
   - **Testing** — today: one behavioral test, `TestGetMissingSiteNames`
     (cmd/site-replication_test.go), + 12 msgpack round-trip tests in
     `-metrics_gen_test.go`/`-utils_gen_test.go`. No `TestSiteReplication*`
     exists; the 6,284-line core is ~0 covered (needs live cluster/mock IAM).
     Deps injection makes newly unit-testable: AddPeerClusters state diffing,
     IAMChangeHook/BucketMetaHook routing, resync scheduling, heal routine —
     all drivable with fake hooks.
4. ~~**Handler dedup round 2 (multipart family)**~~ — **DONE (2026-10-13,
   commit 5dd147251)**: the shared shape turned out to be the crypto/identity
   preamble rather than crypto-MD5/mod-tags. Extracted
   `multipartPartParams` (uploadId/partNumber), `multipartPartKey` (SSE-C parse
   + multipart-key unwrap), `encryptMultipartPart` (part-key derivation +
   upload/part derived nonce) and `encryptedSizeOrUnknown` (which also collapsed
   the same 4-line block in `PutObjectHandler`, `PutObjectExtractHandler` and the
   POST-form handler). The two handlers keep their own precondition order.
   Review follow-up (regression introduced in `5dd147251`, fixed in `f15faba46`):
   moving the SSE-C key
   parse inside PutObjectPart's source-replication guard dropped the
   S3+SSE-C incompatibility check on the non-re-encrypting path, because
   `putOptsFromReq` validates key material through `crypto.SSEC.ParseHTTP`, which
   does not do that compatibility test - `ParseSSECustomerRequest` does. The
   check is now explicit before the guard. **Test gap**: no test drives
   PutObjectPart with a source-replication request plus both SSE headers, so the
   regression and its fix are verified by reading, not by execution.
5. **ObjectLayer capability split (opportunistic)** — 44-method interface,
   1 production implementation. When a new operation forces an edit, split
   by consumer capability (`ObjectReader`/`ObjectWriter`/...); do not do a
   big-bang split. Note for the IAM extraction: `ObjectLayer.Walk` takes
   `chan<- itemOrErr[ObjectInfo]`, an unexported cmd type, so the interface is
   literally unsatisfiable outside `cmd` — a consumer needs a narrower seam,
   not `ObjectLayer`.
6. **Bucket/Object domain types** — introduce in newly extracted packages
   first; no repo-wide sweep (130 bare `bucket, object string` signatures).

## Deferred with reasons (decisions recorded in docs/adr/)

- **peer-rest → grid migration completion** (~2.4k lines): finishing the
  upstream in-flight internode-RPC migration requires protocol-level
  testing across mixed-version clusters; not safe as a drive-by.
- **sio-go / minio-sio consolidation**: dual authenticated-stream-crypto
  modules exist because stored config encryption must stay decryptable;
  needs a migration path (decrypt-with-old, re-encrypt-with-new) before the
  old module can go.
- ~~**Console keep-or-cut**~~ — **DECIDED: KEEP (2026-10-14, user)**. The web
  UI is the lowest-friction management surface for non-CLI users (mc CLI,
  S3 API and madmin-go remain the other three layers). The ~45-module
  dependency tree is the accepted cost; revisit only if a maintenance
  burden (e.g. another minio/console freeze) forces it.
- **Module path** stays `github.com/minio/minio` for now — renaming touches
  every import and buys little; revisit only if we need to publish the
  module for direct import.

## Toolchain and lint pins

- **Fixed (2026-10-13)**: `Makefile` built the lint cache path from
  `$(GOLANGCI_VERSION)` but never defined it, and `getdeps` passed no version to
  install.sh - so "pinned" meant "whatever is newest today". It is now
  `GOLANGCI_VERSION ?= v2.14.0`, passed to the installer. Adopting the version CI
  was already fetching produced **20 findings** that the fork's local v2.6.2 had
  reported as zero, so every historical "0 issues" claim was made against a tool
  the pipeline does not run. The first pass found and fixed only 19, because it
  linted `./cmd/... ./internal/...` while `make lint` lints the **whole module** -
  the 20th was the same redundant-paren pattern in
  `docs/debugging/xl-meta/main.go`, which kept `make test` red until it was
  caught. All 20 are now fixed (redundant-paren formatting under the newer
  gofumpt - none in `*_gen.go`, so `check-gen` is unaffected;
  `WriteString(fmt.Sprintf(...))` -> `fmt.Fprintf`; three dead statements in
  `os-readdir_test.go` that sat between an append "for cleanup" and a `t.Fatalf`,
  which never returns). Lint verification from now on means
  `golangci-lint run --timeout=10m --config ./.golangci.yml` with no package
  list, exactly as the Makefile invokes it. gomodguard -> gomodguard_v2;
  CI Go matrices 1.24.x ->
  1.26.x to match `go.mod`; `go mod tidy -compat=1.21` lost the flag (verified a
  no-op at go 1.26).
- **Note for whoever bumps it again**: in v2.14.0, `golangci-lint fmt` and the
  gofumpt check inside `golangci-lint run` disagree on a multi-line composite
  literal returned directly (`internal/ioutil/wait_pipe.go`), so the file
  ping-ponged between two forms. The construct was rewritten to sidestep it; if
  it recurs elsewhere, prefer changing the code over toggling one formatter off.
- **CI actions that float are a supply-chain issue, not a style one.** Two
  workflows pinned third-party actions to `@master` (`crate-ci/typos`,
  `luizm/action-sh-checker`), meaning every run executed mutable code from an
  external repository - the opposite of ADR 0003's immutable-pin rule. Both are
  now pinned to release tags whose `action.yml` was fetched and checked
  (typos v1.51.1, sh-checker v0.10.0). Note `.typos.toml` excludes `docs/`, so
  the docs rewrite in this fork is not spell-checked by CI.
- **Still open**: `actions/checkout@v4` / `actions/setup-go@v5` are behind
  (v7 exists for both, plus `azure/setup-helm@v4` and artifact actions). Bumping
  14 workflow files cannot be validated from a sandbox without a runner, so it is
  deliberate that they were left alone rather than blind-edited. Prefer
  `go-version-file: go.mod` (as `release.yml` already does) over the per-matrix
  literals when touching these files, so the drift cannot recur.
- `make getdeps` needs raw.githubusercontent.com; some sandboxes return content
  whose checksum does not verify (it does here via curl, but the installer's own
  hash check failed), which makes `make test` unrunnable end to end. Run its two
  halves separately: `go build ./...`, then
  `MINIO_API_REQUESTS_MAX=10000 CGO_ENABLED=0 go test -count=1 ./...`.

## Upstream tag cleanup

Done on the remote (2026-10-13): `git push origin --delete` removed all 523
inherited tags, after confirming the deletion set was exactly
`origin ∩ upstream` — origin had **zero** fork-created tags, so nothing of ours
was at risk (method, re-runnable: `git ls-remote --tags origin` and
`git ls-remote --tags upstream`, drop the `^{}` peel refs, sort, `comm` in both
directions - 523 common, 0 unique to origin). `git ls-remote --tags origin` now
returns nothing, so the situation ADR 0002 was written against is gone: a fresh
clone can no longer `git describe` its way to an upstream release name - it
fails outright with "No names found", which is exactly why the `VERSION`
fallback below matters. The fork's own `RELEASE.2026-10-13T04-04-59Z` exists
locally only and has never been pushed, so no release has been published; do not
push that stale tag just to fire `release.yml` over unreviewed code.

Two consequences of an empty remote tag set, both handled:

- `Makefile` derived `VERSION` from `git describe --tags`, which fails with no
  tags at all and produced the image reference `ghcr.io/fly88oj/minio:`. It now
  falls back to the commit date in the fork's `RELEASE.<timestamp>Z` shape.
- Tests that called `rand.Seed` for determinism stopped being deterministic when
  the toolchain moved past go1.24, where Seed is a no-op; the full suite then
  failed intermittently on `TestDynamicTimeoutAdjustExponential` depending on
  test order. Fixed by giving those tests their own `rand.New(rand.NewSource(...))`.

Recovery if the tags are ever wanted again: they still exist on the `upstream`
remote (minio/minio) and a name+SHA list was kept at
`/tmp/minoss-origin-tags-backup.txt` while this session lasted.

## Verification limits worth knowing

- **`govulncheck` results depend on the toolchain that builds the code.** With
  this repo's declared `toolchain go1.26.9` it reports 0 vulnerabilities
  affecting our code; with the machine's own `go1.26.8` (`GOTOOLCHAIN=local`,
  which is this sandbox's default) it reports **13**, all in the standard
  library. So "0 reachable vulns" claims - including the one in
  `2026-10-13-performance-plan.md` - are only true for builds that actually
  use >= go1.26.9. CI does (`go-version: 1.26.x` + the toolchain directive);
  a developer who pins `GOTOOLCHAIN=local` to an older Go ships them.
- **`docs/debugging/{inspect,pprofgoparser,reorder-disks,s3-verify,xattr}` are
  separate Go modules**, so the root `go build ./...`, `go test ./...` and
  golangci-lint skip them entirely (only `xl-meta` is in the root module).
  Covered as of `1d1dc6662`: `make submodules` vets/builds/tests each one, is a
  prerequisite of `verifiers`, and has its own `go.yml` job. None of them carry
  `go:generate` directives, so `check-gen` has nothing to add there.
- **`check-gen` now catches stringer drift too** (`bcd2c4fb9`). It watched only
  `_gen.go` and `go.sum`, which is how 19 `cmd/*_string.go` files plus 5 under
  `internal/` sat stale through two sessions while every "check-gen clean"
  stayed true, and how a toolchain bump could keep rewriting them unnoticed.
- **Most workflows are `on: pull_request`**, including the verifiers
  (`go-lint.yml`, `go.yml`, `shfmt.yml`, `typos.yml`, `vulncheck.yml`), so a
  branch push runs nothing: the first real execution of this fork's lint/test
  gates will be the PR. `helm-lint.yml` now lints *and* renders with monitoring
  enabled (`a71f816f8`) - with default values it created no monitoring objects
  at all, which is why the deleted v2 scrape paths survived so long.

## Monitoring on a real cluster (2026-10-14)

The chart's scrape config was measured against a live Prometheus, not just
against the CRD schemas: an LXC container running k3s `v1.36.5+k3s1`,
prometheus-operator `v0.94.1` with the CRDs from `kube-prometheus-stack 92.3.1`,
and this fork's binary (built in-container with go1.26.9) installed through
`./helm/minio` with `metrics.serviceMonitor.enabled=true`.

What the build actually serves:

- all nine collector paths in `values.yaml` answer `200`; an unknown v3 path
  answers `404`; `/minio/v2/metrics/cluster` answers **403**, not 404 - the route
  is simply not registered any more (`cmd/metrics-router.go` registers only
  `/metrics/v3`) and the request falls through to the reserved-bucket handler.
- `cluster/config` and `cluster/usage/objects` answered `200` with an **empty
  body** on a fresh server, and in a two-node distributed setup `cluster/health`
  reported only `minio_cluster_health_nodes_online_count` (the capacity metrics
  were absent). A scrape succeeding and yielding nothing is normal here, so
  "collector returns no series" is not evidence of a broken scrape - `up` is the
  signal.

The defect this found, and the fix:

- Prometheus creates one scrape pool **per endpoint**, and nothing in the chart
  gave those pools distinct identities. With the v3 defaults (4 node paths per
  pod + 5 cluster Probes) the 13 targets collapsed onto **3** distinct label
  sets, so all four collector pools of a pod wrote the same `up` /
  `scrape_duration_seconds` series. Pointing one Probe at a dead path made
  `up{instance="minio.minio"}` read `0` while `max_over_time` stayed `1` and
  `min_over_time` was `0` - the cluster scrape appeared to flicker healthy and
  broken at the same time, and there was no way to tell which collector failed.
  Rendering `404`-clean YAML and CRD-validating it could never have shown this.
- Fixed by labelling every scrape with the collector it reads: a
  `targetLabel: collector` relabeling on each ServiceMonitor endpoint (merged
  *before* any `relabelConfigs.relabelings` the operator supplies, so it cannot
  be replaced by accident) and `targets.staticConfig.labels.collector` on each
  Probe. After the upgrade: 13 targets, 13 distinct label sets, and breaking
  `cluster-erasure-set` moved exactly that one series to `0`.
- `helm-lint.yml` now asserts one collector label per endpoint and per Probe,
  whatever the path lists are (verified to fail on a render with the labels
  stripped, so the guard has teeth).

Decision, reversing the earlier plan:

- **keep the `Probe` objects** for `clusterPaths`. The proposed
  ServiceMonitor-only alternative - fold the cluster paths into the
  ServiceMonitor and relabel down to `<fullname>-0` - was rejected by
  measurement: the Probe's single static target is what gives cluster metrics a
  topology-free identity (`instance=<fullname>.<namespace>`), and because the
  scrape goes through the ClusterIP it keeps working when any one pod dies,
  whereas keeping pod-0 would stop all cluster collection when pod-0 is down.
- The caveat to remember when touching this template: MinIO is **not** an
  external prober. The generated URL is `…?target=<static>`, and the server
  ignores `target`, which is exactly why each Probe must carry exactly one
  static entry. A future change that lists several static targets would
  silently produce N identical series with different `instance` labels.

The `public: false` path, verified in the failing direction:

- rendering it was already covered by the CI matrix; on the live cluster it was
  checked end to end. With `public=false` the chart stops setting
  `MINIO_PROMETHEUS_AUTH_TYPE=public`, the token Secret is wired into both
  object kinds (`bearerTokenSecret: {name: prom-token, key: token}`), and a
  scrape with a bogus token gets **403 on all 13 targets** - the mode fails
  closed rather than quietly collecting nothing. Restoring `public=true` returned
  all 13 to `up`. What is *not* verified is a genuine `mc admin prometheus
  generate` token, because `mc` cannot be pulled in this sandbox (see below).
- A `minio` binary has no `admin` subcommand (`minio admin …` is an `mc`
  command), so minting that token needs the client image.

Chart behavior inherited from upstream, now measured:

- `helm install ./helm/minio` with **no** values always creates the post-job:
  `values.yaml` ships a live default user (`users: [{accessKey: console,
  secretKey: console123}]`), and the post-job gate
  (`if or .Values.buckets .Values.users .Values.policies .Values.customCommands
  .Values.svcaccts`) is therefore true out of the box. That job runs the `mc`
  image, so in any environment where `mc` is not pullable the release reports
  `UPGRADE/INSTALL FAILED - post-upgrade hooks failed` even though the server,
  the ServiceMonitor and the Probes are all healthy (that is exactly how the
  runs above ended). Two separate things to decide, not one:
  (a) the default `console`/`console123` credentials are upstream's, and are a
  real default-password problem if this fork ships the chart; (b) with no fork
  image published yet, there is no combination of `image`/`mcImage` defaults
  that is both pullable and consistent with the v3-only scrape paths.

Still unverified, and now the blocking question for publishing the chart:

- the chart's default images - `quay.io/minio/minio:RELEASE.2024-12-18T13-15-44Z`
  and `quay.io/minio/mc:RELEASE.2024-11-21T17-21-54Z` - could not be pulled from
  the test cluster (`401 UNAUTHORIZED`), while `quay.io/prometheus/prometheus:latest`
  pulled fine over the same path. So either `quay.io/minio` is no longer
  anonymously pullable or the sandbox egress refuses that namespace; from here
  the two cannot be told apart, and `docker.io` was unreachable too. A fresh
  `helm install ./helm/minio` therefore could not be validated end to end - the
  pod only ran because the test mounted this fork's own binary over a
  busybox-rootfs carrier image.
- resolving it means deciding the image story and the metrics story **together**:
  pointing the chart at upstream OSS images restores a pullable default, but
  those builds serve `/minio/v2/metrics/*` and not the v3 collectors the chart's
  scrape defaults assume, so `appVersion` (still `RELEASE.2024-12-18T13-15-44Z`)
  and `metrics.serviceMonitor.paths` would disagree. Publishing a fork image is
  the only way to make both true.

Environment gotchas worth keeping (they cost real time):

- the e2e harness copies the project while excluding any directory named
  `target`, which silently drops `internal/logger/target` and
  `internal/event/target`; `go build` then tries to resolve them from
  `github.com/minio/minio` and fails. Push those two trees in after provisioning.
- `helm` in a k3s container needs `KUBECONFIG=/etc/rancher/k3s/k3s.yaml`;
  `kubectl` works without it because k3s aliases it, which makes the failure look
  like a malformed `helm upgrade` argument list.
- a privileged LXC can run k3s with `security.privileged` + `security.nesting` +
  `raw.lxc` containing `lxc.apparmor.profile=unconfined`,
  `lxc.mount.auto=proc:rw sys:rw cgroup:rw`, `lxc.cap.drop=`, plus a
  `unix-char` device for `/dev/kmsg` (the kubelet refuses to start without it).
  The host's already-loaded `br_netfilter`/`overlay` modules mean the container's
  failed `modprobe` is harmless.
- `kube-prometheus-stack` cannot be installed here at all: its chart tarball
  lives on GitHub releases and its admission job image on `ghcr.io`, both
  unreachable. The workable shape was CRDs from the chart plus hand-written
  operator/Prometheus manifests pinned to `quay.io` - and pinning
  `prometheus.spec.image` is wrong, because an operator newer than the pinned
  Prometheus writes `storage.tsdb.retention.time`, which the older binary
  refuses to parse.


## Documentation debt

The earlier framing - "46 of 68 `docs/**/*.md` files link out to min.io
enterprise documentation" - was wrong and the inventory command was the reason:
`grep -rl "min.io/docs\|docs.min.io"` also matched `slack.min.io` and the
*community* documentation. Split by what the links actually are:

- **Kept deliberately**: every `docs.min.io/community/minio-object-store/...`
  link (upstream OSS behavior is still the correct reference), AWS/third-party
  docs, license links, and the upstream container/`dl.min.io` install
  instructions - this fork's release pipeline has never run, so pointing
  installers at fork artifacts would be an unverified claim.
- **Done (2026-10-13)**: the `slack.min.io` badges/support pointers (106 `*.md` occurrences at the session
  base; two remain in this file's own text, and one more was hiding where a
  `--include='*.md'` inventory can never see it - the runtime error string
  `config.ErrUnexpectedError` told operators to "contact MinIO at
  https://slack.min.io"; it now points at this repository's issues)
  and the MinIO-corp commercial pointers (`min.io/pricing`, AIStor framing,
  `docs.min.io/enterprise/...`, `subnet.min.io`) were removed or repointed to
  this repository's issue tracker; `CONTRIBUTING.md` and `README.md` now describe
  this fork instead of telling contributors to clone `minio/minio`; documents
  that describe subsystems this fork deleted (S3 Select, Object Lambda, metrics
  v1/v2, NSQ/STAN) carry a note naming the removal commit, and
  `docs/metrics/README.md` lists the 21 v3 collector paths this build serves.
- **Open**: `helm/minio/README.md` told readers to `helm repo add minio
  https://charts.min.io/`, which serves the **upstream** chart - the fork
  publishes no chart repository, so the section now says so and points at
  installing from a clone. Same class as the `go install` lines below, and the
  same unresolved question: whether the fork should publish a chart repo at all.
- **Open**: `README.md`, `docs/chroot/README.md` and
  `docs/orchestration/docker-compose/README.md` tell readers to `go install
  github.com/minio/minio@latest`, which installs **upstream**, not this fork -
  needs a maintainer decision (module path is still `github.com/minio/minio`, see
  the deferred list above). `docs/kms/README.md` points at MinIO's hosted
  `play.min.io` KES demo (free, but a corp-run service with no fork equivalent).
  `docs/lambda/README.md` and `docs/metrics/prometheus/list.md` are annotated
  rather than deleted - deleting a document is a product decision.
  `.github/ISSUE_TEMPLATE/bug_report.md` still carries upstream's "we are no
  longer accepting feature requests for the community edition", which sits oddly
  next to a fork whose only contact channel *is* its issue tracker - also a
  maintainer decision, so it was left as-is.
- **Fixed (2026-10-13)**: `helm/minio` was scraping the removed v2 endpoints.
  The chart now targets metrics v3, with `metrics.serviceMonitor.paths` (per-pod
  collectors) and `clusterPaths` (one `Probe` per cluster collector) instead of
  two hard-coded URLs, validated by rendering the chart and schema-checking every
  object against the Prometheus Operator CRDs. Two latent defects surfaced while
  doing that and are fixed too: `interval`/`scrapeTimeout`/`bearerTokenSecret`
  were nested under `targets.staticConfig`, where the Probe CRD's structural
  schema silently drops them (scrape interval and auth were therefore ignored),
  and `public: false` referenced a `<release>-prometheus` Secret the chart never
  creates - it now names a Secret the operator must provide, or the render fails
  loudly. Chart `apiVersion` went v1 → v2 (with `maintainers` → `authors`, since
  this fork is not maintained by MinIO, Inc) and the chart version was bumped.
  How to re-verify: render with `--set metrics.serviceMonitor.enabled=true
  --set metrics.serviceMonitor.includeNode=true` (add `tls.enabled` +
  `tls.certSecret`, and `public=false` + `bearerTokenSecret`, to exercise both
  guards) and walk every object's field placement against the CRDs. There is no
  schema test wired into the repo, and plain jsonschema is worthless here
  because CRD structural schemas do not forbid unknown fields - only a
  property-path comparison catches mis-nesting. Note also that
  `endpoints[].name` is **not** a ServiceMonitor field: the first cut added it
  and only a correct check noticed it would be silently pruned.
- **Still open in the chart**: `appVersion` is the upstream release snapshot and
  the icon still hotlinks min.io's wordmark - both are identity decisions, not
  technical ones.

A link inventory can be regenerated with:

    grep -rn "slack\.min\.io\|subnet\.min\.io\|min\.io/pricing\|aistor\|dl\.min\.io" docs README.md CONTRIBUTING.md helm .github --include='*.md'
