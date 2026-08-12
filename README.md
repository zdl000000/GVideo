# GVideo

GVideo 是一个面向视频创作者与观众的社区型全栈项目，交互参考主流视频社区，但不复制 B 站品牌、代码或架构。当前基线已打通注册登录、发现与播放、投稿管理、互动评论、作者空间、关注关系和关注动态这一条完整社区流程。

参考项目 [My-TuDo/B-B](https://github.com/My-TuDo/B-B) 只用于理解常见视频社区功能。GVideo 当前采用更适合学习和单机部署的模块化单体，暂不引入微服务、RabbitMQ、Redis、MinIO 或复杂推荐系统。

当前工作区已接通用户资料与头像、创作者工作台、投稿可见性、媒体鉴权、可观察的转码进度和字幕轨道管理，并提供可选 HTTPS 网关、SQLite 与媒体卷备份清单及隔离恢复演练。后端路由、领域类型、SQLite 字段与兼容迁移已经加入，前端已提供工作台、统一头像展示、资料编辑、可见性控件、处理进度和字幕管理界面。

## 功能范围

- 注册、登录、退出和可撤销服务端 session
- 通过首页 `/` 浏览推荐内容，通过独立的最新页 `/latest` 查看发布时间线，通过热门发现页 `/popular` 查看热度榜，并支持关键词与分区筛选
- MP4、WebM、Ogg 视频上传及真实文件头校验
- FFprobe 后台探测时长、分辨率、码率和编解码信息，FFmpeg 自动截取封面并生成 HLS 自适应播放文件，也可上传 JPEG/PNG/WebP 封面
- HLS 自动清晰度与播放器内手动档位菜单；按源分辨率提供不超过原片的 `360p / 480p / 720p`，失败时保留原始 MP4 回退
- 按视频保存本机播放进度，重新进入时可继续播放或从头播放；播放页支持系统分享或复制链接
- SQLite 持久化媒体任务，支持服务重启恢复、失败后手动重试和投稿处理状态展示
- 服务升级时自动为缺少媒体元数据的旧视频补建探测任务
- 视频播放、播放量、点赞、收藏和评论
- 首页、最新、热门和我的投稿均支持服务端分页，页码保存在 URL 中
- 作者空间 `/users/:id` 展示资料、加入时间、投稿/粉丝/关注统计和服务端分页投稿；本人空间进入投稿管理，其他用户可关注或取消关注
- 登录用户可通过 `/following` 按发布时间浏览所关注作者的最新投稿，未登录访问会先跳转登录并在成功后返回
- 后端支持登录用户编辑用户名、最多 300 字简介和 JPEG/PNG/WebP 头像；头像路径会同步出现在用户、作者、视频和评论响应中
- 创作者工作台 `/creator` 使用本人聚合统计接口展示投稿、粉丝、播放、互动、可见性分布、处理中数量和最近投稿
- 投稿支持 `public`、`unlisted`、`private` 三种可见性；公共列表只显示公开投稿，本人投稿列表显示全部投稿
- 我的投稿支持编辑标题、简介、分区、可见性和封面，支持删除投稿及相关媒体文件
- 播放页、创作者工作台和投稿管理页展示后台转码阶段与百分比；失败响应只返回安全、可操作的分类文案
- 视频作者可在播放页上传、设为默认或删除字幕轨道；删除默认轨道后自动选择剩余最早轨道
- `/media/*` 在返回原视频、封面、字幕和 HLS 文件前按所属视频鉴权；头像公开，私密视频媒体仅作者可读
- 浅色/深色主题切换，并记住用户选择
- 结构化日志、请求 ID、统一 JSON 错误响应、CSRF 防护
- Go 单元测试、HTTP 核心流程测试、TypeScript 检查和生产构建

暂未实现：弹幕、通知、审核后台、推荐算法和微服务拆分。

## 技术栈

| 区域 | 技术 | 选择原因 |
| --- | --- | --- |
| 后端 | Go 1.26、Chi 5 | 标准库风格清晰，路由层轻量，适合学习分层和 HTTP 基础 |
| 前端 | React 19、TypeScript 7、Vite 8 | 组件生态成熟，严格类型能尽早暴露接口契约问题 |
| 数据 | SQLite、modernc.org/sqlite | 纯 Go 驱动，不需要本地数据库服务即可运行完整流程 |
| 媒体 | 本地文件目录、FFmpeg 8 | MVP 部署简单，文件接口边界可在后续替换为 MinIO/S3 |
| 部署 | Docker Compose、Nginx | 前后端同源代理，避免生产环境额外的跨域与 cookie 配置 |

学习成本主要在 Go 的错误处理与 `context`、React 的状态与 Effect、HTTP cookie/CSRF，以及视频文件处理。建议先跑通接口，再逐层阅读代码。

## 架构

后端是按职责分层的模块化单体：

```text
浏览器
  -> frontend/src/api.ts
  -> HTTP / JSON / multipart
  -> httpapi     输入解析、认证接入、CSRF、响应转换
  -> service     业务校验、会话、上传与媒体规则
  -> repository  参数化 SQL 和数据映射
  -> SQLite / media

后台媒体流程：

上传原始文件 -> 同一事务创建视频与任务 -> worker 领取任务
  -> FFprobe 结构化探测 -> FFmpeg 生成 HLS 临时目录
  -> 完整成功后发布 master/variant/segment -> 回写媒体信息与状态

浏览器播放流程：

接口返回 HLS master 地址和原始文件地址
  -> 原生支持 HLS 时直接播放
  -> 其他现代浏览器由 hls.js 加载并提供自动/手动清晰度
  -> HLS 不可用或发生致命错误时回退原始文件
```

目录说明：

```text
backend/
  cmd/server/             应用入口与优雅关闭
  internal/httpapi/       HTTP 路由、中间件、接口测试
  internal/service/       核心业务规则
  internal/repository/    数据访问
  internal/domain/        实体与稳定业务错误
  internal/platform/      SQLite 初始化和迁移
  internal/media/         FFprobe、FFmpeg HLS 转码器与后台媒体 worker
frontend/
  src/api.ts              统一请求、cookie 与 CSRF 接入
  src/App.tsx             页面、路由和交互组件
  src/styles.css          响应式视觉系统
docs/project-standards.md 当前项目专项开发规范
deploy/nginx/             HTTPS 网关配置
scripts/                  检查、数据状态、备份与隔离恢复演练
compose.yaml              本机 HTTP 部署
compose.https.yaml        可选生产 HTTPS overlay
scripts/                  Windows 开发与检查脚本
```

协议层不会直接访问数据库，前端组件也不会绕开 `api.ts` 拼接认证请求。完整专项规范见 [docs/project-standards.md](docs/project-standards.md)。

### 页面路由

| 路由 | 用途 |
| --- | --- |
| `/`、`/latest`、`/popular` | 首页精选、最新投稿和热门发现 |
| `/video/:id` | 播放、互动、评论与作者入口 |
| `/users/:id` | 作者资料、统计、关注操作和分页投稿 |
| `/following` | 登录用户的关注动态，未登录时跳转 `/auth?next=/following` |
| `/upload`、`/me/videos` | 发布视频和管理本人投稿 |
| `/creator` | 登录用户的创作者工作台，展示聚合表现、可见性分布、处理状态和最近投稿 |
| `/auth` | 独立布局的登录与注册页，支持 `next` 回跳 |

### 作者与关注 API

| 方法与路径 | 说明 |
| --- | --- |
| `GET /api/v1/users/{userID}` | 作者资料、投稿/粉丝/关注统计和当前 viewer 的关注状态 |
| `GET /api/v1/users/{userID}/videos` | 作者投稿列表，支持 `page` 与 `page_size` |
| `POST /api/v1/users/{userID}/follow` | 登录并通过 CSRF 校验后切换关注关系；禁止关注自己 |
| `GET /api/v1/me/following/videos` | 仅返回当前用户已关注作者的投稿，按最新发布排序并分页 |

SQLite 使用 `user_follows(follower_id, followed_id, created_at)` 保存关系，复合主键保证同一关系唯一，`CHECK` 禁止自关注，两端外键均使用 `ON DELETE CASCADE`。`idx_user_follows_followed` 支持粉丝统计和被关注用户查询；数据库合并会重映射两端用户 ID，旧来源库没有该表时仍可兼容导入。

### 用户资料、创作统计与可见性 API

| 方法与路径 | 说明 |
| --- | --- |
| `PATCH /api/v1/me/profile` | 登录并通过 CSRF 校验后更新 `username`、`bio` 和可选 `avatar`；使用 `multipart/form-data` |
| `GET /api/v1/me/creator/stats` | 返回本人投稿、粉丝、播放、点赞、收藏、评论、三种可见性数量、处理中数量和最近 5 条投稿 |
| `POST /api/v1/videos` | 上传表单接受 `visibility`；空值兼容为 `public`，其他值必须是三种合法枚举之一 |
| `PATCH /api/v1/videos/{videoID}` | 作者可同时更新标题、简介、分区、`visibility` 和可选封面 |
| `GET /api/v1/me/videos` | 登录用户查看本人全部投稿，包括 `unlisted` 和 `private` |
| `GET /api/v1/videos/{videoID}` | `public` 与 `unlisted` 可通过直链读取；`private` 仅作者可读，其他访问统一返回 404 |
| `GET /media/*` | 头像路径公开；其他媒体按所属视频可见性和当前 viewer 鉴权，并由 `http.ServeFile` 保留 Range/206 能力 |

资料更新沿用注册时的用户名规则：3 到 24 位中文、字母、数字或下划线，简介最多 300 个 Unicode 字符。头像和封面共用真实文件头校验，只接受 JPEG、PNG、WebP，单文件上限 10 MiB；新头像写入成功但数据库更新失败时会回收新文件，更新成功后才清理旧头像。

SQLite 在 `users` 表增加 `avatar_path TEXT NOT NULL DEFAULT ''`，在 `videos` 表增加 `visibility TEXT NOT NULL DEFAULT 'public'`。新建表使用 `CHECK (visibility IN ('public', 'unlisted', 'private'))`；旧库启动时自动补列，数据库合并会检测来源库是否包含新字段，不存在时分别回退为空头像和 `public`，从而保持旧库导入兼容与幂等性。

可见性规则如下：

| 场景 | `public` | `unlisted` | `private` |
| --- | --- | --- | --- |
| 首页、最新、热门、搜索 | 显示 | 不显示 | 不显示 |
| 作者空间、关注动态 | 显示 | 不显示 | 不显示 |
| 本人投稿、创作者工作台 | 显示 | 显示 | 显示 |
| 视频详情直链 | 可访问 | 可访问 | 仅作者可访问 |
| 原视频、封面、字幕、HLS | 可访问 | 可访问 | 仅作者可访问 |

### 媒体处理进度与字幕 API

`Video` 响应增加 `processing_progress` 和 `processing_stage`。阶段按持久化任务推进：

| 状态 | 进度 | 阶段 |
| --- | ---: | --- |
| 新上传、自动重试、手动重试 | 0 | `queued` |
| worker 领取任务 | 10 | `probing` |
| FFprobe 完成 | 35 | `transcoding` |
| FFmpeg 完成 | 90 | `finalizing` |
| 任务完成 | 100 | `ready` |
| 最终失败 | 保留最后进度 | `failed` |

SQLite 使用 `videos.processing_progress INTEGER NOT NULL DEFAULT 100` 和 `videos.processing_stage TEXT NOT NULL DEFAULT 'ready'`。旧库启动迁移和数据库合并在字段缺失时根据 `processing_status` 回退，且不会覆盖已迁移库中的现有进度。

原始 FFprobe/FFmpeg 错误只保存在数据库和结构化日志中。API 的 `processing_error` 由服务层分类为源文件不可用、处理超时、格式或编码不支持、存储不足或通用处理失败，不返回本机路径、命令输出和内部日志。

| 方法与路径 | 说明 |
| --- | --- |
| `PATCH /api/v1/videos/{videoID}/subtitles/{subtitleID}/default` | 视频作者将指定轨道设为唯一默认轨道，返回最新完整轨道列表 |
| `DELETE /api/v1/videos/{videoID}/subtitles/{subtitleID}` | 视频作者删除轨道，返回最新完整轨道列表 |

两个接口都要求登录与 CSRF 校验。删除默认轨道时，剩余轨道中 ID 最小的一条自动成为默认；删除最后一条后返回空数组。字幕数据库事务成功后才清理媒体文件。

## 环境准备

已在当前 Windows 环境验证：

- Go `1.26.5`
- Node.js `24.16.0`、npm `11.13.0`
- FFmpeg Essentials `8.1.1`
- Git `2.54.0.windows.1`

安装依赖：

```powershell
cd frontend
npm install
```

Go 依赖若访问默认代理超时，可在当前终端使用：

```powershell
$env:GOPROXY = "https://goproxy.cn,direct"
$env:GOSUMDB = "sum.golang.google.cn"
cd backend
go mod download
```

## 配置

默认配置可直接用于本地开发。配置项参考 [.env.example](.env.example)：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | 后端监听地址 |
| `DATABASE_PATH` | `./data/gvideo.db` | SQLite 文件路径 |
| `MEDIA_DIR` | `./media` | 视频与封面目录 |
| `SESSION_TTL` | `168h` | 登录有效期 |
| `MAX_UPLOAD_BYTES` | `536870912` | 单个视频最大 512 MiB |
| `COOKIE_SECURE` | `false` | HTTPS 生产环境必须设为 `true` |
| `HTTPS_HTTP_PORT` | `80` | HTTPS overlay 的 HTTP 重定向端口 |
| `HTTPS_PORT` | `443` | HTTPS overlay 的 TLS 端口 |
| `TLS_CERT_FILE` | 无 | 宿主机证书链绝对路径，仅供 HTTPS overlay 只读挂载 |
| `TLS_KEY_FILE` | 无 | 宿主机私钥绝对路径，仅供 HTTPS overlay 只读挂载 |
| `FFMPEG_PATH` | `ffmpeg` | FFmpeg 命令或绝对路径 |
| `FFPROBE_PATH` | `ffprobe` | FFprobe 命令或绝对路径 |
| `MEDIA_WORKER_ENABLED` | `true` | 是否启动进程内媒体任务 worker |
| `MEDIA_WORKER_POLL_INTERVAL` | `2s` | worker 空闲时轮询间隔 |
| `MEDIA_PROBE_TIMEOUT` | `30s` | 单次 FFprobe 探测超时 |
| `HLS_ENABLED` | `true` | 是否为媒体任务生成 HLS 输出 |
| `HLS_TRANSCODE_TIMEOUT` | `30m` | 单个视频 HLS 转码最长时间 |
| `HLS_SEGMENT_SECONDS` | `4` | HLS 切片目标时长，允许 2 到 10 秒 |

项目不会自动读取 `.env` 文件；可以在启动命令前设置环境变量，或由部署平台注入。不要提交真实密钥或用户数据。

## 本地运行

Windows 一键开发：

```powershell
.\scripts\dev.ps1
```

浏览器打开 `http://127.0.0.1:5173`。脚本会先启动 Docker 后端，再启动 Vite；`5173` 与 Docker 页面 `8088` 都通过后端 `8080` 访问同一个 `gvideo_gvideo-data` 数据卷，因此注册账号、投稿、评论和互动不会因切换页面入口而分成两套数据。

如需完全脱离 Docker 调试 Go 后端，可以分别启动：

```powershell
# 终端 1
cd backend
go run ./cmd/server

# 终端 2
cd frontend
npm run dev
```

这种方式使用 `backend/data/gvideo.db` 和 `backend/media/`，与 Docker 命名卷是两套独立存储，只用于明确需要本机后端调试时。日常开发优先使用 `scripts/dev.ps1`，不要同时启动本机后端与 Docker 后端占用 `8080`。

如果新终端仍找不到 winget 安装的 FFmpeg，设置 `.env.example` 中两个路径变量为本机 `ffmpeg.exe` 和 `ffprobe.exe` 的绝对路径。

## 测试与构建

执行全部检查：

```powershell
.\scripts\check.ps1
```

等价的分步命令：

```powershell
cd backend
$env:GOCACHE='..\tmp\go-build'
go fmt ./...
go test ./... -count=1
go vet ./...

cd ..\frontend
npm run typecheck
npm run build

cd ..
git diff --check
```

HTTP 流程测试使用临时数据库和媒体目录，覆盖注册、上传、分页、媒体读取、点赞、评论、投稿编辑权限和删除，不会写入开发数据。服务层测试还覆盖失败任务重新入队、旧 HLS 清理，以及转码中的投稿禁止删除。

关注功能测试覆盖关注切换、粉丝与关注统计、登录 viewer 的 `followed` 状态、关注动态过滤、自关注禁止、目标用户不存在、401、CSRF 403、作者资料与投稿分页、删除用户后的级联清理、`DatabaseStats.Follows`，以及数据库合并对 `user_follows` 的兼容与幂等性。

用户资料、创作者统计、可见性与媒体鉴权测试覆盖资料更新与用户名冲突、非法头像、头像替换和失败回收、无投稿创作者统计、三种可见性的列表与详情矩阵、关注动态不泄露非公开投稿、私密视频互动权限、原视频/封面/字幕/HLS 鉴权、Range/206、HTTP 401/403/multipart，以及旧库升级和数据库合并对 `avatar_path`、`visibility` 的兼容与幂等性。

媒体处理与字幕管理测试覆盖任务领取、阶段推进、自动和手动重试、最终失败、完成状态、安全失败文案、旧库进度回填、数据库合并兼容与幂等、默认轨道唯一性、删除默认轨道回退、删除最后轨道、文件清理、作者权限、用户不存在、HTTP 401 和 CSRF 403。

2026-08-10 已真实执行并通过以下阶段验收：

- 后端在项目 `tmp/go-build` 缓存下通过 `go test ./... -count=1` 与 `go vet ./...`
- 前端通过 `npm run typecheck` 与 `npm run build`；Vite 仅提示单个 chunk 体积较大，不影响构建
- 根目录通过 `git diff --check`
- `docker compose -f compose.yaml up --build -d` 构建并启动成功，`8080/healthz` 与 `8088/healthz` 均返回 `200`
- 桌面与 `390 × 844` 移动视口完成只读浏览器验收，覆盖首页、播放页、作者字幕管理入口、创作者工作台、投稿管理、独立登录布局、未登录关注动态回跳、深浅主题、焦点样式和横向溢出检查；控制台无警告或错误
- 匿名验收使用同一服务的 `localhost:8088` 隔离现有 `127.0.0.1` 登录 Cookie，`/following` 准确跳转到 `/auth?next=%2Ffollowing`；全程未填写凭证、提交表单、创建账号、上传内容或修改业务数据

以上浏览器结果在 2026-08-12 以最新镜像重新验收：桌面首页、作者空间、播放页作者/评论/相关推荐链接、移动 `390 × 844` 横向溢出、关注动态登录态页面、独立登录布局、深浅主题、移动菜单语义、投稿编辑弹窗焦点与 Escape 关闭均完成只读检查；控制台无错误或警告。匿名验收使用同一服务的 `localhost:8088` 隔离现有 `127.0.0.1` 登录 Cookie，并确认 `/following` 跳转 `/auth?next=%2Ffollowing`。验收过程未填写凭证、提交表单、创建账号、上传内容或修改业务数据。

临时数据库只存在于 `go test` 自动化测试中，测试结束即删除。浏览器访问 `5173` 或 `8088` 时不使用临时数据库。

## Docker 部署

当前开发机已经安装并验证 Docker Desktop、Docker Engine、Docker Compose 和 WSL2。项目使用两个命名卷：

- `gvideo_gvideo-data`：保存 SQLite 数据库，包括用户、会话、视频元数据和评论。
- `gvideo_gvideo-media`：保存上传的视频、封面和 `hls/<video-id>/` 自适应播放文件。

当前网络无法直接访问 Docker Hub，因此 Compose 默认从 `dockerproxy.net` 拉取基础镜像，并在 Go 构建阶段使用 `https://goproxy.cn,direct`。如所在网络可以直连官方服务，可在启动前覆盖：

```powershell
$env:DOCKER_IMAGE_REGISTRY = "docker.io"
$env:GOPROXY = "https://proxy.golang.org,direct"
```

构建并启动：

```powershell
docker compose up --build -d
```

开发期间的断联检查由 Codex 任务自动化负责，仓库不包含常驻守护进程。Compose 自身的 `restart: unless-stopped` 继续负责容器进程级重启；自动化只在检查到 Docker 或 HTTP 健康异常时恢复服务，不删除数据库与媒体命名卷。

查看状态和日志：

```powershell
docker compose ps
docker compose logs --tail 100 backend frontend
```

查看持久数据库中的业务记录数量：

```powershell
.\scripts\data-status.ps1
```

创建数据库与媒体备份包；默认在被 Git 忽略的 `backups/` 中生成不可覆盖的时间戳目录，包含 `database.db`、`media.tar.gz` 和带 SHA-256、大小、记录统计、媒体文件数的 `manifest.json`：

```powershell
.\scripts\backup-data.ps1
```

备份数据库使用 SQLite `VACUUM INTO`，可正确处理 WAL 模式；媒体卷通过独立容器只读挂载后归档，不停止现有服务，也不写入媒体卷。由于数据库快照与媒体归档不是同一个跨资源事务，这是一份可校验的在线近一致备份，建议在没有上传、删除、资料图片修改和转码发布的低写入窗口执行。需要严格恢复点时，应先在网关层进入维护窗口并等待当前媒体任务完成。

在不挂载真实命名卷、不发布端口、不写回现有数据的情况下验证最新备份，或指定一个备份目录：

```powershell
.\scripts\verify-backup.ps1
.\scripts\verify-backup.ps1 -Backup .\backups\gvideo-20260810-120000-000
```

恢复演练会校验清单、大小、SHA-256、归档路径与条目类型；以只读方式执行 SQLite `integrity_check` 和 `foreign_key_check`；在 `tmp/restore-verification-*` 隔离目录解包媒体；确认头像、原视频大小、封面、字幕以及 HLS 主清单、变体和切片都存在；最后在数据库副本上执行当前版本迁移和统计比对。脚本结束后只删除自己创建的 `tmp` 验证目录，不删除备份包。

### HTTPS 部署

默认 Compose 仍用于本机 HTTP 开发，后端和前端分别只绑定 `127.0.0.1:8080`、`127.0.0.1:8088`。公网部署使用 HTTPS overlay；真实证书和私钥必须位于仓库外或被 `.gitignore` 排除，并通过只读挂载注入：

```powershell
$env:TLS_CERT_FILE = "C:\secure\gvideo\fullchain.pem"
$env:TLS_KEY_FILE = "C:\secure\gvideo\privkey.pem"
docker compose -f compose.yaml -f compose.https.yaml config
docker compose -f compose.yaml -f compose.https.yaml up --build -d
```

HTTPS overlay 增加 `gateway` 服务，HTTP 使用 `308` 跳转 HTTPS，TLS 入口继续同源代理前端、API 和媒体请求，并强制后端 `COOKIE_SECURE=true`。启动前应检查渲染后的 Compose 配置中只有网关端口面向公网，证书路径正确，backend 环境为 `COOKIE_SECURE: "true"`。自定义监听端口可设置 `HTTPS_HTTP_PORT` 与 `HTTPS_PORT`；网关启动脚本会由 `HTTPS_PORT` 自动派生重定向端口后缀，避免端口配置不一致。网关 healthcheck 同时检查 HTTP 健康端点和 HTTPS TLS 响应；正式反向代理或负载均衡部署仍应只开放实际 TLS 入口。

访问地址：

- Web 页面：`http://127.0.0.1:8088`
- 健康检查：`http://127.0.0.1:8088/healthz`

停止并移除容器和项目网络时，使用下面的命令；命名卷会保留，下次启动仍能读取原有数据：

```powershell
docker compose down
```

不要执行 `docker compose down -v`，其中的 `-v` 会同时删除 SQLite 与媒体命名卷。只有明确需要清空全部业务数据时才应删除卷。

本机已经实际验证镜像构建、容器健康检查、前端路由、API 反向代理，以及注册用户和登录会话在 `docker compose down`、`docker compose up -d` 后仍然存在。

2026-08-09 已将早期本机开发库安全合并到 Docker 数据卷，并同步本机媒体文件。合并后实际包含 12 个用户、3 个视频、6 条评论、5 个点赞和 1 个收藏；完整执行 `docker compose down` 与 `docker compose up -d` 后数量保持一致，三个视频的 Range 请求均返回 `206 Partial Content`。迁移前的本机库和 Docker 库备份保存在本机 `backups/`，不会进入 Git。

### 真实视频流程验证

Docker 环境中已使用 FFmpeg 生成并通过公开 API 上传一条 3 秒 H.264/AAC 视频，实际验证以下流程：

- 上传请求保存原始 MP4，并在未提供封面时自动生成 JPEG 封面。
- 媒体 worker 使用 FFprobe 写入时长、`640 × 360` 分辨率和音视频编码信息，状态从 `pending` 变为 `ready`。
- Nginx 与后端可完整返回视频，也支持播放器拖动进度所需的 HTTP Range 请求（`206 Partial Content`）。
- 首页能够展示真实视频卡片，播放页能够加载视频、封面、作者、简介和评论。
- 点赞、收藏、评论及其统计数经过 API 实际验证。
- 浏览器中的 `<video>` 元素成功读取 3 秒时长和 `640 × 360` 视频尺寸，未产生媒体错误。

验证视频当前保留为开发数据，可直接访问 `http://127.0.0.1:8088/video/1`。本阶段已完成 HLS 播放验证：3 条历史视频均生成 master playlist 并可通过 HTTP 返回 `200`；视频 1 按源分辨率提供 `360p`，视频 2 提供 `360p / 480p / 720p`，视频 3 提供 `360p / 480p`。master、variant playlist 和 `.ts` 切片均已实际读取成功；支持 HLS 的浏览器可以直接播放并显示清晰度选项，其他现代浏览器通过 `hls.js` 提供自动/手动切换，HLS 发生致命错误时回退原始 MP4。清晰度菜单位于播放器原生控制按钮旁并向上展开，不遮挡全屏按钮；断点续播已实际验证可在刷新后恢复到约 `2:00`。

若 Docker Desktop 已启动但命令无法连接 daemon，先确认 Docker Desktop 使用 Linux containers，并执行 `wsl --status` 与 `docker context show`；本项目验证使用的 context 是 `desktop-linux`。

## 常见问题

**Go 下载依赖超时**

默认 `proxy.golang.org` 在当前网络曾解析到不可达 IPv6。使用上面的 `GOPROXY` 和 `GOSUMDB` 后已成功下载。

**上传后长期显示“等待媒体处理”或“HLS 处理失败”**

确认 `FFMPEG_PATH` 和 `FFPROBE_PATH` 可执行，并检查后端结构化日志中的 `job_id` 与 `video_id`。前端会显示排队、探测、转码和收尾进度；任务最多自动尝试 3 次。探测或转码失败不会删除原始视频，播放器仍可使用原始文件；对外只展示安全分类后的失败建议。作者可在“我的投稿”中点击“重试”，任务会清空旧错误和不完整的 HLS 目录后重新进入队列。

**状态修改提示页面凭证过期**

刷新页面重新获取 CSRF token；若登录 cookie 已过期则重新登录。前端不会把 session token 暴露给 JavaScript。

**生产环境登录 cookie 没有发送**

确认站点通过 HTTPS 访问，前后端保持同源，并设置 `COOKIE_SECURE=true`。若改为跨域部署，需要重新评估 SameSite、CORS 和 CSRF 策略。

**为什么注册或上传后换一个地址就看不到数据**

旧版开发脚本曾让 `5173` 使用 `backend/data/gvideo.db`，而 `8088` 使用 Docker 命名卷，两者互不相通。当前 `scripts/dev.ps1` 已统一复用 Docker 后端；用 `scripts/data-status.ps1` 可确认实际数据数量。只有手动执行 `go run ./cmd/server` 时才会再次启用独立的本机数据库。

## 关键设计决策

- 当前只有一个部署单元和一套数据一致性边界，模块化单体比微服务更容易理解和测试。
- SQLite 和本地媒体让初学者无需先维护多个服务；接口边界为后续 PostgreSQL 与对象存储迁移保留空间。
- 使用随机服务端 session 而非不可撤销 JWT，退出登录可立即删除会话。
- 上传校验读取真实文件头，不相信文件扩展名和浏览器声明的 MIME。
- 上传请求只负责可靠保存文件并原子创建媒体任务；FFprobe 在可取消、可恢复的单 worker 中执行，避免大文件探测占住上传请求。SQLite 当前只启一个 worker，迁移 PostgreSQL 后再评估并行领取。
- HLS 只生成不超过源视频尺寸的档位。FFmpeg 先写入临时目录，所有档位成功后才发布到稳定路径；失败时清理临时文件，并保留原始 MP4 作为播放回退。
- 转码进度与任务状态一起持久化，而不是依赖进程内状态；worker 只通过仓储推进阶段，服务层统一隐藏原始媒体工具错误。
- 浏览器 HLS 逻辑使用成熟的 `hls.js`，React Effect 销毁播放器实例；清晰度默认由带宽自适应算法选择，也可手动锁定可用档位。
- 首页采用“主推荐封面 + 次推荐网格”的视频社区信息结构，内容全部来自真实上传，不复制参考站点的品牌与视觉皮肤。
- 首页、最新和热门发现使用独立路由与页面结构。最新页按发布时间倒序排列；热门页按“播放量 + 点赞数 × 4”综合排序。旧地址 `/?sort=latest` 和 `/?sort=popular` 会自动跳转到对应页面并保留其他筛选参数。
- 视频列表使用服务端分页，响应统一包含 `items`、`page`、`page_size`、`total` 和 `has_next`；筛选条件变化时回到第一页，热门排名会包含分页偏移。
- 播放页桌面端采用“主内容 + 相关推荐”双栏布局，评论区位于视频信息下方；窄屏下按播放器、视频信息、评论、相关推荐的顺序单栏展示。
- 浏览器字幕统一使用 WebVTT。上传入口接受 UTF-8 编码的 `.vtt` 和 `.srt`，后端会校验文件并将 SRT 转换为 VTT；字幕元数据保存在 SQLite 的 `video_subtitles` 表，文件保存在 `MEDIA_DIR/subtitles/`，不会写入前端代码或浏览器本地存储。
- 字幕文件最大 2 MB。上传视频时可以附带首条字幕，视频作者也可以在播放页补传、切换默认轨道和删除轨道；同一视频的同一语言只保留一条轨道，同一视频最多一个默认轨道。删除默认轨道时由数据库事务选择回退轨道，提交成功后再清理字幕文件。
- 投稿删除先在数据库事务中删除视频记录并级联清理字幕、互动、评论和转码任务，再清理原视频、封面、字幕文件及 `hls/<video-id>/`。转码中的视频禁止删除，避免后台 worker 与文件清理并发冲突。
- 用户头像与视频媒体使用同一媒体根目录，但权限边界不同：`avatars/*` 可公开读取，视频原文件、封面、字幕和 HLS 先映射到所属视频，再按 `visibility` 和当前 viewer 决定是否返回。
- 非公开投稿不能只靠前端隐藏。公共列表统一由 `VideoFilter.IncludeNonPublic=false` 增加 `v.visibility = 'public'` 条件；只有本人投稿和创作者统计显式请求 `IncludeNonPublic=true`。
- `private` 对非作者统一表现为不存在，视频详情、评论读取与创建、点赞、收藏以及媒体请求都复用视频可访问性检查，避免通过关联接口或静态文件路径泄露存在性。
- `unlisted` 保留直链分享能力，但不进入首页、最新、热门、搜索、作者空间和关注动态。
- 默认 Compose 只暴露本机 HTTP 调试端口；可选 HTTPS overlay 单独承担公网 TLS、HTTP 重定向和 Secure Cookie，不把真实证书打入镜像或提交到仓库。
- SQLite 与媒体位于两个一致性边界。在线备份对数据库使用一致性快照，对媒体使用只读归档并生成可验证清单；严格恢复点需要维护窗口，文档不得把不停服备份描述为跨资源原子快照。

## 视频管理接口

列表接口接受 `page` 和 `page_size`，旧的 `limit`、`offset` 参数仍兼容：

```text
GET    /api/v1/videos?page=1&page_size=24
GET    /api/v1/me/videos?page=1&page_size=12
GET    /api/v1/me/creator/stats
PATCH  /api/v1/me/profile
PATCH  /api/v1/videos/{videoID}
DELETE /api/v1/videos/{videoID}
POST   /api/v1/videos/{videoID}/retry
PATCH  /api/v1/videos/{videoID}/subtitles/{subtitleID}/default
DELETE /api/v1/videos/{videoID}/subtitles/{subtitleID}
```

资料与投稿编辑接口使用 `multipart/form-data`。资料更新支持 `username`、`bio` 和可选 `avatar`；投稿编辑支持 `title`、`description`、`category`、`visibility` 和可选 `cover`。状态变更要求登录、CSRF token，并在投稿和字幕操作中校验当前用户是视频作者。只有 `failed` 状态可重新转码；`processing` 状态不可删除。

## 推荐学习顺序

1. 从 `backend/cmd/server/main.go` 看依赖如何组装和关闭。
2. 跟踪注册请求：`httpapi -> service -> repository -> SQLite`。
3. 阅读上传流程，理解 multipart、文件头检测、原子写入和 FFmpeg 子进程超时。
4. 阅读 `frontend/src/api.ts`，理解 cookie、CSRF 和统一错误处理。
5. 阅读 `User.AvatarURL`、`UpdateProfile` 和 `users.avatar_path`，跟踪 multipart 头像从文件头校验、原子保存、数据库更新到旧文件清理的完整生命周期。
6. 阅读 `CreatorProfile`、`user_follows` 和 `VideoFilter.FollowingUserID`，理解关注关系如何贯穿 domain、repository、service 与 HTTP 层。
7. 阅读 `Video.Visibility`、`VideoFilter.IncludeNonPublic` 和 `VideoByID`，对照公开列表、本人投稿与直链详情的三种可见性规则。
8. 阅读 `AuthorizeMedia`、`MediaAccessByPath` 和 `/media/*` Handler，理解原视频、封面、字幕与 HLS 如何复用视频权限并保留 Range 请求。
9. 阅读 `CreatorStats` 和前端 `/creator` 数据类型，理解聚合统计、可见性分布、处理中数量和最近投稿如何形成工作台。
10. 阅读 `ClaimTranscodingJob`、`UpdateTranscodingProgress`、worker 和 `publicVideo`，理解进度持久化、阶段推进、重试和安全失败文案如何跨层协作。
11. 阅读 `SetDefaultSubtitle`、`DeleteSubtitle` 与前端 `SubtitleManager`，理解事务内唯一默认、删除回退、文件清理和播放器状态同步。
12. 对比作者空间、关注动态、创作者工作台、首页、播放页、上传页和投稿管理页，观察受保护路由、`next` 回跳、URL 分页、局部状态和异步状态如何组织。
13. 阅读 `compose.https.yaml`、`deploy/nginx/https.conf.template` 和前端 Nginx 配置，理解同源代理、可信转发协议、Secure Cookie 与证书只读挂载。
14. 阅读 `backup-data.ps1`、`verify-backup.ps1`、`data-verify` 与 `VerifyMediaFiles`，理解 WAL 一致性快照、近一致媒体归档、清单校验和隔离恢复。
15. 最后阅读数据库升级、合并和相关测试，理解缺失新字段时的默认值、关系重映射、级联删除与幂等导入。

字幕相关后端测试可在 `backend` 目录运行：

```powershell
$env:GOPROXY='https://goproxy.cn,direct'
$env:GOSUMDB='sum.golang.google.cn'
go test ./... -count=1
```

## 后续演进

建议按真实瓶颈渐进升级：

1. 增加通知中心和更细的创作数据趋势，但继续以真实使用需求决定统计维度。
2. 为严格恢复点增加应用级维护/备份屏障，并把备份复制到异机或异盘的加密存储。
3. 多实例写入成为需求后迁移 PostgreSQL；媒体容量或多机共享成为需求后迁移 MinIO/S3。
4. 热门榜与频繁读取造成数据库压力后再评估 Redis。
5. 只有上传转码需要独立扩缩容和故障隔离时，先拆出媒体处理 worker；不要一次性把所有模块拆成微服务。
