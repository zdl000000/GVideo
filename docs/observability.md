# 可观测性契约

本仓库沿用 Go `slog` 结构化日志，不在请求或媒体任务中记录原始请求内容与底层命令输出。生产进程使用 JSON handler；测试直接解析 JSON 字段验证契约。

## HTTP 请求事件

每个完成的 HTTP 请求记录 `event=http_request`，字段固定为：

- `request_id`：请求关联 ID。
- `method`：HTTP 方法。
- `route`：Chi 路由模板，例如 `/api/v1/videos/{videoID}`；未匹配请求统一为 `unmatched`，绝不回退到原始 URL。
- `status`：最终 HTTP 状态；未显式写响应的请求归一为 `200`。
- `bytes`：响应字节数。
- `duration_ms`：非负整数毫秒。

禁止记录 URL/query、Cookie、Authorization、CSRF、token、密码、请求体、上传内容或媒体路径。恢复 panic 时另记 `event=http_panic` 和固定 `error_class=panic`，不记录 panic 值。

## 媒体任务事件

已领取任务使用以下有限事件：

- `media_job_started`
- `media_job_completed`
- `media_job_failed`
- `media_job_interrupted`

每条任务日志都包含 `job_id`、`video_id`、`attempt`、`stage`、`retry` 和 `duration_ms`。`stage` 使用 `claimed`、`resolve`、`probe`、`transcode`、`finalize` 或 `complete`。失败或中断只记录固定安全分类：`invalid_media_path`、`probe_timeout`、`probe_failed`、`transcode_timeout`、`transcode_failed`、`storage_failed` 或 `context_canceled`。

`error_class` 不是底层错误文本。日志不得包含绝对路径、视频路径、HLS 输出路径、FFmpeg/FFprobe 原始命令、参数或 stderr。尚未领取具体任务的存储故障使用 `event=media_worker_error` 和固定分类，避免泄露数据库内部信息。
