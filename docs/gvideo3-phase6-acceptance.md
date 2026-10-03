# GVideo 3.0 Phase 6 — Final QA / Release Candidate 验收报告

> **后续状态（2026-10-03）**：用户已通过 Phase 6 最终工程与视觉验收，确认 GVideo 3.0 Release Candidate，不需要 Phase 6.1。Phase 6 已经由 [PR #43](https://github.com/zdl000000/GVideo/pull/43) squash merge 到 main，RC 基线为 [`e2c3a63`](https://github.com/zdl000000/GVideo/commit/e2c3a63fb84faea647d396b5e0ff622d535e09ba)。默认主题正式选择 OPTION A：保持 Light。未创建新的正式 Release Tag，未部署生产环境。
>
> 以下正文是**合并前的原始验收快照**；测试结果、失败及复验记录、日期、`af48544` 基线与“未提交”状态均保持原样。`C:/Users/...` 与 `tmp/` 证据路径仅为历史本地记录，不是公共素材链接。已记录的未验证范围继续保留；新版 README 正式截图另见 [素材说明](assets/gvideo3/sources.md)。

日期：2026-10-03。**Phase 6 工程验收完成：完整 isolated E2E、SAFE、Phase 1–5 回归、headers、真实只读回归、16 张截图、项目综合检查和 diff 检查均通过。正式必测范围内未发现未解决的 release blocker，建议交用户最终确认 Release Candidate；尚未 commit、push、创建 PR 或发布。**

状态约定：**PASS** = 有已完成检查证据；**FIXED** = 明确缺陷已修复并通过对应回归；**KNOWN LIMITATION** = 已知边界；**NOT VERIFIED** = 未验证。未验证项目不能当作通过。

## 1. Baseline commit

基线为 `af48544 feat(frontend): complete GVideo publish auth activity and governance (#42)`，工作分支 `codex/gvideo3-final-qa`。`docs/gvideo3-progress.md` 记录从干净 main 创建该分支；当前 HEAD 仍是 `af48544`，尚未创建 Phase 6 commit。

范围是 Phase 1–5 的明确缺陷修复与系统验收。没有新产品布局、功能、API、数据库字段、依赖或播放器功能；默认主题保持 light。不 commit、不 push、不创建 PR、不发布。

## 2. Changed files

以下为最终核对 `git status --short` 的完整正式文件清单，共 **41 个文件：32 个修改、9 个新增**。暂存区为空，HEAD/main/origin/main 与远端 main 均为 `af48544e41e5b43cb56782f5a5695799c91427b7`，远端没有本阶段分支。临时环境、日志、截图、媒体测试 seed 均在忽略的 `tmp/`，不在清单中；敏感文件和构建产物没有进入暂存区。

**新增 9 个文件：**

- `docs/gvideo3-phase6-acceptance.md`
- `frontend/e2e/final-qa.fixture.ts`
- `frontend/e2e/final-qa.spec.ts`
- `frontend/e2e/final-qa-focus.spec.ts`
- `frontend/e2e/final-qa-motion.spec.ts`
- `frontend/e2e/final-qa-busy.spec.ts`
- `frontend/e2e/final-qa-stateful.spec.ts`
- `frontend/src/shared/hooks/useDialogFocus.test.tsx`
- `frontend/src/shared/lib/colorContrast.test.ts`

**修改 32 个文件：**

- `docs/design-system.md`
- `docs/gvideo3-progress.md`
- `frontend/e2e/app.spec.ts`
- `frontend/e2e/comments.spec.ts`
- `frontend/e2e/notifications.spec.ts`
- `frontend/e2e/playback.spec.ts`
- `frontend/vite.config.ts`
- `frontend/src/app/shells/ApplicationShell.test.tsx`
- `frontend/src/app/shells/useShellNavigation.ts`
- `frontend/src/features/auth/AuthPage.tsx`
- `frontend/src/features/auth/AuthPage.test.tsx`
- `frontend/src/features/creator/CreatorPage.test.tsx`
- `frontend/src/features/creator/EditProfileDialog.tsx`
- `frontend/src/features/moderation/AdminReportsPage.tsx`
- `frontend/src/features/moderation/AdminReportsPage.test.tsx`
- `frontend/src/features/notifications/NotificationCenterPage.tsx`
- `frontend/src/features/notifications/NotificationCenterPage.test.tsx`
- `frontend/src/features/upload/UploadPage.tsx`
- `frontend/src/features/upload/UploadPage.test.tsx`
- `frontend/src/features/videos/MyVideosPage.tsx`
- `frontend/src/features/videos/MyVideosPage.test.tsx`
- `frontend/src/features/watch/VideoPlayer.tsx`
- `frontend/src/features/watch/VideoPlayer.test.tsx`
- `frontend/src/shared/hooks/useDialogFocus.ts`
- `frontend/src/styles/base.css`
- `frontend/src/styles/governance.css`
- `frontend/src/styles/motion.css`
- `frontend/src/styles/notifications.css`
- `frontend/src/styles/publish.css`
- `frontend/src/styles/responsive.css`
- `frontend/src/styles/studio-workspace.css`
- `frontend/src/styles/tokens.css`

已静态确认 `backend/`、API client、`types.ts`、package/lock、`App.tsx`、`theme-init.js`、`index.html`、Nginx 配置、`hlsLoader.ts` 无 diff。WATCH 页面结构、CreatorDashboard、VideoPage、VideoCover、ContentActionMenu 的生产代码无 diff；播放器只有第 13 项记录的一行缺陷修复。

## 3. Visual fixes

**FIXED，SAFE 浏览器 matrix 与截图视觉扫查 PASS。** 没有重排已验收页面。只修浅色 semantic foreground 对比度、表单 focus、hover 条件和移动命中区域。Rose/Cyan 原职责保留，没有新增粉色 surface、装饰 banner、照片或假视频封面。

**FIXED，生产复验 PASS：** 真实 Nginx 首轮发现 Vite 将小型 `gvideo-mark.svg` 内联为 data URI，原安全 CSP `img-src 'self' blob:` 因而阻断 logo/placeholder mark 并产生 console error。`vite.config.ts` 只对该文件禁止 assets inline、输出同源独立资源；没有放宽 CSP，没有改 App/Shell/Auth/VideoCover JSX。最终 matrix 每个主页面断言实际品牌图 `naturalWidth>0`、src 非 data URI，并保留 CSP/console 审计。

最终 16 张截图位于忽略的 `tmp/phase6-screenshots/`，最后采集时间为 **2026-10-03 04:50:55–04:51:05**。主任务实际查看02/06/10/16，contrast agent实际查看其余12张，全部16张已视觉审阅，**PASS**：没有 broken mark、页面级横向溢出或主页面异常布局重叠，WATCH/Studio/Auth 保持同一品牌且各自层次保留。生产镜像仍为第15项 e205，截图没有为掩盖状态而修改。

| 页面 | 视口/主题 | 截图 |
| --- | --- | --- |
| Home | 1440 / Dark | [01-home-dark.png](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase6-screenshots/01-home-dark.png) |
| Video | 1440 / Dark | [02-video-dark.png](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase6-screenshots/02-video-dark.png) |
| Dashboard | 1440 / Dark | [03-dashboard-dark.png](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase6-screenshots/03-dashboard-dark.png) |
| Publish | 1440 / Dark | [04-publish-dark.png](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase6-screenshots/04-publish-dark.png) |
| Home | 1440 / Light | [05-home-light.png](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase6-screenshots/05-home-light.png) |
| Video | 1440 / Light | [06-video-light.png](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase6-screenshots/06-video-light.png) |
| Dashboard | 1440 / Light | [07-dashboard-light.png](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase6-screenshots/07-dashboard-light.png) |
| Publish | 1440 / Light | [08-publish-light.png](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase6-screenshots/08-publish-light.png) |
| Home | 390 / Light | [09-mobile-home.png](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase6-screenshots/09-mobile-home.png) |
| Video | 390 / Light | [10-mobile-video.png](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase6-screenshots/10-mobile-video.png) |
| Content | 390 / Light | [11-mobile-content.png](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase6-screenshots/11-mobile-content.png) |
| Publish | 390 / Light | [12-mobile-publish.png](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase6-screenshots/12-mobile-publish.png) |
| Auth | 390 / Light | [13-mobile-auth.png](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase6-screenshots/13-mobile-auth.png) |
| Notifications | 390 / Light | [14-mobile-notifications.png](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase6-screenshots/14-mobile-notifications.png) |
| Admin | 1440 / Dark | [15-admin-desktop.png](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase6-screenshots/15-admin-desktop.png) |
| Admin | 390 / Light | [16-admin-mobile.png](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase6-screenshots/16-admin-mobile.png) |

截图使用明确隔离的内容夹具与独立环境真实媒体。播放截图等待 `readyState >= 2`，暂停并实际 seek 到第 2 秒且 `seeking=false`，不会用伪造帧代替解码画面。

## 4. Responsive fixes

**FIXED，八视口双主题 matrix PASS。** 原 320px 分页按钮 32×32 改为 44×44，分页允许折行；移动 Studio text link 改为最少 44px，并用 `.gv-shell .gv-studio-text-link` 防止后导入的 workspace 40px 规则覆盖。

既有 Hero/Video title clamp、安全换行、Creator identity、Content 行的移动布局保持。新 QA 夹具包含长中文、连续 ASCII/数字/underscore、长用户名/简介/错误/文件名；检查 document/body 横向溢出、player 边界、移动 Content 非缩小桌面表格、Publish 单列和 Auth 首屏 CTA。

**FIXED，生产复验 PASS：** 实际 touch 检查发现 mobile Content title link 命中高度不足 44px，已在原 mobile media query 用 `.gv-shell .gv-content-title { min-height: 44px; }` 最小修复，正式 coarse-pointer E2E 同步核对。

## 5. Accessibility fixes

**FIXED，unit / SAFE 浏览器验收 PASS。**

- `useDialogFocus` 排除 disabled、负 tabindex、display:none/hidden 元素；所有控件 busy 禁用时将 Dialog panel 作为可聚焦目标，Tab/Shift+Tab 留在 Dialog，busy Escape 不关闭。
- Edit Video / Edit Profile 的隐藏 file input 增加 `tabIndex={-1}`；真实键盘按钮仍触发 FileChooser。
- 表单字段以显式 `:focus-visible` 覆盖旧 `outline:0`，保留现有品牌 focus token。
- Auth、Publish、通知单条/全部已读、Governance 操作补齐 busy/成功/失败时的焦点；用户已主动移走焦点时保留新的位置。
- 移动 drawer 打开后 resize 到 desktop，将焦点交回可见 main，恢复背景 inert/overflow。

没有新增通用弹窗框架或改变接口契约。原 aria-modal、可读 label、允许的 backdrop close、关闭后 trigger return 继续回归。

## 6. Contrast audit

正式审计为 `frontend/src/shared/lib/colorContrast.test.ts`，使用实际 CSS token、WCAG linearized sRGB 相对亮度公式、透明 tint 合成；以黑白 21:1、相同颜色 1:1 校准。没有引入 axe 或其他新依赖。

覆盖 Dark root、Light root/Auth、WATCH Discovery、Playback/Creator、Studio/Content、Publish、Notifications、Governance 的 `bg`、`bg-elevated`、`surface`、`surface-2`。正文/次要/品牌/Cyan/success/danger/warning/info 前景目标 ≥4.5:1，focus ≥3:1；状态 tint 与主要 CTA、brand badge、danger-button 的默认/hover 文字另测。

**FIXED；公式 unit 与主要页面双主题浏览器 matrix PASS。** Light `text-2 #5d6674 → #596270`、`text-3 #626d7d → #5d6472`、`cyan-text #087777 → #066969`，确保 warm 次级 surface 也满足目标；danger-button 白字改为 `var(--gv-on-brand)`，修复 Rose 背景对白字不足的问题。品牌背景色、页面 canvas/surface 不变。

| 实际配色 | 修复前 | 修复后 |
| --- | --- | --- |
| text-2 / WATCH secondary `#e4e2dc` | 4.4804 | 4.7597 |
| text-3 / WATCH secondary | 4.0497 | 4.5906 |
| cyan-text / WATCH secondary | 4.1402 | 5.0152 |
| text-3 / WATCH canvas `#eeede9` | 4.4782 | 5.0763 |
| text-3 / Studio secondary `#e6e8e4` | 4.2538 | 4.8219 |
| cyan-text / Studio secondary | 4.3488 | 5.2678 |
| danger-button text / Rose rest | 3.5990 | 5.5070 |
| danger-button text / Rose hover | 3.2736 | 6.0543 |

最终源码只读公式复算：9 前景 ×4 surface ×8 profile 最小 **4.5087:1**（success / WATCH secondary）；Light focus 最小 **5.0901:1**、Dark focus **6.4692:1**；brand/cyan/danger/warning soft 合成背景最小 **4.7557:1**，Governance success/info 8% tint 最小 **4.6245 / 5.3233:1**。正式单测共 27 cases、444 比率断言、2 公式校准，纳入第 20 项通过的全量 unit。

该公式审计覆盖主要 token 对，没有对任意媒体帧或浏览器原生字幕 cue 叠层做逐像素对比度认证，不等同整个网站的独立 WCAG 认证。desktop 暂停时原生 cue 位于原有自定义 control 叠层范围，属于既有布局边界；本阶段没有修改字幕/control 布局。

## 7. Keyboard / focus

**FIXED，新增 focus/busy E2E PASS。** 新 `final-qa-focus.spec.ts` 覆盖 busy Delete/Subtitle trap、Edit/Profile FileChooser、移动分页、Content 菜单 ArrowUp/Down/Home/End/Escape、collision/resize/scroll/focus return、通知 read-one/read-all 成功与失败、Auth login/register busy、drawer resize。

`final-qa-busy.spec.ts` 另覆盖 Publish 取消/失败/成功、Governance 成功/失败，以及用户主动移走焦点的分支。以上纳入第15项 SAFE 最终通过结果；已有 discovery/app/playback/studio/publish/auth 用例继续覆盖导航、搜索、主题、Video card、播放器键盘与 menu、关注/收藏/评论和 Dialog，Phase 1–5 完整回归 **PASS**，见第13项。

## 8. Reduced motion

**FIXED，reduced-motion 浏览器 E2E PASS。** `motion.css` 补齐 Related 等旧媒体 selector 在 reduce 下取消 transform，保留原统一缩短 animation/transition、取消 smooth scroll 的规则。没有删除加载/忙碌的文字语义。

`final-qa-motion.spec.ts` 浏览器检查真实加载的 Hero/Card/Related cover 不 scale/translate；controlled loading/skeleton、通知忙碌、processing badge/数值进度仍可辨识且动画停止。fine-pointer 用例在 desktop 运行、coarse-pointer 用例在 mobile 运行，分别跳过不适用项目，不将 skip 当成失败或实测成功。

## 9. Touch

**FIXED，touch E2E PASS。** Studio/Publish/Notifications/Governance 原无条件 hover enhancement 统一放入 `(hover: hover) and (pointer: fine)`；active、selected、focus-visible 继续独立生效。Content menu danger 前景保持原 cascade，不能被一般 hover 文字覆盖。

移动分页与 Studio text link、Content title 命中区域已修；既有 menu/theme/mobile drawer、player、like/favorite/share/report、follow、dialog close、notification read、admin action、Publish selector 在 390/320 回归。关键操作不依赖 hover，browser 尺寸与 coarse-pointer 用例通过；没有宣称物理手机实机验收。

## 10. Theme

**产品默认行为保持。** `App.tsx` / `public/theme-init.js` / `index.html` 无 diff：无值或非法保存偏好 → light；保存 light/dark → 对应主题。WATCH light 保留 warm `#eeede9 / #faf9f6`，Studio light 保留 `#f0f1ee / #fafbf8`；player media surface 保持深色。

新 matrix 在八视口逐页检查两主题真实 computed canvas/surface/文字，bootstrap 用例阻塞 App JS 后先检查 html theme、color-scheme、theme-color、空 root，再放行 JS 检查 direct/deep link/protected/Auth/refresh；两项目均 **PASS**。这是 theme bootstrap 的自动化验证，未改成默认深色。

## 11. Loading / error / empty

**新增 SAFE QA /完整已有 workflow 回归 PASS。** QA 用实际请求门控覆盖 Discovery、Video、Creator、Studio、Content、Notifications、Governance 的加载反馈；500 夹具使用长错误，检查可见表达及 320px 无溢出。上传/Auth/通知/Governance 的 busy/error 在既有和新增 workflow 用例中覆盖。

空态/零值覆盖 Following、Favorites、Creator、Studio、Content、Notifications、Governance filtered、Related、Comments；processing 覆盖 pending/processing/ready/failed。没有重设计空态或新增大量 Skeleton。

## 12. Media fallback

**既有系统保持；SAFE/真实只读回归 PASS。** VideoCover 生产逻辑无 diff。空 URL、404、loading 继续共用品牌 placeholder；失败远程 img 移除，没有 broken img icon，也不生成替代照片。

真实开发环境基线用 GET 检查 20 条 cover，10 条返回 404，证据 `tmp/phase6-real-cover-audit-get.json`。以下路径均相对 `http://127.0.0.1:5173`（同源代理到原 backend）：

| 视频 ID | 404 cover URL |
| --- | --- |
| 65 | `/media/covers/ca8acc832e381b50aa7d5bdf.jpg` |
| 64 | `/media/covers/60ac3d348e3fdf0300fd8d87.jpg` |
| 63 | `/media/covers/726c89b20d7da99d03051bc9.jpg` |
| 62 | `/media/covers/f242cdb072767fb2dc4f1889.jpg` |
| 43 | `/media/covers/29ff3bc2cb03218af9bd1795.jpg` |
| 42 | `/media/covers/192b871ade92f9b8e60c6837.jpg` |
| 41 | `/media/covers/3dd8c152fad43f7fe930bb8d.jpg` |
| 40 | `/media/covers/70775177006cf617fe24d5c8.jpg` |
| 37 | `/media/covers/82d94b9b9d3577dbafd564d1.jpg` |
| 36 | `/media/covers/4b49e123b00585158e47418f.jpg` |

65/64 的 URL 在 Phase 2 报告中已有 Phase 1 历史记录；其他 URL 仅确认本阶段基线即缺失，没有逐一核实首次发生时间。它们可出现在真实 Home/Latest/Popular/Search 等返回这些记录的页面。本阶段未修改原数据库、媒体目录、URL 或后端，不能把旧文件缺失归因于此次前端改动。独立 404/空 URL 夹具、真实只读 regression 与截图检查通过；缺失文件本身仍未恢复。

## 13. Functional regression

**SAFE、真实只读、Phase 1–5 完整回归与完整 isolated suite 均 PASS，新增 stateful 两项目最终 PASS。下表列出对应验证入口，真实写入与隔离 API 契约验证边界在表后说明。**

Phase 1–5 独立环境首轮 **127 passed /1 failed /8 skipped，4.9m**：comments 用例只查 cover link，独立环境初始只有一条视频，被 Home 作为 Hero 展示而没有普通 cover link。旧 E2E 定位更新为 Hero CTA 或 cover link，支持已验收的两种实际入口，没有改业务源码或削弱后续评论断言。

**11 spec /136 case 最终完整重跑 PASS，exit0：128 passed /8 skipped /0 failed，3.9m**，日志 `tmp/phase6-phase1-5-regression-final.log`。全程目标为独立 `18088` DB/media 环境。8个条件 skip 为原 desktop/mobile 不适用行为2项，以及未启用旧 Phase5 Auth/Notifications/Publish 截图采集的6项；对应功能用例正常执行，最终截图使用本阶段独立16张RC集。

| 功能 | 主要证据入口 |
| --- | --- |
| 注册/登录/安全 next/权限/Auth expiry | auth-experience、app、final-qa、final-qa-focus、final-qa-stateful |
| 搜索/分类/Latest/Popular/Following/Favorites | discovery、visual-polish、final-qa、final-qa-stateful |
| Follow/Favorite/Like/Report/评论创建与删除 | playback、comments、final-qa-stateful；真实写入仅独立环境 |
| Share 复制与取消 | playback 的 clipboard/share API 夹具；不等同系统分享面板实测 |
| HLS/direct/fallback/quality/speed/subtitle | playback、VideoPlayer unit，使用独立 seed 真实媒体 |
| 音量/mute/resume/progress save | playback 新真实 direct media 用例；seed 20 秒，保存 8/9 秒进度后刷新 |
| Fullscreen/PiP/theater/同一 media DOM | playback；最终两项目 fullscreen/PiP capabilities 均 true，实际进入/退出及同一 media DOM 检查 PASS |
| 路由 video A→B/resize/theater 时间保持 | playback 已有真实媒体与 element identity 用例 |
| Creator profile/edit profile | playback、studio、final-qa-focus、CreatorPage unit |
| Studio stats/content/edit/subtitle/retry/delete | studio、final-qa、final-qa-focus、对应 unit |
| Upload/progress/cancel | publish-experience 原生 XHR 隔离进度、final-qa-busy；notifications 旅程独立环境真实上传 |
| Notifications/read one/read all/Admin reports | notifications-experience、notifications、governance-experience、final-qa-focus/busy |
| Theme | app、final-qa bootstrap 与双主题 matrix |

**真实数据库验证边界：** 独立 18088 环境实际验证了注册/登录、上传与媒体处理、评论创建/删除、通知 read-all，以及 like/favorite/follow/report；没有写原开发库。EditProfile、EditVideo、字幕管理 mutation、重试处理、删除视频、管理员审核等持久操作仍是隔离 API 契约/界面验证，没有逐项在真实数据库执行；播放器读取真实 VTT 和准备 seed 时上传 VTT，不等同真实验证全部字幕管理流程。通知 read-one 的焦点/成功/失败也由隔离 fixture 验证。完整 suite 通过与否都不改变这些边界。

**FIXED：** 已复现 direct video 404 只留下空播放器，`VideoPlayer.tsx` 原 `sourceMode === "fallback"` 才设失败改为所有媒体 error 都复用 `setStreamStatus("failed")`，提供既有错误文案/重试；只有这一行行为修复，没有重写播放器。新 unit 与真实 404 E2E 覆盖；`hlsLoader.ts` 未改。native HLS 的真实浏览器验证边界见第 25 项。

## 14. E2E classification

运行前逐份读取 spec，以整个文件的最高持久写入风险分类。当前 inventory 为 **17 份 spec / 240 个项目用例**（列举结果，不是执行通过数），证据 `tmp/phase6-e2e-inventory.log`。

| Spec | 类别 | 实际边界 |
| --- | --- | --- |
| auth-experience | A | 全部 business API fulfill，注册/登录不写真实数据 |
| governance-experience | A | reports/审核 PATCH/权限/分页均隔离 |
| notifications-experience | A | feed/read-one/read-all/bell 全隔离 |
| publish-experience | A | XHR progress/send 与上传 API 全隔离 |
| studio | A | stats/content/edit/retry/delete/subtitle API 全隔离 |
| final-qa-focus | A | 自有 fail-closed 全 API fixture |
| discovery | B | 真实列表/搜索/categories GET；取消收藏 mutation 隔离 |
| visual-polish | B | 部分夹具，auth/me/categories 仍可真实 GET |
| playback | B | seed metadata 明确 count_view=false；真实媒体 GET，业务 mutation 隔离 |
| security-headers | B | 真实 SPA/API/list/health/media404 GET，无计数写入 |
| final-qa | B | seed count_view=false + 真实媒体；浏览器 business API 全隔离 |
| final-qa-motion | B | 使用上述共享 fixture 的真实只读 seed |
| final-qa-busy | B | 使用上述共享 fixture；所有忙碌 mutation 本地 fulfill |
| app | C（混合） | Home/theme/nav 单例只读，但视频用例默认 count_view=true 写 views_count |
| comments | C | 真实注册、评论创建与视频 view counter |
| notifications | C | 两用户注册/session、MP4 上传/处理媒体、评论、read-all、view counter |
| final-qa-stateful | C | 独立环境真实注册/赞/收藏/关注/举报/评论创建删除与 view counter |

汇总 **A 6 / B 7 / C 4**。localStorage 的主题/进度改变不写数据库；默认 video GET 的 views counter 则是真实写入，不能因 HTTP GET 而归 B。

## 15. Isolated full E2E result

**FULL isolated E2E 最终 PASS，exit0：229 passed /11 skipped /0 failed，6.2m。** 完整运行全部 **17 spec /240 case**、desktop-chromium/mobile-chromium，包含 C 类真实旅程，日志 `tmp/phase6-full-isolated-final-2.log`。实际环境为 `E2E_BASE_URL=http://127.0.0.1:18088`、`E2E_MEDIA_VIDEO_ID=1`、`E2E_DISPOSABLE=true`、`E2E_CAPTURE_PHASE6=true`。11个 skip 为第13项旧回归8项、fine/coarse pointer各1个不适用项目，以及 mobile 不重复采集RC截图1项；适用项目和功能对应验证已执行。

完整 240 case 首轮已结束：**228 passed /1 failed /11 skipped，6.3m，exit1**，原始日志保留为 `tmp/phase6-full-isolated-first.log`，失败 error-context/trace 保留于 `tmp/phase6-full-first-failure/`。唯一失败是既有 app mobile drawer 的严格几何断言：CSS max-width280px，但 Chromium boundingBox 浮点值为 `280.00001335144043`，严格 `<=280` 失败。11 个条件 skip 为上述 SAFE 的3项及第13项已有回归的8项。

已仅修改 `frontend/e2e/app.spec.ts`：先断言 sidebar 非 null、width>0，再允许最大280px +0.01 CSSpx浮点测量容差；没有修改 CSS、布局或其他生产代码。修改后重新执行上述完整240 case并通过，首轮失败记录保留，没有用片段复验替代完整回归。

**SAFE 最终 PASS，exit0：87 passed /3 skipped /0 failed，2.9m**，共90 case，日志 `tmp/phase6-safe-isolated-final.log`。实际命令选择 `final-qa.spec.ts`、`final-qa-focus.spec.ts`、`final-qa-motion.spec.ts`、`final-qa-busy.spec.ts`，没有把这90 case称为完整 suite。3个条件 skip 是 fine-pointer 用例不适用 mobile、coarse-pointer 用例不适用 desktop，以及 RC 截图不在 mobile 项目重复采集；各自适用项目已运行。

过程保留：首轮旧镜像发现 inline SVG/CSP 真实缺陷，修复后重新构建；随后 SAFE 首轮 **83 passed /4 failed /3 skipped，5.4m**，针对失败6项复核 **6 passed，41.4s**，再跑上述完整 SAFE90 通过。测试修正分别是：切换 API fixture 时清除 unroute/install 间隙的真实 bell401；Notifications 的 route-key remount 会移除旧 page1 按钮，改用 browser Back 验证旧响应不能覆盖当前页面；bootstrap 改用 managed project context + page.close，避免16个手工 context.close 导致 trace zip stream 错误。没有放宽 console/CSP 或旧响应断言，没有新增 production 修复。

独立环境第三轮构建/启动 readiness **PASS**，证据 `tmp/phase6-env/production-readiness.json`。本轮包含 final-5 的 CSP mark 与 mobile title 修复；HTTP/资源检查通过不等于完整浏览器 QA 通过：

- Compose project `gvideo-phase6-rc-20261003`；frontend `127.0.0.1:18088`、backend `127.0.0.1:18080`，两容器 healthy。
- 独立 DB volume `gvideo-phase6-rc-20261003-data`、media volume `gvideo-phase6-rc-20261003-media`、network `gvideo-phase6-rc-20261003_default`。
- backend 复用 image SHA `sha256:a5e3ceb34bee93dd4d7dc476e8fc9c8cd19e7c19d22894d41870a20f5d795036`，没有新后端构建/改动。
- frontend 从最终源码使用原 Dockerfile/真实 Nginx 构建，实际 `docker container inspect .Image` 为 `sha256:e205d52d6e961049ed0c8135b40958c9e45d3dd326aea8f6df623f918b81eb93`；tag 的 `docker image inspect .Id` / RepoDigest 同为 e205。Compose `com.docker.compose.image` 标签为构建日志中的平台 manifest `sha256:9637ce902727f9d2ed4c859f53901d48cf8b7a1c328eb76f4b6dee16733e7e41`，与 manifest list/index 的 SHA 分开记录，不混称旧/新代码。
- 实际 CSS `/assets/index-BMDf8bQh.css` 含 mobile Studio text link / Content title 44px；入口 JS `/assets/index-B_CtqMO3.js` 引用同源 `/assets/gvideo-mark-BOScsom7.svg`，mark GET200。header/CSP 未放宽。
- 首页、`/video/1` deep link、实际 MP4/HLS/VTT GET 均 200；seed ID 1 ready、20 秒，准备时 count_view=false、views_count=0。seed 是明确的 FFmpeg 测试图案，仅存在独立卷。
- 原 8080/8088/5173 服务、原持久卷未替换；没有复制开发数据库、修改 `.env`/权限或清理数据。独立环境和存储保留供验收。

完整运行须使用：

```powershell
# 在 frontend 目录；这些是执行约束，结果以日志为准。
$env:E2E_BASE_URL = 'http://127.0.0.1:18088'
$env:E2E_MEDIA_VIDEO_ID = '1'
$env:E2E_DISPOSABLE = 'true'
$env:E2E_CAPTURE_PHASE6 = 'true'
npm run test:e2e
```

RC 截图另由 `E2E_CAPTURE_PHASE6=true` 启用，仅 desktop project 采集一次；不启用时条件 skip 需明确说明。

## 16. Real-env read-only regression

**PASS，exit0：48 passed /0 failed，2.6m**，日志 `tmp/phase6-real-readonly-final.log`。真实 `http://127.0.0.1:5173` 只运行明确 B 类 discovery/visual-polish/playback 子集，metadata 使用原 video 56 的 `count_view=false`；没有运行 C 类整个 spec，也没有执行真实注册/上传/互动。

首轮 `tmp/phase6-real-readonly-b.log`：**47 passed / 1 failed，5.4m**。失败为 desktop `playback media stays mounted while playing, resizing and toggling theater`，设 `currentTime=2` 后 5 秒仍 `seeking=true`。是否环境负载造成尚未证实；不能忽略或据此宣称回归通过。原开发 video56 仅 10 秒，新增需 >14 秒的 `Phase 6 direct playback restores` 不在本只读子集，改在独立 20 秒 seed 上验证。

原断言不变的单例复核 `tmp/phase6-real-seek-recheck.log`：**1 passed，6.3s**，随后完整48 case重跑通过。首轮47/1记录保留，超时原因未被证明，不推断为已确认的环境负载问题。

原开发 video56 只有10秒，resume 新用例被明确排除；该用例已在独立20秒 seed 的两个项目中通过，并随第15项完整 isolated suite最终重跑通过。

## 17. Stateful E2E strategy / result

**策略已执行；既有 C 类回归 PASS，新增 stateful 两项目最终 PASS。** comments、notifications、app 写计数用例及 final-qa-stateful 均只在已检查独立卷和端口的 18088 环境运行。既有 comments/notifications/app 已随第13项136 case回归通过，真实注册/评论/上传/通知操作没有写原开发环境。

`final-qa-stateful.spec.ts` 双项目实际注册不同用户、like/favorite/follow/report、创建并删除本次用户自己的评论，检查 Favorites/Following/Creator 结果；要求 `E2E_DISPOSABLE=true`，并断言 baseURL port=18088。共享 QA fixture 的其他 mutation 都 locally fulfill，不混称真实写入旅程。

新增 `final-qa-stateful.spec.ts` 的 desktop/mobile **2项最终 PASS，分别11.8s /25.9s**，证据为 `tmp/phase6-full-isolated-final-2.log` 对应用例。首轮两项也通过（8.4s /12.3s），但该轮完整suite因drawer浮点断言失败而exit1，最终以完整重跑为准。测试生成的记录仅保留于独立库，不清理旧用户数据或媒体，独立环境暂不执行 down -v/删除。

这里只将上述实际执行的行为记为真实数据库写入验证；EditProfile/EditVideo/字幕管理/重试处理/删除视频/管理员审核等仍属隔离 API 契约验证，见第13项，不据此声称全部持久操作已经真实跑通。

## 18. Security headers

**PASS，exit0：12 passed /0 failed，3.8s**，日志 `tmp/phase6-security-headers-final.log`。`security-headers.spec.ts` 原6用例 × 两项目，针对真实 Nginx `18088` 验证单份精确 SPA/API CSP、nosniff、DENY、Referrer-Policy、Permissions-Policy、隐藏 Nginx 版本、deep-link fallback、theme-init、health endpoints、media404。

Nginx 和 backend header 配置无 diff。mark 的同源独立资源修复遵守原 `img-src 'self' blob:`，没有放宽 data URI 或 inline 策略。HTTPS/TLS 未验证，见第27项。

## 19. Typecheck

最终命令 `npm run typecheck`：**PASS，exit 0**。CSP mark/mobile title touch 修复后的有序 final-5 日志 `tmp/phase6-typecheck-final-5.log`，无 tsc 诊断。

完整 E2E 之后的最终 `scripts/check.ps1` 再次执行 typecheck，**PASS**，日志 `tmp/phase6-check-final.log`。

## 20. Unit

最终命令 `npm test`：**PASS，exit 0，21 files / 252 passed，18.29s**。CSP mark/mobile title touch 修复后的有序日志 `tmp/phase6-unit-final-5.log`。

完整 E2E 之后的最终综合检查再次执行 `npm test`：**21 files /252 passed，31.75s，PASS**，日志 `tmp/phase6-check-final.log`。两轮都是完整运行，不将片段相加计算测试数量。

上一轮曾出现 lazy 匿名守卫标题等待超时（251 passed / 1 failed），没有忽略失败或删除既有断言。最终结论以本轮完整结束结果为准。新增验证涵盖 contrast、busy focus、通知 stale/unmount、drawer resize、direct media error。

## 21. Build

最终命令 `npm run build`（tsc + Vite + HLS budget）：**PASS，exit 0**。CSP mark/mobile title touch 修复后的日志 `tmp/phase6-build-final-5.log`；Vite1921 modules，构建阶段 **692ms**（不等于整个 npm 命令耗时）。同一源码独立镜像 build 同样 exit0，Vite951ms、产物 hash 与本地相同，SAFE浏览器复验已通过；production source 自 final-5 后未改。

最后综合检查中的 build 同样 **PASS**：1921 modules，Vite阶段 **12.90s**，CSS/入口JS/mark等产物 hash 与已测试生产镜像相同，HLS budget通过。耗时变化不影响产物核对，也不等同整个命令耗时。

## 22. Bundle sizes

final-5 修复后本地及第三轮生产镜像构建的实际产物：

| 产物 | Phase 5 | 当前观察值 | 差异 |
| --- | --- | --- | --- |
| CSS raw | 145.89 kB | 146.52 kB | +0.63 kB（约 0.43%） |
| CSS gzip（Vite） | 25.59 kB | 25.69 kB | +0.10 kB |
| HLS raw（budget） | 360.41 kB | 360.41 kB | 无变化 |
| HLS gzip（budget level9） | 113.16 kB | 113.16 kB | 无变化 |

CSS 增长来自少量 focus、hover 条件和 44px 修复，没有新样式体系。入口 JS **25.39 kB raw /7.99 kB gzip**；独立 mark SVG **0.77 kB raw /0.46 kB gzip**，只改变内联策略，原素材未改。Vite 表中的 HLS gzip 为 114.51 kB，与 budget 脚本 gzip level9 的 113.16 kB 口径不同，不能混用。budget 上限 raw550 kB / gzip170 kB；final-5 本地及第三轮生产镜像均 **PASS**。

`App.tsx` 的 React.lazy + Suspense 和 route splitting 保留，`hlsLoader.ts` 动态 import 保留。正式 QA 文件不进入产品 bundle；没有新依赖。

## 23. scripts/check.ps1

**PASS，exit0**，日志 `tmp/phase6-check-final.log`。在完整 isolated E2E 后执行未修改的原 `scripts/check.ps1`：PowerShell syntax、Compose HTTP config、Go formatting、`go test ./... -count=1`、`go vet ./...`、frontend unit/typecheck/build/HLS budget、git diff --check 均通过。Go有测试的14个package通过，domain没有测试文件；没有修改后端源码。

**NOT VERIFIED：** TLS_CERT_FILE/TLS_KEY_FILE未设置，脚本明确输出 `Skipping HTTPS Compose config`，不宣称HTTPS配置或TLS部署已通过。

单独最终 `git diff --check`：**PASS，exit0**，日志 `tmp/phase6-diff-check-final.log`。没有空白错误；日志中的VideoPlayer CRLF将转换为LF提示属于Git换行提示，没有为消除提示重写文件。

## 24. Viewports

正式矩阵：**1536×960、1440×900、1180×820、920×900、768×1024、430×932、390×844、320×720**。

`final-qa.spec.ts` 11主要页面逐页遍历八视口 × 两主题，**176组合/项目，最终两项目均 PASS**；实际 computed theme/overflow/品牌资源断言全部执行。角色/空态用例覆盖 Following/Favorites，因此全13路由均有入口。既有 discovery/studio/playback 等 Dialog/menu/长标题/边界交互已随完整旧回归通过，没有把 spec 定义数量当成通过数量。

## 25. Browser scope

正式必测范围为配置现有 **desktop-chromium / mobile-chromium**。SAFE、headers、真实只读子集及完整 isolated suite两个项目均 **PASS**。mobile project 是 Chromium 设备模拟，不等于真实 Safari/iOS。

**NOT VERIFIED：** 本机没有可用 WebKit browser，不新增/下载浏览器依赖；不宣称所有 Safari/iOS。native HLS 仅有既有 `canPlayType` 与 manifest 行为 unit 模拟，未在 native-HLS 浏览器实测。最终 Chromium 两项目日志均记录 `fullscreen=true`、`pictureInPicture=true`，实际进入/退出与同一 media DOM断言通过；不将结果外推到未验证浏览器。

## 26. Console / network audit

**SAFE及完整 suite内 QA matrix/行为审计、最终截图 audit PASS。** matrix/行为用例跨 navigation 收集 console.error、pageerror、unhandledrejection、requestfailed、response status 与 business API 请求，JSON 附在 Playwright artifacts；截图另保存 [console-network-audit.json](C:/Users/SkyShow/Documents/ChatGPT/GVideo/tmp/phase6-screenshots/console-network-audit.json)。这是有显式审计的用例范围，不将所有旧spec都视为具有同样console/network收集。

16张截图的实际 audit：**pageErrors0、unhandled0、unknownApi0、requestFailures0**；consoleErrors10，逐项均为9条 `/phase6-missing-cover.jpg` 的预期404和1条匿名 `/api/v1/auth/me` 的预期401。没有新 CSP/品牌资源异常，没有把这10条写成“console.error完全为0”。

只允许精确路径/状态的 expected missing-cover404、匿名 auth/me401、显式错误夹具500/过期401；其他 console/HTTP 异常均 fail。AbortController 取消仅允许 API/media 的 ERR_ABORTED，不吞掉未知资产失败；未知 API 不论正常/error/expired 场景均 fail-closed404并记录。没有笼统忽略所有 404/console error。

controlled gate 检查 Discovery/Notifications 新 query 不被迟到旧响应覆盖、Video/Creator unmount abort、Governance filter stale、Video ready 后 polling 停止、Content route unmount 后停止 polling。**FIXED：** Notification load 原无 AbortSignal，现每个 page effect 独立 controller，卸载/翻页 abort，then/catch/finally 均检查 signal；正式 unit/E2E 覆盖。其他稳定请求处理没有为减少请求而重写。

## 27. Known limitations

- **KNOWN LIMITATION：** 真实开发数据有第 12 项旧封面404；UI fallback 无法恢复丢失文件。本阶段不修数据库/媒体。
- **KNOWN LIMITATION：** 原 API client 只转发 XHR lengthComputable=true progress；真正未知 total 可能停留0%。页面 null-total 分支已有 unit，但不冒称原生未知长度 XHR E2E 验证。本阶段 API client 无 diff。
- **NOT VERIFIED：** WebKit/Safari/iOS/native HLS 真浏览器、物理触控设备、HTTPS/TLS 部署未验证。
- **KNOWN LIMITATION：** clipboard/share 语义以 browser API fixture 验证，没有系统分享面板或操作系统剪贴板实测。
- **KNOWN LIMITATION：** 任意媒体帧及浏览器原生字幕cue叠层未做逐像素对比度认证；desktop暂停时原生cue处于原有自定义control叠层范围，本阶段保持字幕/control布局。
- **KNOWN LIMITATION：** 保留 `--coral`、`--teal`、`--ink`、`--paper` 等 migration alias。只调整已证实的 hover/cascade/focus 冲突，未大拆 CSS 或无证据删除 selector。
- **KNOWN LIMITATION：** EditProfile/EditVideo/字幕管理/重试处理/删除视频/管理员审核等持久操作仅隔离 API 契约验证；真实数据库范围限第13/17项已实际执行的行为。

## 28. Release blockers

**正式必测范围内未发现未解决的 release blocker。** 真实只读48 case、CSP mark/mobile title生产浏览器复验、SAFE90、Phase1–5旧回归136、headers12、完整isolated240、真实stateful2项、16张截图、最终综合检查和diff检查均已完成并通过。首轮drawer测量断言在测试层修正后完整重跑通过；所有失败轮次及修复/完整复验过程保留。第27项限制仍存在，不因本结论被写成已验证。

本结论覆盖已执行矩阵中的关键路由、登录/权限/安全next、上传/播放器、数据互动、横向溢出、Dialog trap、主要token contrast、header、build/unit/typecheck/full isolated E2E。它不代替未测浏览器、物理设备、TLS部署或第27项仅mock验证的持久操作实测。尚未发现需扩大scope的后端代码缺陷；`backend/` 无diff。最终封版仍由用户确认。

## 29. Final theme decision

**当前实现继续默认 Light；建议 OPTION A，等待用户决定，不擅自切换。**

| 选项 | 视觉/产品影响 | 所需修改/测试 |
| --- | --- | --- |
| A：保持默认 Light（建议） | 维持原无偏好首访行为；warm WATCH 与较中性的 Studio 仍保留区分。用户已保存 Dark 可正常恢复，dark-first 品牌视觉不受影响。 | 无需改产品代码；保留本次双主题、bootstrap、刷新/守卫验证。 |
| B：默认 Dark | 仅改变无值/非法保存值用户的首次入口；已有合法 Light/Dark 仍应恢复。需要用户明确批准产品行为变化。 | 改 `App.tsx` 默认 fallback、`public/theme-init.js` fallback、`index.html` 初始 theme-color；同步默认主题测试/文档，重跑 JS前 bootstrap/深链接/刷新/权限/Auth/全站双主题与截图。 |

本阶段没有为 OPTION B 预先修改代码。

## 30. Release recommendation

**建议作为 GVideo 3.0 Release Candidate 交用户最终验收。** 规定的工程终验已完成，正式Chromium矩阵内没有未解决阻塞项；默认主题建议保留OPTION A的Light。用户审阅16张RC截图、本报告及第27项限制后，再明确决定是否封版和后续提交/发布。

本阶段到此停止：代码、正式测试、规范和报告保留在 `codex/gvideo3-final-qa` 的未提交工作区；暂存区为空。截图、日志、独立环境及媒体seed保持忽略；独立环境保留供验收，未删除容器、卷或用户数据。没有commit、push、创建PR或发布。
