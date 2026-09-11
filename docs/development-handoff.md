# GVideo 开发交接

> 更新日期：2026-09-11
> 当前分支：`codex/security-hardening`（自 `codex/architecture-hardening` 的 `84125fc` 切出）
> 交接原则：实现、目标验证、只读复审、显式暂存、提交和推送必须串行；分支推送不代表生产发布获批。

## 1. 当前状态

SEC-02「请求速率限制」已完成实现、目标测试、独立只读复审（1 项 P1 已修复）与本轮全量门禁，正在进入 Git 交付阶段。

本批为后端新增零依赖 token bucket 限流：登录/注册按客户端地址、评论与上传按用户分档；超限返回 429 + `Retry-After`；新增 `domain.ErrRateLimited` 与错误契约 `rate_limited`；用自研 `clientAddress` 中间件替换弃用的 chi RealIP（仅信任 loopback/私网对端的 `X-Forwarded-For`/`X-Real-IP`，永不信任 `True-Client-IP`，XFF 取最右侧公网地址，IPv6 按 /64 归一）；网关与前端代理同步清理/覆写转发头。同批附带会话 token 形状预检，避免伪造 cookie 触发数据库查询。

待办：显式暂存本批文件并复核 staged diff 后提交，推送 `codex/security-hardening` 并核对本地/远端一致。不得使用 `git add .`，不得 amend、force push 或直接推送 main。

## 2. 已完成的实现（SEC-02）

- 限流核心：`internal/httpapi/middleware_ratelimit.go`，零依赖 token bucket（rate=perMinute/60，burst=perMinute，桶硬上限 8192 + 随机淘汰，空闲 10 分钟按分钟清扫）。
- 分档：`auth`（登录/注册，按客户端地址）、`comment`（发表评论，按用户）、`upload`（视频与字幕上传，按用户）；用户档挂在 requireAuth→requireCSRF 之后，CSRF 拒绝不消耗配额。
- 配置：`RATE_LIMIT_AUTH_PER_MINUTE`（默认 20）、`RATE_LIMIT_COMMENT_PER_MINUTE`（30）、`RATE_LIMIT_UPLOAD_PER_MINUTE`（10）；`0` 表示禁用该档，负值/非数字启动报错；compose 与 `.env.example` 已透传。
- 错误契约：`domain.ErrRateLimited` → 429「请求过于频繁，请稍后再试」，响应含 `Retry-After`（整数秒向上取整）与 `Connection: close`；`docs/api-error-contract.md` 新增 `rate_limited` 行。
- 客户端地址解析：替换弃用的 `middleware.RealIP`；仅 loopback/私网对端可提供转发地址；XFF 从右向左取最后一个公网条目（抵御 Cloudflare 等"保留客户端前缀"的追加型代理），全私网链回退最右有效地址；IPv6 归一 /64 防止地址轮换绕过。
- 代理加固：网关 `X-Forwarded-For` 由 `$proxy_add_x_forwarded_for` 改为覆写 `$remote_addr`；网关与前端 `/api/`、`/media/` 均清空 `True-Client-IP`。
- 会话预检：`validSessionToken` 在数据库查询前校验 64 位小写十六进制形状，伪造 cookie 不再触发查库。
- 文档：`docs/deployment.md`（限流语义、替换网关必须覆写 XFF、NAT 共享配额、429 与大文件上传说明）。

## 3. 验证证据边界

2026-09-11 本批证据：

- 目标测试：`internal/httpapi` 限流相关 10 项 + `internal/service` 管理员/会话相关 7 项全部通过，覆盖 burst/补充、`Retry-After`、按地址与按用户隔离、`True-Client-IP` 永不信任、伪造前缀与全私网链、IPv6 /64 归一、硬上限淘汰、定时清扫、CSRF 拒绝不消耗配额、伪造会话 token 拒绝。
- 全量后端：`go test ./...`（全包）、`go vet ./...`、`gofmt -l` 通过。
- 全量门禁：`scripts/check.ps1`（Compose 配置、全仓 Go 测试与 vet、前端 14 文件 49 测试、typecheck、build、bundle 预算、`git diff --check`）通过。
- 独立只读复审（agent）：确认自带链路（网关覆写 → 前端追加 → 后端取最右公网）正确；发现 1 项 P1（XFF 取最左段可被追加型上游代理伪造）已修复并补测试；P2/P3 中采纳了会话形状预检、`Connection: close`、`.env.example`、部署文档说明；其余（未认证洪泛的通用限流、LRU 淘汰、acceptance 显式 429 断言、并发 `-race`）记录为后续队列。
- 待补运行时证据：Docker 重建与运行态限流演练（合并阶段执行）。

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
14. `SEC-01`：管理员提权修复（迁移 v2、保留名锁定、moderation 改布尔、`data-grant-admin`、合并携带标志、升级文档）；独立安全复审无 P0，P2/P3 修复完成。
15. `SEC-02`：请求速率限制（auth/comment/upload 分档、429 + Retry-After、可信代理地址解析、会话形状预检、代理头加固）；独立复审 1 项 P1 已修复。本批。

## 5. 当前任务与后续队列

SEC-02 交付后，后续安全与质量队列（按优先级）：

- `SEC-02b`：未认证洪泛的通用限制（GET/retry/delete 与伪 cookie 请求已有形状预检缓解，可加宽松的全局 IP 档）；桶淘汰升级为 LRU；acceptance 脚本增加显式 429 断言；并发 `-race` 用例由 CI 覆盖。
- `SEC-03`：管理端点（`METRICS_ADDR`/`PPROF_ADDR`）非生产环境绑定非 loopback 时的启动警告。
- `SEC-04`：上传配额（每用户总量/频率）与安全响应头（CSP、X-Frame-Options、Referrer-Policy，nginx 侧对齐）。
- `SEC-05`（低危批次）：登录用户名时序枚举防护（dummy bcrypt）、CSRF 恒定时间比较、搜索 LIKE 通配符转义、frontend/nginx 容器降权。
- 既有独立队列：OpenAPI、依赖安全扫描、更广泛 E2E 覆盖；`A2 后续`：notifications/comments/interactions 模块迁移与进程内事件总线。
- 媒体管线独立（Storage 接口缝、worker 出进程、快慢队列）等待触发信号。

## 6. 团队调度与验收

- 总控执行写入、测试和 Git 操作；架构评审、安全复审与前端工程 Agent 只读/受限并行。
- 本批调度：架构师评审（采纳 RealIP 弃用风险、True-Client-IP 伪造、IPv6 /64、硬上限、CSRF 顺序、上传默认值）；DeepSeek 侧总控实现；只读复审 agent 复查 diff（1 项 P1 + 若干 P2/P3，P1 与可快速修复项已处理）。
- 任一审查发现阻断，只修复有证据的最小范围并重新运行相关验证。
- 目标测试、全仓静态门禁和代码只读联合审查均无阻断后，可以提交并推送开发分支。
- 合并和实际生产部署仍遵循仓库审查与组织变更批准。

## 6.1 2026-09-11 SEC-02 交付点

- 分支：`codex/security-hardening`（承接 SEC-01 的 `aba0e6f`）。
- 预期修改文件：`CHANGELOG.md`、`.env.example`、`compose.yaml`、`docs/deployment.md`、`docs/api-error-contract.md`、`docs/development-handoff.md`、`deploy/nginx/https.conf.template`、`frontend/nginx.conf`、`backend/internal/domain/domain.go`、`backend/internal/config/config.go`、`backend/internal/config/config_test.go`、`backend/internal/httpapi/httpapi.go`、`backend/internal/service/service_auth.go`、`backend/internal/service/service_admin_guard_test.go`，以及新增 `backend/internal/httpapi/middleware_ratelimit.go`、`backend/internal/httpapi/middleware_ratelimit_test.go`。
- 静态门禁：`scripts/check.ps1`、`go test ./...`、`go vet`、`gofmt`、`git diff --check` 通过。
- Git 交付顺序：显式暂存上述文件，复核 `git diff --cached --check` 与 staged diff，提交 `feat(security): add request rate limiting`，推送当前开发分支并核对本地/远端一致。
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
