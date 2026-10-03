# GVideo Roadmap

本文记录 GVideo 的公开开发方向。具体实现范围以对应 Issue 和 Pull Request 为准，优先保证现有功能、数据与部署契约稳定。

## 当前阶段：GVideo 3.0 UI Release Candidate 与维护

- Phase 1–6 已完成工程与视觉验收，Phase 6 经 PR #43 合并，UI RC 基线为 `e2c3a63`；默认主题正式保持 Light。不需要 Phase 6.1，当前只做文档与展示素材收尾。
- 当前规范与验证边界见 [设计系统](design-system.md)、[重构进度](gvideo3-progress.md) 与 [Phase 6 验收报告](gvideo3-phase6-acceptance.md)。没有新的正式 Release Tag，未部署生产环境。
- `v1.0.0` tag 与 release notes 已于 2026-09-15 发布，是历史工程版本；[工程案例](case-study.md) 保留其原始交付快照，不代表 UI 3.0 正式发布。
- 维护队列（计划内、按需触发）：
  - 存储配额可观测性（超配额指标、字幕字节计量、`processing` 卡死的人工释放流程）。
  - frontend/nginx 容器降权（nginx-unprivileged，涉及端口映射变更与运行态演练）。
  - 开发库历史残留清理；OpenAPI 描述与依赖安全扫描。

## 中期方向（分布式演进）

- 引入 Redis：分布式限流与缓存（当前限流为进程内实现）。
- 事件总线迁移为 Outbox + 消息队列：至少一次投递、幂等消费、死信队列（[ADR-0003](adr/0003-in-process-event-bus.md) 的回退路径）。
- 媒体文件迁移到 S3 或 MinIO 兼容对象存储；转码任务拆分为可独立扩展的 Worker（[ADR-0002](adr/0002-modular-monolith.md) 的拆分顺序：存储接口缝 → worker 出进程 → PostgreSQL）。
- 在多实例写入成为实际需求后，将 SQLite 迁移到 PostgreSQL。
- 容器编排（探针与自动扩缩容）与跨队列链路追踪。

## 暂不计划

- 在单机部署仍能满足需求时拆分微服务。
- 在明确需求（多实例部署或异步削峰）出现前，引入复杂消息队列或搜索集群。
- 为追求目录形式而进行高风险全仓重写。

## 参与方式

开发建议和功能请求请提交 GitHub Issue。准备贡献代码前，请阅读 [贡献指南](../.github/CONTRIBUTING.md) 和 [架构说明](architecture.md)。
