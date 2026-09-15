# GVideo 开发交接

> 更新日期：2026-09-15
> 当前分支：`codex/security-hardening`（自 `codex/architecture-hardening` 的 `84125fc` 切出）
> 交接原则：实现、目标验证、只读复审、显式暂存、提交和推送必须串行；分支推送不代表生产发布获批。

## 1. 当前状态

✅ **批次 29/30/31 已完成交付**——批次 29 为领域文档回填：新增根 `CONTEXT.md`（12 个领域术语，与代码枚举逐项核对）与 `docs/adr/` 首批 5 篇 ADR（`0001` SQLite 单写、`0002` 模块化单体、`0003` 进程内事件总线、`0004` 安全头单值原则、`0005` 渐进式模块提取），每篇含背景、决策、被拒替代方案与回退条件，均为既有事实回填，不引入新决策。批次 30 为 P1 打包材料：`docs/case-study.md`（工程案例：架构、三个难题解法、工程方法、ADR 索引、演进路线）与 roadmap 收敛（维护队列 + 分布式演进方向）。批次 31 为 README 演示资产：五张运行态实拍截图（`docs/assets/*.jpg`，素材为 CC-BY 开放影片演示投稿）与文档导航入口（工程案例/ADR/领域词表）。

同时记录方向调整（2026-09-14 确认）：**主线转为打包发布（P1）、新方向立项（P2）与分布式演进（P3），停止功能与加固开发**；§5 队列已按此重排。个人技能库 `agent-skills`（`/grill`：方案拷问 + 决策落盘，改写自 mattpocock/skills）已独立建库、安装到 `~/.agents/skills/` 并推送私有远端（`main` = `9c554e6`），不属本仓库范围。

**P1 已闭环（2026-09-15）**：PR #19 经 Backend / Backend Race / Frontend 三项 CI 通过后以 rebase 方式合并进 `main`（仓库禁用 merge commit），`v1.0.0` tag 与 GitHub Release 已发布（release notes 含升级注意）；main 与分支内容一致（rebase 后 SHA 不同）。

交付程序：批次 29/30 已按同纪律完成（提交 `dcc26ce`、`03f74e6`，已推送）；批次 31 显式暂存 `README.md`、`docs/assets/`、`CHANGELOG.md`、本文件并复核 staged diff 后提交 `docs: add README demo gallery and screenshots`，推送 `codex/security-hardening` 并核对本地/远端一致。不得使用 `git add .`，不得 amend、force push 或直接推送 main。

## 2. 已完成的实现（批次 27 三工作流）

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

2026-09-15 批次 32（发布收尾）证据：

- 纯文档批次；`git diff --check` 通过；发布事实以 GitHub API 响应与 `git ls-remote` 为准（PR #19 merged、tag `v1.0.0` → `daf94b6`、Release 已发布）。
- 复审：状态收尾类改动，由总控逐项核对（CHANGELOG 链接、handoff 批次号/日期/链接、与远端一致）。

2026-09-15 批次 31（README 演示资产）证据：

- 截图来自运行栈实拍（Playwright 脚本流程：注册/登录 → 上传 9 段真实素材 → 转码完成 → 评论 → 逐页截图），非合成图；演示素材为 Blender 开放影片（Big Buck Bunny / Sintel / Jellyfish，CC-BY），投稿简介中标注来源。
- 资产：5 张 JPEG（1440/1600 宽，总计 1.3MB）；README 图片链接与文档导航逐条核对；`git diff --check` 通过。
- 复审：复审 agent 启动失败（provider 服务端错误），按 §6 纪律降级为总控复审并留痕——README 标记与链接、图片内容抽查（5 张全部人工查阅）、文档一致性（批次号/日期/提交信息）、资产体积与 git 卫生逐项核对通过。
- 附带数据操作：清理 9 条早期渐变演示投稿（经 UI 删除，属本批自建演示内容），开发库既有历史 E2E 残留未动（仍为独立维护项）。

批次 30（P1 打包材料）证据：

- 纯文档批次；`git diff --check` 通过；独立只读复审（数字与事实核对、ADR/handoff 互链、无夸大表述）通过。

批次 29（领域文档回填）证据：

- 事实核对：`CONTEXT.md` 术语与 ADR 中的表名/状态机/测试名逐项对照代码（`transcoding_jobs`、`processing_status` 四态、`video_reports` 四态、通知六类（四类社交 + 两类处理）、`TestModulesDoNotImportCoreLayers`、`security-headers.spec.ts`）。
- 门禁：`scripts/check.ps1`、`git diff --check` 通过（纯文档批次，无代码改动）。
- 复审：独立只读复审（事实一致性、文档互链、与既有 CHANGELOG/handoff 无矛盾）通过。

批次 28（SEC-03c 收尾）证据（存档）：

- 修复前实测：前端源 `:8088` 上 `/api/v1/videos`、`/healthz` 每个安全头两份（deny-all CSP + SPA CSP 并存）；网关 `:8443` 上 `/api/v1/videos` CSP 两份（四项非 CSP 头经网关 hide+re-add 已单份）。
- 修复后实测：前端源 `/`、`/upload`（SPA 回退）、`/theme-init.js`（含 `Cache-Control: no-cache`）、`/api/v1/videos`（deny-all CSP）、`/healthz` 各头恰一份，`Server: nginx` 无版本号；网关 `/`、`/api/v1/videos` 各头恰一份，`/gateway-healthz` 保持网关自产四头。
- 后端：`go test ./... -count=1` 全包通过（含 main_test 新增 PPROF_ADDR 命名断言）。
- E2E 全量回归（重建后的 frontend 运行栈）：**24 passed / 2 skipped**——新增 `security-headers.spec.ts` 六类响应断言全过（2 项目 × 6）。
- 烟雾：`issues=[]` 零违规、`cards=27`、有效视频播放正常、收紧 CSP 头在位——SEC-03c「CSP 观察期确认无回归」闭环。
- 批次 27 证据（存档）：后端全包测试、`go vet`、`gofmt`、架构守卫通过；e2e 12 passed / 2 skipped；复审降级为总控逐行比对（agent 两次启动失败留痕）。

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
23. `A2-2`：comments 纵向模块（含通知镜像写、VideoAuthorID、三层测试）。
24. `SEC-03c`：CSP 收紧（去 `unsafe-inline`/`data:`）与管理端点警告边界用例。
25. `FIX-01`：`/auth?next` 注册竞态修复（AuthRedirect）与真实 MP4 E2E 夹具治理。
26. `A2-3`：interactions 纵向模块（Toggle 三方法迁移、通知镜像写、消费方查询、三层测试）。
27. `A2-4`：进程内事件总线（platform/bus 类型化同步分发、notifications 写所有者迁移、comments/interactions 改事件发布、三份镜像 INSERT 消除）。
28. `SEC-03c`：CSP 收尾——前端源代理路径重复安全头修复（SPA 头集收敛到文档 location、代理纯透传后端单份头）、e2e 安全头 spec 六类断言、管理端点警告 PPROF 命名用例、CSP 观察期确认（e2e + 烟雾零违规）。
29. `DOC-01`：领域词表与架构决策记录回填（`CONTEXT.md` 12 术语 + `docs/adr/` 首批 5 篇，含被拒替代方案与回退条件）。
30. `DOC-02`：P1 打包材料——`docs/case-study.md`（工程案例：架构、三个难题解法、工程方法、ADR 索引、演进路线）与 `docs/roadmap.md` 收敛（v1.0.0 发布与维护 + 分布式演进）。
31. `DOC-03`：README 演示资产——五张运行态实拍截图（首页/播放/工作台/通知中心/深色主题，JPEG 压缩总计 1.3MB，素材为 CC-BY 演示投稿）与文档导航入口。
32. `DOC-04`：发布收尾——`CHANGELOG.md` 标记 `[1.0.0] - 2026-09-15`；handoff 记录 P1 闭环（PR #19 rebase 合并、`v1.0.0` tag、GitHub Release、技能库私有远端）。本批。

## 5. 当前任务与后续队列

2026-09-14 方向调整（已确认）：**停止功能与加固开发**，主线转为打包发布与分布式演进：

- `P1` 打包发布：✅ 全部完成——`docs/case-study.md` 与 roadmap 收敛（批次 30）、README 演示资产（批次 31）、PR #19 合并 main、`v1.0.0` tag 与 GitHub Release（批次 32 收录状态）。
- `P2` 新方向立项：用 `/grill` 技能做 greenfield 拷问，定域、技术栈与运行形态，产出首批 ADR 与项目骨架。
- `P3` 分布式 Stage A/B/C：Redis 分布式限流与缓存、事件总线迁移 Outbox + 队列（ADR-0003 的回退路径）、转码 worker 出进程与 PostgreSQL、k8s 与跨队列 trace。

维护队列（计划内、按需触发，不再作为主线）：`SEC-04b` 配额可观测性、`SEC-05b` nginx 容器降权、环境治理（历史损坏测试视频清理）、OpenAPI、依赖安全扫描。

## 6. 团队调度与验收

- 总控执行写入、测试、容器重建和 Git 操作；架构评审、安全复审与前端工程 Agent 只读/受限并行（平台并发上限 2）。
- 本周期两波并行：第一波 notifications 模块 + E2E 旅程（均已随 `2e01b27` 交付）；第二波 comments 模块 + CSP 收紧 + auth 竞态修复（批次 27）。复审按批分片，agent 启动失败时降级为总控复审并留痕。
- 任一审查发现阻断，只修复有证据的最小范围并重新运行相关验证。
- 目标测试、全仓静态门禁和代码只读联合审查均无阻断后，可以提交并推送开发分支。
- 合并和实际生产部署仍遵循仓库审查与组织变更批准。

## 6.1 2026-09-12 A2-4 交付点

- 分支：`codex/security-hardening`（承接 A2-3 的 `0ca849e`）。
- 修改/新增文件：新增 `backend/internal/platform/bus/{bus.go,bus_test.go}`；新增 `backend/internal/modules/comments/`（事件发布改造 + 镜像删除）；修改 `backend/internal/modules/{comments,interactions,notifications}` 三模块、`backend/cmd/server/main.go`（总线装配 + 订阅）、删除 `backend/internal/repository/repository_notifications.go`（写路径迁入 notifications 模块）、相关测试迁移。
- 静态门禁：`scripts/check.ps1`、`go test ./... -count=1`、`go vet`、`gofmt`、`git diff --check` 通过。
- 行为保持：同步分发（订阅者在发布方 goroutine 内联）、自评抑制与空值防护随迁、通知失败仅告警不影响主流程。
- Git 交付顺序：显式暂存上述路径，复核 `git diff --cached --check` 与 staged diff，提交 `feat: add in-process event bus for notifications`，推送当前开发分支并核对本地/远端一致。
- 不创建自动守护任务；本批工作在当前会话内完成交付。

## 6.2 2026-09-12 SEC-03c 收尾交付点

- 分支：`codex/security-hardening`（承接 A2-4 后的 `5203374`）。
- 修改/新增文件：`frontend/nginx.conf`（SPA 头集收敛到 `location /` 与 `location = /theme-init.js`，代理 location 纯透传）、`frontend/e2e/security-headers.spec.ts`（新增六类响应头断言）、`backend/cmd/server/main_test.go`（PPROF_ADDR 命名断言）、`CHANGELOG.md`、`docs/deployment.md`、本文件。
- 行为保持：后端 deny-all CSP 继续随 API/媒体响应下发；`theme-init.js` 的 no-cache 重校验语义不变；网关模板未改动（其 hide+re-add 行为与本修复组合后全链路单值）。
- 静态门禁：`scripts/check.ps1`、`git diff --check` 通过；运行态验证见 §3。
- 复审：独立只读复审 A-F 六项全部通过，结论「可提交」；一条 P2 观察项留痕——代理 location 上 nginx 自产错误响应（>524m 的 413、后端不可达的 502/504）修复后不再携带安全头（修复前由 server 级 `add_header always` 覆盖），经网关时四项非 CSP 头由边缘补齐、仅 CSP 缺失，直连前端源仅限开发内网，属防御纵深轻微收窄，留待后续批次评估。
- Git 交付顺序：显式暂存上述路径，复核 `git diff --cached --check` 与 staged diff，提交 `fix(security): single-value security headers on proxied responses`，推送当前开发分支并核对本地/远端一致。
- 不创建自动守护任务；本批工作在当前会话内完成交付。

## 6.3 2026-09-14 批次 29 交付点

- 分支：`codex/security-hardening`（承接批次 28 的 `62bb88c`）。
- 新增文件：`CONTEXT.md`、`docs/adr/0001-sqlite-single-writer.md` ～ `0005-incremental-module-extraction.md`；修改 `CHANGELOG.md`、本文件。
- 术语与决策来源：与代码枚举逐项核对；决策为既有事实回填（含被拒替代方案与回退条件），不引入新决策。
- 静态门禁：`scripts/check.ps1`、`git diff --check` 通过。
- 复审：独立只读复审通过（事实一致性、文档互链、与既有文档无矛盾）。
- Git 交付顺序：显式暂存上述路径，复核 `git diff --cached --check` 与 staged diff，提交 `docs: add domain glossary and architecture decision records`，推送当前开发分支并核对本地/远端一致。
- 后续：P1 打包发布按 §5 执行；合并 main 走 PR，不直接推送。

## 6.4 2026-09-14 批次 30 交付点

- 分支：`codex/security-hardening`（承接批次 29 的 `dcc26ce`）。
- 新增/修改：`docs/case-study.md`（新增）、`docs/roadmap.md`（收敛）、`CHANGELOG.md`、本文件。
- 内容边界：案例中的数字均取自仓库实测（代码行数、测试占比、e2e 通过数、批次数）；不含未验证的宣称。
- 静态门禁：`git diff --check` 通过；复审：独立只读复审通过。
- Git 交付顺序：显式暂存上述路径，复核 staged diff，提交 `docs: add engineering case study and converge roadmap`，推送当前开发分支并核对本地/远端一致。
- 后续：P1 剩余项（PR 合并 main、v1.0.0 tag、README 演示资产）按 §5 执行。

## 6.5 2026-09-15 批次 31 交付点

- 分支：`codex/security-hardening`（承接批次 30 的 `03f74e6`）。
- 新增/修改：`README.md`（演示区 + 文档导航）、`docs/assets/`（5 张 JPEG 截图）、`CHANGELOG.md`、本文件。
- 内容边界：截图全部来自运行栈实拍；演示投稿素材为 CC-BY 开放影片并在简介标注来源；不涉及产品代码改动。
- 静态门禁：`git diff --check` 通过；复审：复审 agent 启动失败，降级为总控复审并留痕（见 §3）。
- Git 交付顺序：显式暂存上述路径，复核 staged diff，提交 `docs: add README demo gallery and screenshots`，推送当前开发分支并核对本地/远端一致。
- 后续：P1 剩余项（PR 合并 main、v1.0.0 tag 与 GitHub Release）按 §5 执行。

## 6.6 2026-09-15 批次 32 交付点（发布收尾）

- 分支：`codex/security-hardening`（承接批次 31 的 `81ceaa8`）。
- 内容：`CHANGELOG.md` 标记 `[1.0.0] - 2026-09-15` 并新增空的「未发布」段与链接；本文件 §1/§3/§5 记录 P1 闭环（PR #19、tag、Release、技能库远端）。
- 发布事实：PR #19（`md`：v1.0.0：安全加固、架构治理与工程文档）CI 三项通过后 rebase 合并；`v1.0.0` tag 指向 main `daf94b6`；GitHub Release `https://github.com/zdl000000/GVideo/releases/tag/v1.0.0`。
- 静态门禁：`git diff --check` 通过。
- Git 交付顺序：显式暂存上述路径，复核 staged diff，提交 `docs: mark v1.0.0 release and close P1`，推送分支后创建 PR 并按仓库保护规则合并（rebase）。
- 后续：`P2` 新方向立项（greenfield 拷问）与维护队列按 §5 执行。

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
