# GVideo 开发交接

> 更新日期：2026-09-12
> 当前分支：`codex/security-hardening`（自 `codex/architecture-hardening` 的 `84125fc` 切出）
> 交接原则：实现、目标验证、只读复审、显式暂存、提交和推送必须串行；分支推送不代表生产发布获批。

## 1. 当前状态

SEC-05「低危加固」已完成实现、目标测试、后端全量测试、独立只读复审与本轮全量门禁，正在进入 Git 交付阶段。

本批修复三项低危发现：登录对不存在用户执行预计算 dummy bcrypt 比较（消除用户名枚举时序差）；CSRF 比较改为 `crypto/subtle` 恒定时间（保留空 expected 直接拒绝）；搜索关键词转义 LIKE 通配符（`%`、`_`、`\`，SQL 增加 `ESCAPE '\'`），防止 `%` 构造全表匹配扫描。

待办：显式暂存本批文件并复核 staged diff 后提交，推送 `codex/security-hardening` 并核对本地/远端一致。不得使用 `git add .`，不得 amend、force push 或直接推送 main。

## 2. 已完成的实现（SEC-05）

- 登录时序：`service.Login` 在用户不存在时比较预计算的 `dummyPasswordHash`（包级常量，无启动开销），未知用户与错误密码走同一错误与近似耗时路径。
- CSRF：`requireCSRF` 使用 `subtle.ConstantTimeCompare` 并前置长度比较；`expected` 为空仍直接拒绝（防御纵深，正常会话的 token 为 24 字节 hex 编码的 48 个字符）。
- 搜索：`likeEscaper` 转义 `\`、`%`、`_`，`videoFilterSQL` 的三处 LIKE 增加 `ESCAPE '\'`；用户输入的通配符按字面匹配。
- 测试：`repository_search_test.go`（`100%`、`a_b` 字面匹配与普通搜索对照）、`TestLoginUnknownUserReturnsUnauthorized`（未知用户/错密码/正确登录三路径）。
- 文档：`CHANGELOG.md` 已同步。

## 3. 验证证据边界

2026-09-12 本批证据：

- 目标测试：`TestSearchTreatsLikeWildcardsLiterally`、`TestLoginUnknownUserReturnsUnauthorized`、既有 CSRF 契约与限流/配额/管理员用例全部通过。
- 后端全量：`go test ./...`（全包）、`go vet ./...`、`gofmt -l` 通过。
- 全量门禁：`scripts/check.ps1` 通过。
- 独立只读复审（agent）：结论记录于 §6.1。

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
14. `SEC-01`：管理员提权修复（迁移 v2、保留名锁定、moderation 改布尔、`data-grant-admin`、合并携带标志、升级文档）。
15. `SEC-02`：请求速率限制（auth/comment/upload 分档、429 + Retry-After、可信代理地址解析、会话形状预检、代理头加固）。
16. `SEC-03`：安全响应头（后端 deny-all CSP + 框架拒绝；SPA CSP 含 blob/worker 白名单；主题脚本外置）与管理端点非 loopback 绑定警告；顺手修复 `/livez` 代理缺失。
17. `SEC-04`：每用户上传存储配额（迁移 v3 计量封面与 HLS、413 `storage_quota_exceeded`、Compose 默认 20 GiB）。
18. `SEC-05`：低危加固（登录时序 dummy bcrypt、CSRF 恒定时间比较、搜索 LIKE 通配符转义）。本批。

## 5. 当前任务与后续队列

SEC-05 交付后，后续安全与质量队列（按优先级）：

- `SEC-03b`：网关（`deploy/nginx/https.conf.template`）443 自产响应（`limit_req` 429、502、`/gateway-healthz`）补齐同组响应头，并评估经由 `proxy_hide_header` 去重上游重复头；CSP 进一步收紧（评估移除 `style-src 'unsafe-inline'` 与 `img-src data:`）。
- `SEC-04b`：配额可观测性与运维完善——超配额计数指标/Runbook、存量用户超过默认 20 GiB 时的升级指引与扩容说明、字幕字节计量、`processing` 卡死时的人工释放流程、配额精确边界（`used+size == quota`）与删除释放的自动化用例。
- `SEC-05b`：frontend/nginx 容器降权（改用 nginx-unprivileged 镜像或调整监听端口，涉及 compose 端口映射变更，需运行态演练）。
- 既有独立队列：OpenAPI、依赖安全扫描、更广泛 E2E 覆盖；`A2 后续`：notifications/comments/interactions 模块迁移与进程内事件总线。
- 媒体管线独立（Storage 接口缝、worker 出进程、快慢队列）等待触发信号。

## 6. 团队调度与验收

- 总控执行写入、测试和 Git 操作；架构评审、安全复审与前端工程 Agent 只读/受限并行。
- 历批调度：SEC-01/02 由架构师评审 + 只读复审双环节；SEC-03/04/05 由总控实现 + 只读复审 agent 复查 diff（发现项按严重度当批修复或记录后续）。
- 任一审查发现阻断，只修复有证据的最小范围并重新运行相关验证。
- 目标测试、全仓静态门禁和代码只读联合审查均无阻断后，可以提交并推送开发分支。
- 合并和实际生产部署仍遵循仓库审查与组织变更批准。

## 6.1 2026-09-12 SEC-05 交付点

- 分支：`codex/security-hardening`（承接 SEC-04 的 `3206b2e`）。
- 预期修改文件（7 个）：`CHANGELOG.md`、`docs/development-handoff.md`、`backend/internal/httpapi/middleware.go`、`backend/internal/repository/repository.go`、`backend/internal/repository/repository_search_test.go`（新增）、`backend/internal/service/service_admin_guard_test.go`、`backend/internal/service/service_auth.go`。
- 静态门禁：`scripts/check.ps1`、`go test ./...`、`go vet`、`gofmt`、`git diff --check` 通过。
- Git 交付顺序：显式暂存上述文件，复核 `git diff --cached --check` 与 staged diff，提交 `fix(security): harden login timing, CSRF compare and search escaping`，推送当前开发分支并核对本地/远端一致。
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
