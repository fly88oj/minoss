# minoss 性能优化第二轮（2026-10-15）

继 2026-10-13 第一轮（工具链/构建链/启动/测试/并发五批次）之后的第二轮，
输入为 6 个并行调研（本地构建/CI、测试套件、启动路径、并发调度、Go 工具链
生态、上游 MinIO 实践）+ 全新基线实测。本轮重点：CI 墙钟时间、测试套件
墙钟时间、启动关键路径的并行化、每请求热路径去锁，以及一个依赖刷新引入
的**生产回归修复**。

## 修复的前置阻塞：yaml 库不匹配（生产回归）

bb517b3f7（依赖刷新 46 个直接依赖）把 minio/pkg v3.1.3→v3.10.0，其中
xtime 包从 `gopkg.in/yaml.v3` 切到了官方后继库 `go.yaml.in/yaml/v3`。
两个库的 `Unmarshaler` 接口类型不兼容，导致本仓库用 gopkg.in 反序列化
`BatchJobRequest` 时 `xtime.Duration` 的自定义解析失效，退化为反射解析：
**带 `7d` 这类天单位的 batch replicate/expire 任务 YAML 解析直接失败**
（`TestParseBatchJobReplicate` 红）。修复：cmd 下 7 个文件统一切到
`go.yaml.in/yaml/v3`（API 兼容，官方维护线），`gopkg.in/yaml.v3` 从
go.mod 彻底移除。教训：跨库接口型依赖（Unmarshaler/Marshaler）升级时
必须跑全量相关测试。

## 基线（2026-10-15 实测，32 核 / 93GB，go1.26.9）

| 维度 | 基线 |
|---|---|
| 全量 `-short` 测试套件 | 6m32s（**cmd 328.8s 占 84%**；dsync 18.3s、lsync 9.7s、grid 6.6s） |
| `make build`（热） | 5.3s（含 checks + gen-ldflags 解析期 `go run` 固定税） |
| CI（go-lint.yml 单 job） | make / make test -count=1 / make test-race 串行，verifiers 跑 2 遍 |
| 启动（单盘，API ready） | ~0.15s（单盘本地，DNS/KMS/IAM 风险点见下） |

## 已实施

### A. 构建 / CI

1. **go-lint.yml 拆分为 5 个并行 job**（build / verifiers / test /
   test-race / no-new-globals）：原先单 runner 串行三套 make，`verifiers`
   （lint 下载 + go generate 57 条 + tidy + 5 个子模块测试）被执行两遍。
   PR 关键路径从"三者之和"变为"三者之最大者"。`make test`/`test-race`
   相应去掉 `verifiers build` 前置（开发者内环不再付 CI 级门禁），本地
   全量门禁收敛到 `make verifiers`。setup-go 显式 `cache: true`；新增
   golangci-lint 二进制的 actions/cache（key 钉 v2.14.0，与 Makefile 同步）。
2. **race.sh 并行化**：75 包逐包串行（每包独立进程 + `-v` 全量日志）→
   `go test -race -count=1 -p 2 ./...`（race 内存放大 5-10x，-p 保守为 2；
   -count=1 保证重跑诚实——原脚本在热缓存下会静默跳过整包 race）。
3. **Makefile**：`LDFLAGS := $(shell go run gen-ldflags)` 改懒求值（`make
   clean/help` 不再触发工具链往返）；`getdeps` 加二进制存在性检查（重复
   make 调用不再重复 curl 安装）；`submodules` 5 个独立模块 xargs -P 并行；
   `make test` 去掉 `-v`（失败仍打印）。
4. **cross-compile.sh**：15 平台串行 → `xargs -P` 并行（上限
   `min(nproc, 8)`——每个 go build 峰值 ~1-2GB，无上限的 -P $(nproc)
   在大主机上会挤占并发的其他任务）；`go list` 提出循环只算一次；
   删除死标签 `-tags kqueue`（与 Makefile 统一 GOCACHE 键，延续 W2
   结论）。
5. 杂项：`make docker` 去掉 `--no-cache`（镜像为二进制 COPY 模式，内容
   随产物变化，缓存安全）；.dockerignore 补 `minio`（121MB 构建上下文）；
   vulncheck.yml `cached:` 拼写修正为 `cache:`。

### B. 测试套件（-short 模式计时）

| 包 | 前 | 后 | 手段 |
|---|---|---|---|
| internal/lsync | 9.68s | **1.56s** | 耦合时序（读锁保持 2s/3s vs 写锁超时 5s/1s）统一 /10 缩放，相对语义不变 |
| internal/dsync | 18.30s | **2.94s** | 共享超时常量 /5 + `lockRetryMinInterval` 250ms→25ms（hammer 测试竞争退避占 6.9s；注意 `testing.Short()` 在 init/早期 TestMain 会 panic，需先 `flag.Parse()`） |
| internal/ioutil | 1.51s | 0.65s | deadline/sleep 配对统一缩放 |
| cmd/signature-v4 | +4s | ~0 | 固定 4s sleep → 25ms 轮询（PolicyDBSet 本就在锁内直写缓存） |
| cmd/sts（root STS） | +2s | ~0 | 固定 2s sleep → 条件轮询（2s 为失败上限而非固定成本） |
| cmd/storage-rest | 无界 | 30s 上限 | grid 连接等待循环加 deadline，握手失败现在报错而非挂死整个包 |

预期全套件 -short 约 **-25s**（6m32s → ~6m05s）；cmd 328s 的大头是每测试
现场装配集群（ExecObjectLayer* 68 处 + 套件矩阵 ~45 次起服），共享 fixture
方案已被第一轮 A/B 实验否决（状态累积反慢 18%），维持现状。

**评估后跳过**：internal 纯函数包补 t.Parallel() —— `go test ./...` 包间
已按 -p=GOMAXPROCS 并行，各包本身仅 20-40ms，墙钟收益≈0。

### C. 启动关键路径

1. **KMS 探活加界**（common-main.go）：`GenerateKey(GlobalContext)` 原无
   deadline，KMS 不可达时启动在 listener 绑定前**无限挂起**。现为每次
   10s 超时 × 5 次重试（间隔 5s，可被取消打断），失败 loud fatal——语义
   从"永不 ready"变为"约 75s 内明确失败"，systemd 重启循环可恢复。
2. **console 与 IAM 解耦**（server-main.go）：console 初始化原先串行排在
   `globalIAMSys.Init`（含 OpenID discovery，1min 响应超时 + 无限重试）
   之后。现仅当 OpenID 已配置时保持原顺序（console 的 provider 列表构建
   一次、依赖 discovery 结果）；未配置 OpenID 时（多数部署）console 并行
   启动，慢 IAM 后端（etcd 不可达等）不再拖住 UI。FTP/SFTP 鉴权依赖 IAM，
   保持原顺序。判断函数 `openIDConfigured()` 用 `KVS.Empty()` 镜像
   LookupConfig 的输入，不触发 discovery。
3. **endpoint DNS 并行解析**（endpoint.go UpdateIsLocal）：每个 unique
   host 串行 `isLocalHost`（DNS 5s 超时 + 端口探测）→ 同轮所有未解析
   endpoint 并行探测；K8s/Docker 的 500ms tick 重试语义、非编排环境的
   错误即返语义均保留。多主机分布式部署下省 N×RTT（DNS 慢时 5s/主机）。
4. **getLocalDisksToHeal 并行 DiskInfo**：本地盘串行 statvfs（HDD 单次
   可达 10ms+，位于启动关键路径）→ 全盘并行。64 盘 HDD 约省数百 ms。

**评估后跳过**：--config YAML 三次 Unmarshal 合一（<10ms）、selftests
并行（<50ms）——收益不抵改动。

### D. 并发 / 调度热路径

1. **96 处 audit defer 的 JWT 惰性求值**：`defer logger.AuditLog(ctx, w, r,
   mustGetClaimsFromToken(r))` 的参数在**登记时**求值——即每请求完整 HMAC
   JWT 解析，而 `AuditLog` 在无审计目标时（默认部署）直接 return，解析
   纯属浪费。改为闭包惰性求值；其中 79 处用**参数捕获** `ctx`（闭包引用
   捕获在 handler 中途重赋 ctx 时会拿到可能已超时取消的值，语义变化），
   17 处 `r.Context()` 变体用普通闭包（请求 ctx 对象终身不变，调用廉价）。
2. **globalObjectAPI 原子化**（api-router.go / object-api-common.go）：
   `newObjectLayerFn()` 每请求 RLock → `atomic.Pointer[ObjectLayer]` 快照
   读（与第一轮 requestsPool 同模式），写路径仅 setObjectLayer/测试。
   globalObjLayerMutex 保留给 globalConsoleSrv/globalHTTPServer。
3. **heal 任务通道加缓冲**（background-heal-ops.go）：无缓冲 channel +
   worker 全忙时 noWait 任务静默丢弃（降级盘恢复要等下个 scanner 周期，
   分钟级）→ 缓冲 4×workers，突发任务存活率提高；丢弃逻辑语义不变。
4. **事件 target worker 可配**：`Init(GOMAXPROCS)` 硬编码（上游留有
   TODO）→ 新增 `_MINIO_EVENT_TARGET_WORKERS` 旋钮（内部 env 惯例，
   正整数生效），慢 target 多的部署可调。

## 平台矩阵决策（本轮唯一影响用户面的变更）

依赖刷新后发现的第二个回归：**32 位平台（linux/386、linux/arm、linux/mips）
与 netbsd/amd64 的交叉编译已经坏了**：
- `aead.dev/mtls` v0.2.1→v0.4.0 在 32 位下 `math.MaxUint32` 溢出 int，
  且 v0.4.0 已是最新版（无上游修复）。
- `coreos/go-systemd` v22.7.0 新增的 `daemon/sdnotify_unix.go`（`//go:build
  unix`）引用 `unix.CLOCK_MONOTONIC`，x/sys 在 netbsd 上不导出该常量。

处置：**32 位目标从 cross-compile.sh 与 release.yml 矩阵移除**（降级加密
依赖换 32 位编译是错误取舍；恢复路径：fork-patch mtls `key.go:91` 为
int64 比较后重新加回）；**netbsd 保留**，go-systemd 经 go.mod replace 钉回
v22.6.0（plain `go get` 降级会 MVS 级联把 etcd 拖到 alpha、minio/pkg 拖到
v3.8.1，replace 不级联）。发布二进制因此少 3 个 32 位变体——2026 年的
对象存储 32 位部署面可忽略，但这是显式决策，记录在案。

## 验证（全部串行执行，避免叠加负载）

- 全量 `-short` 测试套件：**2m43s，42 包全绿**（基线 6m32s 且含 1 红；
  注意基线当日机器有 67GB 内存被外部进程占用，cmd 包 328.8→154.2s 的
  降幅大部分来自环境差异，可归因于本轮改动的确定收益是 internal 慢包
  -24s 与 cmd 内 ~6s 固定 sleep + OOM 当晚的教训：重型验证任务必须串行）
- golangci-lint（钉 v2.14.0）：**0 issues**；gofmt 0 差异；go vet ✓
- 交叉编译：**12 平台全绿，26s**（原 15 平台串行约 ~6 分钟 CPU 总和；
  并行度 min(nproc,8) 上限防止内存挤占）
- 启动冒烟：单盘 server API ready **0.11-0.16s**，/minio/health/live 200，
  console 并行启动后 HTTP 200 ✓
- yaml 修复回归验证：TestParseBatchJobReplicate/Expire ✓（go.mod 变更后
  复跑）

### 多容器分布式实测（2026-10-16 补充，HEAD=33be22cbf）

4 × alpine:3.19 容器（各自命名卷，bridge 网络主机名 minio1..4）组成
单 set 4 盘分布式纠删集群，二进制为本 HEAD 现编译。Docker Hub 不可达，
基础镜像用本地缓存。通过 minio-go/madmin-go 实测：

| 阶段 | 结果 |
|---|---|
| 冷启动成集群 | 4 节点各 ~1s ready；`/minio/health/cluster` 200（并行 endpoint 解析 + bootstrap 对等校验路径均在容器 DNS 下工作） |
| 常规操作 | 建桶；PUT 1MiB + 12MiB（multipart 走 erasure 分片）；GET 后 SHA256 逐一校验一致；List 正确 |
| 单节点宕机 | `docker stop minio4`：cluster health 仍 200，宕机期间 PUT/GET+校验全部成功（4 盘 EC 2+2 预期内） |
| 节点回归+自愈 | `docker start minio4`：~2s 重新就绪，ServerInfo 4/4 盘 state=ok（后台 heal 已补齐），madmin Heal API 正常，愈后读取校验通过 |
| 并发 | 50 worker × 4 轮 put+get+校验（200 对象）零错误，0.4s |
| console | 解耦改动后在容器内同样正常（HTTP 200） |

## 明确暂缓（记入 backlog）

- IAM 存储锁内磁盘 I/O（PolicyDBGet miss 路径）与每请求 3 次 RLock 的
  xsync 化——第一轮已列 L2，涉及缓存插入原子性。
- session policy 每请求 JSON 重解析缓存（需失效钩子）。
- NS 锁 getSource（runtime.Caller）惰性化；小对象 PUT buffer 分级池；
  multiWriter 慢盘队头阻塞（写路径正确性敏感，需 warp 级压测配套）。
- scanner 磁盘截断语义（erasure.go:471 `disks[:maxProcs]` 变相拉长周期）。
- release.yml docker job 去 QEMU（复用 binaries matrix 产物）——发布路径，
  与"是否发布镜像"的既定决策绑定，暂不动。
- warp / `mc admin speedtest` 性能回归门（nightly workflow）与
  docs/tuning 2026 官方 tuned 配置分档更新——纯增量工作，价值在防回归。
- Go 1.27（小对象分配专项 malloc 最高 -30%，整体 ~1%）——下个窗口期随
  依赖窗口一起评估。

## 执行结果

| 维度 | 前 | 后 |
|---|---|---|
| 全量 -short 套件 | 6m32s（含 1 红，机器有外部内存压力） | **2m43s 全绿**（环境部分还原后可归因改善 ~-30s） |
| internal 慢包（lsync+dsync+ioutil） | 29.5s | **5.0s（-83%）** |
| CI PR 关键路径 | 3 段串行 + verifiers×2 | 5 job 并行，各付一次门禁 |
| race 测试 | 75 包串行 + -v | -p 2 包级并行 |
| 交叉编译 | 15 平台串行（且 4 平台已静默损坏） | 12 平台并行 26s 全绿（-P 上限 8） |
| 每请求（无审计目标部署） | 入口 JWT 解析 + objLayer RLock | 均消除 |
| KMS 不可达时启动 | 无限挂起 | ≤75s 明确失败 |
| OpenID 未配置时 console | 等 IAM Init（最坏分钟级） | 并行启动 |
| 生产正确性 | batch 任务 `7d` 时长 YAML 解析失败（deps 回归） | 已修复（go.yaml.in 统一） |
