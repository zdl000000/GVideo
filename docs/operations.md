# GVideo 运维、验收与恢复

本文覆盖项目检查、上线验收、容量管理、数据备份和隔离恢复。部署与环境变量见 [deployment.md](deployment.md)。

## 日常检查

```powershell
.\scripts\check.ps1
```

检查内容包括 PowerShell 语法、Docker Compose 配置、Go 格式与测试、`go vet`、Vitest、TypeScript 类型检查、Vite 生产构建和 `git diff --check`。

## 管理诊断端点

指标和 pprof 默认关闭，分别由 `METRICS_ADDR`、`PPROF_ADDR` 独立启用。只绑定 loopback 或可信管理网络；不要通过面向用户的 Nginx/公网入口代理这些端点。

```powershell
$env:METRICS_ADDR = "127.0.0.1:9090"
$env:PPROF_ADDR = "127.0.0.1:6060" # 仅在限时排障时启用
cd backend
go run ./cmd/server
```

- 指标：`http://127.0.0.1:9090/metrics`
- pprof：`http://127.0.0.1:6060/debug/pprof/`

采集系统如在容器或远端网络，应使用隔离管理网络和来源 ACL，而不是把端口发布到 `0.0.0.0`。排障完成后清空 `PPROF_ADDR` 并重启服务。

## 告警与 Runbook

供应商无关的初始阈值、查询语义、升级条件与恢复判定见 [observability-alerts.md](observability-alerts.md)，结构化日志字段见 [observability.md](observability.md)。这些文档是监控接入规格，不表示仓库已经部署告警产品。

部署监控前必须启用受保护的 `METRICS_ADDR`，配置 `/metrics` 采集和 `/livez`、`/readyz` 外部探测，并按部署角色确认 `media_queue_depth` 是否应存在。当前没有备份成功时间戳指标或 Worker heartbeat；不得用文件时间、进程存活或 `/readyz` 冒充这些信号。

## 完整验收

首次运行浏览器验收前安装 Chromium：

```powershell
cd frontend
npx playwright install chromium chromium-headless-shell
cd ..
.\scripts\acceptance.ps1
```

验收覆盖健康检查、容量检查、注册登录、上传、媒体处理、Range、HLS、字幕、关注、点赞、收藏、评论、通知、CSRF，以及桌面与移动浏览器流程。字幕管理会断言只存在于 `/me/videos`。

常用参数：

```powershell
.\scripts\acceptance.ps1 -SkipBuildChecks
.\scripts\acceptance.ps1 -SkipCapacityCheck
.\scripts\acceptance.ps1 -IncludeBackupRestore
.\scripts\acceptance.ps1 -IncludeBackupRestore -KeepDrillBackup
.\scripts\acceptance.ps1 -BaseURL https://video.example.com
```

API 验收会删除本轮视频、字幕、评论和互动。由于当前没有安全的用户删除接口，随机验收账号会保留。

## 容量检查

```powershell
.\scripts\check-capacity.ps1
.\scripts\check-capacity.ps1 -WarningFreePercent 30 -CriticalFreePercent 15
.\scripts\check-capacity.ps1 -SkipHostDrive
```

默认剩余空间低于 20% 时警告，低于 10% 时失败。正式部署前、批量上传前和定时运维任务中都应执行。

## 数据状态

```powershell
.\scripts\data-status.ps1
```

该命令用于确认 Docker 持久数据库中的主要记录数量，避免混淆本机调试库与 Docker 持久库。

## 自动迁移前备份

后端启动时会先以只读方式检查 SQLite 迁移状态。已有业务表或迁移账本且确有待执行迁移时，系统会在任何迁移写入前通过 `VACUUM INTO` 创建事务一致快照，执行 `integrity_check` 与 `foreign_key_check`，再以不覆盖既有同名文件的方式发布到数据库同目录的 `.migration-backups/`。全新空库和已经处于当前版本的数据库不会创建该备份；checksum 不匹配、未来版本或迁移账本空洞会在备份及迁移前拒绝启动。

备份文件名包含目标版本、UTC 时间和随机后缀，例如 `gvideo-pre-migration-v2-20260910T004700.123456789Z-<suffix>.db`。Linux 上目录权限为 `0700`、文件权限为 `0600`。如果创建、验证、收紧权限或发布备份失败，应用会关闭数据库连接并停止启动，不执行迁移；如果迁移本身失败，已经发布的迁移前备份会保留用于恢复和诊断。

系统不会自动清理 `.migration-backups/` 中的历史文件。值班人员应监控数据卷容量，在确认备份已归档且不再需要后按变更流程清理指定文件，不要删除整个目录。自动迁移前备份只覆盖 SQLite，不包含媒体卷；完整恢复点仍应使用下述数据库加媒体备份流程。
## 创建备份

```powershell
.\scripts\backup-data.ps1
```

备份默认写入被 Git 忽略的 `backups/gvideo-<timestamp>/`，包含：

- `database.db`：通过 SQLite `VACUUM INTO` 创建的一致性快照
- `media.tar.gz`：通过只读挂载归档的媒体卷
- `manifest.json`：SHA-256、大小、记录统计、媒体文件数和一致性说明

数据库与媒体是两个事务边界，因此不停服备份属于“可校验的在线近一致备份”。建议在低写入窗口执行；严格恢复点需要维护窗口并等待媒体任务结束。

## 验证备份

```powershell
.\scripts\verify-backup.ps1
.\scripts\verify-backup.ps1 -Backup .\backups\gvideo-20260810-120000-000
```

验证过程不会挂载真实命名卷或写回生产数据。脚本会校验清单、大小、SHA-256、危险归档路径、SQLite 完整性、媒体引用和当前版本迁移。

## 备份恢复演练

```powershell
.\scripts\backup-restore-drill.ps1
.\scripts\backup-restore-drill.ps1 -RemoveBackupOnSuccess
```

失败时脚本保留本轮备份用于诊断，不删除既有备份、真实数据库、媒体文件或 Docker 命名卷。

## 生产预检

```powershell
.\scripts\preflight-production.ps1
docker compose -f compose.yaml -f compose.https.yaml config
```

预检不会启动业务服务，也不会创建、删除或写入业务命名卷。

## 常见故障

### 后端根地址显示 404

后端主要提供 API 与健康检查，业务页面由前端入口提供：

- 开发页面：`http://127.0.0.1:5173`
- Docker 页面：`http://127.0.0.1:8088`
- 后端就绪检查：`http://127.0.0.1:8080/readyz`
- 后端存活检查：`http://127.0.0.1:8080/livez`
- 兼容健康端点：`http://127.0.0.1:8080/healthz`

`/livez` 只表示后端进程与 HTTP 服务仍可响应，不检查 SQLite、媒体 Worker、任务队列或 FFmpeg 等外部命令。`/readyz` 只表示启动初始化与迁移已经完成，且 SQLite 能在 1 秒内响应；它不检查队列、转码、Worker 或 FFmpeg。`/healthz` 保留存活语义以兼容既有客户端，backend Compose healthcheck 使用 `/readyz`。

### 上传后长期等待处理

```powershell
docker compose ps
docker compose logs --tail 150 backend
```

确认 FFmpeg 与 FFprobe 可执行、媒体卷有足够空间。失败不会删除原视频，作者可以在“我的投稿”中重试。

### 登录 Cookie 未发送

生产环境确认站点使用 HTTPS、前后端同源，且 `COOKIE_SECURE=true`。跨域部署需要重新评估 SameSite、CORS 和 CSRF。

### Docker 无法连接

```powershell
wsl --status
docker context show
docker info
```

### Go 依赖下载超时

```powershell
$env:GOPROXY = "https://goproxy.cn,direct"
$env:GOSUMDB = "sum.golang.google.cn"
cd backend
go mod download
```

## 数据安全边界

- 不要运行 `docker compose down -v`，除非明确需要永久删除业务数据。
- 不要把数据库、媒体、备份、证书、日志或隧道状态加入 Git。
- 恢复验证只能使用脚本创建的隔离目录或临时卷。
- 任何迁移和高风险运维操作前都应先创建并验证备份。

## 版本回退与真实命名卷恢复

应用回滚和数据恢复必须分开决策。仅当新版本没有写入不兼容数据时，才可只回退应用镜像；数据库或媒体已损坏、迁移失败或需要恢复到既定时间点时，使用已验证备份恢复。开始前记录当前 Git 提交、镜像标签、备份路径和事故时间线，并停止外部写入。

### 回退应用版本

1. 选择最后一个通过验收的 Git 提交或不可变镜像标签，不要使用浮动 `latest`。
2. 保留现有命名卷，只重建应用容器：

```powershell
git checkout <last-known-good-commit>
docker compose build backend frontend
docker compose up -d --no-deps --force-recreate backend frontend
docker compose ps
docker compose logs --tail 150 backend frontend
```

3. 验证 `http://127.0.0.1:8080/healthz`、`http://127.0.0.1:8088/healthz`，再运行 `./scripts/acceptance-api.ps1`。若旧版本拒绝当前迁移账本或接口验收失败，停止服务，不要反复重启或修改迁移记录，改走备份恢复。

### 恢复真实命名卷

默认真实卷为 `gvideo_gvideo-data` 和 `gvideo_gvideo-media`（可分别由 `GVIDEO_DATA_VOLUME`、`GVIDEO_MEDIA_VOLUME` 覆盖）。这是破坏性操作：会用备份内容替换当前数据。先保留故障现场备份，并确认目标备份已通过 `verify-backup.ps1` 的校验和隔离恢复验证；若要重新创建备份并演练完整链路，另行运行无 `-Backup` 参数的 `backup-restore-drill.ps1`。

```powershell
$backup = Resolve-Path .\backups\gvideo-<timestamp>
.\scripts\verify-backup.ps1 -Backup $backup
.\scripts\backup-data.ps1 # 保存当前故障现场；失败时停止并人工评估

docker compose down
$env:GVIDEO_DATA_VOLUME = "gvideo_gvideo-data"
$env:GVIDEO_MEDIA_VOLUME = "gvideo_gvideo-media"
```

随后由值班人员在维护窗口内使用一次性容器清空并恢复这两个**已核对名称**的卷。不要运行 `docker compose down -v`，不要把命令中的卷名替换为未经 `docker volume inspect` 确认的值：

```powershell
docker volume inspect $env:GVIDEO_DATA_VOLUME
docker volume inspect $env:GVIDEO_MEDIA_VOLUME

docker run --rm -v "${env:GVIDEO_DATA_VOLUME}:/restore" alpine:3.22 sh -c "rm -rf /restore/* /restore/.[!.]* /restore/..?*; mkdir -p /restore"
docker run --rm -v "${env:GVIDEO_DATA_VOLUME}:/restore" -v "${backup}:/backup:ro" alpine:3.22 sh -c "cp /backup/database.db /restore/gvideo.db"
docker run --rm -v "${env:GVIDEO_MEDIA_VOLUME}:/restore" alpine:3.22 sh -c "rm -rf /restore/* /restore/.[!.]* /restore/..?*; mkdir -p /restore"
docker run --rm -v "${env:GVIDEO_MEDIA_VOLUME}:/restore" -v "${backup}:/backup:ro" alpine:3.22 sh -c "tar -xzf /backup/media.tar.gz -C /restore"

docker compose up -d
docker compose ps
docker compose logs --tail 150 backend frontend
.\scripts\data-status.ps1
.\scripts\acceptance-api.ps1
```

恢复后必须确认双健康检查、迁移账本可打开、记录统计与备份清单一致、媒体引用可读取，并抽查登录、播放、Range/HLS 与一次受控上传。任一步失败时立即重新停止服务，保留容器日志和恢复现场；不要在真实卷上手工改表、删除 `schema_migrations` 或重复覆盖。回到隔离演练复现问题，必要时选择更早且已验证的备份，再由负责人批准第二次恢复。
