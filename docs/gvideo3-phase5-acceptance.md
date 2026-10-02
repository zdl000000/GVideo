# GVideo 3.0 Phase 5 验收报告

日期：2026-10-03。基线：Phase 4 已通过 PR #41 squash merge，main `0642b13de7746a3fdff3a86141a121fcfb4300e8`。工作分支：`codex/gvideo3-publish-auth-activity-governance`。

范围仅 `/upload`、`/auth`、`/notifications`、`/admin/reports` 的呈现、必要的交互安全与测试/文档。用户已通过最终工程与视觉验收，Publish / Auth / Notifications / Governance 作为 Phase 5 稳定检查点封版，不再继续视觉调整。

用户已授权创建独立提交、推送 Phase 5 分支并创建 PR，等待 Frontend、Backend、Backend Race 全部通过后 squash merge 到 main，再同步本地 main。发布状态以 Git 和 GitHub 为准，完成后停止，不进入 Phase 6。以下为提交前验收记录，暂存区与 HEAD 描述保留当时状态；临时产物继续排除，源码、正式测试、规范和本报告保留。

## 1. 新增文件（完整列表）

- frontend/src/styles/publish.css
- frontend/src/styles/auth-experience.css
- frontend/src/styles/notifications.css
- frontend/src/styles/governance.css
- frontend/e2e/publish-experience.spec.ts
- frontend/e2e/auth-experience.spec.ts
- frontend/e2e/notifications-experience.spec.ts
- frontend/e2e/governance-experience.spec.ts
- docs/gvideo3-phase5-acceptance.md

## 2. 修改文件（完整列表）

- frontend/src/features/upload/UploadPage.tsx
- frontend/src/features/upload/UploadPage.test.tsx
- frontend/src/features/auth/AuthPage.tsx
- frontend/src/features/auth/AuthPage.test.tsx
- frontend/src/features/notifications/NotificationCenterPage.tsx
- frontend/src/features/notifications/NotificationCenterPage.test.tsx
- frontend/src/features/moderation/AdminReportsPage.tsx
- frontend/src/features/moderation/AdminReportsPage.test.tsx
- frontend/src/styles.css
- GVideo_3_UI_Kit/docs/component-spec.md
- GVideo_3_UI_Kit/docs/visual-spec.md
- GVideo_3_UI_Kit/docs/acceptance-checklist.md
- docs/gvideo3-progress.md

四份新增 CSS 分别限定 `.gvideo-publish-page`、`.gv-auth-experience`、`.gv-notification-center`、`.gv-governance-page`。styles.css 仅增加四个 import；没有搬运或删除旧 studio/auth/WATCH 样式。场景主题覆盖以包含相应页面的 Shell 为条件，没有重新定义全局 token。没有新依赖或通用 Wizard、Form、DataGrid、Toast 框架。

App/Shell、WATCH、VideoPage、VideoPlayer.tsx、hlsLoader.ts、CreatorPage、CreatorDashboard、MyVideosPage、ContentRow、ContentActionMenu、studio-workspace.css、watch.css、watch-playback.css、API client、types、backend、package/lock、theme-init.js 和 index.html 的 diff 均为空。

截图、日志、一次性预览与隔离夹具位于被忽略的 tmp/、frontend/tmp/ 和 Playwright 产物目录，不属于上面文件。暂存区为空，没有 .env、数据库、媒体文件、证书或构建产物进入待提交范围。

## 3. Publish 最终 workflow

单页连续的六个视觉章节：01 MEDIA → 02 STORY → 03 COVER → 04 SUBTITLE → 05 VISIBILITY → 06 PUBLISH。桌面使用章节标签与内容的阅读轴，移动端依次单列。章节没有下一步/完成步骤状态或持久化。

点击式原 file input 入口；选择后显示真实文件名、大小和媒体类型，支持更换。STORY 保留 title 2–80、description ≤2000 与真实 categories。封面未选复用 VideoCover 抽象品牌占位，明确“未选择封面时会自动截取视频画面”；选中后只显示真实本地 blob。字幕明确为可选单条初始中文字幕，更多轨道仍在原投稿管理字幕 Dialog 中处理。

## 4. Upload 业务逻辑是否修改

没有修改原业务处理块。归一化 CRLF/LF 后，从 UploadPage 函数声明到 JSX return 前的 state/effect/handler 与 HEAD 逐字一致。categories、cover URL 创建/替换/卸载回收、AbortController、upload callback、成功路由均保持。呈现层新增 busy 时禁用文件/字段/可见范围编辑，防止上传途中编辑误解；取消完成后恢复编辑且保留状态。

没有拖放逻辑、新播放器、草稿、自动保存、定时发布、AI、续传或多步骤持久化。原 accept 提示和服务端文件类型/大小校验边界保留。

## 5. FormData 是否保持

仍只提交一次原 api.upload。必选 title、description、category、visibility、video；可选 cover；选中字幕时添加 subtitle、subtitle_language=`zh-CN`、subtitle_label=`中文`。没有额外业务字段、拆分请求或多轨字幕。unit 与隔离原生 XHR E2E 都核对字段。

## 6. Progress / abort

真实 upload callback 驱动百分比与 progressbar，E2E 验证 0 → 45 → 100；100% 后等待原上传响应，成功才跳 `/video/:id`。文案区分上传与随后后台媒体处理，没有虚构转码进度。

取消沿用 AbortController 与 AbortError 原消息“上传已取消，文件和表单内容已保留”；title、description、category、visibility、video、cover、subtitle 均保留。beforeunload 只在 busy 时注册，普通填写不弹离页警告；卸载仍 abort。

未知 total 的页面 callback 分支显示“计算中”、不设置 aria-valuenow，unit 已直接验证。原 API client 只转发 lengthComputable=true 的 XHR progress，真正 lengthComputable=false 会被原 client 忽略，可能继续显示 0%；此既有边界未修改，不能把该 unit 分支声称为原生未知长度 XHR E2E 已验证。

## 7. Auth layout

桌面品牌叙事与紧凑表单双区；左侧 WATCH/CREATE/SHARE、品牌句与低强调 media geometry，右侧登录/注册、用户名与密码。移动缩短品牌叙事，390px 登录表单和 CTA 在首屏内。复用原 GVideo mark 与 Button，没有新照片、OAuth、邮箱/手机号、验证码或找回密码入口。

保留 login/register API、onAuth、原字段限制与 autocomplete：username、current-password/new-password。增加 busy 防重复提交和控件禁用、可见处理状态、关联 label 与错误 alert；模式切换保留输入内容。

## 8. Auth redirect security

提取原 AuthRedirect 的站内规则为本地 authTarget，同时供登录/注册成功与已登录访问 `/auth` 使用。`/video/1` 允许，`//evil.com` 与 `http://evil.com` 返回 `/`；未弱化原边界，修复原提交成功路径直接使用未经检查 next 的不一致。unit 与两项目 E2E 覆盖三种目标及两种提交/已登录路径。App 原登录失效/守卫保持。

## 9. Notification layout

WATCH 紧凑 activity feed：类型/icon、标题、detail、time、未读 indicator、单条已读操作，以 divider 与 typography 建层级。六种 follow/like/favorite/comment/processing_ready/processing_failed 仍来自同一个 notificationCopy，只增加可读类型 label，没有第二套业务映射。未读轻 surface + 圆点 + 文本；处理失败同时使用标签与不同图标，不只靠颜色。

空态 ACTIVITY / 00、“暂时没有通知”、真实说明，低强调行几何；没有假 CTA、筛选、删除、设置或 websocket。

## 10. Mark read / global unread

原 page_size=20、mark-one、mark-all、分页、目标跳转和本地 unread 更新完整保留；state/effect/handler 块与 HEAD 逐字一致。点击未读通知仍导航并异步标记；`gvideo-notifications-changed` 继续通知 Header bell 刷新。unit 验证事件，E2E 同时断言页面和全局 bell 数量。

单条忙碌有可读 label 和转动图标；全部已读 busy/disabled，原错误继续显示。没有扩大 API 或状态能力。

## 11. Admin layout

Creator OS 的 Governance 阅读工作台。桌面按状态/编号、举报内容、审核操作三列；内容包括 reason、video Link、detail、reporter/author Link、created/updated。过滤器仍 reportFilters 和 URL `?status=&page=`，移动内部横向滚动，不让页面横向溢出。

pending/reviewed/resolved/dismissed 通过 label、各自 icon、颜色共同表达；长 title/detail 安全换行。列表是扁平行，没有大型 Card、搜索、日期/原因过滤或批量治理。

## 12. Moderation actions

原 App admin guard、读取 AbortController、review(report,nextStatus)、busy、API update 和服务端响应 map 合并不变；state/effect/handler 块与 HEAD 逐字一致。审核中、处理完成、驳回保留直接按钮，当前状态按钮禁用。

更新后当前 filter 下该记录继续显示直到下次 fetch，保持原逻辑，没有乐观移除。测试核对 reviewed/resolved/dismissed、忙碌互斥、更新失败和原记录保留；没有真实 moderation mutation。

## 13. Mobile

Publish 单列六章节、长文件名换行、cover 真实比例、可见范围一列、CTA 全宽。Auth 390×844 首屏表单和 CTA 可见。通知类型/时间与正文按窄屏阅读顺序排列；Admin 按 status/id → video → reason/detail → reporter/date → actions 单列组织，390px 三动作同排且各有 ≥44px 热区，320px 改为 2+1 排列。

原移动 Shell/drawer 不变。页面操作、文件按钮、radio、filter、Auth 输入/切换都检查触控与键盘边界。

## 14. Light / dark

默认产品行为原样：没有合法保存偏好 light；已保存 dark/light 各自恢复。没有修改 App/theme-init/index 或 token 定义。

Publish/Admin 使用 Creator OS light canvas #f0f1ee、surface #fafbf8、field #e6e8e4；Notifications 使用 WATCH warm light #eeede9/#faf9f6；Auth 使用全局浅色 canvas #f7f8fa 与品牌 geometry。Dark 共用原 #080a0d 品牌画布。E2E 验证这些实际 canvas，以及主题切换、刷新和八视口；原 theme regression 另外执行。

## 15. Typecheck

`npm run typecheck`：退出码 0。日志 tmp/phase5-typecheck.txt。最初按用户指定顺序执行 typecheck → test → build，再执行 Phase 5 E2E。

## 16. Unit

`npm test`：最终综合检查中退出码 0，19 文件 / 201 项全部通过，18.46s；首次按用户指定顺序的独立运行也为 201 项通过（12.84s）。比 Phase 4 的 154 项净增加 47 项，四页面当前合计 57 项，没有删除原安全/播放器断言。覆盖 Upload 文件/URL生命周期/FormData/progress/abort/保留字段/离页、Auth 安全回跳和 busy、通知各类型/已读事件、Admin 过滤/分页/取消/动作/错误/状态。

日志 tmp/phase5-unit-final.txt。最初 Upload 新测试有 label 定位同时匹配 section 与 input 的问题，已将这些定位明确为 input，未改业务或放宽行为断言。综合检查首次重跑时，cancel 后 DOM 提示先于 passive effect cleanup，beforeunload 单测立即检查 remove spy 偶发失败；仅将该 cleanup 断言置于 waitFor，并保留忙碌时拦截/空闲时不拦截两项事件行为断言。相关 11 项复验全部通过（2.40s），最终全量结果见下一项综合检查。日志 tmp/phase5-upload-cleanup-test.txt。

## 17. Build

`npm run build`：最终综合检查退出码 0，1921 modules，35.26s，CSS 145.89 kB / gzip 25.59 kB；首次独立运行通过（24.53s），构建内容相同。HLS bundle budget：raw 360.41 kB / gzip 113.16 kB，通过。数值来自项目预算脚本；Vite 自身显示的 gzip 与预算脚本压缩方式不同。日志 tmp/phase5-build-final.txt 与 tmp/phase5-project-check.txt。

## 18. Phase 5 E2E

四份新 spec 在 desktop-chromium/mobile-chromium 执行：50 passed、4 skipped、0 failed，4.0m。四项 skip 是移动 project 不重复执行截图采集；每个 desktop collector 内实际采集 desktop/mobile 图片，所有功能用例两项目均执行。

验证原 file input、真实选中文件/blob 封面、初始字幕、native title validity、一次 FormData、原 XHR progress、取消及保留、成功实际路由；Auth 三类 next 与已登录/提交一致安全规则；通知单条/全部已读和 bell；Admin guard/filter/status update/mobile actions。**/api/v1/** 全量隔离 fulfill，未真实注册、上传、修改通知或治理数据。

首轮 45 passed / 5 failed：两个 Admin 首屏等待、一条通知跨 lazy 页面等待、两条深色 canvas 断言把透明 Shell 当成画布。仅修正 readiness/同步夹具和真实 body canvas 断言，保留行为检查；完整重跑以上结果通过。日志 tmp/phase5-e2e-first.txt、tmp/phase5-e2e-final.txt。

Auth 截图随后仅清除滚动/focus/hover 后补采，同时复验原登录保护用例：3 passed、1 项移动 project 重复截图采集 skip、0 failed（30.3s）。三张补采图已逐张复核，登录与注册无 hover/focus 干扰，移动浅色 CTA 在首屏内。日志 tmp/phase5-auth-capture-and-guard.txt。

## 19. Regression / 项目综合检查

App/theme、Discovery、visual-polish、Playback、Studio 原五份 spec 完整回归最终退出码 0：70 passed、2 项既有设备条件 skipped、0 failed（4.4m）。包含保存/默认主题、守卫回跳、Discovery 真实数据与隔离边界、播放器控件/续播/剧场/实际 fullscreen/PiP、Studio polling/menu/三个 Dialog/字幕工作流与八视口双主题。日志 tmp/phase5-regression-final.txt。

恢复暂停进度时 5173 和 8080 均拒绝连接，已停止当轮无服务回归，恢复原 Vite 与 Docker backend 后两服务 HTTP 200、backend healthy，未修改后端镜像或持久卷。恢复后的首轮回归为 69 passed / 2 skipped / 1 failed：桌面登录保护正确跳到 `/auth?next=%2Fupload`，Auth lazy 模块 200（8.435ms）、mark 304（2.673ms），但原 5 秒表单断言时仍处于 Suspense。现有证据不足以明确根因，不能归因于慢模块网络请求。未修改原 app.spec.ts、未延长其断言；同一原用例 desktop/mobile 独立复验通过，再完整重跑五份 spec。首轮日志 tmp/phase5-regression-service-unavailable.txt、tmp/phase5-regression-cold-start.txt 保留。

scripts/check.ps1 最终退出码 0：PowerShell 语法、HTTP Compose、Go format/test/vet、201 项 unit、typecheck、build/HLS budget 与 git diff --check 全部通过。HTTPS Compose 因 TLS_CERT_FILE/TLS_KEY_FILE 未设置按脚本规则跳过。首轮 Go 检查通过，frontend unit 因第 16 项 cleanup 等待问题为 200 passed / 1 failed 并正确中止；修正测试后完整重跑脚本通过，没有绕过检查。日志 tmp/phase5-project-check-first.txt 与 tmp/phase5-project-check.txt。没有执行会写入真实业务数据的综合 API acceptance 脚本，也没有进入全站 Phase 6。

## 20. 八视口

1536×960、1440×900、1180×820、920×900、768×1024、430×932、390×844、320×720；四页面在 dark/light 下均验证页面 scrollWidth 与控件边界。

包含连续 ASCII/下划线长文件名、80 字 title/长 description、通知长标题/评论 preview、举报长 title/detail、过滤器内部滚动、≥44px 页面触控区域、Auth keyboard 和 390px 首屏 CTA；Publish 320px 上传 progress 状态也无页面级溢出。不能据此宣称已完成所有网站/浏览器/对比度的 Phase 6 验收。

## 21. 已知问题与验证边界

- 原 upload client 不转发 lengthComputable=false，未知总量原生事件的限制见第 6 项；没有修改 API client。
- 点击式文件入口保留原 accept；格式/大小真正由后端限制，没有新前端校验系统。未实现拖放，因此不引入额外拖放校验路径。
- 通知原读取不使用 AbortController，点击即导航 + 后台已读保持。Admin 原读取错误可能同时显示原空态判断；未为 UI 改写业务分支。
- 18 张验收图与 E2E 使用隔离数据和明确抽象封面文件，不是实际生产账户/媒体。未用假摄影图、假视频帧或 reference 人物，也没有将夹具写入产品。
- 全站 E2E、全站 contrast/reduced-motion/a11y 与 HTTPS 环境如未配置，不归本阶段完成声明。Phase 1–4 受保护源码零改动不代替实际回归测试。

## 22. Phase 6 前建议确认

请先验收本阶段四页面的视觉方向与状态，尤其 Publish click-only 六章节、Auth mobile 首屏、Notifications 密度和 Admin 直接三动作。主题默认仍浅色；将来是否改深色应按原约定留到完整深浅主题验收后决定。

Phase 6 建议使用隔离数据环境进行全站视口、contrast、reduced-motion、focus/触控与完整 E2E；未知总量 XHR 支持、草稿/上传增强、通知筛选或治理扩展如需要，应单独确认业务范围。这里仅记录建议，没有实施 Phase 6 或新增功能。

## 18 张截图

1440×900 desktop 与 390×844 mobile 的完整页面截图。文件在被忽略的 tmp/phase5-screenshots，源码只保留可重复的隔离 E2E capture；以下 18 张图片已全部逐张复核，按页面和状态分类。

| # | 状态 | 文件 |
| --- | --- | --- |
| 01 | Publish dark | [打开](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase5-screenshots/01-publish-dark.png) |
| 02 | Publish light | [打开](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase5-screenshots/02-publish-light.png) |
| 03 | Publish selected files | [打开](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase5-screenshots/03-publish-selected-files.png) |
| 04 | Publish uploading 45% | [打开](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase5-screenshots/04-publish-uploading.png) |
| 05 | Publish mobile 390 | [打开](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase5-screenshots/05-publish-mobile390.png) |
| 06 | Auth login desktop | [打开](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase5-screenshots/06-auth-login-desktop.png) |
| 07 | Auth register desktop | [打开](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase5-screenshots/07-auth-register-desktop.png) |
| 08 | Auth mobile 390 | [打开](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase5-screenshots/08-auth-mobile390.png) |
| 09 | Notifications dark | [打开](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase5-screenshots/09-notifications-dark.png) |
| 10 | Notifications light | [打开](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase5-screenshots/10-notifications-light.png) |
| 11 | Notifications unread | [打开](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase5-screenshots/11-notifications-unread.png) |
| 12 | Notifications empty | [打开](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase5-screenshots/12-notifications-empty.png) |
| 13 | Notifications mobile 390 | [打开](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase5-screenshots/13-notifications-mobile390.png) |
| 14 | Admin dark | [打开](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase5-screenshots/14-admin-dark.png) |
| 15 | Admin light | [打开](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase5-screenshots/15-admin-light.png) |
| 16 | Admin pending | [打开](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase5-screenshots/16-admin-pending.png) |
| 17 | Admin filtered empty | [打开](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase5-screenshots/17-admin-filtered-empty.png) |
| 18 | Admin mobile 390 | [打开](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase5-screenshots/18-admin-mobile390.png) |
