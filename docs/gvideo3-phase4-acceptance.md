# GVideo 3.0 Phase 4 验收报告

日期：2026-10-02。基线：Phase 3 / 3.1 已通过 PR #40 squash merge，main `4aeadaf7c53aaae6f88cebb578298a66e01e43b1`。工作分支：`codex/gvideo3-studio-creator-os`。本阶段已由用户通过最终工程与视觉验收并封版，不再继续视觉调整。

用户已授权独立提交、推送并创建 PR，等待 Frontend、Backend、Backend Race 全部通过后 squash 合并到 main，再同步本地 main。发布状态以 Git 和 GitHub 为准；完成后停止，不进入 Phase 5。以下为发布前的验收记录，保留当时的暂存区与临时产物状态。

## 1. 新增文件（完整列表）

- frontend/src/features/videos/ContentRow.tsx
- frontend/src/features/videos/ContentActionMenu.tsx
- frontend/src/shared/components/StudioEmptyState.tsx
- frontend/src/features/captions/SubtitleManager.test.tsx
- frontend/src/styles/studio-workspace.css
- frontend/e2e/studio.spec.ts
- docs/gvideo3-phase4-acceptance.md

## 2. 修改文件（完整列表）

- frontend/src/features/creator/CreatorDashboard.tsx
- frontend/src/features/creator/CreatorDashboard.test.tsx
- frontend/src/features/videos/MyVideosPage.tsx
- frontend/src/features/videos/MyVideosPage.test.tsx
- frontend/src/features/captions/SubtitleManager.tsx
- frontend/src/styles.css
- GVideo_3_UI_Kit/docs/component-spec.md
- GVideo_3_UI_Kit/docs/visual-spec.md
- GVideo_3_UI_Kit/docs/acceptance-checklist.md
- docs/gvideo3-progress.md

以上为有意保留的源码、测试、规范和验收记录。临时日志、14 张截图以及 frontend/tmp/phase4-preview.html 均位于 Git 忽略目录，不进入生产入口或待提交文件。暂存区为空；没有 .env、数据库、媒体文件、日志、证书或构建产物进入 Git 变更列表。package.json、lockfile 没有改动，没有新增依赖。

## 3. Dashboard 最终数据结构

`api.creatorStats()` 和原加载/错误状态保留。主数字 videos_count；第二层 views_count/followers_count；次层 likes_count/favorites_count/comments_count。累计数字使用实际千分位与 mono/tabular 排版，没有五张同权 KPI 卡。

visibility 区仅展示 public_count/unlisted_count/private_count。processing_count 使用独立 status strip，明确与可见范围重叠；没有合并成 100% 分布。recent_videos 使用真实封面、标题、日期、播放、可见范围和 ProcessingBadge；VideoCover 统一处理空 URL/404。videos_count=0 显示发布第一条作品的工具型空态；非零总数但 recent_videos=[] 显示“暂无最近投稿”，不谎称没有任何投稿。Heading 显示真实 username，个人空间/管理投稿为低强调链接，发布视频为 primary CTA。

## 4. 未实现的 reference 数据

API 没有提供的 30 天趋势、增长率、同比/环比、播放曲线、收入、套餐、Creator Level、存储空间、流量来源、人口画像、内容评分、AI、草稿、系列、定时发布、批量编辑均未实现。SubtitleTrack 仅提供 id/language/label/url/is_default，没有原始文件名/格式字段，因此没有虚构 file/format。所有测试与截图隔离数据都明确是验收夹具，不进入产品数据。

## 5. Content Row 信息层级

语义 article，真实 Video 类型。第一层封面/两行标题、现有处理状态、可见范围、播放；第二层类别/文件大小、低强调简介、likes/comments、created_at 发布日期。不存在 updated_at，故不用“更新日期”冒充。processing/pending 的真实进度和 stage 占完整附加行，failed 展示真实错误或原业务 fallback。保留现有 ProcessingBadge/ProcessingProgress 和 visibilityLabels/visibilityHelp，没有创建第二套领域映射。

## 6. Context menu

单一工作流的本地 ContentActionMenu，无通用 DataGrid/Chart/Popover/CommandPalette。仅编辑视频、字幕管理、failed 时重新处理、删除。真实 button trigger，44px 触控区域、aria-expanded/haspopup/controls；menu/menuitem、ArrowUp/Down、Home/End、Escape、Tab、外部点击、禁用项、danger 项。测量菜单和 trigger，翻转/夹到视口内，并在滚动/resize 时更新定位。processing 的删除仍禁用。

## 7. Processing polling 是否修改

逐段与 HEAD 对比：requestPage/refreshPage、processingKey 与完整 polling effect、setPage 分页函数均 UNCHANGED。仍是 page_size=12、pending/processing 触发 2500ms、AbortController 与卸载/页面切换取消。轮询仅 setResult，不重新进入 loading，不卸载同一 key 的行、菜单或已打开 Dialog。unit 与浏览器验证了 20 -> 60 -> ready、同一行 DOM、菜单/弹窗保留。

## 8. Retry

api.retryVideo、busy/error、成功 notice 与恢复轮询保持。成功后仍 pending，绝不直接 ready。唯一小幅同步：成功反馈将旧 stage/progress 清为 queued/0，避免短暂出现“pending 但处理失败/100%”。这不是伪造 stage：backend/internal/repository/repository_videos.go 的既有 RetryTranscoding 就把 videos 更新为 pending、progress=0、stage=queued；后端未修改。下一轮仍完全由实际 API 覆盖。

## 9. Delete pagination edge case

原 remove 行为保留：当前页唯一条目且 page>1 时 setPage(page-1)，否则 refreshPage(page)。unit 和隔离 E2E 均验证第二页最后一条删除后回第一页面且看到剩余行。同页删除成功显示 role=status，并转焦点到 heading。没有删除真实开发视频。

## 10. Edit Dialog

保留 title/description/category/visibility/cover、原校验、FormData、api.updateVideo、busy/error。新增 Studio context/媒体预览和两列到单列布局。VideoCover 同时支持原 URL 与现有 blob File preview；createObjectURL/revokeObjectURL 原生命周期保留并有测试。空或 404 封面显示共享品牌占位；不会冒充真实内容。保存时更换封面按钮禁用。

## 11. Subtitle Dialog

仍由行菜单进入，不新增 route。SubtitleManager 使用 workspace 可选标记限定新样式，非 workspace 的调用保持旧行为。保留上传的 language/label/file FormData、2 MB/VTT/SRT 业务语义、默认/删除 API 与 native confirmation。默认 badge/危险删除保留，空态变成安静的轨道区域。onBusyChange 仅将既有请求状态传到父 Dialog，使上传/默认/删除期间 Escape、backdrop、关闭按钮都不可关闭；没有增加字幕处理功能。

## 12. Focus management

从菜单打开 Dialog 时保存该行持久的 ... trigger，不保存马上卸载的 menuitem。Dialog 继续使用原 useDialogFocus 进行背景 inert/aria-hidden、Escape 与 Tab trap。关闭/保存后由 MyVideosPage effect 在 Dialog cleanup 后恢复 trigger，替代会抢在 inert 恢复之前执行的 0ms timer；浏览器专项确认。删除取消返回 trigger；同页删除完成转 heading。shared hook 与 Shell 焦点逻辑没有改动。

## 13. Mobile strategy

<=700px 隐藏列标题，用 thumbnail+title、status+visibility、views/likes/comments、date、完整行 progress/error 组合；不缩小 desktop table，不引入整页横向滚动。菜单 44px，标题两行/overflow-wrap:anywhere。编辑 Dialog 单列；Dialog 自身保留视口内滚动和 footer 操作；字幕字段两列、文件整行。

## 14. Light/Dark

新增 scoped studio-workspace.css，导入在 responsive/watch-playback 之后、motion 之前。Dark 沿用语义令牌；Light workspace canvas #f0f1ee、surface #fafbf8、field #e6e8e4，媒体仍是深色 frame。只有包含这两个页面的 StudioShell 应用这些覆盖；Upload/Admin/WATCH 样式不修改。默认仍 light，保存 dark/light 后刷新和导航恢复；App、theme-init、index.html 零修改。

## 15. Typecheck

`npm run typecheck`：退出码 0。最后一次独立执行在焦点修复之后；综合检查另有最终结果。

## 16. Unit

`npm test`：19 个文件、154 项全通过，0 失败，12.88s；最终综合检查再次为 154/154（20.93s）。相对 Phase 3.1 的 131 项增加 23 项：Dashboard 增加 5、MyVideos 增加 14、SubtitleManager 新增 4。原 VideoPlayer 26 项保留。未改 Vitest 配置，沿用共享 worker 的测试边界 resetModules。

## 17. Build / bundle budget

`npm run build`：退出码 0，1921 modules；CSS 116.67 kB / gzip 21.40 kB。HLS budget 通过，raw 360.41 kB / gzip 113.16 kB。CreatorDashboard / MyVideosPage 继续独立 lazy chunk；App lazy/Suspense 原架构不变。

## 18. Studio E2E

`E2E_BASE_URL=http://127.0.0.1:5173 npm run test:e2e -- e2e/studio.spec.ts --workers=1`：20 passed、0 failed、0 skipped（3.0m），desktop/mobile Chromium 各 10 项。包括真实字段形状的隔离 aggregate、默认及保存主题、0 数据、菜单键盘/定位、Dialog 焦点/Tab/busy、retry、2500ms 进度与 DOM、末页删除、同页 status、字幕上传/默认/删除、加载错误和八视口两主题。

首轮联合运行发现 Studio 的保存焦点时序问题（已修复）；其余失败来自 StrictMode 的重复初始请求使计数夹具过早跳到 60，以及既有导航 remount 清掉上一页的 inline notice。夹具改为明确开始计数；末页删除断言真实新页结果，同页单独断言 notice/heading。没有放宽焦点或处理状态断言。

## 19. Phase 1–3 regression / 综合检查

最终原阶段回归命令：`E2E_BASE_URL=http://127.0.0.1:5173 npm run test:e2e -- e2e/app.spec.ts e2e/discovery.spec.ts e2e/visual-polish.spec.ts e2e/playback.spec.ts --workers=1`。50 passed、2 skipped、0 failed（3.7m）；两个 skip 为既有设备条件：desktop 项目的 mobile navigation，以及 mobile 项目的 desktop keyboard；对应设备用例均通过。App/默认与保存主题/Protected、Phase 2 Discovery/2.1 极长 Hero/空态、Phase 3 Playback/3.1 light surface/Related/Creator、媒体 DOM 保留、键盘、速度/质量/字幕、fullscreen/PiP 均保留并通过。连同独立 Studio 20 项，最终合计 70 passed、2 条既有设备 skip、0 failed。`scripts/check.ps1` 退出码 0：PowerShell 语法、HTTP Compose、Go 格式/所有包 test/vet、19 文件/154 项前端 unit（20.93s）、typecheck、build/budget 与 git diff --check 通过。HTTPS Compose 因未设置 TLS_CERT_FILE/TLS_KEY_FILE 按脚本规则跳过，未声明已验证 HTTPS。受保护路径自查：App/Shell、Watch、VideoPlayer/hlsLoader、CreatorPage、Discovery、Upload、Notifications、Auth、Admin、shared/api/client.ts、types.ts、package.json/lock、backend 无 diff。Auth/Protected、默认主题与 lazy/Suspense 保留。

## 20. 八视口

| 视口 | Dashboard 大数字 | Content 12 行/长文本/404 | 菜单首/末行 | 编辑/字幕/删除 Dialog | 两主题横向溢出 |
|---|---|---|---|---|---|
| 1536×960 | 通过 | 通过 | 通过 | 通过 | 无 |
| 1440×900 | 通过 | 通过 | 通过 | 通过 | 无 |
| 1180×820 | 通过 | 通过 | 通过 | 通过 | 无 |
| 920×900 | 通过 | 通过 | 通过 | 通过 | 无 |
| 768×1024 | 通过 | 通过 | 通过 | 通过 | 无 |
| 430×932 | 通过 | 通过 | 通过 | 通过 | 无 |
| 390×844 | 通过 | 通过 | 通过 | 通过 | 无 |
| 320×720 | 通过 | 通过 | 通过 | 通过 | 无 |

上表来自两个 Chromium 项目的隔离 E2E，每个项目都对八视口×两主题遍历。实际应用浏览器另检查已有账户的真实 /creator，以及隔离预览的 desktop/mobile 场景并逐张检查截图。其他浏览器和原生触屏设备尚未覆盖。原 App 播放器回归访问已有真实公开视频，播放计数按原行为增加；新增 Studio 的编辑/重试/字幕/删除测试全部隔离，未修改真实投稿资料或媒体文件。

## 21. 已知问题与范围限制

- 开发数据仍有既有缺失媒体封面；本轮只提供品牌 placeholder，不修复/生成媒体文件，VideoCover 状态逻辑未改。
- 既有 ShellContent 用 location.key 重置错误边界。删除末页唯一条目后导航到前页会 remount，上一页的短暂 inline notice 不会跨页保留；新页内容正确。同页成功 notice、删除取消焦点均已验证。本轮不改 Shell 或引入全局 Toast。
- 字幕删除仍使用原 native confirm，没有创建新的嵌套确认系统；接口无文件格式元数据，故不补假字段。
- 视觉方向等待用户最终验收；没有全站全量 E2E，也没有发布/通知/Auth/Admin 的 Phase 5 视觉改动。

## 22. Phase 5 前建议确认

请验收 Dashboard 数字层级、Content Row 信息密度、深浅 workspace、390px 阅读顺序、菜单与三个 Dialog。确认 Phase 4 后再按用户指令创建稳定 checkpoint；本轮没有 commit/push。Phase 5 的 Upload/Notifications/Auth/Admin 保持当前版本，尚未开始。

## 截图（全部为明确标注的隔离视觉验收夹具）

数据来自忽略的 frontend/tmp/phase4-preview.html，运行实际 App/Studio 组件；所有持久写入方法都拒绝执行。没有真实图片替身，没有伪造产品 Dashboard 数据。源码页面仍走原 API。

- [Studio Dashboard dark](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase4-screenshots/01-dashboard-dark.jpg)
- [Studio Dashboard light](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase4-screenshots/02-dashboard-light.jpg)
- [Studio Dashboard zero-data fixture](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase4-screenshots/03-dashboard-zero.jpg)
- [Content Management dark](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase4-screenshots/04-content-dark.jpg)
- [Content Management light](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase4-screenshots/05-content-light.jpg)
- [Content processing](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase4-screenshots/06-content-processing.jpg)
- [Content failed（含真实重试入口）](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase4-screenshots/07-content-failed.jpg)
- [Context menu open](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase4-screenshots/08-context-menu.jpg)
- [Edit Video Dialog](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase4-screenshots/09-edit-dialog.jpg)
- [Subtitle Dialog empty](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase4-screenshots/10-subtitle-empty.jpg)
- [Subtitle Dialog with tracks](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase4-screenshots/11-subtitle-tracks.jpg)
- [Delete Dialog](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase4-screenshots/12-delete-dialog.jpg)
- [Studio Dashboard mobile 390](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase4-screenshots/13-dashboard-mobile-390.jpg)
- [Content Management mobile 390](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase4-screenshots/14-content-mobile-390.jpg)
