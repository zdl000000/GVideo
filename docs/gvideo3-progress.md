# GVideo 3.0 重构进度

更新日期：2026-10-02。

## 当前范围


Phase 4 — STUDIO CREATOR OS 已由用户通过最终工程与视觉验收，Studio Dashboard、Content Management 与 Dialog workflow 作为稳定检查点封版，不再继续视觉调整。基线是 PR #40 squash 后 main `4aeadaf`，工作分支 `codex/gvideo3-studio-creator-os`。范围仅 /creator、/me/videos、三个行操作弹窗、SubtitleManager workspace 表现层、新内容行/菜单/空态、scoped studio-workspace.css 与测试文档；无 Phase 5 内容。

用户已授权创建 Phase 4 独立提交、推送当前分支并创建 PR；须等待 Frontend、Backend、Backend Race 全部通过后 squash 合并到 main，再同步本地 main。发布状态以 Git 和 GitHub 为准，完成后停止，不进入 Phase 5。提交仅保留源码、测试、设计规范与验收报告；临时预览、隔离截图夹具、截图、日志及其他忽略产物不进入提交。

Dashboard 只展示真实 CreatorStats 分层累计数字，visibility 与 processing 独立。内容行保留原分页、2500ms polling、AbortController、请求取消；菜单 Dialog 回持久 trigger，保存焦点移到 cleanup 后。重试成功显示 pending/queued/0%，与既有后端重置一致；字幕 busy 状态通知父 Dialog，防止请求中关闭。所有受保护路径和依赖未修改。

当前独立 typecheck、19 文件/154 项 unit、build/HLS budget 均通过；Studio E2E desktop/mobile 共 20 项全部通过（3.0m），含八视口×两主题。14 张明确标记的隔离夹具截图位于忽略目录。最终 Phase 1–3 E2E 回归 50 passed、2 条既有设备条件 skip、0 failed（3.7m），与 Studio 合计 70 passed；scripts/check.ps1 退出码 0，包括 Go test/vet、154 项 unit、typecheck、build/budget 和 diff 检查。HTTPS Compose 因未设置 TLS 文件按脚本规则跳过。完整 22 项记录及已知边界见 docs/gvideo3-phase4-acceptance.md。

以下为此前阶段的历史记录，保留当时“未 commit/push/等待验收”等状态；Phase 3 的发布基线现已由 PR #40 确认。

Phase 3 + Phase 3.1 已由用户通过最终工程与视觉验收，WATCH Playback & Creator Channel 作为稳定检查点封版，不再继续视觉调整。用户已授权创建独立 commit、推送当前 Phase 3 分支、创建 PR，并在 Frontend / Backend / Backend Race 均通过后 squash 合并到 main，再同步本地 main。发布状态以 Git 和 GitHub 为准；没有 Phase 4 内容，发布完成后停止。以下“等待验收／未 commit/push”保留各轮验收当时状态。

Phase 3 工程及整体视觉方向已由用户通过。commit 前完成指定的 Phase 3.1 三项增量：light playback 逐区域颜色／token 断言（确认原 warm light 已正确生效，默认行为代码未改）；Related compact 缺图保留 26px GVideo mark 和轻框、隐藏占位文案；频道背景改为 identity 右侧透明几何，取消横幅表面及独立高度。仅修改 scoped CSS、对应 E2E 与设计／验收文档，VideoPlayer、VideoCover 状态逻辑、API／后端零修改。

Phase 3.1 按序 typecheck、18 文件 131 项单测（17.39s）、build 均通过；CSS 101.81 kB / gzip 18.91 kB，HLS budget 通过。随后相关 E2E 26 passed、0 failed、0 skipped（3.0m），含新增 6 项 light/dark／broken cover／channel 视觉检查和原播放、频道、主题回归。五张截图在忽略的 tmp，缺图状态使用隔离夹具；其余为真实视频／作者数据。完整记录见验收报告顶部 Phase 3.1。没有再运行综合脚本，没有 commit／push，不进入 Phase 4。

Phase 2 / 2.1 已通过 PR #39 squash 合并到 main，基础为 `81076db`。当前在 `codex/gvideo3-watch-playback-channel` 开发 Phase 3，仅处理 `/video/:id`、`/users/:id` 与对应表现层。VideoPlayer.tsx、hlsLoader.ts、API client、types、App、Shell、后端和 Phase 2 Discovery 均未修改，没有新增依赖。新增 scoped `watch-playback.css`；播放器始终作为主列首个子组件，Theater 和 resize 只改变 CSS 布局。

Phase 3 工程与截图验收完成，等待用户视觉验收；19 项报告见 `docs/gvideo3-phase3-acceptance.md`。关机后恢复原开发服务与 Docker 持久卷，没有清理数据。首次单测遇到 worker 启动超时；后续发现共享 worker 的播放器模块缓存使 HLS mock 未生效，已在 VideoPlayer 测试边界依照项目既有方式清理模块，不改变原测试断言。按序 typecheck、全量 unit（18 文件 / 131 项，13.80s）、build 均通过；播放器 26 项（原 21 项 + 新 5 项）全部保留并通过。CSS 101.44 kB / gzip 18.84 kB，HLS budget raw 360.41 kB / gzip 113.16 kB 通过。最终相关 E2E 41 passed、1 既有条件 skip、0 failed（4.8m），其中新增播放／频道 16 项全部通过，两个 Chromium 项目都实际进入／退出 fullscreen 和 PiP。最终 scripts/check.ps1 退出码 0，含 Go test/vet、18 文件 131 项单测（35.15s）、typecheck、build 和 diff 检查；HTTPS Compose 因未提供 TLS 文件按规则跳过。

真实播放页／频道及 Dialog 完成八个指定视口检查；极长中英文标题、用户名、bio、零统计、空作品与 related 404 使用隔离夹具补验。13 张截图（10 张关键页面 + 3 张长文本边界）和临时预览位于忽略目录，不进入生产构建或提交。没有写入真实关注／收藏／评论／举报数据；已还原匿名／浅色／默认视口并保留真实播放页预览。无新增依赖、暂存区为空、HEAD 仍为 81076db；不 commit、不 push、不进入 Phase 4。

Phase 1 已通过 PR #33 squash 合并到 GitHub main，基础提交为 `7ac6b94`。Phase 2 和 Phase 2.1 均已由用户通过工程与视觉最终验收，作为 WATCH Discovery 稳定检查点封版，不再继续本阶段视觉调整。发布分支为 `codex/gvideo3-watch-discovery`，提交及 PR 合并状态以 Git 和 GitHub 记录为准。未使用 qiaomu-design 技能。`GVideo_3_UI_Kit/` 是用户提供的参考资料，未修改；PNG 中的示意内容未写入产品数据。

用户已授权为 Phase 2 / 2.1 创建独立提交、推送开发分支，通过 PR 检查后合并 main 并同步本地。提交仅保留本阶段源码、测试、设计规范与验收文档；`tmp/`、验收截图、临时长标题预览均被忽略，不进入提交。未包含 Phase 3 内容，完成 Git 发布后停止。以下各阶段验收记录中的“未 commit/push”描述保留验收当时状态。

## Phase 2.1 Visual Polish

仅调整品牌化抽象封面占位、发现页浅色画布与媒体边框、Hero 极长标题边界、关注与收藏的编辑式空状态。空 URL/404 回退状态逻辑、API、搜索、分页和收藏行为保持 Phase 2 实现。默认浅色及已保存主题不变；未修改 VideoPage、VideoPlayer 或 Studio。桌面标题最多 3 行，单条 Hero 和移动端保持 2 行；窄屏收紧描述与间距，保证元信息和 CTA 留在 Hero 内。

按要求依次执行 typecheck、unit、build，退出码均为 0；18 个单元测试文件、106 项通过。相关 discovery、主题、菜单及新增 polish E2E：25 项通过、1 项按桌面设备条件跳过、0 失败（2.2m）。中英文极长标题覆盖 1440、920、768、430、390、320px；原 discovery 边界用例继续覆盖八个指定视口。七张关键截图和完整范围记录见 `docs/gvideo3-phase21-acceptance.md`。截图与临时标题预览位于忽略目录，不进入构建或提交。到此停止，不进入 Phase 3，不 commit/push。

## Phase 2 当前状态

以下保留 Phase 2 原验收记录（用户已通过）。范围为首页、最新、热门、关注和收藏；没有进入 Phase 3，没有修改后端、播放器、Studio、Auth、App 路由及主题初始化代码，没有新增依赖。完整文件列表与 15 项验收信息见 `docs/gvideo3-phase2-acceptance.md`；后续视觉精修增量见上方 Phase 2.1。

- 首页读取真实 latest 第一页（36 条），按 ID 去重后，前 4 条 ready 视频组成一个 Hero 和最多 3 个 secondary story；Latest 排除这些 ID。Popular preview 使用独立热门请求（12 条），排除所有已展示视频，保留 API 原始排名，最多显示 3 条；数据不足时省略区域。请求取消和失败恢复独立于主列表。
- `q` 或 `category` 存在时进入普通结果列表，不展示 Hero，也不请求热门预览。全局搜索仍使用 `/?q=`；分类、查询和分页保留原契约。首页第二页是常规列表。
- VideoCard 使用 standard、editorial、compact、ranked 四个显式变体，共用 Video 类型。其他阶段原有调用不指定 variant 时保留原卡片密度；共享封面失败处理同时改善这些调用。
- VideoCover 在空 URL、加载中和 404 时展示中性深色表面与已有品牌标记；失败后移除远程 img，不循环切换 src。普通卡片 lazy，Hero eager。
- 热门第一页 01–03 采用不同媒体尺度，4+ 紧凑排列；排名使用 `(page - 1) * page_size + index + 1`，第二页从 37 开始。没有伪造周月榜、涨跌、推荐原因或数据。
- 关注突出真实 creator identity 和发布时间，保留 Protected。收藏保留原取消 API、请求防重复和条目失败重试，成功后保留原位置并转移焦点到状态；主动“更新列表”重新请求实际内容，末页为空时回到仍有效的页码。
- 发现页常规网格桌面三列、平板两列，<=520px 单列；Hero 移动端最多 410px，分类横向滚动，控件可键盘操作。Shell 断点保持 Phase 1。最终 E2E 发现单条长标题 Hero 在 920px 桌面视口因 aspect-ratio 与最小高度产生宽度溢出，已给该 Hero 明确 `width: 100%`，所有边界复验通过。
- 最终实际浏览器再次检查 8 个指定视口的真实首页，无横向溢出；9 张截图涵盖首页 desktop dark/light、390px 首页、搜索、热门 desktop/mobile、最新、真实账户的关注与收藏空状态，保存在忽略的 `tmp/`。没有为了截图创建内容或修改个人关注/收藏数据。

最终按顺序运行 `npm run typecheck`、`npm test`、`npm run build`，均退出码 0。unit 为 18 文件、106 项全部通过（22.09s）；build 为 1917 modules、7.11s，CSS 84.42 kB / gzip 15.92 kB；HLS 预算通过（raw 360.41 kB、gzip 113.16 kB）。随后相关 E2E 为 19 项通过、1 项按设备条件跳过、0 失败（1.6m）；跳过仅为 desktop 项目的移动菜单测试，mobile 对应用例通过。

此前开发服务中断时，默认 Vitest 两次 worker 启动超时，forks 诊断运行还有一项既有守卫等待超时。用户重启 Docker 后，前后端容器均 healthy，8080/readyz 和 5173 返回 200，默认测试运行通过；没有放宽旧断言或修改测试配置，不能据此认定 Docker 是超时的唯一原因。错误浏览器标签页恢复后完成截图，最终还原匿名/浅色、默认视口并保留首页预览。未清理媒体卷、数据库、容器或用户数据，未修改系统权限和安全设置。Phase 2 到此停止，等待用户确认，不进入 Phase 3，不 commit/push。

## Phase 1 基础设施（已验收）

Phase 1 已建立以下基础：

- 语义主题令牌、无合法保存偏好时默认浅色，保留已有 dark/light 偏好；视觉系统保持 dark-first，主题初始化仍使用外部脚本。
- `styles.css` 改为入口，拆分 tokens、base、watch、studio、auth、shells、responsive、motion 八个样式文件。旧页面暂时使用集中变量别名。
- WatchShell、StudioShell、AuthShell 分离。Watch 品牌轨、Studio 工作导航、共享搜索和账户入口、移动抽屉已接入原路由。
- Button、IconButton、Badge、Skeleton、EmptyState，以及兼容原调用的反馈组件。
- 移动抽屉的背景隔离、滚动锁定、Tab 焦点循环、Escape 关闭及关闭后焦点返回。
- 原用户与管理员守卫、next 回跳、CSRF 和登录失效处理保留在 App；新增回归测试覆盖这些边界及旧通知请求取消。

没有修改后端、数据库、API 契约、播放器播放逻辑、上传逻辑或 feature 页面业务代码。页面构图仍待后续阶段迁移，不能将 Phase 1 当作整个 GVideo 3.0 的完成状态。

## 验证记录

主题默认行为修正后的最新验证：`npm run typecheck` 通过；`npm test` 为 16 个文件、67 项测试全部通过；`npm run build` 及 HLS 预算检查通过；主题 E2E 桌面与移动各 1 项，共 2 项通过、无失败或跳过。覆盖无保存值、非法保存值、已保存 dark/light，以及切换、导航和刷新恢复。本次仅执行以上指定检查，没有重跑综合 Go 检查或其他 E2E。

以下保留 Phase 1 初次验收记录：

- `npm test`：16 个测试文件、64 项测试全部通过。
- `npm run typecheck`：通过。
- `npm run build`：通过；HLS 预算检查通过，raw 360.41 kB、gzip 113.16 kB。
- 选择性浏览器回归：首页溢出与守卫回跳、主题切换与刷新保存、移动菜单，5 项通过、1 项按设备条件跳过。
- 真实浏览器检查 Watch 外壳的 1536×960、1440×900、1180×820、920×900、768×1024、430×932、390×844、320×720；后端恢复后使用真实首页内容再次检查，以上外壳未出现页面横向溢出。1440/1536 品牌轨为 176px，1180 为 72px，其余视口隐藏；不代表已完成全部页面或视频控件的视觉验收。
- Auth 外壳在 1440×900 和 390×844 检查通过，无 Watch 品牌轨及横向溢出；Auth 内容设计在 Phase 5 继续调整。
- 仓库综合脚本 `scripts/check.ps1`：通过，包括 PowerShell 语法、HTTP Compose 配置、Go 格式/测试/vet、前端测试/类型检查/构建及 diff 空白检查。HTTPS Compose 因未提供 TLS 文件按脚本规则跳过。
- 最终焦点修复后单独重跑移动菜单浏览器用例：1 项通过。
- 全量 `npm run test:e2e` 留到 Phase 6，在隔离数据环境执行；本次选择性用例未注册用户、上传视频或修改业务数据。

新增 App 集成测试会加载真实 lazy 页面。项目测试配置复用 worker，因此新增测试在进入和退出套件时清理模块缓存，避免 API mock 被其他页面测试继承；未修改测试隔离配置或放宽原有断言。

## 开发服务

前端开发预览：`http://127.0.0.1:5173/`。后端：`http://127.0.0.1:8080/`，恢复后 `/readyz` 返回 200。恢复使用现有 Docker backend 镜像及持久卷，没有重新构建后端或清理数据。

原 Docker frontend 容器也已恢复。它使用既有镜像，3.0 本轮开发效果以 5173 为准。发现当前持久数据中至少两条既有视频封面 URL 从后端返回 404，前端代理亦返回 404；本次没有生成替代业务素材或修改媒体卷。Phase 2 卡片与 Hero 迁移需要完善缺图呈现，媒体缺失原因另行定位。

`tmp/` 中的检查日志和实际页面截图是忽略的临时文件，不属于源码。`tmp/gvideo3-phase1-css.mjs` 是按旧样式文件行号编写的一次性拆分辅助文件，不应再次执行。

## 后续安排

Phase 2 / 2.1：已通过最终验收并封版，Git 发布范围见本文开头。

Phase 3：播放页和创作者空间。保持 VideoPlayer 的 HLS、fallback、质量、速度、字幕、音量、全屏、剧场、续播与键盘行为，仅修改呈现。

Phase 4：创作者概览、投稿内容行和弹窗，统计只使用 CreatorStats，行操作迁移到上下文菜单。

Phase 5：发布流程、紧凑通知 feed、Auth 内容与审核页面，保留全部现有提交和状态处理。

Phase 6：全站八个视口、浅色、动效、对比度、触控、弹窗与播放器回归，并执行隔离环境全量 E2E。
