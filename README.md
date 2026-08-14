# GVideo

GVideo 是一个面向视频创作者与观众的社区型全栈项目。项目参考成熟视频社区的常见使用习惯，但产品设计、代码和架构均独立实现，当前以可部署、可维护的模块化单体为核心。

当前版本已打通注册登录、视频投稿、媒体处理、发现与播放、作者空间、关注动态、点赞收藏、评论通知、举报审核、创作者工作台和字幕管理等主要流程。

## 核心能力

- 服务端 Session、CSRF 防护、用户资料与头像管理
- MP4、WebM、Ogg 上传及真实文件头校验
- FFprobe 媒体探测、FFmpeg 自动封面和 HLS 自适应转码
- 首页、最新、热门、搜索、分区和服务端分页
- 视频播放、断点续播、清晰度选择、点赞、收藏和评论
- 作者空间、关注关系、关注动态和通知中心
- 创作者工作台、投稿编辑、处理进度、失败重试和删除
- `public`、`unlisted`、`private` 投稿可见性及媒体鉴权
- VTT/SRT 字幕上传、默认轨道切换和删除
- 用户举报与管理员审核
- 浅色默认主题和可选深色主题
- Docker Compose、HTTPS 网关、备份恢复和自动化验收脚本

字幕创作与管理仅位于 `/me/videos` 投稿管理流程，播放页只负责选择已有字幕轨道。

暂未实现弹幕、复杂推荐算法和微服务拆分。

## 技术栈

| 区域 | 技术 |
| --- | --- |
| 后端 | Go 1.26、Chi 5 |
| 前端 | React 19、TypeScript 7、Vite 8 |
| 数据 | SQLite、modernc.org/sqlite |
| 媒体 | FFprobe、FFmpeg、HLS、本地媒体目录 |
| 部署 | Docker Compose、Nginx |
| 测试 | Go test、Vitest、Playwright、PowerShell 验收脚本 |

## 快速开始

环境要求：

- Go 1.26 或更高版本
- Node.js 24 和 npm 11 或更高版本
- Docker Desktop 与 Docker Compose
- 仅在脱离 Docker 运行媒体处理时需要本机 FFmpeg 和 FFprobe

安装前端依赖：

```powershell
cd frontend
npm ci
cd ..
```

启动开发环境：

```powershell
.\scripts\dev.ps1
```

浏览器访问 `http://127.0.0.1:5173`。开发脚本使用 Docker 后端，因此 `5173` 与 Docker 页面 `8088` 共享同一套持久数据。

也可以直接启动完整 Docker 环境：

```powershell
docker compose up --build -d
```

访问 `http://127.0.0.1:8088`，健康检查地址为 `http://127.0.0.1:8080/healthz`。

## 验证

运行格式检查、后端测试与 vet、前端测试、类型检查和生产构建：

```powershell
.\scripts\check.ps1
```

运行本地完整验收：

```powershell
cd frontend
npx playwright install chromium chromium-headless-shell
cd ..
.\scripts\acceptance.ps1
```

验收脚本覆盖健康检查、真实 API 用户旅程、媒体处理、权限、互动、字幕和桌面/移动浏览器流程。备份恢复可通过 `-IncludeBackupRestore` 纳入验收。

## 项目结构

```text
backend/
  cmd/server/             Go 服务入口
  internal/httpapi/       HTTP 路由、中间件与响应转换
  internal/service/       业务规则、权限和事务流程
  internal/repository/    SQLite 查询与数据映射
  internal/domain/        领域实体与稳定业务错误
  internal/media/         FFprobe、FFmpeg 与媒体任务 worker
  internal/platform/      数据库初始化和兼容迁移
frontend/
  src/app/                目标应用组合与全局 Provider
  src/features/           按业务能力组织的功能模块
  src/shared/             共享 API、组件、工具与类型
docs/                     架构、部署、运维、设计和开发规范
deploy/nginx/             HTTPS 网关配置
scripts/                  开发、检查、验收、备份和恢复脚本
```

项目正在渐进迁移到 Feature-first 结构。迁移期间保持 `app -> features -> shared` 的依赖方向，同时保留现有 API、数据库和部署兼容性，不进行高风险全仓重写。

## 文档导航

- [架构与 Feature-first 迁移](docs/architecture.md)
- [部署与配置](docs/deployment.md)
- [运维、验收与备份恢复](docs/operations.md)
- [前端设计系统](docs/design-system.md)
- [项目专项开发规范](docs/project-standards.md)
- [当前实施计划](docs/implementation-plan.md)
- [贡献指南](CONTRIBUTING.md)
- [安全策略](SECURITY.md)
- [变更记录](CHANGELOG.md)

## 安全提示

- 不要提交 `.env`、证书、私钥、数据库、媒体、备份、日志或临时隧道状态。
- 生产环境必须使用 HTTPS 并设置 `COOKIE_SECURE=true`。
- 不要运行 `docker compose down -v`，除非明确需要永久删除数据库和媒体卷。
- 安全漏洞请按 [SECURITY.md](SECURITY.md) 私下报告，不要提交公开 Issue。

## 许可证

GVideo 使用 [MIT License](LICENSE)。
