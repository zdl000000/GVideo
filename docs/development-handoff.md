# GVideo 开发交接

> 更新日期：2026-09-12
> 当前分支：`codex/security-hardening`（自 `codex/architecture-hardening` 的 `84125fc` 切出）
> 交接原则：实现、目标验证、只读复审、显式暂存、提交和推送必须串行；分支推送不代表生产发布获批。

## 1. 当前状态

本周期为第二个多 agent 并行集成批次，三个工作流已完成实现、验证与复审，正在进入 Git 交付阶段：

1. **A2-2 comments 纵向模块**：评论功能迁入 `backend/internal/modules/comments/`（notifications 模板：repository/service/handler + HTTPPort + 三层测试）。关键决策：CreateComment 成功后的作者通知写入在模块 repo 内**镜像**核心 `CreateNotification`（自评抑制逐字保留，注释标注待事件总线统一接管）；视频可见性校验为模块自有 `VideoAuthorID`（谓词与核心 VideoByID 逐字一致，多返回 title 供通知快照）。总控逐行比对确认行为零改变。
2. **SEC-03c CSP 收紧**：SPA CSP 移除 `style-src 'unsafe-inline'` 与 `img-src data:`（React 走 CSSOM 设样式、全仓无 data: 图片，复审逐项确认安全）；`diagnosticsBindingWarnings` 补 4 个边界断言（主机名、缺端口、IPv4-mapped loopback、通配 IPv6）。
3. **产品修复 + 夹具治理**：修复 `/auth?next` 注册竞态（`AuthRedirect` 组件按 next 回跳站内路径，拒绝 `//` 与绝对 URL）；E2E 上传夹具由假签名 MP4 换为**真实 MP4**（后端容器内 ffmpeg 生成，可正常转码），根治环境库损坏卡片污染；烟雾脚本改为只挑 `ready` 视频。

待办：显式暂存本批 22 个路径并复核 staged diff 后提交，推送 `codex/security-hardening` 并核对本地/远端一致。不得使用 `git add .`，不得 amend、force push 或直接推送 main。

## 2. 已完成的实现（本批三工作流）

### 2.1 A2-2 comments 纵向模块

- `modules/comments/`：repository（List/Create/Delete + VideoAuthorID + 镜像 createNotification）、service（1-500 字校验、可见性检查、通知写路径）、handler（HTTPPort：Principal/VideoID/CommentID/DecodeJSON/WriteJSON/WriteError）。
- 接线：httpapi Handler 增 comments 字段、New 增参数、Routes 三端点改指模块、commentsHTTPPort 适配器（保留「无效的评论编号」400 文案）；main.go 装配。
- 删除核心 `handlers_comments.go`/`service_comments.go`/`repository_comments.go`；`VideoByID` 的评论计数子查询与 `CreatorStats` 属核心职责未动（计数直接源自 comments 表，始终同步）。
- 连带必改：notifications 模块测试的种子改用 comments 模块（恰好双向钉住镜像 INSERT 正确性）。

### 2.2 SEC-03c CSP 收紧与警告用例

- `frontend/nginx.conf`：`style-src 'self'`（去 `'unsafe-inline'`）、`img-src 'self' blob:`（去 `data:`）；`media-src/worker-src/child-src blob:` 保留。
- `main_test.go`：警告分类补 4 个边界（主机名、缺端口、IPv4-mapped loopback、通配 IPv6）并断言警告文本包含端点名。

### 2.3 产品修复与夹具治理

- `AuthRedirect` 组件（features/auth）：已登录访问 /auth 按 next 回跳站内相对路径，拒绝 `//` 与绝对 URL；注册/登录成功跳转与该回跳一致，原竞态无害化。
- `frontend/e2e/fixtures/sample.mp4`：后端容器内 ffmpeg 生成的真实 1 秒 MP4（16,969 字节，ffprobe 验证）；`notifications.spec.ts` 改用该夹具，上传可正常转码，不再产生 `processing_failed` 残留。
- 烟雾脚本（tmp/shots/csp-smoke.mjs，不入库）：改从 API 挑选 `processing_status === 'ready'` 的视频。

## 3. 验证证据边界

2026-09-12 本批证据：

- 后端：`go test ./... -count=1` 全包通过（含 comments 模块三层测试、main_test 警告边界、notifications 模块镜像 INSERT 集成用例）；`go vet ./...`、`gofmt -l` 通过；架构守卫 `TestModulesDoNotImportCoreLayers` 通过（模块生产代码零反向依赖）。
- 前端：`npx tsc -b` 0 错误；vitest 全量通过（AuthPage 文件 5 项含 AuthRedirect 3 用例）。
- E2E 全量回归（对重建后的 backend+frontend 运行栈）：**12 passed / 2 skipped**——评论旅程、跨用户通知旅程（真实 MP4 上传→转码→评论→通知→全部已读）在 comments 模块迁移后的后端上全部通过。
- CSP 运行态：重建后实测响应头为收紧后的 CSP；烟雾 `issues=[]` 零违规、`cards=8`、有效视频播放正常。
- 复审：comments 模块由总控逐行比对（agent 两次启动失败：模型请求失败 + captcha，已降级为总控复审并留痕）；CSP/E2E 由复审 agent 结论「可提交，无 P0/P1」。
- QA 发现的产品疑点（`/auth?next` 竞态）本批已修复。遗留：库中既有损坏测试视频（历史 E2E 残留）待清理；E2E 评论对象不筛 processing 状态（P3）。

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
20. `A2-1`：notifications 纵向模块（读侧端点迁移、HTTPPort 模式、三层测试、守卫扩展）。
21. `E2E-2`：评论旅程与跨用户通知旅程 spec（4 项，对运行栈实测）。
22. `SEC-04b`：配额边界/删除释放强断言测试与升级指引。
23. `A2-2`：comments 纵向模块（含通知镜像写、VideoAuthorID、三层测试）。本批。
24. `SEC-03c`：CSP 收紧（去 `unsafe-inline`/`data:`）与管理端点警告边界用例。本批。
25. `FIX-01`：`/auth?next` 注册竞态修复（AuthRedirect）与真实 MP4 E2E 夹具治理。本批。

## 5. 当前任务与后续队列

本周期交付后，后续队列（按优先级）：

- `A2-3`：interactions 纵向模块（like/favorite/follow 端点迁移；注意 follow 的通知写路径与 comments 同模式）。
- `A2-4`：进程内事件总线——统一 notifications 写路径（删除 comments 模块镜像 INSERT 与核心 CreateNotification 的双份），随后核心 service_test 的通知断言归位模块。
- `SEC-03c`（可选收尾）：CSP 剩余项（`style-src` 已收紧完毕；观察期后确认无回归）；nginx 头行为与警告用例的补充覆盖。
- `SEC-04b`（可选收尾）：配额可观测性（超配额计数指标/Runbook）、字幕字节计量、`processing` 卡死时的人工释放流程。
- `SEC-05b`：frontend/nginx 容器降权（nginx-unprivileged，涉及端口映射变更与运行态演练）。
- 环境治理：清理开发库中的历史 E2E 残留损坏视频（需停服 SQL 维护窗口或逐 owner API 删除）。
- 既有独立队列：OpenAPI、依赖安全扫描；媒体管线独立（Storage 接口缝、worker 出进程、快慢队列）等待触发信号。

## 6. 团队调度与验收

- 总控执行写入、测试、容器重建和 Git 操作；架构评审、安全复审与前端工程 Agent 只读/受限并行（平台并发上限 2）。
- 本周期两波并行：第一波 notifications 模块 + E2E 旅程（均已随 `2e01b27` 交付）；第二波 comments 模块 + CSP 收紧 + auth 竞态修复（本批）。复审按批分片，agent 启动失败时降级为总控复审并留痕。
- 任一审查发现阻断，只修复有证据的最小范围并重新运行相关验证。
- 目标测试、全仓静态门禁和代码只读联合审查均无阻断后，可以提交并推送开发分支。
- 合并和实际生产部署仍遵循仓库审查与组织变更批准。

## 6.1 2026-09-12 第二并行集成批次交付点

- 分支：`codex/security-hardening`（承接集成批次 `2e01b27`）。
- 预期修改/新增文件（22 个路径，以 `git status` 为准）：删除 `backend/internal/httpapi/handlers_comments.go`、`backend/internal/repository/repository_comments.go`、`backend/internal/service/service_comments.go`；新增 `backend/internal/modules/comments/`（6 文件）、`frontend/e2e/fixtures/sample.mp4`；修改 `backend/cmd/server/{main.go,main_test.go}`、`backend/internal/httpapi/{health_test.go,httpapi.go,httpapi_test.go,middleware_ratelimit_test.go,notifications_http_test.go,quota_http_test.go}`、`backend/internal/modules/notifications/{repository_test.go,service_test.go}`、`backend/internal/repository/repository_test.go`、`backend/internal/service/service_test.go`、`frontend/{e2e/notifications.spec.ts,nginx.conf,src/app/App.tsx,src/features/auth/AuthPage.tsx,src/features/auth/AuthPage.test.tsx}`、`docs/development-handoff.md`。
- 静态门禁：`scripts/check.ps1`、`go test ./... -count=1`、`go vet`、`gofmt`、`git diff --check` 通过。
- 运行门禁：backend/frontend 重建后 healthy；`/readyz` 200；E2E 全量回归 12 passed / 2 skipped（含评论旅程在 comments 模块后端上的实测）；CSP 烟雾零违规。
- Git 交付顺序：显式暂存上述路径，复核 `git diff --cached --check` 与 staged diff，提交 `feat: extract comments module and tighten CSP`，推送当前开发分支并核对本地/远端一致。
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
