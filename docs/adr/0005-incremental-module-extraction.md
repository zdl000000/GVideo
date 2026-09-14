# 渐进式架构迁移：模块提取 + 架构守卫

## 背景

核心单体随功能增长出现大文件（handlers/service/repository），模块边界靠约定维持、会漂移（通知写路径曾出现三份镜像 INSERT）；同时不接受高风险全仓重写。

## 决策

采用 strangler 式渐进迁移：按业务纵向逐个提取模块（repository / service / handler 三层 + HTTPPort 协议端口），每批坚持「行为保持 + 目标验证 + 分批交付 + 可独立回滚」；用架构守卫测试（`TestModulesDoNotImportCoreLayers`）禁止模块生产代码反向依赖 core；跨模块协作经事件总线与消费方自定义接口；模块提取伴随测试归位。已交付 notifications、comments、interactions、moderation 四模块与进程内事件总线（A2 系列）。

## 被拒替代方案

- 一次性全仓重写：风险集中、验证成本高、测试与历史资产全部重来；roadmap 明确「为追求目录形式而进行高风险全仓重写」在暂不计划之列；
- 只定纸面边界不改代码：边界会持续漂移，镜像 INSERT 即实证代价。

## 回退条件

每批提交可独立回滚（提取以行为保持为前提，回归时按批回滚且保留守卫测试）；若某模块的 HTTPPort 适配层成为持续复杂度来源，重新评估该模块的边界划分再继续后续提取。
