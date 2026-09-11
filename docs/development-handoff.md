# GVideo 开发交接

> 更新日期：2026-09-11
> 当前分支：`codex/security-hardening`（自 `codex/architecture-hardening` 的 `84125fc` 切出）
> 交接原则：实现、目标验证、只读复审、显式暂存、提交和推送必须串行；分支推送不代表生产发布获批。

## 1. 当前状态

SEC-01「管理员提权修复」已完成实现、全量测试、独立只读安全复审（结论：无 P0，提权面关闭）与复审问题修复，正在执行最终静态门禁与 Git 交付。

本批将管理员身份从「按用户名实时比较」改为 `users.is_admin` 持久标志（迁移 v2），关闭了改名提权、保留名大小写变体抢占与合并丢标三条路径；新增 `gvideo data-grant-admin` 显式授予命令、启动配置 fail-fast 与未认领告警。后端 `go test ./... -count=1`、`go vet`、`gofmt` 与本轮 `scripts/check.ps1` 已通过（门禁结果见 §3）。

待办：显式暂存本批 20 个文件并复核 staged diff 后提交，推送 `codex/security-hardening` 开发分支并核对本地/远端一致。不得使用 `git add .`，不得 amend、force push 或直接推送 main。

## 2. 已完成的实现（SEC-01）

- 数据库迁移 v2：`users.is_admin INTEGER NOT NULL DEFAULT 0`（`ensureColumn` 幂等；v1 identity 与 checksum 未改动，老库升级前自动备份、reopen 幂等由 fixture 覆盖）。
- 注册：用户名匹配 `ADMIN_USERNAME`（忽略大小写与首尾空白）且库中尚无管理员时，通过单条原子 `INSERT ... WHERE NOT EXISTS(管理员)` 授予并返回会话；已有管理员时保留名注册返回 409。
- 改资料：改名到保留名（含大小写变体）返回 403；本人名称未变的资料编辑不受影响；管理员标志不随改名丢失。
- 身份判定：用户与 Session 查询直接携带 `is_admin`；`moderation` 模块改为接收会话布尔值，不再自行比较用户名；没有任何 API 可写该标志。
- 启动：`ADMIN_USERNAME` 不符合用户名规则时 fail-fast 退出；配置了名称但尚无管理员时记录 `admin_unclaimed` 警告，不写库（取消原「启动自动认领」设计，避免把已占用该名称的入侵者合法化）。
- 运维命令：新增 `gvideo data-grant-admin <username>`（不存在用户报错并包含用户名；重复授予幂等）。
- 合并：`MergeDatabase` 在源库存在 `is_admin` 列时把管理员标志携带到目标库，避免合并静默丢失唯一管理员。
- 文档：`docs/deployment.md`（管理语义与升级说明）、`docs/operations.md`（授予与恢复）、`CHANGELOG.md` 已同步。

## 3. 验证证据边界

2026-09-11 本批证据：

- `go test ./... -count=1`：cmd/server、config、httpapi、media、moderation、platform、repository、service 全部通过。新增回归用例覆盖：保留名首认领、大小写变体 409、改名 403、空/空白 `ADMIN_USERNAME`、CLI 授予与重复授予、合并携带管理员标志、迁移账本与夹具 v1+v2。
- `go vet ./...`、`gofmt -l`、`git diff --check` 通过；`scripts/check.ps1`（Compose 配置、全仓 Go 测试与 vet、前端 14 个测试文件 49 项测试、typecheck、build、bundle 预算）通过。
- 独立只读安全复审（agent）：无 P0；确认写 `is_admin` 仅剩「首次注册」与「CLI 授予」两个入口，session 每请求读库使授予即时生效，迁移 v2 与既有 fixture 兼容。复审提出的 2 项 P2（合并丢标、文档升级说明）与 3 项 P3（创建与授权非原子、CLI 错误包装、测试缺口）已全部修复并复验通过。
- 前端：本批无源码变更。路由级懒加载、错误边界与 bundle 预算在上批已存在；复核确认 `frontend/src/app/App.tsx` 与 HEAD 一致（一次不必要的 fallback 文案改动已还原）。

历史证据（BASE-06，2026-09-10/11）：`scripts/check.ps1`、`docker compose up --build -d --force-recreate`、`/livez`、`/readyz`、frontend `/healthz`、桌面 Playwright E2E、`acceptance.ps1 -SkipBuildChecks -IncludeBackupRestore` 与 Linux/CGO Backend Race CI 均已通过；详见 git 历史与 Pull Request #16。

## 4. 已完成队列

1. `BASE-03A`：管理监听地址生产安全校验。
2. `BASE-03B`：管理端口独立启停和端口冲突 fail-fast 测试。
3. `OPS-01`：backend `stop_grace_period: 30s` 配置；运行态停止验证归入 BASE-06。
4. `FE-01`～`FE-04`：懒加载错误边界、播放器快捷键作用域、HLS 状态与异步生命周期测试。
5. `OPS-02`：应用回退和真实命名卷恢复 Runbook；隔离演练归入 BASE-06。
6. `GUARD-01`、`GUARD-02`：后端 HTTP 持久层和前端 shared 反向依赖守卫。
7. `CONTRACT-01`、`CONTRACT-02`：API 错误与 Auth API 契约。
8. `DB-01`、`DB-02`：迁移夹具及迁移前备份门禁。
9. `MODULE-01`：moderation 纵向模块。
10. `FE-PERF-01`：HLS 动态加载 bundle 预算。
11. `CI-01`：Ubuntu Backend Race Job workflow 已实现，并由 Pull Request #16 在 Linux/CGO 环境实际运行通过。
12. `OPS-OBS-01A`～`01D`：存活/就绪端点、结构化日志及告警规格；最终审查修复已在 `8be7007` 完成、复审并推送。
13. `BASE-06`：Compose 运行态、健康检查、优雅停止、完整验收、隔离备份恢复和 Linux/CGO race 证据闭环。
14. `SEC-01`：管理员提权修复（迁移 v2、保留名锁定、moderation 改布尔、`data-grant-admin`、合并携带标志、升级文档）；独立安全复审无 P0，P2/P3 修复完成。本批。

## 5. 当前任务与后续队列

SEC-01 交付后，后续安全与质量队列（按优先级）：

- `SEC-02`：登录/注册/评论/上传速率限制（零依赖 token bucket；429 + Retry-After；按 IP/用户分档）。
- `SEC-03`：管理端点（`METRICS_ADDR`/`PPROF_ADDR`）默认绑定策略加固：非生产环境绑定非 loopback 时启动警告，生产拒绝；文档补充。
- `SEC-04`：上传配额（每用户总量/频率）与安全响应头（CSP、X-Frame-Options、Referrer-Policy，nginx 侧对齐）。
- `SEC-05`（低危批次）：登录用户名时序枚举防护（dummy bcrypt）、CSRF 恒定时间比较、RealIP 可信代理白名单、搜索 LIKE 通配符转义、frontend/nginx 容器降权。
- 既有独立队列：OpenAPI、依赖安全扫描、更广泛 E2E 覆盖；`A2 后续`：notifications/comments/interactions 模块迁移与进程内事件总线。
- 媒体管线独立（Storage 接口缝、worker 出进程、快慢队列）等待触发信号。

## 6. 团队调度与验收

- 总控执行写入、测试和 Git 操作；架构评审、安全复审与前端工程 Agent 只读/受限并行。
- 本批调度：GLM 侧架构师评审（采纳取消启动自动认领、保留名检查、合并路径核查）；DeepSeek 侧总控实现；只读安全复审 agent 复查 diff 后修复全部 P2/P3；前端工程 agent 复核懒加载现状（结论：已存在，仅还原一处文案）。
- 任一审查发现阻断，只修复有证据的最小范围并重新运行相关验证。
- 目标测试、全仓静态门禁和代码只读联合审查均无阻断后，可以提交并推送开发分支。
- 合并和实际生产部署仍遵循仓库审查与组织变更批准。

## 6.1 2026-09-11 SEC-01 交付点

- 分支：`codex/security-hardening`；起点为 `codex/architecture-hardening` 的 `84125fc`。
- 预期修改文件（20 个）：`CHANGELOG.md`、`docs/deployment.md`、`docs/operations.md`、`docs/development-handoff.md`、`backend/cmd/server/main.go`、`backend/cmd/server/main_test.go`、`backend/internal/httpapi/httpapi.go`、`backend/internal/httpapi/httpapi_test.go`、`backend/internal/modules/moderation/{handler.go,handler_test.go,service.go,service_test.go}`、`backend/internal/platform/{database.go,database_test.go,migrations.go,migrations_fixture_test.go}`、`backend/internal/repository/{repository_sessions.go,repository_users.go}`、`backend/internal/service/{service_auth.go,service_users.go}`，以及新增 `backend/internal/service/service_admin_guard_test.go`。
- 静态门禁：`scripts/check.ps1` 通过；`go test ./... -count=1`、`go vet`、`gofmt`、`git diff --check` 通过。
- Git 交付顺序：显式暂存上述文件，复核 `git diff --cached --check` 与 staged diff，提交 `fix(security): close admin privilege escalation`，推送当前开发分支并核对本地/远端提交一致。
- 不创建自动守护任务；本批工作在当前会话内完成交付。

## 7. 最终门禁与 Git

```powershell
cd C:\Users\SkyShow\Documents\ChatGPT\GVideo
.\scripts\check.ps1
git diff --check
git status -sb
```

提交前检查 `git status --short`，只显式暂存本批实际修改文件；运行 `git diff --cached --check` 后提交。推送使用：

```powershell
git -c http.version=HTTP/1.1 push -u origin codex/security-hardening
```

本机没有 GitHub CLI 时不虚构 PR。推送后可从以下地址创建 PR：

https://github.com/zdl000000/GVideo/compare/main...codex/security-hardening
