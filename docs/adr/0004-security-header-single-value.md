# 安全响应头单值原则与两侧 CSP 分工

## 背景

后端中间件为 API/媒体响应设置安全头（含 deny-all CSP）；前端 nginx 曾以 server 级 `add_header` 向所有响应（含代理路径）追加 SPA 头集，实测代理路径每个头出现两份、两种 CSP 并存；HTTPS 网关对其自产响应有四头覆盖与上游同头隐藏，但 CSP 按设计一律透传。

## 决策

- **前端源只为自己服务的内容下发 SPA 头集**（SPA 文档、静态资源、`/theme-init.js`）；代理路径（`/api`、`/media`、健康端点）纯透传后端单份头，保留 deny-all CSP；
- **网关为其自产响应**（限流 429、502、`/gateway-healthz`）**提供四个非 CSP 安全头**，隐藏上游同头副本后统一重加；**CSP 一律透传不注入**（避免与前端白名单冲突）；
- **不变量**：任一响应每个安全头恰好一份；SPA CSP 只来自前端源，API/媒体 CSP 只来自后端。

## 被拒替代方案

- 保留 server 级 `add_header` + 代理路径 `proxy_hide_header` 去重再重加：值相同属无谓改写，且会把代理路径的 deny-all CSP 替换成 SPA CSP，丢失后端策略；
- 网关注入 CSP：会与前端 `blob:`/`worker:` 白名单冲突。

## 回退条件

替换前端 nginx 或网关时须保留本分工；如前端不再由 nginx 服务，改为在边缘统一下发四种头并重新验证 CSP 单值。回归由 `frontend/e2e/security-headers.spec.ts` 六类响应断言钉住（单值 + 两侧 CSP 精确文本）。

## 验证证据

`frontend/e2e/security-headers.spec.ts`（SPA 文档/SPA 回退/theme-init/API/健康端点/媒体 404）；批次 28 运行态 curl 记录（前端源与网关各头恰一份）。
