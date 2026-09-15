# GVideo 开发交接

> 更新日期：2026-09-15
> 当前状态：`main`（v1.0.0 已发布；发布后完成批次 33–35，其中批次 35「仓库整理」随本文件所在提交交付）
> 交接原则：实现、目标验证、只读复审、显式暂存、提交和推送必须串行；分支推送不代表生产发布获批。

## 1. 当前状态

✅ **v1.0.0 已发布；发布后完成批次 33–35**——批次 35 为仓库整理：README 文档导航补全（契约与可观测性四份文档）、`scripts/merge-local-data.ps1` 登记到 operations.md、历史交接记录归档到 [handoff-archive.md](./handoff-archive.md)（批次 27–31 的实现细节、证据与交付点），主文档回归当前状态与近期记录。批次 29–31 的领域词表、ADR、工程案例与演示截图已随 v1.0.0 进入仓库。

**批次 34（2026-09-15，HLS light 构建 + 测试稳定性）**：`hls.js` 升到 1.7.3 并改用 `hls.js/light`（字幕走原生 TextTrack，未使用备用音轨/DRM/CMCD/变量替换），HLS chunk 从 509.49 kB 降到 **360.41 kB / gzip 113.16 kB**，解除 bundle 预算阻塞；`vitest.config.ts` 启用 `isolate: false` 复用单 worker（配套 `unstubGlobals: true`，并补齐三个测试文件的 `afterEach(cleanup)`），套件耗时从 20–140s 降到典型 **3.4–4.5s**（重负载下最长观察 32s），消除 worker 启动超时（vitest 硬编码 60s）的主要诱因；测试基线 **14 文件 / 52 例**。验证：`check.ps1` 全绿、e2e 24/2、播放烟雾 `playing + advanced + issues=[]`。

同时记录方向调整（2026-09-14 确认）：**主线转为打包发布（P1）、新方向立项（P2）与分布式演进（P3），停止功能与加固开发**；§5 队列已按此重排。个人技能库 `agent-skills`（`/grill`：方案拷问 + 决策落盘，改写自 mattpocock/skills）已独立建库、安装到 `~/.agents/skills/` 并推送私有远端（`main` = `9c554e6`），不属本仓库范围。

**P1 已闭环（2026-09-15）**：PR #19 经 Backend / Backend Race / Frontend 三项 CI 通过后以 rebase 方式合并进 `main`（仓库禁用 merge commit），`v1.0.0` tag 与 GitHub Release 已发布（release notes 含升级注意）；main 与分支内容一致（rebase 后 SHA 不同）。

**批次 33（2026-09-15，依赖维护）**：一次性应用待处理的 dependabot 升级并在本地完成全量验证——后端 `chi` 5.3.2 / `x/crypto` 0.56.0 / `sqlite` 1.58.0，前端 `lucide-react` 1.45.0 / `vite` 8.3.0 / `@types/react-dom` 19.2.5 / `vitest` 5.0.0；`hls.js` 1.7.x 因超出 HLS bundle 预算（raw 575.83 kB / gzip 177.04 kB vs 550/170）**暂缓**，对应 PR #15 保留待专项评估。**数字更正**：前端 vitest 全量基数为 **14 文件 / 52 例**；此前 PR/Release 文案中的「31 项」来自一次 VideoPlayer 测试文件 worker 启动超时的残缺运行（该 flake 表现为 13 文件/31 例 + 1 error、check.ps1 非零退出，重跑恢复），已在 GitHub 文案中更正并作为已知问题记录。

交付程序（现行）：改动走短生命周期分支 → PR → Backend / Backend Race / Frontend 三项 CI 通过后 rebase 合并 `main`；不得直接推送 main、不得 amend / force push、不得使用 `git add .`。批次 29–34 的提交流水见 §6.3–§6.8 与归档文件。

## 3. 验证证据边界

2026-09-15 批次 35（仓库整理）证据：

- 审计口径：`git ls-files` 215 个跟踪文件全量分类核对；忽略规则（`.gitignore`）覆盖日志/缓存/数据/密钥；无空目录、无跟踪残留、无幽灵文件；前端 33 个模块依赖图扫描零死代码（仅两个 `.d.ts` 环境声明无需被引用）；文档与脚本引用计数逐项核对（发现 4 份文档未进 README 导航、1 个脚本未登记）。
- 归档拆分由脚本执行（`tmp/handoff-split.cjs`），核对：归档保留批次 27 实现细节与 27–31 证据、主文档保留批次 34/32 证据与 §6.6–6.8；主文档拆分后 138 行，本批补充（批次 35 证据、§4 第 35 项、§6.9）后交付为 152 行。
- 门禁：`git diff --check` 通过（纯文档批次）。

2026-09-15 批次 34（HLS light 构建 + 测试稳定性）证据：

- 体积：`npm run build` 后 HLS chunk **360.41 kB / gzip 113.16 kB**（对照：全量 1.6.17 为 509.49/155.52，全量 1.7.3 为 575.83/177.04 超预算），`check-bundle-budget.mjs` 通过。
- 功能：`tsc -b` 0 错误（新增 `src/types/hls-light.d.ts` 复用主包类型）；vitest 14 文件 / 52 例；重建 frontend 镜像后 e2e 24 passed / 2 skipped；播放烟雾（真实 HLS 播放）`playResult=playing`、`advanced=true`、`issues=[]`。
- 稳定性：`isolate: false` 后连续 5 次全量运行全绿（3.43–4.49s；重负载下最长观察 31.99s），此前单 worker 每文件启动的高成本与 60s 硬编码启动超时不再构成主要诱因；复审建议的三项加固已落实（`unstubGlobals: true`、三个文件补 `afterEach(cleanup)`、类型声明注明 light 裁剪差异）。
- 门禁：`scripts/check.ps1` 全绿。

2026-09-15 批次 32（发布收尾）证据：

- 纯文档批次；`git diff --check` 通过；发布事实以 GitHub API 响应与 `git ls-remote` 为准（PR #19 merged、tag `v1.0.0` → `daf94b6`、Release 已发布）。
- 复审：状态收尾类改动，由总控逐项核对（CHANGELOG 链接、handoff 批次号/日期/链接、与远端一致）。

更早批次（27–31）的实现细节与证据已归档到 [handoff-archive.md](./handoff-archive.md)。

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
32. `DOC-04`：发布收尾——`CHANGELOG.md` 标记 `[1.0.0] - 2026-09-15`；handoff 记录 P1 闭环（PR #19 rebase 合并、`v1.0.0` tag、GitHub Release、技能库私有远端）。
33. `DEPS-01`：依赖维护——应用 7 项 dependabot 升级（后端 3、前端 4，含 vitest 5 大版本），本地全量门禁 + e2e 通过；hls.js 1.7.x 因 bundle 预算暂缓并留 PR #15；更正 vitest 全量基数并记录 VideoPlayer worker flake。
34. `PERF-02`：HLS light 构建——hls.js 1.7.3 + `hls.js/light`（字幕原生 TextTrack，未用被裁剪能力），chunk 509.49 → 360.41 kB；vitest `isolate: false` 使套件 20–140s → 典型 3.4–4.5s 并消除启动超时诱因。
35. `DOC-05`：仓库整理——README 文档导航补齐契约/可观测性四份文档；`merge-local-data.ps1` 登记到 operations.md；`development-handoff.md` 拆分出 `docs/handoff-archive.md`（批次 27–31 实现细节/证据/交付点）并修正 §1 与交付程序；清理工作区残留日志。本批。

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

历史交付点（批次 27–31，原 §6.1–§6.5）已归档到 [handoff-archive.md](./handoff-archive.md)；近期交付点见下。

## 6.6 2026-09-15 批次 32 交付点（发布收尾）

- 分支：`codex/security-hardening`（承接批次 31 的 `81ceaa8`）。
- 内容：`CHANGELOG.md` 标记 `[1.0.0] - 2026-09-15` 并新增空的「未发布」段与链接；本文件 §1/§3/§5 记录 P1 闭环（PR #19、tag、Release、技能库远端）。
- 发布事实：PR #19（`md`：v1.0.0：安全加固、架构治理与工程文档）CI 三项通过后 rebase 合并；`v1.0.0` tag 指向 main `daf94b6`；GitHub Release `https://github.com/zdl000000/GVideo/releases/tag/v1.0.0`。
- 静态门禁：`git diff --check` 通过。
- Git 交付顺序：显式暂存上述路径，复核 staged diff，提交 `docs: mark v1.0.0 release and close P1`，推送分支后创建 PR 并按仓库保护规则合并（rebase）。
- 后续：`P2` 新方向立项（greenfield 拷问）与维护队列按 §5 执行。

## 6.7 2026-09-15 批次 33 交付点（依赖维护）

- 分支：`chore/deps-maintenance`（自 main `11c8f63` 切出）。
- 修改：`backend/go.mod`、`backend/go.sum`、`frontend/package.json`、`frontend/package-lock.json`；`CHANGELOG.md`、本文件。
- 版本：chi 5.3.2 / x/crypto 0.56.0 / sqlite 1.58.0 / lucide-react 1.45.0 / vite 8.3.0 / @types/react-dom 19.2.5 / vitest 5.0.0；hls.js 保持 ^1.6.17。
- 验证：`scripts/check.ps1` 全绿（后端全包测试、vitest 5 全量 14 文件/52 例、tsc、构建与 HLS 预算 509.49 kB/155.52 kB）；重建镜像后 e2e 24 passed / 2 skipped。
- 发现：hls.js 1.7.3 构建体积 raw 575.83 kB / gzip 177.04 kB，超预算（550/170）→ 暂缓，PR #15 保留并批注实测值。
- 收尾：合并后关闭被取代的 dependabot PR（#10–#14、#17、#18）并注明取代关系；更正 GitHub Release 与 PR #19 文案中的 vitest 数字（31 → 52）；删除已合并的 `codex/security-hardening`、`codex/architecture-hardening` 分支（本地与远端，内容均在 main）。

## 6.8 2026-09-15 批次 34 交付点（HLS light 构建 + 测试稳定性）

- 分支：`perf/hls-light-build`（自 main `1982247` 切出）。
- 修改/新增：`frontend/src/features/watch/hlsLoader.ts`（改 `hls.js/light` + 说明注释）、`frontend/src/types/hls-light.d.ts`（新增类型声明）、`frontend/vitest.config.ts`（`isolate: false` + 注释）、`frontend/package.json` / `package-lock.json`（hls.js ^1.7.3）、`CHANGELOG.md`、本文件。
- 行为边界：字幕仍由原生 TextTrack 渲染；light 构建裁剪的备用音轨 / DRM / CMCD / 变量替换在本项目未被使用（已逐项核验）。
- 静态门禁与验证：`scripts/check.ps1`、e2e、播放烟雾见 §3。
- Git 交付顺序：显式暂存上述路径，复核 staged diff，提交 `perf(player): ship hls.js light build and stabilize the test runner`，推送分支后创建 PR 并按保护规则合并（rebase）；合并后关闭 dependabot PR #15（注明经 light 构建落地）。

## 6.9 2026-09-15 批次 35 交付点（仓库整理）

- 分支：`chore/repo-tidy`（自 main `37f1923` 切出）。
- 新增/修改：`docs/handoff-archive.md`（新增归档）、`docs/development-handoff.md`（拆分瘦身 + §1/交付程序修正 + §4 第 34 项耗时口径更正为「20–140s → 典型 3.4–4.5s」）、`README.md`（文档导航分组补齐）、`docs/operations.md`（登记 merge-local-data.ps1 + 标题空行修复）、`CHANGELOG.md`、本文件。
- 删除/移动：无仓库文件删除；工作区残留日志（`frontend/tmp-vitest-*.log`）移入被忽略的 `tmp/vitest-runs/`。
- 静态门禁：`git diff --check` 通过（纯文档批次）。
- Git 交付顺序：显式暂存上述路径，复核 staged diff，提交 `docs: tidy repository layout and archive older handoff records`，推送分支后创建 PR 并按保护规则合并（rebase）。

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