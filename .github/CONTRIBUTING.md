# 参与 GVideo 开发

感谢你帮助改进 GVideo。项目采用模块化单体，并正在渐进迁移到 Feature-first 结构。我们偏好范围清晰、容易审查、保持 API 与数据兼容的小批次改动。

## 开始之前

1. 阅读 [架构说明](../docs/architecture.md) 与 [项目专项开发规范](../docs/project-standards.md)。
2. 提交新 Issue 前先检索已有问题。
3. 涉及行为、数据库、认证、安全或部署的大改动，请先通过 Issue 讨论。
4. 不要提交凭证、本机数据库、上传媒体、备份、构建产物、日志或 Cloudflare 隧道状态。

## 开发环境

要求：

- Go 1.26 或更高版本
- Node.js 24 和 npm 11 或更高版本
- Docker Desktop 与 Docker Compose
- 仅在 Docker 外运行媒体处理时需要 FFmpeg 与 FFprobe

```powershell
cd frontend
npm ci
cd ..
.\scripts\dev.ps1
```

## 提交前验证

所有改动至少运行：

```powershell
.\scripts\check.ps1
```

认证、上传、媒体处理、投稿可见性、互动、字幕或部署相关改动还应运行：

```powershell
.\scripts\acceptance.ps1
```

## 代码边界

- HTTP Handler 只负责协议解析和响应转换。
- 授权与业务校验放在 Service。
- SQL 必须参数化并由 Repository 管理。
- 新前端行为放入对应 `features/`，不要继续扩大全局文件。
- 保持 `app -> features -> shared` 依赖方向。
- 除非有明确版本化迁移，不改变 API 路径、响应结构、数据库兼容和媒体鉴权。
- 字幕创作管理保留在 `/me/videos`，播放页只选择字幕轨道。

## Pull Request

PR 应说明：

- 修改内容与原因
- 用户和运维影响
- API、数据或部署兼容性
- 实际运行的验证命令与结果
- 可见前端改动的前后截图

保持提交聚焦，不混入无关格式化和生成文件。
