# Auth API 契约

> 状态：目标契约（CONTRACT-02）
> 适用范围：`/api/v1/auth/register`、`/api/v1/auth/login`、`/api/v1/auth/logout`、`/api/v1/auth/me`
> 本任务只冻结规格与契约测试；不修改现有 Handler、Service、会话存储或线上响应。

本文沿用 [`api-error-contract.md`](./api-error-contract.md) 定义的成功/错误 envelope、稳定错误码、request ID 和兼容性规则。当前实现的错误 `error` 仍是字符串；切换为对象形式必须由后续独立实现任务完成。

## 1. 通用约束

- 所有路径都只接受表中列出的方法，并返回 `application/json; charset=utf-8`。
- 所有响应都包含 `X-Request-ID`；JSON envelope 中的 `request_id` 必须与该响应头相同。
- 所有 Auth 响应都发送 `Cache-Control: no-store` 和 `X-Content-Type-Options: nosniff`。
- 客户端使用同源 Cookie 会话，并在请求中包含凭据。会话令牌只存在于 HttpOnly Cookie，绝不出现在 JSON、URL、日志或 CSRF header 中。
- `register` 与 `login` 的请求体必须是且只能是一个 JSON object；未知字段、无效 JSON 或尾随第二个 JSON 值返回 `invalid_request`。
- 修改会话状态的已登录请求必须同时通过会话认证与 CSRF 校验。认证先于 CSRF：没有有效会话时返回 `authentication_required`，不得通过响应泄露 CSRF 校验结果。
- 错误响应遵循 `api-error-contract.md`。客户端只能依据 `error.code` 分支，不得依据可本地化的 `error.message` 判断。

## 2. 数据模型

### 2.1 User

成功的注册、登录和当前会话响应都返回同一完整 `user` object：

| 字段 | JSON 类型 | 约束 |
| --- | --- | --- |
| `id` | number | 正整数 |
| `username` | string | 注册时去除首尾空白后保存；3–24 个中文汉字、ASCII 字母、数字或 `_` |
| `bio` | string | 可为空字符串 |
| `avatar_url` | string | 可为空字符串 |
| `is_admin` | boolean | 服务端根据配置计算，客户端不可提交或覆盖 |
| `created_at` | string | RFC 3339 时间字符串 |

密码、密码哈希、会话令牌以及内部存储路径不得出现在 `user` 或任何响应中。

### 2.2 AuthPayload

```json
{
  "user": {
    "id": 1,
    "username": "example_user",
    "bio": "",
    "avatar_url": "",
    "is_admin": false,
    "created_at": "2026-09-10T00:00:00Z"
  },
  "csrf_token": "opaque-csrf-token"
}
```

`csrf_token` 是非空、不透明字符串。注册和登录响应还必须在 `X-CSRF-Token` 响应头返回相同值；`me` 返回当前会话已有的值，不轮换会话或 CSRF token。

## 3. 端点

### 3.1 注册

`POST /api/v1/auth/register`

请求：

```json
{
  "username": "example_user",
  "password": "password123"
}
```

- `username` 必填，去除首尾空白后必须符合 User 表中的字符和长度约束。
- `password` 必填，长度为 8–72 bytes；服务端只接收明文用于本次校验与哈希，不回显。
- 成功：HTTP `201`，`data` 为 `AuthPayload`；创建新会话，同时发送会话 Cookie 和 `X-CSRF-Token`。
- 错误码：`invalid_request`（请求无法解析）、`invalid_input`（字段校验失败）、`conflict`（用户名已被使用）、`internal_error`（未分类服务端失败）。

### 3.2 登录

`POST /api/v1/auth/login`

请求字段与注册相同。`username` 在查询前去除首尾空白；登录失败不得泄露用户名是否存在。

- 成功：HTTP `200`，`data` 为 `AuthPayload`；创建独立新会话，同时发送会话 Cookie 和 `X-CSRF-Token`。
- 错误码：`invalid_request`、`invalid_credentials`、`internal_error`。
- 用户不存在与密码错误都返回相同的 HTTP `401`、`invalid_credentials` 和泛化消息。

### 3.3 登出

`POST /api/v1/auth/logout`

- 必须携带有效 `gvideo_session` Cookie 和与该会话匹配的 `X-CSRF-Token` header；没有请求体。
- 成功：HTTP `200`，服务端先使当前会话失效，再发送删除 Cookie，响应 `data` 为 `{"logged_out": true}`。
- 错误码：`authentication_required`、`csrf_failed`、`internal_error`。
- 成功后复用旧 Cookie 必须按无有效会话处理。

### 3.4 当前会话

`GET /api/v1/auth/me`

- 必须携带有效 `gvideo_session` Cookie；GET 不要求 CSRF header。
- 成功：HTTP `200`，`data` 为 `AuthPayload`。不得创建或轮换会话、会话 Cookie 或 CSRF token。
- 错误码：`authentication_required`、`internal_error`。

## 4. Cookie 与 CSRF

会话 Cookie 的固定属性：

| 属性 | 注册/登录 | 登出 |
| --- | --- | --- |
| 名称 | `gvideo_session` | `gvideo_session` |
| 值 | 非空、不透明会话令牌 | 空 |
| `Path` | `/` | `/` |
| `HttpOnly` | true | true |
| `SameSite` | `Lax` | `Lax` |
| `Secure` | 由 `COOKIE_SECURE` 决定；生产必须为 true | 与创建 Cookie 时使用相同配置 |
| `Max-Age` | `SESSION_TTL` 的整秒数 | `-1`（立即删除） |

CSRF 约束：

1. CSRF token 与服务端会话绑定，不等同于会话令牌。
2. 注册和登录返回的 `data.csrf_token` 与 `X-CSRF-Token` 必须相同。
3. 登出必须在 `X-CSRF-Token` 请求头原样发送该 token；Cookie 或请求体中的同名值不参与校验。
4. token 缺失、错误或过期统一返回 HTTP `403`、`csrf_failed`，不得回显期望值。
5. 会话无效时优先返回 HTTP `401`、`authentication_required`。

## 5. 兼容与安全

- 上述字段、HTTP 状态、Cookie 名称和稳定错误码发布后不得删除、更名或改变类型/含义；客户端必须忽略新增未知字段。
- 服务端可以调整面向用户的错误消息，但不得调整既有错误码的语义。
- 注册、登录和鉴权错误不得泄露密码、密码哈希、会话令牌、用户存在性、SQL、堆栈或内部路径。
- 限流、账户锁定、Cookie Domain、跨站部署或刷新 token 均未在当前实现中定义；引入时需单独设计、记录稳定错误码并补充契约测试。
