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
| `ADMIN_USERNAME` | 空 | 保留的管理员用户名，忽略大小写与首尾空白；首个注册该名称的账号获得管理员标志，此后该名称锁定。值必须符合用户名规则（3-24 位中文、字母、数字或下划线），否则启动 fail-fast |
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
| `RATE_LIMIT_AUTH_PER_MINUTE` | `20` | 登录/注册按客户端地址的每分钟上限；`0` 表示禁用该档 |
| `RATE_LIMIT_COMMENT_PER_MINUTE` | `30` | 发表评论按用户的每分钟上限；`0` 表示禁用该档 |
| `RATE_LIMIT_UPLOAD_PER_MINUTE` | `10` | 视频与字幕上传按用户的每分钟上限；`0` 表示禁用该档 |
| `USER_STORAGE_QUOTA_BYTES` | `0`（二进制默认，禁用）；Compose 部署默认 20 GiB | 每个用户的存储配额，按「源视频 + 封面 + HLS 转码实际产出」字节累计，超出后上传返回 413；HLS 字节在转码完成时入账，因此转码队列中的上传可能短暂超出配额 |
| `GVIDEO_DATA_VOLUME` | `gvideo_gvideo-data` | SQLite 命名卷 |
| `GVIDEO_MEDIA_VOLUME` | `gvideo_gvideo-media` | 媒体命名卷 |
| `GVIDEO_HOST` | `video.example.com` | HTTPS 公网主机名 |
| `HTTPS_HTTP_PORT` | `80` | HTTP 重定向端口 |
| `HTTPS_PORT` | `443` | TLS 端口 |
| `TLS_CERT_FILE` | 无 | 仓库外证书链绝对路径 |
| `TLS_KEY_FILE` | 无 | 仓库外私钥绝对路径 |

管理员升级说明：自 v2 迁移起管理员身份存储为 `users.is_admin` 持久标志，不再按用户名实时比较。升级后启动时若配置了 `ADMIN_USERNAME` 但库中还没有管理员，会记录 `admin_unclaimed` 警告——请注册该保留用户名，或使用 `gvideo data-grant-admin <username>`（见[运维文档](operations.md)）显式授予。若 `ADMIN_USERNAME` 不符合用户名规则，启动会拒绝运行，请在升级前修正配置。

真实密钥、证书、数据库和媒体不能进入 Git。

backend 容器设置 `stop_grace_period: 30s`，长于应用的 15 秒优雅关闭超时。收到停止信号后 HTTP 与管理端口先停止接收请求，媒体 Worker 通过同一取消上下文退出；若转码子进程未及时响应取消，Docker 会在 30 秒宽限期结束后强制终止容器。

`METRICS_ADDR` 与 `PPROF_ADDR` 是两个独立管理端口，均默认关闭。需要诊断时应绑定到 `127.0.0.1` 或隔离的可信管理网络，并由防火墙限制来源；反向代理不得公开 `/metrics` 或 `/debug/pprof/`。pprof 会暴露运行时与请求行为信息，仅在限时排障窗口启用，用完即关闭。生产环境会拒绝非 loopback/私网的绑定；非生产环境下绑定非 loopback 地址时启动会记录 `diagnostics_binding_exposed` 警告，提示该端点可被 loopback 之外的网络访问。

安全响应头：后端对所有 API 与媒体响应设置 `X-Content-Type-Options: nosniff`、`X-Frame-Options: DENY`、`Referrer-Policy: strict-origin-when-cross-origin`、`Permissions-Policy`（关闭摄像头/麦克风/定位）以及 `Content-Security-Policy: default-src 'none'`；前端 Nginx 为 SPA 与代理响应设置框架拒绝与 CSP，并关闭 `server_tokens`。**自定义域名或替换反代时，如果为 HTML 覆盖 CSP，必须保留 `media-src 'self' blob:` 与 `worker-src 'self' blob:`**，否则 hls.js 的 MediaSource 播放与解码 Worker 会被浏览器拒绝。

限流说明：登录/注册按客户端地址限流，评论与上传按用户限流，超限返回 `429` 与 `Retry-After`（秒）。后端只信任来自 loopback/私网对端的 `X-Forwarded-For`（从右向左取最后一个公网地址）与 `X-Real-IP`，并且永不信任 `True-Client-IP`；**替换或前置新的反向代理时必须覆写 `X-Forwarded-For` 为 `$remote_addr` 并清空 `True-Client-IP`**，否则客户端可伪造转发链绕过按地址限流。同一 NAT 出口后的用户共享登录配额，容量不足时调高 `RATE_LIMIT_AUTH_PER_MINUTE` 或设为 `0` 关闭该档。被限流的大文件上传可能因请求体未读完而收到连接重置，客户端应把网络错误与 429 一并视为退避信号。

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
- 后端就绪检查：`http://127.0.0.1:8080/readyz`
- 后端存活检查：`http://127.0.0.1:8080/livez`
- 兼容健康端点：`http://127.0.0.1:8080/healthz`
- 同源健康检查：`http://127.0.0.1:8088/healthz`

`/livez` 仅用于确认后端进程与 HTTP 服务可响应；`/readyz` 在启动迁移完成后注入检查器，并要求 SQLite 在 1 秒内响应。两者都不检查媒体 Worker、队列、转码或外部命令。`/healthz` 保留原存活语义，backend Compose healthcheck 已切换到 `/readyz`。

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

生产预检会检查 HTTPS Origin、主机名、端口、管理员、固定卷名称、证书有效期、证书域名、证书与私钥匹配，并通过隔离的 `nginx -t` 和渲染配置语义断言验证网关配置。预检不会启动完整服务，也不能替代运行态验收。

生产入口应只公开网关端口。HTTP 使用 `308` 跳转 HTTPS，后端必须保持 `COOKIE_SECURE=true`。

`/gateway-healthz` 只检查 Nginx HTTP/TLS 入口存活；`/gateway-readyz` 经 frontend 转发到 backend `/readyz`，检查完整上游链路和 SQLite readiness。生产负载均衡器应使用 HTTPS `/gateway-readyz` 决定是否分配新流量，并单独监控 `/gateway-healthz` 以区分网关故障和上游故障。Docker healthcheck 失败只会把容器标记为 `unhealthy`，`restart: unless-stopped` 不会因该状态自动重启容器。

## 云平台边界

前端静态资源可以部署到 Vercel，但当前 GVideo 还包含 Go API、SQLite 持久写入、本地媒体目录和 FFmpeg 长任务，不能只靠 Vercel 完成整套生产部署。

现阶段推荐：

1. 免费测试使用本机 Docker + Cloudflare Quick Tunnel。
2. 持续测试使用支持持久磁盘和 Docker 的云服务器。
3. 需要前后端分离时，再将前端部署到 Vercel，并独立部署后端、数据库、对象存储和转码 worker。
4. 多实例写入成为真实需求后迁移 PostgreSQL；媒体容量或多机共享成为需求后迁移 S3/MinIO。

Supabase 可以承接未来的 PostgreSQL、认证或对象存储，但不能直接运行当前 FFmpeg worker，也不能无改造替代本地媒体授权与处理流程。
