# 进程内同步事件总线作为通知写路径唯一入口

## 背景

通知（互动与评论类）写路径曾散落三份镜像 INSERT（核心 service 与 comments、interactions 模块各一份），一致性靠人肉同步与测试钉住；而互动与评论产生的通知需要「随请求落库」，不能被延后。

## 决策

新增 `internal/platform/bus` 类型化同步事件总线：发布方在自身请求 goroutine 内联执行订阅者；notifications 模块 repository 成为**互动与评论类通知**（`comment`/`like`/`favorite`/`follow`）写路径的唯一所有者（含自评抑制与空值防护），媒体处理类通知（`processing_ready`/`processing_failed`）仍由转码事务内的核心路径写入；comments / interactions 经消费方自定义 `EventPublisher` 接口发布，不反向依赖 core；通知写失败仅记录告警，不影响主流程。

## 被拒替代方案

- 保留镜像 INSERT + 测试钉住：模块数量增长后同步成本线性上升，已出现过漂移风险；
- 模块直接调用核心 `CreateNotification`：反向依赖破坏模块边界与架构守卫规则；
- 引入消息队列异步投递：当前单机部署无队列基础设施，且「通知随请求落库」的语义会被破坏。

## 回退条件

多实例部署或需要异步削峰时，以本总线为缝迁移到 Outbox + 消费端：事务内写 outbox 表，relay 投递队列，订阅逻辑移到幂等消费端。迁移前保留现有跨模块集成测试（发布事件 → 通知落库）作为行为基线。
