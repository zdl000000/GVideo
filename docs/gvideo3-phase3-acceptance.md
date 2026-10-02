# GVideo 3.0 Phase 3 — WATCH Playback & Creator Channel 验收报告

日期：2026-10-02。基线：PR #39 合并后的 `81076db`。分支：`codex/gvideo3-watch-playback-channel`。

封版状态：用户已通过 Phase 3 + Phase 3.1 最终工程与视觉验收，授权将当前 WATCH Playback & Creator Channel 发布为稳定检查点。源码、测试、设计规范与本报告保留；截图／临时夹具／构建产物不进入提交。PR 必须在 Frontend、Backend、Backend Race 全部通过后 squash 合并 main，发布完成后停止，不进入 Phase 4。下文保留验收当时的验证与提交状态，发布结果以 Git／GitHub 记录为准。

工程实现与前端验收完成，等待用户视觉验收；没有 commit、push 或进入 Phase 4。范围仅为 `/video/:id`、`/users/:id` 的表现层及必要测试。未使用 qiaomu-design 技能。

## Phase 3.1 Visual Polish — 最新增量验收

用户已通过 Phase 3 工程及整体视觉验收。下方 19 项保留 Phase 3 当时的实现／验证记录；以下为 commit 前指定的三项精修。本增量未 commit、push 或进入 Phase 4。

1. **Light playback verification**：实测现有 scoped light 样式已正确生效，无需重新定义 token。画布 `#eeede9`、输入 surface `#faf9f6`；标题、meta、creator identity、description、comments、related 的透明背景实际透出 warm canvas，播放器仍为 `#0c0d10`。新增视觉 E2E 逐区断言 canvas/surface token、最终可见背景和标题前景，并覆盖无偏好默认 light、dark/light 切换及刷新恢复。原默认主题代码零修改。
2. **Related compact placeholder**：只通过播放页 Related scoped CSS 保留 26px、opacity .62 的现有 GVideo mark 和轻线框，隐藏 placeholder 文案。VideoCover 状态逻辑零修改，没有新增假图片；404 和空 URL 两种状态在两主题、1440/390px 检查品牌标记真实加载、尺寸／边界及失败远程 img 移除。
3. **Creator backdrop refinement**：改为 identity 段落右侧透明绝对定位几何背景，没有整体横幅表面／边框／独立高度；头像、名字与几何共享同一段落，移动端隐藏装饰签名文字，真实 username/bio/stats/actions 保持清晰。两 Chromium 项目的八视口检查通过，原作品网格、分页、本人入口与 Dialog 保留。

实际增量文件：`frontend/src/styles/watch-playback.css`、`frontend/e2e/playback.spec.ts`；同步文档 `docs/design-system.md`、`docs/gvideo3-progress.md` 与本报告。没有修改 VideoPlayer.tsx、VideoCover.tsx、API、后端或依赖。缺图截图用的临时预览更新在被忽略的 `frontend/tmp/phase3-preview.html`，不进入生产构建／提交。

严格按序：`npm run typecheck` 退出码 0；`npm test` **18 文件 / 131 项全部通过（17.39s）**；`npm run build` 退出码 0、1918 modules、CSS **101.81 kB / gzip 18.91 kB**，HLS budget **raw 360.41 kB / gzip 113.16 kB** 通过。随后运行以下 E2E：**26 passed、0 failed、0 skipped（3.0m），退出码 0**，含本增量新增 6 项及已有播放／频道／主题回归。原 26 项播放器单测全部通过。本增量按用户要求没有再运行 scripts/check.ps1。

```powershell
$env:E2E_BASE_URL='http://127.0.0.1:5173'
npm run test:e2e -- e2e/playback.spec.ts e2e/app.spec.ts --grep 'Phase 3.1|playback|channel|theme selection' --workers=1
```

五张实际截图保存在忽略的 `tmp/`：

- `gvideo3-phase31-video-dark.jpg`：真实 `/video/61`，desktop dark。
- `gvideo3-phase31-video-light.jpg`：真实 `/video/61`，desktop light。
- `gvideo3-phase31-related-broken-cover.jpg`：实际 VideoPage/VideoCover 的隔离 404／空 URL 夹具，没有修改真实媒体数据。
- `gvideo3-phase31-creator-desktop.jpg`：真实 `/users/128`，desktop。
- `gvideo3-phase31-creator-mobile.jpg`：真实 `/users/128`，390×844。

验收完成后停在匿名／浅色真实播放页预览，等待用户确认。无新增提交、无暂存文件；截图／临时夹具被忽略。

## 1. 新增文件

- `frontend/src/styles/watch-playback.css`：仅播放页／作者频道生效的样式。
- `frontend/e2e/playback.spec.ts`：8 个测试，桌面与移动 Chromium 共 16 项。
- `docs/gvideo3-phase3-acceptance.md`：本报告。

验收截图、临时预览和极端标题夹具在已忽略的 `tmp/` 或 `frontend/tmp/`；它们不是新增提交文件，也不进入 Vite 生产构建。没有新增依赖。

## 2. 修改文件

- `frontend/src/features/watch/VideoPage.tsx`
- `frontend/src/features/creator/CreatorPage.tsx`
- `frontend/src/styles.css`
- `frontend/src/features/watch/VideoPage.test.tsx`
- `frontend/src/features/watch/VideoPlayer.test.tsx`
- `frontend/src/features/creator/CreatorPage.test.tsx`
- `docs/design-system.md`
- `docs/gvideo3-progress.md`

源码修改是页面 DOM、语义／无障碍属性和 scoped CSS。设计规范同步最终结构与响应式策略；进度文档保留前阶段历史记录。

## 3. VideoPlayer 是否修改

`VideoPlayer.tsx` **零 diff**。`hlsLoader.ts` 也没有修改。控件更新完全通过 `.gv-watch-page` 内的 CSS 完成。

## 4. 播放器业务逻辑 diff

没有。HLS/native HLS/direct fallback、媒体初始化与清理、质量／字幕／倍速状态、音量、快捷键、进度保存／续播、全屏、PiP、错误处理继续使用原实现。API client、types、App、Shell、后端、数据库、Studio／Upload／MyVideos／Auth／Notifications／Admin 和 Phase 2 Discovery 源码均未修改。

`VideoPlayer.test.tsx` 增加 5 项回归覆盖，并在套件边界用项目既有的 `vi.resetModules()` 方式清理共享 worker 的模块缓存，使原 HLS mock 正确绑定。没有删除、skip、弱化原断言或延长原测试超时。

## 5. 播放页最终结构

主列：**播放器 → 非 ready 时状态条 → 标题 → 元信息与互动 → 作者身份 → 简介 → 评论**。右侧为 NEXT UP / 相关推荐。

播放器是 `watch-main` 的首个、始终存在的子组件。桌面标题最多 3 行，移动端最多 3 行，连续 ASCII／数字／下划线可安全换行；完整标题保留在 DOM 与 title 属性。元信息仍来自真实播放量／发布日期；分辨率沿用原条件，只在 ready 且数据存在时展示。

pending/processing 显示轻量状态与原进度；failed 保留原错误／原文件可播放说明；ready 不显示状态条。轮询实现与时间间隔不变。点赞／收藏增加明确 accessible name 和 pressed 状态；busy、计数、分享、举报与登录回跳使用原逻辑。

## 6. Theater mode 实现方式

保留原受控 theater state 和按钮回调，只通过 CSS 将双列变为一列、扩大同一个播放器，将 related 放到后续段落；标题、简介、评论均保留。

集成与 E2E 检查同一个媒体 DOM 节点、source 和 HLS 实例；真实媒体播放期间切 theater、1440 → 920 → 390 resize，不新增 loadstart、不重置 currentTime、不暂停；暂停后关闭 theater，媒体时间保留。没有创建新 fullscreen-like 业务模式。

## 7. Creator Channel 结构

品牌几何 backdrop → 真实头像／用户名／简介／加入时间 → 投稿、粉丝、关注统计 → 操作 → standard 作品网格与原分页。没有 banner 数据字段，没有拿视频封面假装上传 banner。

他人频道保留关注／已关注；本人保留编辑资料、创作者中心、管理投稿，只有管理投稿为主操作。路由仍为原 `/creator`、`/me/videos`；资料修改仍使用现有 EditProfileDialog 与保存／头像上传 API。

作品桌面三列、平板两列、移动一列。空状态复用 WatchEmptyState，原“还没有公开投稿”语义、自身发布／他人浏览热门 CTA 不变。长 username 安全换行；bio 原换行保留；无 bio 使用原说明；零统计如实显示 0。

## 8. Related 复用方案

保留轻量 RelatedVideoCard，媒体部分使用 Phase 2 的共享 VideoCover，空 URL／404 不出现 broken img。title 两行，creator 与 views 同行，保留实际时长。

数据获取仍是原 same category + popular、排除当前 id、slice 8；没有新增推荐原因或排序逻辑。桌面 300px rail，平板两列，移动单列；theater 桌面移到下方三列。

## 9. Comment 结构

轻量 composer：真实当前用户头像（登录时）、textarea、发布按钮。评论行仍为真实 avatar／username／content／time／允许时删除，无逐条大 Card。

fetch/create/delete、计数、匿名 disabled、评论作者和视频作者的删除权限不变。没有 reply、评论点赞或嵌套评论。空状态保留“还没有评论”语义。Report 保持原 inline 表单及 spam/inappropriate/copyright/other，不增加 Dialog 生命周期或新举报类型。

## 10. Mobile playback 策略

<=700px 媒体贴边，16:9，没有厚 Card 边框。timeline 和控件放在媒体下方，播放／时间／全屏在第一行，次级控件在下行，320px 按实际宽度换行。没有隐藏 play、timeline、fullscreen、theater、PiP 等核心功能。

播放器可见按钮与互动／关注／删除的目标至少 44px；音量保持可访问。全屏时视频填满容器，控件回到叠加层，保留明显 focus。默认主题仍是无合法保存值 light、已保存 dark/light 对应恢复；仅这两页的 light 表现沿用 Phase 2.1 warm canvas／媒体边框。

## 11. Typecheck

最终按用户要求先执行 `npm run typecheck`，退出码 0。

## 12. Unit tests

随后执行 `npm test`：**18 文件、131 项全部通过，0 失败、0 skip，13.80s**。Phase 2 基线 106 项；本阶段新增 25 项，包括 VideoPage 16、CreatorPage 4、VideoPlayer 5。

VideoPage 套件共 18 项，覆盖 player-first、加载／迟到旧请求／失败路由切换、like/favorite、四类匿名回跳、follow、评论发布与删除权限、report/share、related 加载／空／404、pending/processing/failed/ready 与轮询。CreatorPage 共 5 项，覆盖本人／他人、无简介／empty／匿名、follow toggle、原保存契约与焦点返回、standard 卡片和分页。

## 13. Build 与 bundle budget

随后执行 `npm run build`：退出码 0，1918 modules。CSS 101.44 kB，gzip 18.84 kB；VideoPage lazy chunk 28.58 kB / gzip 9.26 kB；CreatorPage 7.54 kB / gzip 3.05 kB。

现有 budget 脚本仅约束 HLS chunk：**raw 360.41 kB < 550.00 kB；gzip 113.16 kB < 170.00 kB，通过**。没有把该结果描述成整站 JavaScript 或 CSS 的预算验收；项目没有为这些文件设置相应 budget。原 lazy 与 Suspense 未修改。

## 14. VideoPlayer test

完整套件 **26 项全部通过**，原 21 项全部保留。原覆盖包含键盘 scope、字幕／菜单焦点、播放拒绝、fullscreen/PiP 拒绝、HLS 生命周期／卸载／致命错误 fallback／动态模块错误／native HLS abort。

新增 5 项：theater 保留 DOM/source/time/HLS 实例；实际质量 level 与自动恢复；倍速实际更新与 Escape；续播／暂停保存／结束清理；native HLS manifest 清晰度与不加载 hls.js。

## 15. E2E

最终命令（按 typecheck → unit → build 之后执行）：

```powershell
$env:E2E_BASE_URL='http://127.0.0.1:5173'
npm run test:e2e -- e2e/playback.spec.ts e2e/app.spec.ts e2e/discovery.spec.ts e2e/visual-polish.spec.ts --grep 'playback|channel|video page|video player|theme selection|discovery' --workers=1
```

**42 项：41 passed、1 skipped、0 failed，4.8m，退出码 0**。唯一 skip 是原 app.spec.ts 中移动 Chromium 的桌面键盘限定用例；没有新增 skip。新增 Phase 3 桌面／移动共 16 项全部通过；主题导航／刷新、Phase 2 Discovery 与 Phase 2.1 极长 Hero／空状态回归同时通过。

两项目均实际报告 `fullscreen: true, pictureInPicture: true`，已进入／退出原生 fullscreen 与 PiP 并检查媒体 DOM 保持一致；不是仅检测按钮存在。

新播放 E2E 通过 `page.route` 隔离所有业务 API 响应与写入，只读取现有视频 56 的真实本地媒体（`count_view=false`）来测试播放，没有注册账号、上传文件、真实关注／收藏／评论／举报写入。未运行会向真实环境写入的全量注册／上传 E2E。CSS 对封版 Discovery 的影响通过原有 E2E 回归检查。

仓库综合检查 `scripts/check.ps1` 的最终结果见本报告末尾“最终复核”。

## 16. 八视口检查

实际 Codex 浏览器检查真实视频、真实作者频道与 EditProfileDialog；新增 E2E 在桌面与移动 Chromium 两项目复查长标题、长用户名、长 bio、控件边界和 Dialog。独立预览额外检查纯中文与连续 ASCII／下划线超长标题，均不超过 3 行。

| 视口 | 播放布局／Related | 作者作品 | Dialog 实测尺寸 | 结果 |
| --- | --- | --- | --- | --- |
| 1536×960 | 主列 + 300px rail | 3 列 | 660×450 | 通过 |
| 1440×900 | 主列 + 300px rail | 3 列 | 660×450 | 通过 |
| 1180×820 | 主列 + 300px rail | 3 列 | 660×450 | 通过 |
| 920×900 | 单主列／下方 2 列 | 2 列 | 660×450 | 通过 |
| 768×1024 | 单主列／下方 2 列 | 2 列 | 660×450 | 通过 |
| 430×932 | full-bleed／下方单列 | 1 列 | 396×582 | 通过 |
| 390×844 | full-bleed／下方单列 | 1 列 | 356×582 | 通过 |
| 320×720 | full-bleed／控件可换行 | 1 列 | 286×582 | 通过 |

均无 horizontal overflow，播放器不越界，可见控件不互相覆盖且至少 44px，stats 保持三列，Dialog 不超屏。滚动条会占用部分浏览器可用宽度；full-bleed 相对实际页面可用宽度判断。实际 Dialog Escape 与关闭后编辑入口焦点返回也通过。

## 17. 已知问题与验证边界

没有最终验证中的工程失败。关机恢复后的 worker 启动超时、共享模块缓存及高负载浏览器启动超时均重新定位并复验，没有靠放宽旧断言解决。920px 超长作者名曾撑开 implicit grid track，已在该页设置 `minmax(0, 1fr)`，八视口复验通过。

部分真实旧 cover URL 仍会 404，这是 Phase 2 前后已有的媒体数据现象，本阶段没有修复或替换媒体数据；related 已接入共享 placeholder。部分短视频暂停首帧可能是暗画面，实际截图等待媒体就绪后保存，没有替换成伪造视频帧。

已验证 Windows Chromium 与 Chromium 移动设备模拟，不等同于真机 iOS/Safari 或所有浏览器的 native HLS 验收；native HLS 分支已由单测覆盖。HTTPS Compose 没有提供 TLS 文件，综合脚本按原规则跳过；不代表 TLS 部署已验收。视觉最终结论由用户确认。

## 18. 与参考图未实现的部分

没有增加直播、弹幕、章节、AI、download/cast、推荐原因、认证／等级／会员／付费、自动下一条、playlist、社交链接／所在地、banner 上传、评论回复／点赞。没有伪造在线人数、趋势、观看时长或热度。

简介保留全文与原始换行，本阶段没有增加可选展开／收起或 markdown/linkify。举报保留 inline，以保护现有交互。EditProfileDialog 只通过 scoped CSS 对齐表面，没有更改资料／头像保存业务。

## 19. Phase 4 前建议确认事项

请确认播放页 dark/light 的观看优先层级、移动端可换行的控件排布、频道 abstract backdrop 与本人操作优先级。用户视觉验收通过后，再单独授权稳定检查点 commit／push 或下一阶段。

当前仍停在 Phase 3。没有提交、推送或进入 Phase 4；未来默认深色的决策仍留到 Phase 6。

## 关键截图

截图均保存在忽略目录，使用当前实际页面实现；下表注明真实数据与仅用于验收的隔离夹具。夹具不进入产品，不写真实数据库。

| 要求 | 数据／页面 | 截图（相对仓库根目录） |
| --- | --- | --- |
| Video desktop dark | 真实 `/video/61` | `tmp/gvideo3-phase3-video-dark.jpg` |
| Video desktop light | 真实 `/video/61` | `tmp/gvideo3-phase3-video-light.jpg` |
| Video theater | 真实 `/video/61` | `tmp/gvideo3-phase3-video-theater.jpg` |
| Video mobile 390 | 真实 `/video/61`，390×844 | `tmp/gvideo3-phase3-video-mobile.jpg` |
| Video processing | 隔离 processing 45% 夹具，现有媒体 | `tmp/gvideo3-phase3-video-processing.jpg` |
| Video comments | 真实 `/video/61`，已有评论 | `tmp/gvideo3-phase3-video-comments.jpg` |
| Creator other desktop | 真实 `/users/128` | `tmp/gvideo3-phase3-creator-other.jpg` |
| Creator self desktop | 真实已有账号 `/users/140` | `tmp/gvideo3-phase3-creator-self.jpg` |
| Creator mobile | 真实 `/users/128`，390×844 | `tmp/gvideo3-phase3-creator-mobile.jpg` |
| Creator empty | 隔离空作品／零统计／无 bio 夹具 | `tmp/gvideo3-phase3-creator-empty.jpg` |

额外边界截图：`tmp/gvideo3-phase3-video-long-chinese.jpg`、`tmp/gvideo3-phase3-video-long-ascii.jpg`、`tmp/gvideo3-phase3-creator-long.jpg`，均为明确的验收夹具。

## 最终复核

最终 `scripts/check.ps1` 退出码 0：PowerShell 语法、HTTP Compose、Go 格式／全量测试／vet、前端 18 文件 131 项单测（35.15s）、typecheck、build（1918 modules，11.20s）、HLS budget 和 `git diff --check` 均通过。HTTPS Compose 因未设置 TLS_CERT_FILE/TLS_KEY_FILE 按既有规则跳过。GOPROXY 仅在检查进程中选择可用代理，没有修改仓库配置。

最终暂存区为空；新增／修改文件仅为上列 11 个文件。临时预览与所有截图确认被忽略；没有 .env、数据库、媒体、日志或证书进入暂存区。浏览器已还原匿名／浅色／默认视口并保留真实 `/video/61` 供预览。HEAD 仍是 `81076db`，没有新增 commit、push 或 Phase 4 内容。
