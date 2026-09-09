# GVideo 项目专项开发规范

本规范只覆盖当前 Go 1.26 + React 19 + TypeScript 7 + Vite 8 模块化单体需要的规则。依据已核对可访问的 Go、React、Vite 官方文档，以及 OWASP 会话管理资料；具体行为仍以仓库测试和实际构建为准。

## 模块边界

- `httpapi` 只解析 HTTP 输入、接入认证与 CSRF、调用服务并转换响应。
- `service` 保存业务校验、会话、上传和媒体处理规则，不依赖 HTTP 响应对象。
- `media` 封装 FFprobe、FFmpeg HLS 转码和后台任务执行；worker 只能通过 `repository` 领取任务与更新状态，不直接持有数据库连接。
- `repository` 只通过参数绑定访问数据库，不接收拼接自用户输入的 SQL 片段。
- `domain` 保存跨层实体与稳定业务错误；`platform` 负责数据库等基础设施初始化。
- React 页面只能通过 `api.ts` 调用后端，不在组件中散落请求封装和认证头逻辑。
- 作者资料、作者投稿和关注动态属于同一用户社区模块；分页与关注过滤必须在服务端完成，前端不拉取全量数据后二次筛选。
- 用户资料、创作者统计、视频可见性和媒体访问权限由 `service` 定义业务规则；`repository` 负责字段映射与参数化过滤，`httpapi` 不自行复制权限判断。

## Go

- 所有改动必须通过 `gofmt`、`go test ./...` 和 `go vet ./...`。
- 外部 I/O 接收 `context.Context`；启动的进程、请求和服务器关闭必须有超时或取消路径。
- 后台媒体任务必须先持久化再执行；服务重启时恢复遗留任务，失败重试次数和等待时间必须有上限。
- HLS 只能生成不超过源视频宽高的档位；转码必须有超时和取消路径，子进程退出后释放资源。
- HLS 输出先写同一媒体卷中的临时目录，主播放列表和所有变体成功后再发布稳定目录；失败时清理临时目录，不删除原始视频。
- worker 领取任务后必须持久化 `probing/10`，探测完成后写入 `transcoding/35`，转码完成后写入 `finalizing/90`，成功时写入 `ready/100`；自动或手动重试统一回到 `queued/0`。
- FFprobe/FFmpeg 原始错误只进入内部日志和数据库。API 必须通过服务层输出安全分类文案，禁止暴露绝对路径、命令输出或内部日志。
- 错误增加动作上下文并保留原错误，业务错误用 `errors.Is` 判断。
- 日志使用 `slog` 结构化字段；禁止记录密码、原始 session token、CSRF token 和上传文件内容。
- 文件先写临时路径再原子发布；失败路径清理已创建资源。

## React 与 TypeScript

- 开启严格类型检查；共享 API 数据使用 `types.ts`，不使用无约束的 `any`。
- 状态尽量靠近使用它的页面；认证和 CSRF 由应用入口与 `api.ts` 统一维护。
- Effect 只用于和外部系统同步；派生展示值在渲染期计算或使用 `useMemo`。
- HLS 使用 `hls.js` 或浏览器原生实现，不自行解析媒体段；Effect 清理时必须销毁 HLS 实例，致命错误回退原始文件。
- 所有异步页面必须提供加载、错误和空状态；按钮在请求期间禁用。
- 可交互图标有 `title` 或可访问标签，键盘焦点清晰，移动端不能发生横向页面溢出。
- 用户头像统一通过可复用组件渲染：有 `avatar_url` 时显示图片，无头像或图片失败时显示稳定的用户名首字回退；所有尺寸必须有固定宽高，不能因图片加载造成布局跳动。
- `/creator` 是登录后直接进入的工作台，不做营销式落地页；使用紧凑、可扫描的信息布局，并提供加载、错误、空投稿和处理中状态。
- 上传页与投稿编辑必须暴露完整的 `public`、`unlisted`、`private` 可见性选项；控件文案应说明列表曝光和直链访问差异，不能只显示内部枚举值。
- 播放页、创作者工作台和投稿管理页复用同一进度阶段映射；进度条使用稳定尺寸和 `progressbar` 语义，失败时只展示 API 返回的安全文案。
- 字幕管理操作期间禁用冲突控件。设置默认和删除接口返回完整轨道列表后，以服务端结果替换本地状态，并同步播放器当前轨道。

## 安全与配置

- session token 仅存 HttpOnly、SameSite=Lax cookie；数据库只保存 token 哈希。
- 状态变更接口要求 `X-CSRF-Token`，前端只在运行内存保存该值。
- 密码使用 bcrypt；上传类型根据文件头检测，不信任扩展名和浏览器 MIME。
- 生产环境必须使用 HTTPS，并将 `COOKIE_SECURE=true`；密钥和环境差异只通过环境变量注入。
- SQLite 适合当前单机 MVP。出现多实例写入或独立扩缩容需求时迁移 PostgreSQL，而不是提前双写。
- 浏览器开发入口默认统一使用 Docker 命名卷中的持久数据库；自动化测试使用 `t.TempDir()` 隔离数据，不允许测试连接真实开发库。
- SQLite 备份必须通过数据库一致性快照完成，不能在 WAL 模式下只复制主 `.db` 文件；任何迁移先备份源库与目标库，再验证记录和媒体文件。
- `user_follows` 使用 `(follower_id, followed_id)` 复合主键、禁止自关注约束和两端 `ON DELETE CASCADE`；合并数据库时必须重映射两端用户 ID，并兼容旧来源库不存在该表。
- 用户头像和封面只接受真实文件头识别为 JPEG、PNG 或 WebP 的文件，单文件上限 10 MiB；新文件保存后若数据库更新失败必须回收，更新成功后才允许删除旧文件。
- `/media/*` 不能继续作为无条件静态目录暴露。除 `avatars/*` 外，原视频、封面、字幕、HLS master、variant 和 segment 必须先映射所属视频并执行可见性检查，再通过 `http.ServeFile` 返回以保留 Range/206。
- 路径鉴权必须规范化斜杠、拒绝空路径、NUL 和目录穿越；未知媒体路径与无权访问的私密媒体统一返回 404，避免泄露资源存在性。

## 部署、备份与恢复

- 默认开发 Compose 的后端与前端端口只绑定 `127.0.0.1`。生产 HTTPS 通过独立 overlay 增加网关，HTTP 使用保留方法语义的 `308` 跳转，backend 必须覆盖为 `COOKIE_SECURE=true`；网关由 `HTTPS_PORT` 自动派生跳转端口后缀，healthcheck 必须同时探测 HTTP 和 HTTPS 入口。
- TLS 证书和私钥只能通过只读挂载或部署平台 secret 注入，不进入镜像、Compose 明文、环境变量值、Git 或备份包。HTTPS 网关必须保留上传大小、长请求超时和 `X-Forwarded-Proto=https`。
- 合并 Compose 配置后必须检查最终端口、卷和 backend 环境，不能仅审查 overlay 源文件。基础前端端口即使保留，也只能绑定本机回环地址，不能成为公网 HTTP 绕过入口。
- SQLite 在线备份使用 `VACUUM INTO`，禁止在 WAL 模式下只复制主 `.db` 文件。媒体归档必须通过只读卷挂载生成，不修改媒体卷，也不停止或删除现有容器和命名卷。
- SQLite 与媒体卷不是同一个事务边界。不停服备份只能标记为“可校验的在线近一致备份”，建议在低写入窗口执行；严格恢复点需要维护窗口、禁止新写入并等待当前媒体任务完成。
- 每个备份包必须不可覆盖，并包含数据库、媒体归档和清单。清单至少记录 UTC 时间、SHA-256、文件大小、数据库统计、媒体文件数和一致性说明；备份目录、数据库、媒体、证书和密钥必须被 Git 忽略。
- 恢复演练只能使用 `tmp/restore-verification-*` 或独立临时卷，不挂载真实数据卷、不发布端口、不写回生产数据。原始 SQLite 先以只读方式执行 `integrity_check` 与 `foreign_key_check`，再在副本上执行当前迁移。
- 解包前必须校验清单、大小、SHA-256、路径穿越和归档条目类型，拒绝绝对路径、`..`、符号链接、硬链接和设备文件。解包后必须确认数据库引用的头像、原视频大小、封面、字幕、HLS 主清单、变体与切片存在。
- 验证脚本只能清理自己在项目 `tmp/` 下创建且经过绝对路径边界复核的目录；不得清理历史备份、真实数据库、媒体文件或命名卷。

## 用户空间与关注

- `/users/:id` 对游客开放，返回稳定的作者资料、加入时间、投稿/粉丝/关注统计；`followed` 只表达当前可选登录 viewer 的关系状态。
- `/users/:id/videos` 默认每页 12 条；`/following` 对应的 `/api/v1/me/following/videos` 默认每页 24 条，均返回统一 `VideoPage`。
- 关注切换必须同时经过登录认证和 CSRF 校验；服务层禁止自关注并在目标用户不存在时返回稳定的 not-found 业务错误。
- 本人作者空间不提供关注按钮，入口改为投稿管理；未登录用户触发关注或访问关注动态时携带 `next` 跳转登录页。
- 视频卡片、首页精选、播放页作者区、评论用户和相关推荐作者都必须链接到统一作者空间，避免形成无法继续浏览的用户名文本。
- 关注功能测试至少覆盖开关、统计、viewer 状态、动态过滤、权限错误、分页、级联删除、数据库统计、旧库兼容和合并幂等。

## 用户资料与头像

- `PATCH /api/v1/me/profile` 使用 `multipart/form-data`，同时要求登录认证和 CSRF。字段为 `username`、`bio` 与可选 `avatar`，响应返回更新后的 `User`。
- 用户名沿用注册规则：3 到 24 位中文、字母、数字或下划线；简介去除首尾空白后最多 300 个 Unicode 字符。
- `users.avatar_path` 保存相对 `MEDIA_DIR` 的路径，API 对外统一返回 `avatar_url`，不暴露本机绝对路径。
- `avatar_url` 必须进入 `User`、`CreatorProfile`、`Video` 和 `Comment` 响应，保证顶栏、移动账户、作者空间、视频卡片、播放页作者、评论和相关推荐使用同一头像来源。
- 修改用户名必须保留 SQLite 的大小写不敏感唯一约束；冲突返回稳定的 conflict 业务错误，不覆盖其他用户。
- 头像替换遵循“先保存新文件、再更新数据库、最后清理旧文件”的顺序。未提交新头像时保留原头像；禁止通过资料接口传入任意媒体路径。

## 创作者工作台

- `/creator` 只对登录用户开放，未登录时使用通用受保护路由携带 `next` 跳转 `/auth`。
- `GET /api/v1/me/creator/stats` 返回 `videos_count`、`followers_count`、`views_count`、`likes_count`、`favorites_count`、`comments_count`、`public_count`、`unlisted_count`、`private_count`、`processing_count` 和最多 5 条 `recent_videos`。
- 工作台统计只读取当前登录用户的数据；最近投稿必须包含本人非公开投稿，并继续经过统一的 `Video` 响应转换。
- 没有投稿的用户仍应得到数值为 0、`recent_videos` 为空数组的成功响应，不能把聚合空集误判为用户不存在。
- 工作台应提供去发布、去管理投稿和编辑资料的明确入口，但不复制投稿管理页的完整编辑表格。

## 投稿可见性与媒体鉴权

- `videos.visibility` 只允许 `public`、`unlisted`、`private`，默认值为 `public`。上传和编辑均调用同一规范化与校验规则，空值只用于兼容旧客户端并归一为 `public`。
- 首页、最新、热门、搜索、作者空间和关注动态只返回 `public`；`GET /api/v1/me/videos` 与创作者统计显式包含本人全部可见性。
- `public` 可进入公共列表并通过详情和媒体直链访问；`unlisted` 不进入公共列表但详情和媒体直链可访问；`private` 仅作者可访问。
- 非作者读取 `private` 视频详情、评论列表、媒体文件，或尝试点赞、收藏、评论时统一得到 404。作者可以正常查看和管理自己的私密投稿。
- 可见性过滤必须同时应用于列表查询和数量查询，保证 `total`、`has_next` 与实际项目一致；关注动态不能因 JOIN 或 EXISTS 条件绕过公开过滤。
- HLS 权限覆盖 master playlist 所在目录下的 variant playlist 和切片；字幕、封面和原视频按精确相对路径映射所属视频。
- `avatars/*` 公开读取，但仍必须经过安全路径解析；头像不继承任何视频的可见性。

## 数据库兼容与测试

- `users.avatar_path` 使用 `TEXT NOT NULL DEFAULT ''`；`videos.visibility` 使用 `TEXT NOT NULL DEFAULT 'public'`，新建表带三值 `CHECK` 约束。
- `videos.processing_progress` 使用 `INTEGER NOT NULL DEFAULT 100`，`videos.processing_stage` 使用 `TEXT NOT NULL DEFAULT 'ready'`。新上传和重试必须显式写入 `queued/0`。
- 启动迁移必须幂等补齐旧数据库缺少的字段。数据库合并必须先检查来源库字段：没有 `avatar_path` 时导入空头像，没有 `visibility` 时导入 `public`，没有进度字段时按 `processing_status` 回退。
- 现有关注与核心视频流程测试不能被新增字段破坏；旧测试构造 `NewVideo` 未填写可见性时，仓储应兼容为公开投稿。
- `PATCH /api/v1/videos/{videoID}/subtitles/{subtitleID}/default` 与 `DELETE /api/v1/videos/{videoID}/subtitles/{subtitleID}` 必须要求登录、CSRF 和视频作者权限，并返回最新完整 `SubtitleTrack[]`。
- 设置默认轨道必须在事务内清除同视频其他默认值。删除默认轨道时将剩余最早轨道设为默认；删除最后轨道返回空数组；数据库提交成功后才清理字幕文件。
- 本阶段测试至少覆盖资料与可见性既有流程、任务进度状态转换、安全失败文案、旧库回填与合并幂等、默认字幕唯一性、删除回退、删除最后轨道、文件清理、作者权限、用户不存在、HTTP 401 和 CSRF 403。
- 自动化测试只能使用 `t.TempDir()` 中的 SQLite 与媒体目录，不得创建开发账号、上传验证数据或连接 Docker 持久卷。
- 文档中的“已通过”只允许记录本次真实执行的命令和结果；仅完成代码或局部包测试时，不能声称全量 `go test`、前端构建、Docker 或浏览器验收已完成。

## 完成定义

- 核心流程有仓储/服务测试或 HTTP 流程测试覆盖。
- `scripts/check.ps1` 通过，README 命令与实际项目一致。
- 新配置写入 `.env.example`，真实数据、媒体、密钥和构建产物不进入 Git。
- 媒体处理变更至少覆盖 JSON 解析、任务状态转换、旧数据库兼容、清晰度梯度、临时文件失败清理，以及可行时的一次真实 FFprobe/FFmpeg 集成验证。
- 部署可靠性变更必须实际生成一次完整备份包并完成隔离恢复演练；验证 HTTPS overlay 的渲染配置、TLS 入口、HTTP 跳转、Secure Cookie 配置和基础 HTTP 回归。
- 本阶段还必须通过后端测试与 vet、前端 `typecheck`/生产构建、`git diff --check`、Docker 双健康检查，以及不污染业务数据的桌面与移动端浏览器验收。
