# GVideo 开发交接

> 更新日期：2026-09-12
> 当前分支：`codex/security-hardening`（自 `codex/architecture-hardening` 的 `84125fc` 切出）
> 交接原则：实现、目标验证、只读复审、显式暂存、提交和推送必须串行；分支推送不代表生产发布获批。

## 1. 当前状态

SEC-03b「网关边缘响应头」已完成实现、容器化 `nginx -t` 语法验证与运行态验证（自产 200/429/502 三类响应的头检查），正在进入 Git 交付阶段。

本批为 HTTPS 网关 443 server 增加边缘安全头（`X-Content-Type-Options`、`X-Frame-Options`、`Referrer-Policy`、`Permissions-Policy`），覆盖网关自产响应（`limit_req` 429、上游 502、`/gateway-healthz`）；`location /` 经 `proxy_hide_header` 隐藏上游同头副本、由网关统一重加，浏览器每个头只收到一份；SPA 的 CSP 由前端源下发并原样透传（网关不注入 CSP，避免与前端白名单冲突）。

待办：显式暂存本批 4 个文件并复核 staged diff 后提交，推送 `codex/security-hardening` 并核对本地/远端一致。不得使用 `git add .`，不得 amend、force push 或直接推送 main。

## 2. 已完成的实现（SEC-03b）

- 网关 443 server 级新增四项安全头（`always`），覆盖网关自产的 `limit_req` 429、上游 502 与 `/gateway-healthz` 响应。
- `location /` 增加 `proxy_hide_header` 隐藏上游（frontend 源）的同名头副本，网关统一重加；前端源下发的 CSP 不在隐藏清单内，原样透传。
- `deployment.md` 增补「边缘响应头」说明：替换网关时自产错误页必须有安全头，且不得在边缘注入与前端不同的 CSP。

## 3. 验证证据边界

2026-09-12 本批证据：

- 语法：自签证书 + `gvideo-frontend` 镜像官方入口（envsubst 模板渲染）执行 `nginx -t` 通过。
- 运行态：独立容器发布 18443 → `curl -skI` 实测：`/gateway-healthz` 200 带全组头；`/api/v1/videos`（上游不可达）502 带全组头；突发 16 次登录请求触发 `limit_req`（11×502 → 5×429），429 响应带全组头且 `X-Frame-Options` 计数恰为 1、无上游 CSP 透传。
- 复审：本批无独立复审（配置变更无测试面），以容器化 `nginx -t` 与运行态头检查作为证据；既有安全复审对本批的前置发现（网关自产响应缺头）即为本批范围。
- 待补：生产 HTTPS 链路的完整验收归合并阶段（`preflight-production.ps1` + 真实证书）。

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
19. `SEC-03b`：网关边缘响应头（自产 429/502/healthz 覆盖、上游同头去重、`nginx -t` 与运行态 429/502 验证）。本批。

## 5. 当前任务与后续队列

SEC-03b 交付后，后续安全与质量队列（按优先级）：

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

## 6.1 2026-09-12 SEC-03b 交付点

- 分支：`codex/security-hardening`（承接 SEC-05 的 `4af54e5`）。
- 预期修改文件（4 个）：`CHANGELOG.md`、`docs/deployment.md`、`docs/development-handoff.md`、`deploy/nginx/https.conf.template`。
- 验证门禁：容器化 `nginx -t` 通过；运行态（独立网关容器 18443）`/gateway-healthz` 200、上游不可达 502、`limit_req` 429 三类响应均带全组边缘头且每头一份（`X-Frame-Options` 计数 1、无上游 CSP 透传）；`scripts/check.ps1` 通过。
- Git 交付顺序：显式暂存上述文件，复核 `git diff --cached --check` 与 staged diff，提交 `feat(security): add gateway edge response headers`，推送当前开发分支并核对本地/远端一致。
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
