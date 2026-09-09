# GVideo 部署与配置

本文说明本地开发、Docker、免费公网测试和生产 HTTPS 部署。备份、恢复、容量检查和上线验收见 [operations.md](operations.md)。

## 配置方式

配置模板位于根目录 [.env.example](../.env.example)。项目后端不会自动读取 `.env` 文件，配置应在启动前写入当前环境，或由部署平台注入。

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `APP_ENV` | `development` | 运行环境标识 |
| `HTTP_ADDR` | `:8080` | 后端监听地址 |
| `METRICS_ADDR` | 空 | Prometheus 管理端监听地址；默认关闭，只允许绑定 loopback 或可信管理网络 |
| `PPROF_ADDR` | 空 | Go pprof 管理端监听地址；默认关闭，不得暴露公网 |
| `FRONTEND_URL` | `http://127.0.0.1:5173` | 前端公开入口，必须是无路径 Origin |
| `ADMIN_USERNAME` | 空 | 指定现有用户名为管理员，大小写不敏感 |
| `DATABASE_PATH` | `./data/gvideo.db` | SQLite 数据库路径 |
| `MEDIA_DIR` | `./media` | 媒体文件根目录 |
| `SESSION_TTL` | `168h` | 登录有效期 |
| `MAX_UPLOAD_BYTES` | `536870912` | 单视频上传上限，默认 512 MiB |
| `COOKIE_SECURE` | `false` | 生产 HTTPS 环境必须为 `true` |
| `FFMPEG_PATH` | `ffmpeg` | FFmpeg 命令或绝对路径 |
| `FFPROBE_PATH` | `ffprobe` | FFprobe 命令或绝对路径 |
| `MEDIA_WORKER_ENABLED` | `true` | 是否启动媒体任务 worker |
| `MEDIA_WORKER_POLL_INTERVAL` | `2s` | worker 空闲轮询间隔 |
| `MEDIA_PROBE_TIMEOUT` | `30s` | FFprobe 单次超时 |
| `HLS_ENABLED` | `true` | 是否生成 HLS |
| `HLS_TRANSCODE_TIMEOUT` | `30m` | 单视频 HLS 转码超时 |
| `HLS_SEGMENT_SECONDS` | `4` | HLS 切片目标时长，允许 2 到 10 秒 |
| `GVIDEO_DATA_VOLUME` | `gvideo_gvideo-data` | SQLite 命名卷 |
| `GVIDEO_MEDIA_VOLUME` | `gvideo_gvideo-media` | 媒体命名卷 |
| `GVIDEO_HOST` | `video.example.com` | HTTPS 公网主机名 |
| `HTTPS_HTTP_PORT` | `80` | HTTP 重定向端口 |
| `HTTPS_PORT` | `443` | TLS 端口 |
| `TLS_CERT_FILE` | 无 | 仓库外证书链绝对路径 |
| `TLS_KEY_FILE` | 无 | 仓库外私钥绝对路径 |

真实密钥、证书、数据库和媒体不能进入 Git。

backend 容器设置 `stop_grace_period: 30s`，长于应用的 15 秒优雅关闭超时。收到停止信号后 HTTP 与管理端口先停止接收请求，媒体 Worker 通过同一取消上下文退出；若转码子进程未及时响应取消，Docker 会在 30 秒宽限期结束后强制终止容器。

`METRICS_ADDR` 与 `PPROF_ADDR` 是两个独立管理端口，均默认关闭。需要诊断时应绑定到 `127.0.0.1` 或隔离的可信管理网络，并由防火墙限制来源；反向代理不得公开 `/metrics` 或 `/debug/pprof/`。pprof 会暴露运行时与请求行为信息，仅在限时排障窗口启用，用完即关闭。

## Windows 开发环境

```powershell
cd frontend
npm ci
cd ..
.\scripts\dev.ps1
```

页面地址为 `http://127.0.0.1:5173`。脚本会复用 Docker 命名卷中的持久数据库。

只有明确需要调试本机 Go 进程时，才分别运行：

```powershell
# 终端 1
cd backend
go run ./cmd/server

# 终端 2
cd frontend
npm run dev
```

这种模式默认使用 `backend/data/gvideo.db` 与 `backend/media/`，和 Docker 命名卷是两套独立存储。不要同时启动本机后端与 Docker 后端占用 `8080`。

## Docker 部署

```powershell
docker compose up --build -d
docker compose ps
docker compose logs --tail 100 backend frontend
```

默认入口：

- 页面：`http://127.0.0.1:8088`
- 后端健康检查：`http://127.0.0.1:8080/healthz`
- 同源健康检查：`http://127.0.0.1:8088/healthz`

停止容器与项目网络：

```powershell
docker compose down
```

该命令保留命名卷。不要执行 `docker compose down -v`，`-v` 会删除 SQLite 与媒体卷。

若当前网络无法访问 Docker Hub，可以保留 Compose 默认镜像代理；可直连官方服务时可覆盖：

```powershell
$env:DOCKER_IMAGE_REGISTRY = "docker.io"
$env:GOPROXY = "https://proxy.golang.org,direct"
docker compose up --build -d
```

## 免费公网测试

安装 `cloudflared` 后，可以用 Cloudflare Quick Tunnel 临时公开本机 Docker 站点：

```powershell
.\scripts\start-public-test.ps1
```

脚本会启动或复用 Docker 服务、创建临时 HTTPS 地址、切换公网 `FRONTEND_URL`、启用 Secure Cookie，并把状态写入被 Git 忽略的 `tmp/public-test.json`。

```powershell
.\scripts\acceptance.ps1 `
  -BaseURL https://example.trycloudflare.com `
  -SkipBuildChecks `
  -SkipCapacityCheck
```

结束后运行：

```powershell
.\scripts\stop-public-test.ps1
```

Quick Tunnel 只用于临时测试，不提供固定域名或可用性保证。

## 生产 HTTPS

生产环境使用 `compose.https.yaml` 增加 Nginx TLS 网关。证书和私钥必须保存在仓库外，并以只读挂载方式注入。

```powershell
$env:FRONTEND_URL = "https://video.example.com"
$env:GVIDEO_HOST = "video.example.com"
$env:ADMIN_USERNAME = "admin"
$env:COOKIE_SECURE = "true"
$env:TLS_CERT_FILE = "C:\secure\gvideo\fullchain.pem"
$env:TLS_KEY_FILE = "C:\secure\gvideo\privkey.pem"

.\scripts\preflight-production.ps1
docker compose -f compose.yaml -f compose.https.yaml config
docker compose -f compose.yaml -f compose.https.yaml up --build -d
```

生产预检会检查 HTTPS Origin、主机名、端口、管理员、固定卷名称、证书有效期、证书域名、证书与私钥匹配，并通过隔离的 `nginx -t` 验证网关配置。

生产入口应只公开网关端口。HTTP 使用 `308` 跳转 HTTPS，后端必须保持 `COOKIE_SECURE=true`。

## 云平台边界

前端静态资源可以部署到 Vercel，但当前 GVideo 还包含 Go API、SQLite 持久写入、本地媒体目录和 FFmpeg 长任务，不能只靠 Vercel 完成整套生产部署。

现阶段推荐：

1. 免费测试使用本机 Docker + Cloudflare Quick Tunnel。
2. 持续测试使用支持持久磁盘和 Docker 的云服务器。
3. 需要前后端分离时，再将前端部署到 Vercel，并独立部署后端、数据库、对象存储和转码 worker。
4. 多实例写入成为真实需求后迁移 PostgreSQL；媒体容量或多机共享成为需求后迁移 S3/MinIO。

Supabase 可以承接未来的 PostgreSQL、认证或对象存储，但不能直接运行当前 FFmpeg worker，也不能无改造替代本地媒体授权与处理流程。
