# API 错误响应契约

> 状态：目标契约（CONTRACT-01）
> 适用范围：`/api/v1/**` JSON 响应
> 本任务只冻结规格与契约测试；现有 Handler 的迁移由后续独立任务完成。

## 1. Envelope

成功响应保持现有结构：

```json
{
  "data": {},
  "request_id": "4e1d7d8d913e45869055b94e"
}
```

错误响应的目标结构为：

```json
{
  "error": {
    "code": "invalid_request",
    "message": "请求内容不是有效的 JSON"
  },
  "request_id": "4e1d7d8d913e45869055b94e"
}
```

约束：

- 顶层必须是 JSON object，响应媒体类型为 `application/json`。
- 成功响应必须包含 `data` 和非空字符串 `request_id`，不得包含 `error`。
- 错误响应必须包含 `error` 和非空字符串 `request_id`，不得包含 `data`。
- `error` 必须是 object；`error.code` 与 `error.message` 都是非空字符串。
- `error.code` 是程序判断依据，使用小写 `snake_case`，含义发布后保持稳定。
- `error.message` 是面向用户的可本地化文本，客户端不得按文本内容或语言分支。
- 响应体的 `request_id` 必须与 `X-Request-ID` 响应头一致。有效的调用方 request ID 长度为 1–128 个 ASCII 字符，且只允许字母、数字、`.`、`_`、`:`、`-`；空值、超长值或包含其他字符的值必须丢弃并由服务端重新生成。该值只用于关联日志，不承载业务含义。
- 客户端必须忽略未知字段。服务端可兼容地增加可选字段，但不得删除、更名或改变既有字段类型；需要不兼容变更时发布新的 API 版本。
- 错误响应不得泄露内部错误、堆栈、SQL、文件系统路径、凭据或会话令牌。

## 2. 稳定错误码

同一 HTTP 状态可以对应多个业务码。HTTP 状态表达协议级结果，`error.code` 表达稳定且可操作的原因。

| `error.code` | HTTP | 含义 |
| --- | ---: | --- |
| `invalid_request` | 400 | JSON、路径参数、查询参数或 multipart 请求无法解析 |
| `invalid_input` | 400 | 请求可解析，但字段不满足业务校验 |
| `authentication_required` | 401 | 当前操作要求登录 |
| `invalid_credentials` | 401 | 登录凭据或会话无效；不得用于区分账号是否存在 |
| `forbidden` | 403 | 已识别调用方无权执行操作 |
| `csrf_failed` | 403 | CSRF 凭证缺失、错误或过期 |
| `not_found` | 404 | 资源不存在，或因访问控制而按不存在处理 |
| `conflict` | 409 | 通用资源状态或唯一性冲突 |
| `subtitle_exists` | 409 | 同语言字幕已存在 |
| `video_processing` | 409 | 视频正在处理，当前操作不可执行 |
| `retry_unavailable` | 409 | 当前转码状态不允许重试 |
| `report_exists` | 409 | 当前用户已提交同一举报 |
| `rate_limited` | 429 | 调用方超过速率限制；响应携带 `Retry-After` 秒数 |
| `storage_quota_exceeded` | 413 | 上传会超出该用户的存储配额 |
| `internal_error` | 500 | 未分类服务端错误；消息必须保持泛化 |

规则：

1. 已发布的错误码不得改名、改变含义或复用于另一原因；废弃码只能停止产生。
2. 新增错误码前必须记录其唯一语义与 HTTP 状态，并补充契约测试。
3. 未知或未安全分类的服务端错误统一为 `internal_error`，HTTP 500。
4. 具体路由产生哪些错误码由分组 API 契约定义；CONTRACT-02 将从 Auth API 开始记录，不在本任务修改路由行为。

## 3. 迁移与兼容

当前实现仍返回字符串形式的 `error`。迁移必须作为独立实现任务，一次覆盖服务端写入器、调用点、前端解析与对应测试，不能在规格提交中静默改变线上 JSON。

迁移完成后，前端应优先按 `error.code` 处理，并把 `error.message` 用作展示或安全回退。切换前必须验证登录失效通知、上传 XHR 错误和普通 `fetch` 错误路径。迁移提交应删除旧字符串 envelope 的兼容代码，或明确记录其期限；不得无限期维护双重结构。
