# 可观测性告警规格与 Runbook

本文定义 GVideo 的供应商无关告警契约。它是部署监控系统时的输入，不表示仓库已经部署 Prometheus、Alertmanager 或任何特定告警产品。部署方必须先启用并保护 `METRICS_ADDR`，从管理网络采集 `/metrics`，并从应用网络外主动探测 `/livez` 与 `/readyz`。

## 信号与表达约定

- `scrape_success(instance)`：采集器自身提供的本次 `/metrics` 采集成功信号，不是 GVideo 暴露的指标。
- `probe_status(instance, path)`：外部探测器观察到的 HTTP 状态码；网络错误视为失败。
- `increase(counter, window)`：能正确处理进程重启和 counter reset 的窗口增量。
- `sum(...)`：跨 method、route、status 或 event 聚合；实现时保留 `instance`、环境等路由告警所需维度。
- `sustained(condition, window)`：条件在整个窗口持续成立。短暂缺样本不应自动按数值 `0` 处理。

`/livez` 只证明进程和业务 HTTP 服务可响应；`/readyz` 只证明启动初始化/迁移已完成且 SQLite 能在 1 秒内响应。两者都不证明媒体 Worker、队列或 FFmpeg 正常。`media_queue_depth` 仅在 metrics 和媒体 Worker 都启用时存在；正常值是 pending 与 processing 任务数，数据库查询失败或超时返回 `-1`。

阈值是初始安全值。上线后可依据容量和基线调整，但必须保留本节定义的信号含义、持续窗口和恢复判定。维护窗口应静默通知，而不是修改查询让故障看起来正常。

## 告警目录

### BackendUnavailableOrNotReady

- **查询/信号依据**：`scrape_success(instance) != 1 OR probe_status(instance, "/readyz") != 200`。必须用采集器/外部探测器的信号，不能从缺失的应用指标推导“健康”。可同时查看 `/livez` 区分进程不可达与数据库 readiness 故障。
- **时间窗口**：持续 5 分钟；探测间隔建议不超过 30 秒。
- **严重级别**：Critical。
- **原因**：实例无法采集意味着进程、管理网络或 metrics 监听异常；`/readyz` 持续非 200 意味着实例不应接收流量，常见于 SQLite 不可响应或启动初始化失败。
- **第一检查**：分别请求 `/livez`、`/readyz` 和受 ACL 保护的 `/metrics`，再检查实例/容器状态、最近启动日志、SQLite 文件及数据卷可用性；不要仅因 `/livez` 为 200 就判定服务可用。
- **升级条件**：所有实例都失败、用户流量受影响、数据库完整性可疑，或 10 分钟内无法恢复时立即升级给当班负责人；涉及数据损坏时停止写入并转入备份恢复流程。
- **恢复判定**：同一实例的 metrics 采集成功且 `/readyz` 连续 5 分钟返回 200；若曾有用户影响，还需完成一项只读 API 冒烟检查。

### HTTP5xxRatioHigh

- **查询/信号依据**：`errors = sum(increase(http_requests_total{status is 500..599}, 5m))`，`requests = sum(increase(http_requests_total, 5m))`，当 `requests >= 20 AND errors / requests >= 0.05` 告警。指标的 `route` 是 Chi 模板，未匹配请求为 `unmatched`，不得按原始 URL 聚合。
- **时间窗口**：5 分钟滚动窗口，条件连续两个评估周期成立；建议评估周期 1 分钟。
- **严重级别**：Warning；若比例达到 10% 并持续 10 分钟，或与 readiness 告警同时发生，提升为 Critical。
- **原因**：持续 5xx 通常表示应用缺陷、SQLite/存储故障、依赖超时或容量耗尽；最小请求量用于避免低流量下单次错误造成噪声。
- **第一检查**：按 `route` 和 `status` 分解增量，并用同一时段的 `event=http_request`、`event=http_panic`、`event=http_handler_error` 结构化日志关联 `request_id`；日志中不应搜索请求体、凭据或媒体路径。
- **升级条件**：Critical 条件成立、多个路由同时上升、出现 panic、登录/上传/播放主路径受影响，或 Warning 持续 30 分钟时升级给应用负责人。
- **恢复判定**：最近连续 10 分钟满足 `requests < 20` 或 5xx 比例低于 2%，且受影响路由的冒烟请求成功；低流量恢复需人工确认，不可只依赖比例。

### MediaJobFailuresGrowing

- **查询/信号依据**：`failures = sum(increase(media_jobs_total{event begins with "failed:"}, 10m))`；当 `failures >= 3` 告警。按实际 event（如 `failed:probe`、`failed:transcode`、`failed:complete`）分解以定位阶段。此 counter 统计已归类的任务失败，不是 Worker heartbeat。
- **时间窗口**：10 分钟滚动窗口，条件持续 5 分钟。
- **严重级别**：Warning；`failures >= 10`、所有新任务均失败，或同时出现队列积压时提升为 Critical。
- **原因**：失败增长可能来自无效媒体、probe/transcode 故障、存储写入失败或数据库问题。单次失败可能是输入问题，因此初始阈值允许少量孤立失败。
- **第一检查**：按 `event` 分组指标，查看对应的 `media_job_failed` 安全 `error_class` 与 `stage`，确认磁盘空间、SQLite readiness、媒体卷权限及 FFmpeg/FFprobe 可执行性；不要要求日志包含命令参数或 stderr。
- **升级条件**：达到 Critical 阈值、失败跨多个 stage、用户任务无法完成、数据卷异常，或 Warning 持续 30 分钟时升级给媒体处理负责人。
- **恢复判定**：失败增量连续 20 分钟低于 3，至少一个新的 `media_jobs_total{event="completed"}` 增量出现，并确认测试上传或待处理任务成功完成。

### MediaQueueBacklog

- **查询/信号依据**：`media_queue_depth >= 25`。该 gauge 包含 pending 和 processing 任务；阈值应按实例吞吐与正常峰值校准，但修改必须记录容量依据。
- **时间窗口**：持续 15 分钟。
- **严重级别**：Warning；`media_queue_depth >= 100` 持续 15 分钟，或队列持续增长且已有任务失败时提升为 Critical。
- **原因**：生产速度低于入队速度，可能由 Worker 未运行、转码变慢、媒体/数据卷性能下降或突发上传导致。
- **第一检查**：确认部署确实启用了媒体 Worker，再比较 claimed/completed/failed 的 counter 增量，检查 CPU、内存、磁盘容量和 I/O，以及最近 `media_job_*` 日志；`/readyz` 为 200 不能排除 Worker 故障。
- **升级条件**：达到 Critical 阈值、最老任务等待超过业务目标、队列只增不减 30 分钟，或资源耗尽风险上升时升级给容量与媒体处理负责人。
- **恢复判定**：深度连续 15 分钟低于 10 且 completed counter 持续增长；若流量停止才下降，还需确认 Worker 能完成一个新任务。

### MediaQueueDepthUnknown

- **查询/信号依据**：在预期启用媒体 Worker 的实例上，`media_queue_depth == -1`。`-1` 是 gauge 查询 SQLite 失败或超过 2 秒的明确哨兵值。指标缺失与 `-1` 不同：缺失时先核对 metrics/Worker 配置及采集状态，并由 `BackendUnavailableOrNotReady` 处理采集失败。
- **时间窗口**：连续 3 次采集且至少持续 2 分钟。
- **严重级别**：Critical。
- **原因**：监控无法读取真实队列深度，通常表示 SQLite 查询、连接或数据卷异常；此时积压告警也失去可信输入。
- **第一检查**：检查 `/readyz`、SQLite/数据卷状态、查询超时和进程日志，再确认该实例的 `MEDIA_WORKER_ENABLED` 与 `METRICS_ADDR` 配置；不要把未启用 Worker 的实例缺少 gauge 当作数据库故障。
- **升级条件**：`/readyz` 同时失败、所有 Worker 实例均为 `-1`、出现存储错误日志，或 5 分钟内未恢复时升级给数据库/平台负责人。
- **恢复判定**：gauge 连续 5 分钟为非负值，并确认队列值随一次受控任务的领取/完成产生合理变化。

## 事件处理通用要求

1. 先记录告警开始时间、实例、版本/提交和当前信号，再执行变更。
2. 优先使用安全结构化字段关联事件；不得提高日志敏感度来临时输出 Cookie、token、请求体、绝对媒体路径、FFmpeg 参数或 stderr。
3. `/readyz` 失败时先停止向实例分配新流量；涉及数据库或卷完整性时，不要反复重启、手工修改迁移账本或删除数据。
4. 恢复后记录实际根因、用户影响、采取的操作和阈值是否需要基于容量证据调整。

当前仓库没有备份成功时间戳指标，也没有 Worker heartbeat。备份状态必须通过备份清单与 `verify-backup.ps1` 验证；Worker 健康只能结合任务 counters、队列 gauge、结构化日志和受控任务确认。若未来新增这些信号，必须另行定义其生产、缺失和恢复语义后才能用于告警。