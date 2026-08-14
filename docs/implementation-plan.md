# 开源发布与架构迁移计划

## 目标

在不改变现有产品行为和部署契约的前提下，让 GVideo 达到公开 GitHub 仓库标准，并开始渐进式 Feature-first 迁移。

## 第一批：仓库卫生

- [x] 添加 MIT 许可证、贡献指南、安全策略、行为准则和编辑器规则。
- [x] 扩充 `.gitignore`，排除本地工具、测试产物、缓存、数据库、媒体、备份和隧道状态。
- [x] 清理已确认的日志、PID、构建输出和缓存。
- [x] 保留数据库、媒体、备份和 Docker 命名卷。
- [x] 发布前审计工作树文件，确认不存在密钥、真实绝对本机路径和用户数据。

## 第二批：文档结构

- [x] 将 README 收敛为概览、快速开始、验证命令和文档导航。
- [x] 架构规则集中到 `docs/architecture.md`。
- [x] 配置和部署集中到 `docs/deployment.md`。
- [x] 验收、备份、恢复和容量管理集中到 `docs/operations.md`。
- [x] 设计规范集中到 `docs/design-system.md`。
- [x] 在最终验收后再次核对所有文档命令。

## 第三批：GitHub 自动化

- [x] 添加 Go 与前端 CI。
- [x] 添加 Issue 与 Pull Request 模板。
- [x] 添加 Go、npm、Docker 和 GitHub Actions 依赖更新配置。
- [x] 首轮 CI 不强制运行高成本浏览器验收。

## 第四批：Feature-first 初始迁移

- [x] 应用组合迁入 `frontend/src/app/` 并保留入口兼容。
- [x] HTTP 客户端迁入 `frontend/src/shared/api/` 并保留兼容出口。
- [x] 通用错误处理迁入 `frontend/src/shared/lib/`。
- [x] 字幕管理迁入 `frontend/src/features/captions/`。
- [ ] 选择评论或视频列表作为下一批前端功能模块。
- [ ] 后端先在现有包内按职责拆分大文件，避免 Go 包循环。

## 第五批：验证与发布

- [x] 运行 `scripts/check.ps1`，Go 测试、前端测试、类型检查和生产构建均通过。
- [x] 启动 Docker Compose 并运行本地 API 与 Playwright 验收。
- [x] 创建新的 Cloudflare Quick Tunnel 并运行公网 API 与 Playwright 验收；关闭后恢复本地 HTTP 配置。
- [x] 运行备份恢复演练并校验数据库、媒体文件和隔离恢复结果。
- [ ] 明确 GitHub 仓库名称，配置 Remote，更新仓库相关链接。
- [ ] 审计、提交、推送当前分支并创建 Draft PR。

## 完成标准

- 根目录只保留源码、配置、部署入口和主要项目文档。
- 新贡献者能从 README 找到启动、检查、部署和运维入口。
- CI 能在 Pull Request 上验证前后端确定性检查。
- 单元、集成、浏览器和公网验收保持通过。
- Git 不包含本地数据、媒体、备份、凭证或生成产物。
