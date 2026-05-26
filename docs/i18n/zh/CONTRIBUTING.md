<p align="center">
  <a href="CONTRIBUTING.md">English</a> |
  <a href="CONTRIBUTING.ko.md">한국어</a> |
  <a href="CONTRIBUTING.ja.md">日本語</a> |
  <b>中文</b>
</p>

# 贡献 (Contributing)

> 英文版: [CONTRIBUTING.md](../../../.github/CONTRIBUTING.md) — canonical / 正本

感谢您对 `valkey-operator` 的关注。本文档介绍 PR 流程、如何运行测试,以及在何种情况下需要编写 Architecture Decision Record (ADR)。

## 入门

### 前置工具 (Prerequisites)

| 工具 | 最低版本 | 备注 |
|---|---|---|
| Go | 1.26 | 与 `go.mod` 一致 |
| Docker | 24+ | buildx 默认 builder |
| kind | 0.27+ | 本地端到端 (end-to-end) 测试 |
| kubectl | 1.34+ | 兼容 k3s / kind |
| cert-manager | 1.16+ | Webhook 的 serving cert |
| make | GNU make | 驱动所有 Makefile target |

### 首次构建与测试

```sh
git clone https://github.com/keiailab/valkey-operator.git
cd valkey-operator

# 安装 pre-commit hook (lefthook)。
brew install lefthook       # 或者 `go install github.com/evilmartians/lefthook@latest`
lefthook install

# 单元测试 (envtest 二进制会自动获取)。
make test

# 集成测试 (会启动真实的 Valkey 容器,需要 Docker)。
make integration-test

# 端到端测试 (在 kind 集群中部署 operator)。
make test-e2e
```

## Pull Request 流程

1. **先开 issue**: 任何非琐碎的变更 (架构、API、安全) 都建议先开
   issue。简短的对齐讨论可以避免后续返工。
2. **DCO sign-off 强制**: 所有 commit 必须以 `Signed-off-by:`
   trailer 结尾 (`git commit -s`)。commit-msg lefthook hook 会强制
   检查,未签名的 PR 无法合并。参见
   [Developer Certificate of Origin](https://developercertificate.org/)。
3. **Conventional Commits**: subject 行遵循
   `<type>(<scope>): <subject>` 格式,例如
   `feat(backup): TTL auto-cleanup`。正文可以使用英文或韩文。
4. **必须附带测试**: 任何行为变更都需至少配套一项单元测试以覆盖
   该行为;`make test` 必须通过。
5. **lefthook 必须通过**: 每次 commit 都会运行 `gofmt`、`go vet`
   和 `golangci-lint`;hook 失败将阻断 commit。
6. **PR 正文应包含**:
   - 用户可见的场景 (为何需要这次变更)
   - 验证命令与精简后的输出 (`make test`、
     `kubectl apply -f …` 等)
   - 影响范围 (blast radius) — 您回归测试 (regression test) 过哪些区域
   - 相关 ADR 或 issue 的链接
7. **Review SLA**: 尽力 (best-effort) 在 24 小时内完成首次 review。

## Architecture Decision Records (ADR)

当变更涉及以下情况之一时,请在 `docs/kb/adr/NNNN-<slug>.md` 编写 ADR:

- 新增 CRD,或对现有 CRD 字段进行语义变更
- 新增第三方依赖 (third-party dependency) (ADR 需同时引用
  `sonatype-guide` 与 `context7` 评估)
- 安全 (security)、认证 (authentication) 或数据流 (data-flow)
  表面的变更
- 第三次或更多次以不同方式解决同一问题 (收敛 ADR)

请使用 Nygard 的五段式模板 (Context / Decision / Consequences /
Alternatives Considered / Status)。在同一 commit 中更新
`docs/kb/adr/INDEX.md`。

## 代码风格

- **Go**: `gofmt` 与 `golangci-lint` (通过 lefthook 运行)。强制
  执行 `errcheck`。
- **注释**: 英文或韩文均可。请解释 *为何 (why)*,而非
  *做了什么 (what)* — 代码本身已经展示了做了什么。
- **测试**: 优先使用 fake client;仅在涉及真正的 controller
  集成路径时才使用 `envtest`。始终使用 `WithStatusSubresource`,
  以保持 spec 与 status 隔离。

## 设计探索

在进行较大变更之前:

1. 检查 `docs/plans/` 下已有的 plan。
2. 如果您考虑过 6 个或以上的设计分支,请提前把决定记入 ADR,
   而不是事后补写。
3. 坚持 atomic commit — 每个 commit 代表一个逻辑步骤,且都能通过
   lefthook 的全部四个阶段。

## 质量体系 (SSOT 门禁)

本仓库内置 35+ 个 Single-Source-of-Truth 同步门禁 (在 release cycle 20–77 期间逐步累积)。它们让「对外宣称的表面 == 实际行为」成为一项构建不变量 (build invariant),而非一种期望。

### 门禁所在位置

- `internal/observability/*_test.go` — 33+ 个 SSOT 门禁测试
- 清单: [docs/operations/release-checklist.md §2](../../operations/release-checklist.md)

### 这些门禁阻断的典型情况 (合并前)

- 新增 metric 但缺少 alert-rules + runbook anchor
- 新增 ADR 但缺少 `INDEX.md` 中的行,或缺少必需的 Nygard 章节
- 新增 `kubebuilder:rbac` marker 但 `config/rbac/role.yaml` 没有
  对应更新 (请运行 `make manifests`)
- 新增的 Helm `values` 键在任何 template 中都未被引用
  (用于发现静默的拼写错误)
- 新增 SSOT 门禁但尚未登记到 release checklist §2
  (门禁清单本身也是一道门禁)

### 从源头防止 drift 的自动化

- `make manifests` 自动同步 chart CRD (cycle 38)
- `git push` 会运行包含 6 个 hook 的 lefthook 流水线 —
  完整 lint、gitleaks、`go mod tidy`、helm lint、helm template、
  unit test
- pre-push 的 `go mod tidy` 步骤会阻断 direct / indirect drift
  (cycle 47)

### 热点路径 (hot-path) 基准

- `go test -bench=. ./internal/valkey/` — 五个 parser 的基线 (baseline)。
  相对基线出现 2x 退化即为回归信号。

### 自我解释 (self-explaining) 的门禁失败

大部分门禁会在失败信息中直接打印出修复命令,例如:

- `TestCRDBaseChartSync`: `cp config/crd/bases/X charts/.../crds/X && git commit`
- `TestRBACMarkerResourcesInRole`: `run make manifests`
- `TestReleaseChecklistGatesSync`: 将新门禁加入 release-checklist §2

新贡献者无需猜测应更新哪个相邻表面 — 失败的测试会告诉他们。

## 安全问题 (Security issues)

请**不要**为漏洞 (vulnerability) 开公开 issue。请参阅
[SECURITY.md](../../../.github/SECURITY.md) 获取非公开报告渠道 (GitHub Security
Advisory 与一个 PGP 签名邮箱地址)。

## 许可证 (License)

本项目采用 Apache License 2.0。提交贡献即表示您同意您的贡献
按相同许可证分发。
