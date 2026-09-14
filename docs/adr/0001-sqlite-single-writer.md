# 单机 SQLite 单写节点，暂不引入 PostgreSQL

## 背景

GVideo 为单机 Docker Compose 部署（backend + frontend + HTTPS 网关），写路径集中在投稿、评论、互动与通知，读放量集中在列表与播放。需要决定数据层选型与横向扩展路径。

## 决策

采用 SQLite（WAL）单实例写节点；数据访问收敛在 repository 层（核心 `internal/repository` 与各模块 repository），SQL 保持可移植；schema 迁移在启动时自动执行且前置备份门禁，备份、校验、合并与授权走 `gvideo data-*` 命令（`platform.BackupDatabase` / `VerifyDatabase` 及运维演练脚本）。

## 被拒替代方案

- 直接上 PostgreSQL：连接池、备份脚本、权限体系与运维面显著变大，当前单机负载下没有收益；
- 分库分表 / 分布式数据库：没有多实例写入需求，属于过度设计。

## 回退条件

出现多实例写入需求，或写竞争导致接口 P95 明显抬升时，在 repository 缝上迁移 PostgreSQL（roadmap 中期项）；迁移前保留 `data-backup` / `data-verify` 演练证据作为基线。
