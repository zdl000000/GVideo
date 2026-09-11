# 变更记录

GVideo 的重要变更记录在此。项目在首次正式发布后遵循语义化版本，并参考 Keep a Changelog 的组织方式。

## [未发布]

### 变更

- 前端全量视觉重设计：玫瑰色品牌色体系、胶囊化频道导航与筛选控件、分层圆角与阴影令牌、玻璃质感播放器控件与 shimmer 骨架屏，浅色与深色主题同权重校准。
- 设计令牌体系落地 `styles.css`，组件样式全部语义化；`docs/design-system.md` 同步更新色彩、圆角、阴影与交互状态规范。
- 完成 Feature-first 前端结构迁移：全部页面从 `app/App.tsx` 拆分至 `features/`（videos、watch、creator、upload、auth、notifications、moderation），`App.tsx` 从约 2300 行降至约 340 行纯组合层，删除 `src/App.tsx` 与 `src/api.ts` 兼容出口。
- 新增 shared 层基础设施：格式化工具、Avatar、VideoCard、Pagination、处理状态徽章、加载/错误/空状态组件、弹窗焦点管理 Hook 与可见性常量。
- 新增 16 个组件测试（Vitest + Testing Library + jsdom），测试总数从 9 个增至 25 个。

### 新增

- 用户资料、关注关系、通知、举报审核、投稿可见性、字幕管理和创作者数据工作台。
- FFprobe 媒体探测、FFmpeg 自动封面、HLS 转码、任务恢复和处理进度。
- API、单元、浏览器、容量、备份恢复、生产预检和公网验收工具。
- 浅色优先的响应式界面和可持久化深色主题。
- 开源许可证、协作规范、GitHub 模板、CI 与 Dependabot 配置。

### 架构

- 开始渐进式 Feature-first 迁移，应用组合、共享 API 客户端和字幕管理已建立独立边界。

### 安全

- 服务端 Session、Secure Cookie 支持、CSRF、防伪造上传类型和按可见性鉴权的媒体访问。
- 修复管理员提权漏洞：管理员身份由用户名实时比较改为 `users.is_admin` 持久标志（迁移 v2），改名到保留名、大小写变体抢占与合并丢标均被关闭；新增 `gvideo data-grant-admin` 显式授予命令、启动配置校验与未认领告警。
- 新增请求速率限制：登录/注册按客户端地址、评论与上传按用户分档限流，超限返回 429 与 `Retry-After`，额度可由 `RATE_LIMIT_*_PER_MINUTE` 配置；替换弃用的 chi RealIP，仅信任可信代理的 `X-Forwarded-For`（IPv6 按 /64 归一），网关与前端代理不再透传可伪造的 `True-Client-IP`。
- 新增安全响应头：后端 API/媒体响应统一 `X-Frame-Options: DENY`、`Referrer-Policy`、`Permissions-Policy` 与 deny-all CSP；前端 Nginx 为 SPA 设置含 hls.js 所需 `blob:`/`worker:` 白名单的 CSP 并关闭 `server_tokens`；非生产环境管理端点绑定非 loopback 地址时新增 `diagnostics_binding_exposed` 启动警告。

[未发布]: https://github.com/zdl000000/GVideo/commits/main
