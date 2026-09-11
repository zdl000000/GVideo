# GVideo 开发交接

> 更新日期：2026-09-12
> 当前分支：`codex/security-hardening`（自 `codex/architecture-hardening` 的 `84125fc` 切出）
> 交接原则：实现、目标验证、只读复审、显式暂存、提交和推送必须串行；分支推送不代表生产发布获批。

## 1. 当前状态

本周期为**多 agent 并行集成批次**，三个工作流已完成实现、目标测试、独立只读复审（均为可提交结论）与全量门禁，正在进入 Git 交付阶段：

1. **A2 结构迁移**：通知中心迁入 `backend/internal/modules/notifications/`（moderation 模板：repository/service/handler + 三层测试；`CreateNotification` 跨切写路径按设计保留在核心仓库，事件总线为后续任务）。复审逐行比对确认行为零改变。
2. **E2E 扩容**：新增 `frontend/e2e/comments.spec.ts`（评论旅程 + 空评论拒绝）与 `notifications.spec.ts`（B 上传 → A 评论 → B 收通知 → 全部已读的跨用户旅程），全量 Playwright 12 passed / 2 skipped。
3. **SEC-04b 配额加固**：边界（`used+size == quota` 允许）与「先证配额满、删后释放」的强断言测试；升级指引入 deployment.md。

复审采纳项已落地：架构守卫测试扩展（禁止 modules→core 反向依赖）、模块 repo 镜像注释、无效通知 ID 的 400 契约测试、配额记账精确断言、E2E 种子依赖说明。另记录一个 QA 发现的产品疑点：`/auth?next=/upload` 注册后 `setAuth` 与 `navigate(next)` 存在竞态（AuthPage 已登录重定向可能抢先），列入后续修复。

待办：显式暂存本批文件并复核 staged diff 后提交（三工作流并行集成，单提交交付），推送 `codex/security-hardening` 并核对本地/远端一致。不得使用 `git add .`，不得 amend、force push 或直接推送 main。

## 2. 已完成的实现（三工作流）

### 2.1 A2 结构迁移：notifications 纵向模块

- 新建 `internal/modules/notifications/`：`repository.go`（读侧 List/Mark/MarkAll，SQL 逐字迁移 + mediaURL 镜像注释）、`service.go`（归属校验）、`handler.go`（HTTPPort 模式三端点）+ 三层测试。
- **跨切决策**：`CreateNotification` 保留在核心仓库（follows/comments 写路径调用），模块只拥有读侧；事件总线为后续任务。
- 接线：httpapi.New 第三参数、Routes 三端点改指模块、main.go 装配、notificationsHTTPPort 适配器（保留「通知编号无效」400 文案）；测试接线 15 处 New 调用点改造。
- 删除：`handlers_notifications.go`、`service_notifications.go`；两个核心测试等价迁入模块。

### 2.2 E2E 旅程扩容

- `frontend/e2e/comments.spec.ts`：注册 → 首页首个视频 → 发表评论（含唯一标记）→ 列表断言；空评论按钮禁用。
- `frontend/e2e/notifications.spec.ts`：B 上传（最小 MP4 Buffer）→ A 评论 → 跨登录会话 → B 通知中心断言 → 全部已读。
- 全量 Playwright：12 passed / 2 skipped（既有移动端跳过）；断言全部为条件等待，无硬编码 sleep。

### 2.3 SEC-04b 配额加固与文档

- 边界测试：`used+size == quota` 允许、超出一字节拒绝，并断言成功上传后记账恰好等于自身字节（防封面/HLS 记账漂移假通过）。
- 删除释放测试：先证明配额满（再上传 413/ErrQuotaExceeded），删除后同尺寸上传成功。
- `docs/deployment.md`：存量用户超配额的升级指引（调高/禁用/删除旧投稿）。

## 3. 验证证据边界

2026-09-12 本周期证据：

- 后端：`go test ./... -count=1` 全包通过（含模块内 7 项新测试、守卫测试 `TestModulesDoNotImportCoreLayers`、通知 400 契约往返、配额边界/释放用例）；`go vet ./...`、`gofmt -l` 通过；架构守卫确认模块生产代码零反向依赖 core。
- 前端：`npx tsc -b` 0 错误；单测 14 文件 49 项通过；Playwright 新 spec 4 passed、全量回归 12 passed / 2 skipped（对运行栈实测）。
- 全量门禁：`scripts/check.ps1` 通过。
- 复审：两个分片复审均为「可提交、无 P0/P1」——notifications 迁移逐行比对行为零改变、模块零反向依赖、15 处接线完整；E2E 无硬编码 sleep、通知断言打在真实服务端数据、配额边界语义正确。复审 P2/P3 建议已采纳落地（守卫扩展、镜像注释、400 契约测试、记账精确断言、种子依赖说明）或记录后续（评论删除旅程、点赞/收藏通知、转码完成通知）。
- QA 发现的产品疑点：`/auth?next=...` 注册后 `setAuth` 与 `navigate(next)` 存在竞态，可能被已登录重定向送回首页（E2E 已绕过；产品修复列入后续队列）。
- 待补：容器重建后的运行态验证归下一门禁周期；合并阶段执行完整验收。

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
18. `SEC-05`：低危加固（登录时序 dummy bcrypt、CSRF 恒定时间比较、搜索 LIKE 通配符转义）。
19. `SEC-03b`：网关边缘响应头（自产 429/502/healthz 覆盖、上游同头去重、`nginx -t` 与运行态 429/502 验证）。
20. `A2-1`：notifications 纵向模块（读侧端点迁移、HTTPPort 模式、三层测试、守卫扩展；`CreateNotification` 跨切写保留核心）。本批。
21. `E2E-2`：评论旅程与跨用户通知旅程 spec（4 项，对运行栈实测）。本批。
22. `SEC-04b`：配额边界/删除释放强断言测试与升级指引。本批。

## 5. 当前任务与后续队列

本周期交付后，后续安全与质量队列（按优先级）：

- `SEC-03c`（可选）：CSP 进一步收紧（评估移除 `style-src 'unsafe-inline'` 与 `img-src data:`）；补充 nginx 头行为与主机名/无端口/`[::ffff:127.0.0.1]` 等警告用例。
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

## 6.1 2026-09-12 并行集成批次交付点

- 分支：`codex/security-hardening`（承接 SEC-05 的 `4af54e5`）。
- 预期修改/新增文件（19 个路径，以 `git status` 为准）：删除 `backend/internal/httpapi/handlers_notifications.go`、`backend/internal/service/service_notifications.go`；新增 `backend/internal/modules/notifications/`（6 文件）、`backend/internal/httpapi/notifications_http_test.go`、`frontend/e2e/comments.spec.ts`、`frontend/e2e/notifications.spec.ts`；修改 `CHANGELOG.md`、`docs/deployment.md`、`docs/development-handoff.md`、`backend/cmd/server/main.go`、`backend/internal/httpapi/{architecture_guard_test.go,health_test.go,httpapi.go,httpapi_test.go,middleware_ratelimit_test.go,quota_http_test.go}`、`backend/internal/repository/{repository_notifications.go,repository_test.go}`、`backend/internal/service/{service_admin_guard_test.go,service_quota_test.go,service_test.go}`。
- 静态门禁：`scripts/check.ps1`、`go test ./... -count=1`、`go vet`、`gofmt`、`git diff --check` 通过。
- 运行门禁：backend/frontend 容器 healthy；Playwright 新旅程对运行栈实测通过。
- Git 交付顺序：显式暂存上述文件，复核 `git diff --cached --check` 与 staged diff，提交 `feat: integrate notifications module, e2e journeys and quota hardening`，推送当前开发分支并核对本地/远端一致。
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
