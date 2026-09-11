# GVideo 开发交接

> 更新日期：2026-09-11
> 当前分支：`codex/security-hardening`（自 `codex/architecture-hardening` 的 `84125fc` 切出）
> 交接原则：实现、目标验证、只读复审、显式暂存、提交和推送必须串行；分支推送不代表生产发布获批。

## 1. 当前状态

SEC-03「安全响应头与管理端点绑定警告」已完成实现、目标测试、容器运行态验证（响应头检查 + Playwright CSP 烟雾）、独立只读复审（无本批 P0/P1，顺手修复一项既有 P1）与全量门禁，正在进入 Git 交付阶段。

本批为后端所有响应增加框架拒绝、Referrer-Policy、Permissions-Policy 与 deny-all CSP；前端 Nginx 为 SPA 设置同组头与含 hls.js 所需 `blob:`/`worker:` 白名单的 CSP，并关闭 `server_tokens`；为满足 `script-src 'self'`，把内联主题初始化脚本外置为 `frontend/public/theme-init.js`（no-cache 重验证）；修复前端 `/livez` 缺失代理（此前被 SPA 回退的 200 HTML 掩盖后端宕机）；非生产环境管理端点绑定非 loopback 时记录 `diagnostics_binding_exposed` 警告。

待办：显式暂存本批文件并复核 staged diff 后提交，推送 `codex/security-hardening` 并核对本地/远端一致。不得使用 `git add .`，不得 amend、force push 或直接推送 main。

## 2. 已完成的实现（SEC-03）

- 后端 `responseHeaders`：全部响应携带 `X-Content-Type-Options: nosniff`、`X-Frame-Options: DENY`、`Referrer-Policy: strict-origin-when-cross-origin`、`Permissions-Policy: camera=(), microphone=(), geolocation=()`、`Content-Security-Policy: default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'`；敏感路径保持 `Cache-Control: no-store`。
- 前端 Nginx（server 级）：同组头 + SPA CSP（`script-src 'self'`；`style-src 'self' 'unsafe-inline'`；`img-src 'self' data: blob:`；`media-src 'self' blob:`；`worker-src 'self' blob:`；`child-src 'self' blob:`；`connect-src 'self'`；`frame-ancestors 'none'`；`base-uri 'self'`；`form-action 'self'`；`object-src 'none'`），`server_tokens off`。
- CSP 兼容性：内联主题脚本外置为 `frontend/public/theme-init.js`（保持 `script-src 'self'` 严格性），并加 `expires -1` 让客户端以 ETag 重验证而非启发式缓存；`child-src` 兼容 Safari < 15.4 的 blob Worker。
- 既有 P1 修复：前端 Nginx 增加 `location = /livez` 代理到后端，外部存活探测不再被 SPA 回退的 200 HTML 掩盖（与 `docs/operations.md` 探测规格一致）。
- 管理端点：`diagnosticsBindingWarnings` 在非生产环境对非 loopback 的 `METRICS_ADDR`/`PPROF_ADDR` 记录 `diagnostics_binding_exposed`；生产环境仍由 config 校验直接拒绝。
- 文档：`docs/deployment.md` 补充响应头语义、CSP 维护须知（替换反代必须保留 `blob:`/`worker:` 白名单）与管理端点警告说明。

## 3. 验证证据边界

2026-09-11 本批证据：

- 目标测试：`middleware_headers_test.go`（`/`、`/livez`、`/readyz`、`/api/`、`/media/` 全路径头断言）、`main_test.go` 警告分类用例、既有 httpapi/service 定向用例全部通过；`go vet`、`gofmt` 通过。
- 运行态（容器重建 `docker compose up -d --build backend frontend`）：backend/frontend healthy；`curl -I` 实测头齐全；`/livez` 返回 `application/json` 65B（修复前为 794B `text/html`）；`/theme-init.js` 带 `Cache-Control: no-cache` 与 ETag；`/readyz` 200。
- 浏览器烟雾（Playwright，构建产物经 Nginx）：首页 4 张视频卡片渲染、播放页 video 存在且 `play()` 后 `currentTime > 0`（HLS 在严格 CSP 下可播）、控制台零 CSP 违规。
- 全量门禁：`scripts/check.ps1` 通过。
- 独立只读复审（agent）：无本批引入的 P0/P1；确认 deny-all CSP 对 HLS 分片/m3u8/VTT/Range 无副作用、`add_header` 继承与 `always` 生效、警告分类边界正确。遗留 P2/P3 记录在后续队列（网关自产响应头、头重复去重、CSP 进一步收紧、测试缺口）。

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
16. `SEC-03`：安全响应头（后端 deny-all CSP + 框架拒绝；SPA CSP 含 blob/worker 白名单；主题脚本外置）与管理端点非 loopback 绑定警告；顺手修复 `/livez` 代理缺失。本批。

## 5. 当前任务与后续队列

SEC-03 交付后，后续安全与质量队列（按优先级）：

- `SEC-03b`：网关（`deploy/nginx/https.conf.template`）443 自产响应（`limit_req` 429、502、`/gateway-healthz`）补齐同组响应头，并评估经由 `proxy_hide_header` 去重上游重复头；CSP 进一步收紧（评估移除 `style-src 'unsafe-inline'` 与 `img-src data:`）；补充 nginx 头行为与主机名/无端口/`[::ffff:127.0.0.1]` 等警告用例。
- `SEC-04`：上传配额（每用户总量/频率）。
- `SEC-05`（低危批次）：登录用户名时序枚举防护（dummy bcrypt）、CSRF 恒定时间比较、搜索 LIKE 通配符转义、frontend/nginx 容器降权。
- 既有独立队列：OpenAPI、依赖安全扫描、更广泛 E2E 覆盖；`A2 后续`：notifications/comments/interactions 模块迁移与进程内事件总线。
- 媒体管线独立（Storage 接口缝、worker 出进程、快慢队列）等待触发信号。

## 6. 团队调度与验收

- 总控执行写入、测试、容器重建和 Git 操作；架构评审、安全复审与前端工程 Agent 只读/受限并行。
- 本批调度：DeepSeek 侧总控实现与运行态验证；只读复审 agent 复查 diff（无本批 P0/P1，识别一项既有 P1 已修复；P2/P3 记录后续）。
- 任一审查发现阻断，只修复有证据的最小范围并重新运行相关验证。
- 目标测试、全仓静态门禁和代码只读联合审查均无阻断后，可以提交并推送开发分支。
- 合并和实际生产部署仍遵循仓库审查与组织变更批准。

## 6.1 2026-09-11 SEC-03 交付点

- 分支：`codex/security-hardening`（承接 SEC-02 的 `02e3a85`）。
- 预期修改文件：`CHANGELOG.md`、`docs/deployment.md`、`docs/development-handoff.md`、`deploy/nginx/https.conf.template`、`frontend/index.html`、`frontend/nginx.conf`、`backend/cmd/server/main.go`、`backend/cmd/server/main_test.go`、`backend/internal/httpapi/middleware.go`，以及新增 `backend/internal/httpapi/middleware_headers_test.go`、`frontend/public/theme-init.js`。
- 静态门禁：`scripts/check.ps1`、`go test ./...`、`go vet`、`gofmt`、`git diff --check` 通过。
- 运行门禁：backend/frontend 重建后 healthy；`/livez`、`/readyz` 与首页均 200；Playwright CSP 烟雾通过（含 HLS 播放与控制台零 CSP 违规）。
- Git 交付顺序：显式暂存上述文件，复核 `git diff --cached --check` 与 staged diff，提交 `feat(security): add security response headers`，推送当前开发分支并核对本地/远端一致。
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
