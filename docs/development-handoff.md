# GVideo 开发交接

> 更新日期：2026-09-11
> 当前分支：`codex/architecture-hardening`
> 交接原则：实现、目标验证、只读复审、显式暂存、提交和推送必须串行；分支推送不代表生产发布获批。

## 1. 当前状态

BASE-06 已完成。本次“播放器完整键盘模型”已完成实现、缺陷修复、全仓静态门禁、最终 Docker 重建、健康检查、桌面 Playwright E2E 与联合只读终审，当前进入 Git 交付阶段。最终预期修改共 7 个文件，包含新增的路由切换失败回归测试；提交和推送前仍须显式暂存并复核 staged diff。不得使用 `git add .`，不得 amend、force push 或直接推送 main。Docker Compose backend/frontend 保持 healthy，gateway 孤儿容器及数据卷按要求保留。

## 2. 已完成的实现

- 后端 Handler、Service、Repository 按职责拆分，并新增分层边界守卫。
- metrics/pprof 使用独立、默认关闭且受生产地址校验保护的管理端口；端口占用时 fail-fast。
- SQLite 迁移账本、checksum、事务回滚、历史夹具、迁移前一致快照和 fail-closed 备份已实现。
- React 路由错误边界、播放器快捷键作用域、HLS 状态/重试/异步生命周期与 bundle 硬预算已实现。
- moderation 举报审核已迁入独立纵向模块，公开 API 与数据库 schema 保持兼容。
- `/livez`、`/readyz`、模板 route HTTP 指标/日志、媒体任务结构化日志和供应商无关告警 Runbook 已实现。
- Compose `stop_grace_period` 和版本回退/真实命名卷恢复 Runbook 已实现；停止宽限期与隔离备份恢复已取得运行态演练证据。

## 3. 验证证据边界

2026-09-11 针对本次播放器键盘模型最终 diff 运行 `./scripts/check.ps1` 并通过：Compose 配置、全仓 Go test/vet、前端 14 个测试文件共 49 项测试、TypeScript typecheck、Vite build、bundle 硬预算及 `git diff --check` 均通过。播放器与播放页定向测试为 23 项通过；HLS 构建产物 509.54 kB raw / 155.55 kB gzip，低于硬预算。最终修复包含页面级单栏宽屏布局、相关推荐下移、路由切换复位并清空旧视频/评论、全屏结果播报、PiP 拒绝处理、稳定的宽屏按钮名称及相应回归测试。

最终源代码已通过 `docker compose up --build -d --force-recreate` 重建；backend/frontend 均为 running/healthy，`/livez`、`/readyz`、frontend `/healthz` 和首页均返回 HTTP 200，未使用 `--remove-orphans`，gateway 容器和卷均保留。新增播放器目标桌面 E2E 为 1 项通过；随后 desktop-chromium 串行完整运行 4 项通过、1 项按移动端条件跳过。并发完整 E2E 曾因 Windows Docker 端口瞬时拒绝连接失败，服务未退出且健康检查持续通过，使用单 worker 稳定复跑后全部桌面用例通过。

2026-09-10 的 BASE-06 运行证据：`docker compose up --build -d` 成功，backend/frontend 均为 `running|healthy|0`；`/livez`、`/readyz`、backend `/healthz`、同源 `/healthz` 和首页均返回 HTTP 200。backend 以 30 秒超时停止时在 16.96 秒内收到 SIGTERM（signal 15）并以 exit 0、OOMKilled=false 退出，无 SIGKILL，随后恢复 healthy。

`./scripts/acceptance.ps1 -SkipBuildChecks -IncludeBackupRestore -KeepDrillBackup` 在 05:41 内通过：完整 API 用户旅程、实际 FFmpeg/HLS/字幕/Range 验收通过，Playwright 7 项通过、1 项按条件跳过，隔离备份恢复演练及数据库/媒体 SHA-256 校验通过。审计备份保留在被 Git 忽略的 `backups/gvideo-drill-20260910-081404-350Z-31aac034f7e844fc862ee4e9ac75b8e9/`。

Pull Request [#16](https://github.com/zdl000000/GVideo/pull/16) 在提交 `cae1e679c9fe8ac002db6cac57709629a4e23161` 上触发 [CI run 34454478806](https://github.com/zdl000000/GVideo/actions/runs/34454478806)，Backend、Frontend 和启用 `CGO_ENABLED=1` 的 [Backend Race](https://github.com/zdl000000/GVideo/actions/runs/34454478806/job/102797561522) 均成功。
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

## 5. 当前任务与后续队列

本轮代码任务及 BASE-06 已完成。最终修复覆盖 access log 测试稳定性、媒体失败 metric/log 一致性、cleanup/startup 日志脱敏、预期 Worker gauge 缺失告警、低流量 backlog 恢复语义及交接状态，并已通过代码安全、可观测性和文档三方只读终审。Docker Compose 运行态、健康检查、停止宽限期、完整 API/浏览器验收、隔离备份恢复演练与 Linux/CGO race 均已有成功证据。

后续独立队列中的播放器完整键盘模型已完成实现、静态验证、Docker 运行验证和桌面 E2E：快捷键作用域收回到可聚焦播放器区域，支持播放/暂停、分级跳转、音量、静音、字幕、宽屏和全屏，并补齐快捷键说明、状态语义、边界处理、单元测试及桌面 E2E 用例；播放器外及交互控件不再被劫持。剩余独立队列为 OpenAPI、安全扫描和更广泛的 E2E 覆盖；这些不属于本批播放器交付，也不阻断本轮分支进入合并审查。网关上游健康检查已完成实现与运行态验收：保留 Nginx liveness，并增加经 frontend 到 backend `/readyz` 的 readiness、动态 Docker DNS 解析和生产预检语义断言；backend 停止时 liveness 保持 200、readiness 返回非 2xx 且 gateway 进入 unhealthy，backend 恢复和 frontend 重建换址后 gateway 无需重建即可恢复 healthy。通过仅在显式设置 `E2E_IGNORE_HTTPS_ERRORS=true` 时允许自签证书的 Playwright 验证，HTTPS 网关页面测试为 7 项通过、1 项按条件跳过；生产默认仍严格校验证书。实际生产部署、密钥注入、TLS、监控产品接入与变更窗口仍需在目标环境按组织流程批准和执行。
## 6. 团队调度与验收

- 总控执行写入、测试和 Git 操作；代码安全、可观测性和文档审查 Agent 只读并行。
- 任一审查发现阻断，只修复有证据的最小范围并重新运行相关验证。
- 目标测试、全仓静态门禁和代码只读联合审查均无阻断后，可以提交并推送开发分支。
- BASE-06 与 Linux/CGO race 已取得实际成功证据；合并和实际生产部署仍遵循仓库审查与组织变更批准。


## 6.1 2026-09-11 最终交付点

- 分支：`codex/architecture-hardening`；本批起点为 `cf1fc07`，全部既有未提交修改均已保留并纳入复核。
- 预期修改文件共 7 个：`docs/development-handoff.md`、`frontend/e2e/app.spec.ts`、`frontend/src/features/watch/VideoPage.test.tsx`、`frontend/src/features/watch/VideoPage.tsx`、`frontend/src/features/watch/VideoPlayer.test.tsx`、`frontend/src/features/watch/VideoPlayer.tsx`、`frontend/src/styles.css`。
- 最终静态门禁：`scripts/check.ps1` 通过，前端 14 个测试文件、49 项测试通过；播放器/播放页定向测试 23 项通过；typecheck、build、bundle budget 和 `git diff --check` 通过。
- 最终运行门禁：Compose 使用最终源代码完成 build/recreate，backend/frontend healthy；四个 HTTP 端点均为 200；目标桌面 E2E 1 项通过，完整 desktop-chromium 串行 E2E 4 项通过、1 项按条件跳过。
- Git 交付顺序：联合只读终审无 P0/P1 后，显式暂存上述 7 个文件，复核 `git diff --cached --check`、stat 和 staged diff，提交 `feat(frontend): complete player keyboard controls`，推送当前开发分支并核对本地/远端提交一致。
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
git -c http.version=HTTP/1.1 push -u origin codex/architecture-hardening
```

本机没有 GitHub CLI 时不虚构 PR。推送后可从以下地址创建 PR：

https://github.com/zdl000000/GVideo/compare/main...codex/architecture-hardening
