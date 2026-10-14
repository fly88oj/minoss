# minoss 代码质量基线与完善方案

日期：2026-10-13 · 基线：上游 `58659f26f`（代码未分叉，仅 README 改动）· 工具链：go1.26.8

本文件两部分：**第一部分**是全仓质量基线（Standards 坏味道轴 / 架构与可测试性 / ponytail 审计 / 构建与 CI 依赖健康度）；**第二部分**是基于 codebase-design（深模块/接缝/适配器）原则的分阶段完善方案。

> 说明：code-review 的 Spec 轴缺失——fork 尚无 PRD/章程，本方案第二部分即第一份章程草案。

---

## 第一部分：质量基线

### 0. 一句话结论

**机械健康度出乎意料地好**（构建/vet/测试编译全绿，lint 仅 53 个同类琐碎发现），但背负三类结构性债务：① `cmd/` 单包 45 万行级巨石 + 165 个可变全局变量，使一切改动都是全包改动、隔离测试在结构上不可能；② 发布/更新/CI 深度耦合 MinIO 公司基础设施，fork 今天**不可独立发布**，且 `minio update` 会把 fork 二进制覆盖为上游版本；③ 约 2.4 万行可删的废弃/遗留功能代码钉住了约 45 个依赖。

### 1. 规模与构成

| 维度 | 数值 |
|---|---|
| Go 文件 / 总行数 | 902 / 350,006 |
| `cmd/`（单包） | 454 文件 / 253,924 行 |
| `internal/`（36 子包） | 423 文件 / 90,898 行 |
| 测试 | 247 文件 / 96,813 行 |
| 生成代码（msgp 等） | ~43.9k 行（~12.5%） |
| 直接依赖 | ~95 个，其中 `minio/*` 24 个 |

### 2. Standards 轴（Fowler 坏味道基线，10 项，按严重度排序，均为 judgement call）

1. **Divergent Change（包级）** — `cmd/` 一个包 454 文件，路由/S3+admin 处理器/纠删码存储/IAM/metrics/复制全部同命名空间；`go test -c ./cmd` 测试二进制 **150MB**。*fork 最大维护风险。*
2. **全局状态阻断测试** — `cmd/globals.go` 79 个 `global*` 变量，全仓共 **~165 个可变全局**，被 173 个非测试文件引用；`globalSiteReplicationSys` 被 26 个文件触碰。247 个测试文件里只有 **1 处 `t.Parallel()`**。
3. **Duplicated Code** — `cmd/object-handlers.go:313` `getObjectHandler`（266 行）与 `:746` `headObjectHandler`（238 行）有 **129 行逐字相同**（auth/校验/版本/SSE/MOD-tag 全套前置逻辑）；multipart 家族（`CopyObjectPartHandler`/`PutObjectPartHandler`）同型重复。
4. **接口膨胀** — `ObjectLayer`（`cmd/object-api-interface.go:246-318`）**44 个方法** + ~45 字段的 `ObjectOptions` 上帝结构体；生产实现只剩 1 个（`erasureServerPools`）。
5. **Data Clumps** — `bucket, object string` 参数对出现在 **130 个签名**；极端如 `CopyObject(ctx, srcBucket, srcObject, destBucket, destObject string, …)`，位置参数极易颠倒。
6. **Primitive Obsession** — bucket/object/路径端到端裸 `string`；`importIAM(w, r, apiVer string)` 用 `""`/`"v2"` 魔法串分发（`cmd/admin-handlers-users.go:2455-2464`）。
7. **超长函数** — `CopyObjectHandler` **627 行**、`importIAM` 416、`newMetricGroups` 409、`PutObjectHandler` 400、`registerAPIRouter` 395、`PutObjectExtractHandler` 361。
8. **Repeated Switches** — `api-router.go:255` 起 ~395 行线性路由注册级联，metrics/admin-router 同型；加一个 API 要动三处。
9. **Shotgun Surgery（轻）** — `for _, pool := range z.serverPools` 扇出在 5 个文件重复；`site-replication.go` 122 个函数跨 26 文件经全局变量触达 peers。
10. **Mysterious Name（弱）** — 接收者名 `z *erasureServerPools`、`api objectAPIHandlers` 读作概念而非值。最弱项。

非系统性：消息链全仓仅 2 处；`iam-store.go`（3072 行）内部内聚（54 个方法全在 `IAMStoreSys` 上）。文档化标准无违例（CONTRIBUTING.md 只引 Effective Go；.golangci.yml 已覆盖其能覆盖者）。

### 3. 架构与可测试性（codebase-design 词汇）

**深模块（通过删除测试，挣得其位）：**

| 模块 | 接缝 | 深度判定 |
|---|---|---|
| `StorageAPI`（`cmd/storage-interface.go:29`，~43 方法） | 3 生产适配器（`xlStorage` posix / `storageRESTClient` 远程 / `xlStorageDiskIDCheck` 包装）+ 1 测试适配器（`naughtyDisk` 故障注入） | **深** — 删掉则 posix/REST/重试/磁盘健康复杂度在所有纠删调用点重现 |
| `internal/grid` Manager | 微小注册面（RegisterSingle/Streaming/Stateless/Connection），藏 84 个类型化节点间 RPC | **深** |
| `internal/event` Target（`targetlist.go:41`，6 方法） | **10 个适配器**（amqp/es/kafka/mqtt/mysql/nats/nsq/pg/redis/webhook）+ 队列重试 | **深** — 教科书级 |

**浅模块（重构候选）：** `ConfigSys`（空结构体纯转发全局）、`NotificationSys`（两条 peer-client 切片的扇出）、`IAMStorageAPI`（34 方法、把缓存锁 `lock()→*iamCache` 泄进接口，接口≈实现复杂度）。

**分层栈**：`erasureServerPools → erasureSets → erasureObjects（经 getDisks/getLockers/getEndpoints 函数字段注入）→ StorageAPI 磁盘`。`internal/` 从不 import `cmd/`，单向分层成立；但 `globals.go:133` 的 `init()` 注释自证"防止循环依赖"，`cmd/` 内 11 个非测试 `init()`。

**测试现实**：handler 测试是 HTTP 级（`ExecObjectLayerAPITest` 起真实 1 盘/16 盘临时目录集群 + V4 签名请求）；**没有 ObjectLayer mock**——每个测试都付真实集群的成本。DI 几乎不存在：仅 `erasureObjects` 函数字段和每 handler 的 `ObjectAPI()` 取值闭包两处注入口；全部子系统接线在 `initAllSubsystems`（`server-main.go:450-500`）对全局赋值，测试反复重调 6+ 次。`serverMain` 引导顺序（logger→KMS→creds→subsystems→object layer→IAM→bucket metadata）无类型级保护，重排即 nil-panic。

### 4. ponytail 审计（过度设计/可删减，净估 **-24,000 行 / -45 依赖**）

**产品决策类（需 fork 定夺）：**

| 项 | 依据 | 规模 |
|---|---|---|
| 嵌入式 Web Console（`minio/console`） | 拖入 go-openapi 全家、bubbletea/charmbracelet、**mc**、go-pretty、prom2json 等 ~45 模块——go.mod 最大依赖树 | 仓内 ~650 行 + 依赖树 |
| S3 Select（`internal/s3select/`） | AWS 已废弃该服务（2024 起新桶不可用）；独占 4 依赖（participle、parquet-go、csvparser、simdjson-go） | ~15k 行 |
| Prometheus metrics v1+v2 | v3 已完整并存，三代同仓 | ~5.9k 行 |
| Object Lambda | AWS 已退役 S3 Object Lambda；连带 `internal/config/lambda`（event 的分叉副本） | ~2k 行 |
| Subnet 注册 + Callhome | 回联 subnet.min.io 商业门户，对社区 fork 无意义 | ~750 行 |
| 自更新 `minio update` | **硬编码 `dl.min.io`**（`cmd/build-constants.go:53`），会把 fork 二进制覆盖成上游 MinIO | ~750 行 + selfupdate/pb/minisign 依赖 |

**机械类（无需产品决策）：** peer-rest→grid 收尾（第二套节点间 RPC，上游迁移未完成，~2.4k 行）；stdlib 替换（`mitchellh/go-homedir`→`os.UserHomeDir`、`json-iterator`→`encoding/json`（全部调用点本就用兼容模式）、`pkg/errors`、`go.uber.org/atomic`→`sync/atomic` 类型化、`buger/jsonparser`（仅 jwt）、mousetrap、`cheggaaa/pb`→`x/term`、yaml v2/v3 双轨→v3）；死事件目标（NSQ 生态 2021 年弃维护、STAN 已 EOL）；`secure-io/sio-go` 与 `minio/sio` 双认证流加密并存；`internal/color` 160 行重复造 fatih/color 自带的 NoColor 检测；`internal/ioutil` 双 deadline 助手并存。

已核实非误报的排除项：bpool/ringbuffer（bitbit 流性能）、dsync+lsync（2 locker）、WarmBackend（4 实现）、amztime（确属薄包装但统一语义）。

### 5. 构建与 CI 依赖健康度

**全绿**：`go build ./...` ✓、`go vet ./...` ✓、全部测试编译 ✓、`internal/ioutil`+`internal/config` 烟测 ✓（go1.26.8，无需下载 toolchain）。lint：53 个发现**全部**是陈旧 `// +build` 双写标签（govet buildtag），一个机械修复类别清零。

**CI 可移植性**（21 workflow）：10 个开箱即用（go-cross/go-lint/go-healing/go-resiliency/helm-lint/depsreview/typos/shfmt/vulncheck/lock）；其余依赖公司设施：`mint.yml` 跑在 MinIO 自建 `runs-on: mint` 池（fork 上会永远排队）、`issues.yaml` 需 `BOT_PAT`+miniohq 项目、replication/root-disable/upgrade/go-verify 从 dl.min.io 或 minio/minio master 下载二进制、iam-integrations 拉 quay.io/minio 镜像并 checkout minio/minio-iam-testing。

**安全/发布阻断项：**
- **`root.key`：仓库里提交了私有 CA 密钥**（CI TLS 用）——立即轮换并改为 CI 内生成临时 CA
- `Dockerfile.release*`/`Dockerfile.hotfix` 只是从 dl.min.io 下载产物重新打包（含 MinIO 公钥 minisign 验签）；`Dockerfile.cicd` 基于 `minio/minio:edge`
- 版本劫持：fork 继承 **523 个上游 RELEASE.\* tag**，`git describe` 版本号会报上游版本
- `hotfix*`/`docker-buildx.sh` 发布路径 scp 到 dl-*.minio.io，需 minisign 凭据目录
- 依赖锁定：`console` 与 `kms-go/kms` 用**伪版本**钉住（可变/可消失）；`console` 经 api 层拖入整个 mc 传递图

**各轴最差单项**：Standards 轴 = `cmd/` 单包巨石 + 165 全局（发现 1/2）；架构轴 = ObjectLayer 44 方法单生产实现（宽接口浅复用）；ponytail 轴 = console 依赖树（-45 依赖的最大头）；构建轴 = fork 不可独立发布（update 劫持 + 版本劫持 + CI 公司耦合）。

---

## 第二部分：完善方案（codebase-design 原则）

**设计语言**：Module = 接口+实现；Interface = 调用者必须知道的一切（签名、不变量、错误模式）；Depth = 单位接口换来的行为量（深 = 小接口大行为）；Seam = 不改调用方就能换行为的位置；Adapter = 接缝上的具体实现。**一个适配器 = 假想接缝；两个才算真接缝。删除测试：删掉模块后复杂度在 N 个调用点重现才算挣得其位。**

**总原则**：不搞大爆炸重写。每一阶段独立可发布、可回滚、有测试护航。顺序是"先能独立活着 → 再做减法 → 再深化接缝 → 最后立防线"。

### Phase 0 — 独立可发布地基（~1-2 周，不改架构）

目标：fork 能以自己的身份构建、测试、发版。**这是其余一切的前提。**

1. **摘除 `root.key`/`root.cert`**，CI 改为运行时生成一次性测试 CA（安全债，最先做）。
2. **版本身份**：删除继承的上游 tag（或新开孤儿分支历史），建立 fork 自己的 tag 方案（建议 `v0.Y.Z` + git describe）；改 `buildscripts/gen-ldflags.go` 优先读 fork tag——否则二进制自报上游版本号。
3. **拆除自更新劫持**：`cmd/update.go` + `MinioReleaseBaseURL`（`cmd/build-constants.go:53`）指向 dl.min.io，会把用户装的 fork 覆盖成上游 MinIO。最小改法：默认禁用 `minio update`（返回明确错误指向 fork 发布渠道）。
4. **CI 减脂**：删除 `issues.yaml`/`mint.yml`（mint 池永远排队）；replication/root-disable/upgrade/go-verify 测试改为用 workflow 内自构建产物替换 dl.min.io 下载；iam-integrations 镜像改自建或移除。保留并修复 go-lint/go-cross/go-healing/helm-lint/typos/shfmt/vulncheck 十件套。
5. **发布通道**：`Dockerfile.release*`/`hotfix` 全链路改为 GitHub Releases + 多阶段自构建（模板已有：`Dockerfile.scratch` + golang builder）；Makefile 默认 `REPO` 移出 `quay.io/minio`。
6. **依赖止血**：把 `console`、`kms-go/kms` 两个伪版本钉成不可变 commit SHA；配置 `upstream` remote，安全修复仍可 cherry-pick。
7. **机械清理**：清掉 53 个陈旧 `// +build` 行（lint 清零）。
8. **module path 决策**：暂时保留 `github.com/minio/minio`（重命名触碰全部 import + 生成器，收益低）；记录为 ADR，留待 Phase 2 后再议。

验收：`make build && make lint && make test` 全绿于全新 fork CI；能打出带 fork 版本号的 release；`minio update` 不再指向上游。

### Phase 1 — 减法（~2-4 周，ponytail 排序）

目标：删掉不挣其位的复杂度。**净预期：-2.4 万行 / -45 依赖 / CI 显著提速。**

1. **产品决策三连（建议，待你拍板）**：
   - **S3 Select / Object Lambda / metrics v1+v2**：直接删。AWS 自己都废弃/退役的服务，社区 fork 替 AWS 背书无意义；v3 metrics 完整在场。
   - **Console**：建议**先不删**（用户可见功能），改为 Phase 1 末的独立决策：若社区反馈确认 S3 API + mc 是核心用法，则删除（依赖树收益最大）；否则维护。删时 `MINIO_BROWSER=off` 行为固化为默认。
   - **Subnet/Callhome、自更新**：删（Phase 0 已使后者失效，顺势清码）。
2. **机械清理包**（无决策成本，按 ponytail 清单）：stdlib 替换九项、NSQ/STAN 死目标、`sio-go` 合并（注意存量配置需保留 sio-go 解密迁移路径）、`internal/color` 与 deadline 助手收敛、peer-rest→grid 收尾（上游未竟迁移，完成即删 ~2.4k 行）。
3. 每删一项：对应路由/handler/config 子系统一并摘除，`go.mod` 同步 tidy，回归 `make test`。

验收：`grep -r SelectObjectContent` 归零；直接依赖 ~95→~55；全仓 -15% 行数。

### Phase 2 — 接缝深化（增量式，持续 1-2 个季度，按子系统逐个走）

目标：把"改一处 = 全包重编 + 150MB 测试二进制"变成"改一处 = 一个内聚包"。**手段不是重画架构图，而是逐子系统把全局变量所有权收编为构造函数注入。**

1. **拆包次序**（按内聚度/风险比，从叶子开始，每步一个 PR + 全程绿灯）：
   ① metrics-v3 → `internal/metrics`（Phase 1 后只剩一代，天然边界）
   ② IAM → `internal/iam`（`iam-store.go` 已内聚，54 方法一个接收者；顺手把 `IAMStorageAPI` 从 34 方法瘦身——缓存锁 `lock()/rlock()` 是实现细节，不该在接口上）
   ③ site-replication（6284 行、跨 26 文件的全局触达全部收进一个 `SiteReplicationSys` 结构体字段）
   ④ batch / tier / admin-handlers 按同法推进
2. **规则**：每提出一个新包，其依赖一律**构造函数传入**（accept dependencies, don't create them）；包内测试可直接 `t.Parallel()`。存量 `cmd/` 全局只在调用点就地读取→逐个替换为接口调用。**冻结新增全局**（这是可执行的硬规则，比"减少全局"可落地）。
3. **处理器去重**（独立小步，立即可做）：提取 `getObjectHandler`/`headObjectHandler` 共有的 129 行"对象读上下文解析"（auth→版本→SSE→选项装配）为 `resolveObjectReadContext`；multipart 家族同法。行为不变，纯删除测试护航。
4. **ObjectLayer 接口瘦身（机会主义）**：44 方法 + 单生产实现 = 假想接缝，但它同时是 handler 测试的真接缝——**不整体重构**。规则：新增操作时按消费者能力拆分（如 `ObjectReader`/`ObjectWriter` 小接口），handler 只依赖所需能力；`ObjectOptions` 45 字段在每次被迫触碰时分组（EncryptionOptions/RetentionOptions/…）。存量不动。
5. **Data Clump 渐进**：新拆出的包内部起用 `Bucket`/`Object` 域类型（携带校验/编码规则），存量调用点不做地毯式替换。
6. **引导顺序显性化**：`serverMain` 的全局接线序列收进一个 `bootstrap` 结构体的方法链，每步返回下一步所需依赖——类型系统逐步接管现在的"注释式顺序"。
7. **浅模块三选一处置**：`ConfigSys` 空壳直接删（删除测试：复杂度不重现）；`NotificationSys` 在 site-replication 拆包时一并收编；`IAMStorageAPI` 在 IAM 拆包时瘦身。

验收：`go test -c ./cmd` 二进制显著缩小；`internal/` 新包测试可并行；新增 `t.Parallel()` 数量成为跟踪指标。

### Phase 3 — 防线（与 Phase 2 并行建立）

1. fork 自己的 CI 门禁：lint（清零后）+ 测试矩阵 + cross-compile + vulncheck 全绿为合并前提。
2. 覆盖率门禁只对**新拆出的 `internal/` 包**生效（存量不追溯，避免指标游戏）。
3. ADR 机制：module path 保留、console 去留、ObjectLayer 拆分策略等决策各记一条，防止后续反复。
4. 文档债：46 个外链 min.io 企业文档的 md 建立 fork 版重写清单（低优先级长尾）。

### 不做清单

- 不大爆炸重写、不中途改 module path、不做全仓 `Bucket/Object` 类型地毯式替换、不为"整洁"拆接口（两个适配器才配得上一个接缝）。
- 遵守 AGPLv3：fork 持续开源是义务也是社区 fork 的立足点。

### 里程碑摘要

| 阶段 | 周期 | 核心交付 | 退出标准 |
|---|---|---|---|
| P0 独立地基 | 1-2 周 | 可发版、CI 自持、无安全债 | fork 版本 release 流水线绿灯 |
| P1 减法 | 2-4 周 | -2.4 万行 / -45 依赖 | 删除清单归零，全测通过 |
| P2 接缝深化 | 1-2 季度 | cmd/ 巨石逐块内聚化 | 测试二进制缩小 + 可并行测试 |
| P3 防线 | 持续 | 门禁 + ADR + 文档 | 新增代码默认被护栏接住 |

---

## 执行状态（2026-10-13，分支 `fork/improvement-program`）

四个阶段的落地情况。所有提交均通过 build/vet/lint/test-compile 护航。

### P0 独立可发布地基 — ✅ 完成
- root.key/root.cert 已删（无引用死文件）；523 个上游 tag 已删
- 自更新路径完整移除（admin API、peer verify/commit、notifier、构建常量）
- CI：删 mint/issues/upgrade/iam-integrations/multipart 5 组公司依赖 workflow；测试脚本改为源码构建 mc（固定 tag）；新增 `release.yml`（15 目标交叉编译 + GitHub Releases + GHCR 镜像）
- Dockerfile.release{,.old_cpu} 重写为自建多阶段；hotfix 全链路与 docker-buildx.sh 删除；REPO 默认 ghcr.io/fly88oj
- 伪版本依赖由 go.sum 内容锁定（ADR 0003 记录策略）；upstream remote 已配置

### P1 减法 — ✅ 完成（净 -19,600 行 / -13 个直接依赖）
- S3 Select 删除（-14,373 行，jstream 迁至 internal/jstream，restore-select 拒绝）
- Object Lambda + metrics v1/v2 端点删除（-2,982 行；采集机架保留供 v3/admin/peer）
- SUBNET/callhome 删除（-810 行，含 admin inspect 的 MinIO 簇密钥层）
- stdlib 替换 7 项；NSQ/STAN 死目标删除；DeadlineWorker 并入 RunWithDeadline；internal/color 去间接层

### P2 接缝深化 — ◐ 首批落地 + 首个包提取完成（按计划增量推进）
- ConfigSys 空壳删除（删除测试：复杂度不重现）
- getObject/headObject ~130 行重复提取为 4 个共享助手（GET/HEAD 全测试通过，含加密变体）
- 全局冻结：CONTRIBUTING 规则 + CI 守卫 job（新增 `global*` 于 cmd/ 即失败）
- **metrics-v3 → `internal/metrics` 提取完成**：22 文件/3,437 行整体迁出；
  `metrics.Deps`（30 个窄钩子 + 值字段）取代 20 个全局读取与全部 cmd 类型依赖，
  cmd 侧接线在 `cmd/metrics-v3.go`；新增 8 个全并行（t.Parallel）测试；
  顺带修复上游潜在缺陷（Collect 对 nil 错误恒 panic）。IAM、site-replication、
  multipart 二轮等后续配方见 `docs/fork/backlog.md`

### P3 防线 — ✅ 完成
- ADR 0001-0003（子系统移除、发布独立、依赖策略）
- CONTRIBUTING fork 规则（全局冻结/新包并行测试/依赖门槛/禁止回潮）
- `docs/fork/backlog.md`（P2 延续 + 延期决策 + 文档债清单）
- CI 终态：lint+test+race+globals 守卫 / cross-compile / vulncheck / typos / shfmt / helm / healing / resiliency / release
