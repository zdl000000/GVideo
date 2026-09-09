# GVideo 架构说明

本文定义当前运行架构、目标模块边界和渐进迁移规则。架构调整期间不改变用户可见行为、公开 API、数据库兼容性和部署契约。

## 架构目标

- 保持一个前端、一个 Go 后端、一个 SQLite 数据库和一套媒体存储即可部署。
- 按产品能力组织代码，让页面、业务规则、数据访问和测试更容易定位。
- 在迁移期间保留 HTTP API、SQLite 兼容迁移、媒体鉴权和创作者工作流。
- 共享基础设施保持通用，业务功能通过小而稳定的接口协作。

Feature-first 是代码组织方式，不等于微服务。GVideo 在出现真实的独立扩缩容或部署需求之前继续采用模块化单体。

## 运行架构

```text
浏览器
  -> Nginx / 同源网关
     -> React 应用
     -> Go HTTP API
        -> 应用模块
        -> SQLite
        -> 本地媒体存储
        -> FFprobe / FFmpeg worker
```

前端负责展示和本地交互状态。Go 后端负责认证、授权、投稿可见性、媒体处理和社区数据持久化。

## 当前结构

```text
frontend/src/
  app/App.tsx
  features/             按 auth、watch、videos、creator 等能力组织
  shared/api/
  shared/components/
  shared/lib/
  types.ts
  styles.css

backend/internal/
  httpapi/
  service/
  repository/
  domain/
  media/
  platform/
```

后端当前仍使用技术分层，前端已经开始低风险迁移。现阶段不为了目录形式强行移动高风险媒体、认证和数据库代码。

## 目标前端结构

```text
frontend/src/
  app/
    App.tsx
    router.tsx
    providers.tsx
  features/
    auth/
    captions/
    comments/
    creator/
    interactions/
    moderation/
    notifications/
    profiles/
    upload/
    videos/
  shared/
    api/
    components/
    hooks/
    lib/
    styles/
    types/
```

允许的依赖方向：

```text
app -> features -> shared
```

- `shared/` 不得导入 `features/` 或 `app/`。
- 功能模块不得导入另一个功能模块的私有文件。
- 跨功能页面组合放在 `app/`，或通过功能模块显式导出接口。
- 路由和全局 Provider 属于 `app/`，具体产品行为属于 `features/`。
- 只有至少两个功能稳定复用的代码才进入 `shared/`。

## 目标后端结构

```text
backend/
  cmd/server/
  internal/
    modules/
      auth/
      captions/
      comments/
      creators/
      interactions/
      moderation/
      notifications/
      profiles/
      videos/
    platform/
      database/
      media/
      observability/
    shared/
      httputil/
      pagination/
```

目标依赖方向：

```text
cmd/server -> modules -> shared/platform
```

- HTTP Handler 只解析和输出协议数据，不直接执行 SQL。
- Repository 负责参数化 SQL 与数据映射，不决定授权。
- Service 负责业务校验、授权决策和事务流程。
- 模块间协作使用由消费方定义的窄接口。
- SQLite 迁移在过渡期继续集中管理，避免兼容逻辑被过早拆散。

## 渐进迁移策略

1. 建立开源仓库规范、CI、文档和忽略规则。
2. 迁移前端应用组合、HTTP 客户端和稳定共享工具。
3. 优先迁移已有测试覆盖且边界明确的功能，字幕和评论排在前列。
4. 后端先在现有包内按职责拆文件，降低大文件耦合和 Go 包循环风险。
5. 只有当 Handler、Service、Repository 契约和测试能一起移动时，才提升为 `internal/modules/<feature>`。
6. 所有调用方完成迁移并通过完整验收后，才删除旧入口或重复实现。

新功能遵循目标结构；已有功能在被修改或大文件明显影响维护时顺手迁移，不做一次性全仓重写。

## 已落地边界

- 应用组合位于 `frontend/src/app/App.tsx`，页面按功能拆入 `frontend/src/features/`。
- HTTP 客户端位于 `frontend/src/shared/api/client.ts`，通用组件和工具位于 `shared/`。
- Handler、Service、Repository 已在现有 Go 包内按职责拆文件，尚未提升为独立业务包。
- 字幕管理入口和位置保持不变，仍只属于 `/me/videos` 投稿管理。
- 指标、pprof 使用独立且默认关闭的管理监听端口；SQLite 使用迁移账本记录版本和校验和。

## 架构保护区

以下区域不得在无独立任务、聚焦测试和回滚方案时改动：

- SQLite 兼容迁移、数据库合并和备份恢复。
- FFprobe、FFmpeg、HLS 发布和媒体任务恢复。
- Session Cookie、CSRF 和媒体鉴权语义。
- 公开 API 路径与响应结构。
- 字幕位置规则：播放页只选择，创作管理只在 `/me/videos`。

涉及保护区的改动必须增加聚焦测试，并完成本地与公网完整验收。

## 质量门槛

每批迁移必须通过：

- Go 格式、单元测试和 `go vet`。
- 前端单元测试、TypeScript 检查和生产构建。
- Playwright 桌面和移动流程。
- 认证、上传、媒体访问、互动与清理 API 验收。
- `git diff --check`。
- 密钥、本机绝对路径、生成文件、数据库、媒体和备份扫描。

## 何时进一步重构

出现以下情况时，再扩大 Feature-first 迁移范围：

- 多人经常在同一大文件产生冲突。
- 一个功能修改经常触碰无关模块。
- 测试无法隔离业务行为，必须构造整个应用。
- 弹幕、推荐或分布式媒体存储需要明确所有权边界。
- 某个模块需要独立扩缩容或部署生命周期。

在这些条件出现之前，模块化单体仍是更合适的运行架构。
