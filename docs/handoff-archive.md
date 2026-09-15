# GVideo 交付历史归档

> 本文件保存 [development-handoff.md](./development-handoff.md) 中已归档的历史批次记录（实现细节、验证证据与交付点），保留原始编号以便与 §4 队列逐条对照。当前状态、队列与近期交付点仍在主文档。

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

---

## 3. 验证证据边界（批次 27–31，存档）

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

---

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
