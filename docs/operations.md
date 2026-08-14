# GVideo 运维、验收与恢复

本文覆盖项目检查、上线验收、容量管理、数据备份和隔离恢复。部署与环境变量见 [deployment.md](deployment.md)。

## 日常检查

```powershell
.\scripts\check.ps1
```

检查内容包括 PowerShell 语法、Docker Compose 配置、Go 格式与测试、`go vet`、Vitest、TypeScript 类型检查、Vite 生产构建和 `git diff --check`。

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
- 后端健康检查：`http://127.0.0.1:8080/healthz`

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
