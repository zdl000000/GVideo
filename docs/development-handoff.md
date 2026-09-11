# GVideo 开发交接

> 更新日期：2026-09-11
> 当前分支：`codex/security-hardening`（自 `codex/architecture-hardening` 的 `84125fc` 切出）
> 交接原则：实现、目标验证、只读复审、显式暂存、提交和推送必须串行；分支推送不代表生产发布获批。

## 1. 当前状态

SEC-04「每用户上传存储配额」已完成实现、目标测试、后端全量测试、独立只读复审与全量门禁，正在进入 Git 交付阶段。

本批新增 `USER_STORAGE_QUOTA_BYTES` 配额：上传前按用户累计源文件字节校验，超出返回 413 `storage_quota_exceeded`；二进制默认 0（禁用），Compose 部署默认 20 GiB；文档明确配额只统计源文件、HLS 产物约放大 2-3 倍磁盘。

待办：显式暂存本批文件并复核 staged diff 后提交，推送 `codex/security-hardening` 并核对本地/远端一致。不得使用 `git add .`，不得 amend、force push 或直接推送 main。

## 2. 已完成的实现（SEC-04）

- 配置：`config.UserStorageQuotaBytes`（env `USER_STORAGE_QUOTA_BYTES`，默认 `0` 表示禁用；负数或非数字启动报错）；`compose.yaml` 默认 `21474836480`（20 GiB）；`.env.example` 同步。
- 数据（迁移 v3）：`videos` 表新增 `cover_size_bytes`、`hls_size_bytes`；`repository.UserStorageUsed` 汇总「源视频 + 封面 + HLS 实际产出」；删除投稿为物理 `DELETE FROM videos`，额度即时释放（`processing` 状态的投稿沿用既有“不可删除”规则）。
- 计量覆盖：上传封面与 FFmpeg 自动生成封面均在落盘后计字节；`updateVideo` 换封面同步更新计量（否则可先传小封面再换成大封面绕过）；媒体 worker 在转码完成时统计 HLS 目录实际字节并随 `MediaOutput.HLSBytes` 入账，测量失败仅告警并记 0（不阻塞完成）。
- 服务：`service.enforceStorageQuota` 在尺寸校验之后、任何文件写入之前执行；并发上传为 best-effort（可能瞬时超额，已注释说明）；配额只覆盖视频资产，头像不计量（每用户一份、替换即删除旧文件）。
- 错误契约：`domain.ErrQuotaExceeded` → 413「存储空间已用完，请先删除部分投稿」；`docs/api-error-contract.md` 新增 `storage_quota_exceeded` 行。
- 测试：service 配额内/超限/多用户隔离、封面计量（构造只有计入封面才会超限的边界）、无配额不限量、HTTP 413 契约、`hlsDirectorySize` 单元（含目录缺失）、repository 三项汇总、config 默认值与非法值、迁移 fixture v1+v2+v3。

## 3. 验证证据边界

2026-09-11 本批证据：

- 目标测试：`TestUploadStorageQuota`、`TestUploadWithoutQuotaIsUnlimited`、`TestUploadQuotaReturnsRequestEntityTooLarge`、`TestLoadUserStorageQuota` 全部通过。
- 后端全量：`go test ./...`（全包）、`go vet ./...`、`gofmt -l` 通过。
- 全量门禁：`scripts/check.ps1` 通过。
- 独立只读复审（agent）：发现并已修复 2 项 P1——封面/自动封面不计入（1KB 视频挂 10MiB 封面可放大 ~10^4 绕过配额）与 HLS 产物完全不入账（低码率源放大远超文档估计）；修复方式为迁移 v3 记录 `cover_size_bytes`/`hls_size_bytes`、转码完成时统计 HLS 目录、换封面同步计量，并补齐对应测试。复审给出的 P2（并发 TOCTOU 瞬时超额、存量超配额升级提示、`processing` 卡死期间不可删导致额度无法释放）已在文档说明或记录后续；P3 测试缺口已补删除释放与精确边界之外的主要项。
- 待补：容器重建后的运行态验证（上传超限返回 413）归入合并阶段演练。

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
14. `SEC-01`：管理员提权修复（迁移 v2、保留名锁定、moderation 改布尔、`data-grant-admin`、合并携带标志、升级文档）。
15. `SEC-02`：请求速率限制（auth/comment/upload 分档、429 + Retry-After、可信代理地址解析、会话形状预检、代理头加固）。
16. `SEC-03`：安全响应头（后端 deny-all CSP + 框架拒绝；SPA CSP 含 blob/worker 白名单；主题脚本外置）与管理端点非 loopback 绑定警告；顺手修复 `/livez` 代理缺失。
17. `SEC-04`：每用户上传存储配额（413 `storage_quota_exceeded`、Compose 默认 20 GiB）。本批。

## 5. 当前任务与后续队列

SEC-04 交付后，后续安全与质量队列（按优先级）：

- `SEC-03b`：网关（`deploy/nginx/https.conf.template`）443 自产响应（`limit_req` 429、502、`/gateway-healthz`）补齐同组响应头，并评估经由 `proxy_hide_header` 去重上游重复头；CSP 进一步收紧（评估移除 `style-src 'unsafe-inline'` 与 `img-src data:`）；补充 nginx 头行为与主机名/无端口/`[::ffff:127.0.0.1]` 等警告用例。
- `SEC-05`（低危批次）：登录用户名时序枚举防护（dummy bcrypt）、CSRF 恒定时间比较、搜索 LIKE 通配符转义、frontend/nginx 容器降权。
- `SEC-04b`：配额可观测性与运维完善——超配额计数指标/Runbook、存量用户超过默认 20 GiB 时的升级指引与扩容说明、字幕字节计量、`processing` 卡死时的人工释放流程、配额精确边界（`used+size == quota`）与删除释放的自动化用例。
- 既有独立队列：OpenAPI、依赖安全扫描、更广泛 E2E 覆盖；`A2 后续`：notifications/comments/interactions 模块迁移与进程内事件总线。
- 媒体管线独立（Storage 接口缝、worker 出进程、快慢队列）等待触发信号。

## 6. 团队调度与验收

- 总控执行写入、测试、容器重建和 Git 操作；架构评审、安全复审与前端工程 Agent 只读/受限并行。
- 本批调度：DeepSeek 侧总控实现与全量测试；只读复审 agent 复查 diff（结论见 §3）。
- 任一审查发现阻断，只修复有证据的最小范围并重新运行相关验证。
- 目标测试、全仓静态门禁和代码只读联合审查均无阻断后，可以提交并推送开发分支。
- 合并和实际生产部署仍遵循仓库审查与组织变更批准。

## 6.1 2026-09-11 SEC-04 交付点

- 分支：`codex/security-hardening`（承接 SEC-03 的 `fe607f1`）。
- 预期修改文件：`CHANGELOG.md`、`.env.example`、`compose.yaml`、`docs/api-error-contract.md`、`docs/deployment.md`、`docs/development-handoff.md`、`backend/internal/config/config.go`、`backend/internal/config/config_test.go`、`backend/internal/domain/domain.go`、`backend/internal/httpapi/httpapi.go`、`backend/internal/repository/repository_videos.go`、`backend/internal/service/service_videos.go`，以及新增 `backend/internal/service/service_quota_test.go`、`backend/internal/httpapi/quota_http_test.go`。
- 静态门禁：`scripts/check.ps1`、`go test ./...`、`go vet`、`gofmt`、`git diff --check` 通过。
- Git 交付顺序：显式暂存上述文件，复核 `git diff --cached --check` 与 staged diff，提交 `feat(security): enforce per-user storage quota`，推送当前开发分支并核对本地/远端一致。
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
