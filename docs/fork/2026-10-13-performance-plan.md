# minoss 性能优化方案（2026-10-13）

基于 5 个并行调研（构建/测试/启动/并发调度网络/依赖生态）+ 本地实测基线。
每项含预期收益、风险与验证方式；按价值/风险比排序分批实现。

## 基线（实测，32 核，go1.26.8 编译）

| 维度 | 基线 |
|---|---|
| `go build ./...`（热缓存） | 2.7s |
| `make build`（热/冷） | 2.5s / 21.7s（冷 = kqueue 标签缓存分裂 + 调试工具下载） |
| 二进制 | 138MB（-s -w 后 107.9MB） |
| 测试二进制链接（热） | 2.83s / 151.8MB |
| internal 测试套件 | ~14s（lsync 10s；dsync 156s 为独立大头） |
| cmd 过滤子集（TestAPI/ObjectAPI/Erasure） | 101s（前 2 个测试占 47%，系集群装配） |
| 启动（EC 4 盘 → health 200） | 228ms（其中包初始化 170ms，**binaryChecksum MD5 全量扫描 145MB 二进制占 149ms**） |
| -race 开销 | 2-5x |

## 实施批次

### W1 工具链 + 依赖（最大杠杆，低风险）
1. `go.mod`: `go 1.24.0`→`1.25.0`（**启用容器感知 GOMAXPROCS**，解决 K8s CPU limit 下 CFS 节流；解锁 compress v1.20）；`toolchain go1.24.8`→`go1.26.8`（**Green Tea GC** 默认开启，GC 开销 -10~40%；DWARF5 缩短链接；~30% 更快 cgo；crypto/net 安全修复集）。
2. 依赖批次：klauspost/compress v1.18.0→**v1.20.3**（含 s2 NewDict 越界安全修复 + zstd 并发编码/arm64 asm + flate 提速——直接 PUT/GET CPU 收益）、reedsolomon→v1.14.2（GFNI/AVX512 纠删码路径）、cpuid v2.4.0、lz4 v4.1.33、highwayhash v1.0.4、x/net/x/sys/x/time 至 go1.25 兼容档、client_golang v1.23.2、pgzip v1.2.7、dns v1.1.73。
3. 核对并补齐上游 2025-10-15 安全版本（session-policy 绕过 CVE）中缺失于本 fork 基线的修复。
4. Dockerfile 基础镜像 golang:1.24-alpine→1.26-alpine。
验证：build/vet/lint/test 全绿 + 启动冒烟 + 压缩/纠删码单测。

### W2 构建链
5. Makefile：`build` 解耦 `build-debugging`（热构建 -76%）；确认 `kqueue,dev` 为死标签后从 build/test 目标移除（**统一 GOCACHE 键**，冷构建 21.7s→~2.7s 量级）；gen-ldflags `commitID()` 双次 git 调用合一；.gitignore 补 5 个调试工具条目。
6. Dockerfile.release：BuildKit cache mounts（/root/.cache/go-build + /go/pkg/mod）+ go.mod/go.sum 独立层 + 产物 `-ldflags "-s -w"`（镜像层 138→108MB；重复构建分钟级→秒级）。
验证：make build 前后计时；docker build（如环境可用）。

### W3 启动
7. **binaryChecksum 懒计算**（`cmd/bootstrap-peer-server.go:182` 包初始化即全量 MD5 → sync.OnceValue 或首用时计算，仅分布式纠删码消费）：预期启动 228ms→~85ms。
8. 已确认无需处理：监听器已早绑定（未就绪 503 窗口仅 1-2ms）；IAM/console/scanner 等已后台化。selftests（14ms，Fatal 语义）与 RootCA 解析（17ms）保守不动，记录于 backlog。
验证：启动计时 3 次取均值，前后对比。

### W4 测试套件
9. `make test` 本地去掉 `-count=1`（启用测试缓存，未改包重跑≈0s）；CI 保留 `-count=1`。
10. `ExecExtendedObjectLayerAPITest` 8 变体共享集群（136→17 临时盘/测试；加密/压缩是运行时配置而非盘格式）——前 2 慢测试 47.5s→~20s，过滤子集 101s→~70s。中风险，依赖 resetTestGlobals 卫生。
11. dsync 测试 156s 固定睡眠注入化（测试专用短租约）。
验证：过滤子集计时前后对比 + 全 internal 套件。

### W5 并发/调度/网络
12. 容器感知 GOMAXPROCS：随 W1 的 go 1.25 指令生效（零代码）。
13. `globalAPIConfig` 请求池读取：mu→atomic snapshot（每请求一次 RLock 消除，L3 低风险）。
14. peer 扇出 goroutine 内 `rand.New` 每次分配（cmd/notification.go:112）→ 复用。
15. 评估后暂缓（记 backlog）：erasureDisksMu 快照化（L1，中风险）、IAM 存储锁 2-4 次/请求（L2，中风险）、grid 每请求 goroutine 池化（R2）、数据面 HTTP/1.1→grid 迁移（R4，高风险）、HTTP/2 评估。

### 明确不做
- PGO（增加构建时间换 2-14% 运行时收益，与目标冲突）；minio/pkg v3 单独升级（高 API 变动，应随上游跟踪）；ES v7→v8（应用级迁移）。

## 目标汇总
| 维度 | 基线 | 目标 |
|---|---|---|
| make build（热） | 2.5s | ≤0.8s |
| 冷构建（清缓存） | 21.7s | ~3s（缓存统一） |
| 启动（EC） | 228ms | ≤90ms |
| cmd 过滤子集 | 101s | ≤75s |
| internal+dsync | ~170s | ≤30s |
| 容器 CPU limit 尾延迟 | CFS 节流 | GOMAXPROCS 感知 |
| GC 开销 | - | -10~40%（Green Tea） |

---

## 执行结果（2026-10-13，分支 fork/improvement-program 第 16-20 个提交）

| 维度 | 基线 | 实测 | 变化 |
|---|---|---|---|
| `make build`（热） | 2.5s | **0.26s** | **-90%** |
| 冷构建（缓存分裂惩罚） | 21.7s | 随 kqueue 死标签移除消除 | 统一 GOCACHE |
| 启动（EC 4 盘 → health 200） | 228ms | **78ms**（89/75/70） | **-66%** |
| 可达安全漏洞（govulncheck） | 34 个 | **0 个** | 全清 |
| 工具链/语言级 | go1.24.8 / go 1.24 | go1.26.9 / go 1.26 | Green Tea GC、容器感知 GOMAXPROCS、DWARF5 |
| internal 测试套件 | ~170s（含 dsync 155s） | 常规 40 包全绿；dsync `-short` **17.5s** | 按需 -89% |
| 本地测试缓存 | `-count=1` 强制失效 | TEST_FLAGS 化，本地默认缓存 | 未改包重跑≈0 |
| 每请求锁/分配 | requestsPool RLock + 每调用 rand.New | 原子快照 + 全局 rand | 无锁热路径 |

**提交**：`d0940e702`（W1 工具链+依赖+安全）、`adea00c39`（W2 构建链）、`426e46f9c`（W3 启动）、`69a95233a`（W4 测试）、`84650d52c`（W5 并发）。

**被否决的假设**（对照实验）：扩展测试 8 变体共享集群 —— 复用后前 2 慢测试反慢 ~18%（31.3→36.8s / 16.1→20.0s，共享集群状态累积 + 后台 heal 干扰），已回滚，保留每变体全新集群。

**验证**：build ✓ vet ✓ golangci-lint 0 issues ✓ internal 全套 ✓ cmd API/ObjectAPI/Erasure/Storage 广覆盖 116.7s ✓ govulncheck 0 可达 ✓

**后续候选（backlog 性质）**：erasureDisksMu 快照化、IAM 存储锁减次、grid 每请求 goroutine 池化、数据面 REST→grid、HTTP/2 评估、msgp v1.6.5 再生、minio/pkg v3.11 跟随上游。
