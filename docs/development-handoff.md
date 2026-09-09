# GVideo 开发交接

> 更新日期：2026-09-09
> 当前分支：`codex/architecture-hardening`
> 交接原则：一个任务只解决一个问题；有文件、接口或数据契约冲突的任务必须串行。

## 1. 当前任务

当前不是新增业务功能，而是封口架构加固批次：确认分层拆文件无行为变化，完成 metrics、独立管理端口、SQLite 迁移账本、前端路由懒加载和 HLS 动态加载的测试与联合验收，然后再提交并推送。

当前工作区包含大量未提交、未跟踪的新拆分文件。提交前必须逐项核对并显式暂存，不能使用 `git add .`，也不能遗漏新文件。

## 2. 已完成

- 后端 Handler、Service、Repository 在原包内按职责拆文件，未改变公开 API。
- 新增并发安全 metrics registry、HTTP/Worker 指标和队列深度 gauge。
- metrics 与 pprof 使用独立管理端口，默认关闭；监听失败时启动即报错。
- SQLite 新增 `schema_migrations` 账本、checksum、事务回滚、版本顺序和账本空洞校验。
- React 页面使用 `lazy`/`Suspense`；`hls.js` 改为按需加载并增加卸载清理。
- Compose、环境变量、部署与运维说明已同步；Vitest 暂时单 worker 运行以提高 Windows 稳定性。
- 已清理无效 Git worktree 注册；当前只有主工作区。被 Codex 进程占用的空目录不影响 Git。
- Context7 MCP 已完成 OAuth 接入；仅在查询第三方库最新 API 时使用，不阻塞当前批次。

## 3. 已验证

最近一次完整基线验证已通过：

- `./internal/httpapi`、`./internal/platform`、`./cmd/server` 目标 Go 测试。
- 全仓 Go 测试和 `go vet`。
- 前端 11 个测试文件、25 项测试、TypeScript 类型检查和 Vite production build。
- Docker Compose 配置、PowerShell 脚本语法和 `git diff --check`。

非阻断警告：HLS 动态 chunk 约 509 kB，但未进入首页 preload，只在播放路径加载。

完整浏览器/API 验收和 Go race 尚未在本轮最终状态上重新执行；race 需要 Linux/CGO 可用环境。

## 4. 当前阻断项

总控已汇总架构、前端、后端、UI/UX、测试和 SRE 六方只读审查。后端拆文件等价性未发现阻断，但联合验收仍未通过。以下任务存在代码、测试或配置交集，必须严格按编号串行执行；每完成一项先跑目标测试并复审，再领取下一项。

### BASE-03A 管理监听地址安全校验

状态：已完成（2026-09-09）。生产环境仅允许关闭、`localhost`、loopback 或 literal private IP；拒绝 wildcard、unspecified、公网 IP、任意 hostname 和非法端口。已通过 `go test ./internal/config -count=1`、`go vet ./internal/config` 与目标 `git diff --check`。

- 唯一目标：生产环境拒绝 metrics/pprof 绑定 unspecified、wildcard 或公网地址，允许 loopback 和明确可信的私网地址。
- 允许修改：`backend/internal/config/config.go`、`backend/internal/config/config_test.go`；仅在无法保持边界时最小修改 server 启动代码。
- 禁止范围：指标格式、业务 HTTP 端口、Compose 网络和其他配置。
- 验收：配置表格测试、目标 Go 测试和 `go vet`。

### BASE-03B 管理端口启动测试

- 前置依赖：BASE-03A 通过。
- 唯一目标：自动验证 metrics/pprof 独立启停和端口占用时 fail-fast。
- 允许修改：`backend/cmd/server` 内测试及为可测试性所需的最小无行为重构。
- 禁止范围：管理端业务能力、HTTP 路由和部署配置。
- 验收：目标包测试必须证明端口冲突非零失败；现有人工验证约 529ms 非零退出只作参考。

### OPS-01 容器停止宽限期

- 前置依赖：BASE-03B 通过。
- 唯一目标：使 backend 容器停止宽限期长于应用 15 秒优雅关闭时间。
- 允许修改：`compose.yaml` 及对应部署说明。
- 建议值：`stop_grace_period: 30s`；同时确认媒体 Worker 能在该窗口退出。
- 验收：Compose 渲染、脚本门禁；Docker 可用时执行一次停止验证。

### FE-01 路由懒加载错误边界

- 唯一目标：动态 chunk 加载失败时显示可恢复提示，不让路由区域直接崩溃。
- 允许修改：`frontend/src/app/` 内相关实现与测试。
- 禁止范围：路由结构、页面业务逻辑、视觉系统和后端。
- 验收：失败路径测试、`npm test -- --run`、`npm run typecheck`、`npm run build`。
- 回滚：删除新增错误边界并恢复原 `Suspense` 包装。

### FE-02 播放器快捷键作用域

- 前置依赖：FE-01 通过。
- 唯一目标：Space 快捷键不劫持按钮、链接、表单控件或可编辑区域。
- 允许修改：`VideoPlayer.tsx` 与对应目标测试。
- 禁止范围：播放协议、HLS 生命周期和播放器视觉重构。
- 验收：键盘目标测试及全量前端门禁。

### FE-03 HLS 状态反馈

- 前置依赖：FE-02 通过。
- 唯一目标：为 HLS 准备、回退和最终失败提供可见且可读屏的状态与重试入口。
- 允许修改：播放器、对应样式和目标测试。
- 禁止范围：路由、后端媒体协议和其他页面。
- 验收：状态转换、`aria-busy`/状态播报和重试测试。

### FE-04 HLS 异步生命周期测试

- 前置依赖：FE-03 通过，避免并行修改播放器和测试基础设施。
- 唯一目标：为现有 HLS 动态加载和卸载逻辑补回归测试，不顺手重构播放器。
- 允许修改：`frontend/src/features/watch/VideoPlayer.test.tsx`；仅在测试证明缺陷时最小修改 `VideoPlayer.tsx`。
- 必测：import 完成前卸载、实例销毁、fetch abort、fatal/import 失败回退、卸载后不更新状态。
- 验收：目标测试、全量前端测试、typecheck 和 build。

### OPS-02 恢复与应用回滚 Runbook

- 前置依赖：代码和 Compose 阻断项完成。
- 唯一目标：记录版本回退、真实命名卷恢复、恢复后验证和失败处置流程。
- 允许修改：部署、运维文档；若现有脚本无法安全支持，则另开脚本任务，不在文档任务中伪造已演练结论。
- 验收：命令与现有脚本一致；Docker 可用后完成隔离演练，再决定是否批准生产发布。

### BASE-06 运行态验收

- 前置依赖：以上任务和全仓静态门禁全部通过。
- 唯一目标：执行 Compose 启动、双健康检查、API/浏览器验收和备份恢复演练。
- 当前阻塞环境：Docker Desktop daemon 未运行；这不是代码失败，但在完成前不得宣称生产验收通过。
- Go race 另在 Linux/CGO 可用环境执行，本机未通过不应写成已通过。

非当前批次问题只进入“后续队列”，不得顺手修改。包括 HTTP 空响应指标可能记录状态 `0`、Worker 指标专项测试、迁移 V1 冻结测试、`/livez`/`/readyz`、网关上游健康检查、播放器菜单返焦和完整键盘模型。

## 5. 后续队列

基线通过后按以下顺序推进，每次只领取一项：

1. `GUARD-01`：已完成（2026-09-10）。`internal/httpapi` 的生产 Go 文件由 AST 静态测试禁止导入 `database/sql` 或 `internal/repository`；目标测试与 `go vet` 已通过。
2. `GUARD-02`：已完成（2026-09-10）。`frontend/src/shared` 的生产模块由静态测试禁止反向导入 `features` 或 `app`；目标测试与 TypeScript 类型检查已通过。
3. `CONTRACT-01`：已完成（2026-09-10）。`docs/api-error-contract.md` 已冻结目标成功/错误 envelope、稳定错误码、request ID 与兼容性规则；契约测试只验证规格模型和错误码注册表，未修改生产 Handler、Service 或现有 API 行为。
4. `CONTRACT-02`：已完成（2026-09-10）。`docs/auth-api-contract.md` 已冻结注册、登录、登出和当前会话的请求/响应、Cookie、CSRF、安全及稳定错误码；契约测试只验证规格注册表和 JSON 模型，未修改生产 Handler、Service 或现有 API 行为。
5. `DB-01`：已完成（2026-09-10）。新增冻结历史库 SQL 夹具及空库初始化、历史库升级、迁移失败事务回滚与重试、未来未知版本拒绝且零修改测试；未修改生产迁移代码或业务表结构。
6. `DB-02`：已完成（2026-09-10）。后端对已有持久状态且存在待执行迁移的 SQLite 库，在任何迁移写入前创建、验证并以不覆盖既有文件的方式发布一致快照；备份失败 fail closed，未修改业务表结构。
7. `MODULE-01`：已完成（2026-09-10）。视频举报与审核已按 Handler → Service → Repository → tests 单一纵向切片迁入 `internal/modules/moderation`；公开 API、认证/CSRF、HTTP envelope、稳定错误、重复举报更新语义和数据库 schema 保持不变。
8. `FE-PERF-01`：已完成（2026-09-10）。HLS 继续保持播放路由内动态加载，并新增构建后唯一 chunk、550 kB raw、170 kB gzip 的硬预算；当前 Windows 实测约 509.53 kB / 157.47 kB。Vitest 多 worker 墙钟未稳定改善，继续保留单 worker 和关闭文件并行。
9. `CI-01`：已完成（2026-09-10）。新增独立 Ubuntu Backend Race Job，以 `CGO_ENABLED=1` 执行 `go test -race ./... -count=1`；OpenAPI、迁移、E2E 和安全扫描仍保持为后续独立任务。
10. `OPS-01`：分别设计 `/livez`、`/readyz`、结构化 request/job 日志和告警，不合并成一个大任务。

新功能模块要等当前基线和依赖/契约护栏稳定后再排期；若业务需求紧急，也必须以独立纵向切片进入，不能与架构迁移共享文件。

## 6. 团队调度与验收

- 总控：拆任务、分配文件边界、合并结论、执行最终验证和 Git 操作。
- 架构师：只读审查模块边界、兼容性、安全、迁移与回滚；与总控共同签字。
- 前端/后端：一次只允许一个有冲突风险的写任务；无文件和契约交集时最多两个写任务。
- UI/UX、测试、SRE：默认只读并行审查；发现问题只提交证据和最小修复建议。
- 每个写任务必须包含：唯一目标、前置依赖、允许修改文件、禁止范围、测试、回滚方式、已知未覆盖项。
- 总控和架构师均给出“无阻断项”才可提交；若任一方拒绝，只修复其明确阻断项后复审。

## 7. 最终门禁与 Git

每个原子任务先运行目标测试；批次完成后运行：

```powershell
cd C:\Users\SkyShow\Documents\ChatGPT\GVideo
.\scripts\check.ps1
git diff --check
```

条件允许时再运行：

```powershell
.\scripts\acceptance-api.ps1
.\scripts\acceptance.ps1 -SkipBuildChecks
```

提交前检查 `git status --short`，显式暂存本批文件，按单一意图拆分提交；不 amend、不 force push、不直接推 main。联合验收通过后推送：

```powershell
git push -u origin codex/architecture-hardening
```

如本机没有 GitHub CLI，不虚构 PR；推送后可从以下地址创建 PR：

`https://github.com/zdl000000/GVideo/compare/main...codex/architecture-hardening`
