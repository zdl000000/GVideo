# GVideo 3.0 前端设计系统

## 当前实施状态

2026-10-03：Phase 1–5 已完成并通过用户验收，Phase 5 发布基线为 main `af48544`。Phase 6 只修明确缺陷并验证系统；当前不改变页面布局、默认主题或业务范围，验收证据见 `docs/gvideo3-phase6-acceptance.md`。

本规范以 `GVideo_3_UI_Kit/` 与本次用户提示词为准，替代旧版本横向频道条和桌面无品牌轨的规则。视觉系统保持 dark-first，默认产品行为仍为浅色。界面使用真实 API；参考 PNG 只提供视觉构图，不能成为数据来源。

Phase 6 约束：正文及语义前景在实际 canvas/surface 上达到 4.5:1，focus 至少 3:1；浅色次级前景使用加深后的 semantic token，Rose 实心按钮使用 `--gv-on-brand`。移动分页和关键操作保持至少 44×44，窄屏允许分页换行。Hover 增强限定 hover + fine pointer；reduce 下包括 Related 在内的封面不缩放，忙碌和进度保留文字与数字。

弹窗全部控件禁用时焦点留在 Dialog 根节点；视觉隐藏的 file input 不进入 Tab 顺序，现有按钮仍可打开原生文件选择器。认证请求期间焦点留在表单、失败回原控件；通知读取按钮移除后焦点保留在同一条目链接。移动抽屉跨到桌面断点时返回可见主内容。legacy alias 继续保留，不做无证据的 CSS 删除。

## 视觉与信息架构

WATCH 使用 Kinetic Editorial + Spatial Cinema：封面和播放器为视觉主体，通过字号、留白、分隔线和非对称媒体构图建立层次。覆盖 `/`、`/latest`、`/popular`、`/following`、`/favorites`、`/users/:id`、`/video/:id`，通知中心 `/notifications` 使用同一外壳。

STUDIO 使用 Radical Creator OS：数据和工作流程优先，信息紧凑、操作精准。覆盖 `/creator`、`/me/videos`、`/upload`、`/admin/reports`；审核入口仅对管理员显示，原有守卫仍控制页面访问。

Auth 外壳独立，保留 `/auth`、登录注册与 next 回跳。三种外壳共用品牌、主题与基础交互，路由 URL、Session、CSRF、API 契约不变。

不机械复制视频平台、后台模板或五张同权 KPI 卡。Card 仅用于确有独立交互边界的区域，不把标题、统计、分类和导航都放入 Card。

## 主题与语义令牌

无合法保存偏好时默认浅色。已有 `gvideo-theme=light|dark` 始终优先。视觉系统保持 dark-first；是否更改产品默认主题留到 Phase 6 完整验收后决定。外部 `public/theme-init.js` 在应用加载前设置主题及浏览器主题色，保持生产 CSP 禁止内联脚本的约束。

| 角色 | 深色 | 浅色 |
| --- | --- | --- |
| `--gv-bg` | `#080A0D` | `#F7F8FA` |
| `--gv-bg-elevated` | `#0D1015` | `#FFFFFF` |
| `--gv-surface` | `#11161D` | `#FFFFFF` |
| `--gv-surface-2` | `#171D26` | `#EEF1F5` |
| `--gv-text` | `#F7F8FA` | `#111318` |
| `--gv-text-2` | `#B4BBC6` | `#596270` |
| `--gv-text-3` | `#8E98A7` | `#5D6472` |
| `--gv-brand` | `#FF2F62` | `#FF2F62` |
| `--gv-brand-text` | `#FF7191` | `#B90E39` |
| `--gv-cyan` | `#00D4D3` | `#00D4D3` |
| `--gv-cyan-text` | `#00D4D3` | `#066969` |

弱文本及彩色前景由参考色派生，以可读性优先。Rose 用于主 CTA、当前导航、播放进度和焦点；不得成为页面大面积底色。主 Rose 按钮使用 `--gv-on-brand` 深色文字以满足小字号对比度。危险操作使用独立 danger 语义，不混同品牌高亮。

旧变量 `--canvas`、`--ink`、`--coral` 等只作为 `tokens.css` 中的过渡别名。新样式使用 `--gv-*`。保留播放器局部深色表面，无论页面采用何种主题。

## CSS 与组件边界

`src/styles.css` 保持样式入口，Phase 1 的八个文件为：

- `tokens.css`：主题、排版、间距、圆角、动效令牌和过渡别名。
- `base.css`：基础元素、共享 primitives、反馈、分页、处理状态和弹窗公共样式。
- `shells.css`：三种外壳、品牌轨、工具栏、账户导航和移动抽屉。
- `watch.css`：观看、发现、创作者空间及通知内容。
- `studio.css`：工作台、投稿管理、上传、字幕管理和审核内容。
- `auth.css`：登录注册内容布局。
- `responsive.css`：页面响应式规则；Shell 自身断点位于 shells。
- `motion.css`：交互动效、hover 与 reduced-motion。

Phase 3 增加 `watch-playback.css`，在 responsive 之后、motion 之前导入。规则只作用于 `.gv-watch-page`、`.gv-channel-page` 和包含这两个页面的 WATCH 浅色外壳，保留已封版的发现页和 Studio 视觉。

依赖保持 `app -> features -> shared`。Shell 只组织导航和 Outlet；Auth 状态、用户/管理员守卫留在 App。请求和业务操作保留在现有 feature。共享 UI 不反向导入 app/features。

首批共享 UI 为 Button、IconButton、Badge、Skeleton、EmptyState。现有 Feedback 入口保持兼容。DialogFrame、DropdownMenu、Field、Tabs 等在实际重复使用时加入，不提前制造大型组件库。

## 布局与响应式

| 尺寸 | WATCH | STUDIO |
| --- | --- | --- |
| >= 1280 | 176px 品牌轨，内容上限 1600px | 184px 工作导航 |
| 992–1279 | 72px 图标品牌轨 | 保留 184px 工作导航 |
| < 992 | 紧凑顶栏与抽屉，无桌面品牌轨 | 紧凑顶栏与抽屉 |
| <= 520 | 首页单列优先、播放器贴边 | 内容行单列、统计可两列 |

WATCH 品牌轨仅承载观看导航与进入 Studio 的入口，不混入整套管理导航。工具栏提供搜索、主题、通知、投稿和账户入口。没有真实能力的工作区入口不显示。

移动抽屉拥有键盘焦点循环、Escape 关闭、焦点返回、背景 inert 和滚动恢复。新图标按钮至少 40px，移动端至少 44px。屏幕阅读器可直接跳到正文。

## 排版、状态和动效

系统字体栈：Inter、ui-sans-serif、系统中文字体，不下载或捆绑专有字体。数据使用等宽数字和 tabular-nums。WATCH 大标题用于重点内容构图；STUDIO 用数据层级建立焦点。

覆盖加载、错误、空状态、禁用、忙碌、处理阶段、失败重试和图片回退。状态必须有文字或图标，不能只用颜色。图标按钮必须提供 aria-label，focus-visible 清楚。

封面 hover scale 1.02–1.035，仅在精细指针设备上生效。Drawer/Dialog 不阻塞操作，不使用滚动劫持、大量视差、WebGL 或昂贵 blur。统一尊重 prefers-reduced-motion。

## Phase 2 WATCH 内容发现

首页、最新、热门、关注与收藏使用 `.gv-discovery-page`。共享 VideoCard 的显式变体为 `standard`、`editorial`、`compact`、`ranked`；ranked 必须传入真实排名。其他阶段原有调用省略 variant 时保留旧版信息密度。操作通过 action slot 接入，不把收藏业务移入卡片。

首页 Hero 使用第一页最前 4 条 ready 视频中的第一条，其余最多 3 条为 compact secondary story；Latest 排除 Featured ID，首条 editorial 与普通卡片形成不同密度。独立热门预览再排除已展示 ID，数据不足时减少区域。所有媒体和计数来自现有 API；`q`、`category` 或第二页不展示 discovery Hero。

VideoCover 统一空 URL、加载中和 404 回退。回退使用中性深色表面与已有品牌 SVG；失败后移除远程图片，URL 改变后重新初始化，不循环替换 src。普通卡片 lazy，Hero eager/high priority。封面中的图标是装饰，播放链接提供完整视频标题作为可访问名称。

Phase 2.1：回退统一为抽象 media placeholder，使用既有 mark、极轻 Rose/Cyan glow、细纹与几何线框；不使用摄影图或伪造视频帧。standard 保留占位标签，compact/ranked 减少装饰密度，Hero 图形偏右，移动端进一步简化标签。回退状态处理不变。浅色发现页在 WatchShell 内局部覆盖 canvas/surface/border，暖灰画布与薄媒体边框建立层次，不改变根主题默认或播放页/Studio 的令牌。Hero 桌面标题最多 3 行，单条与移动端最多 2 行，连续 ASCII/数字/下划线安全换行；窄屏描述限制 1 行以保护元信息和 CTA。关注/收藏空状态采用编号 eyebrow、标题与留白、极弱线框图形，不包裹大 Card，文案及 CTA 保持原语义。

热门 01–03 用不同媒体尺度和 Rose 数字建立层级，后续为紧凑 ranked list。分页连续排名使用 API 返回的 page/page_size。关注以 creator identity 和发布时间建立时间流；收藏成功取消后保留条目原有尺寸、隔离隐藏内容并转移焦点，主动更新列表才重新排布。

常规发现网格桌面 3 列、<=991px 2 列、<=520px 1 列。首页桌面为非对称 Hero + secondary story，移动端改为纵向内容；移动 Hero 高度最多 410px，单条长标题需保证 CTA 在 Hero 内。分类可横向滚动，使用原生按钮和 aria-pressed；排序使用真实路由链接并保留查询条件。动效为封面 scale 1.025、轻微抬升，尊重 reduced-motion。

## 后续页面约束

Phase 3 播放页采用 player-first 顺序：播放器、非 ready 的轻量处理状态、标题、元信息与操作、作者身份、简介、评论。>=1180px 为主列与 300px Related rail，较窄桌面/平板相关内容两列，<=700px 为单列。标题最多三行并允许连续 ASCII 换行；简介和评论保留文本换行，不引入 Markdown 或自动链接。

Theater 只切换页面网格和播放器宽度，保持同一 VideoPlayer 和媒体节点。移动播放器贴边，16:9 媒体下方独立放置控件，按钮至少 44px；窄至 320px 时允许控件换行。清晰度、倍速、字幕菜单沿用原焦点和选择逻辑；全屏时恢复媒体覆盖层控件。所有播放器业务保留在原实现，没有修改 HLS 初始化、source、事件生命周期或续播。

Creator Channel 使用抽象线框品牌背景与真实头像、姓名、简介、加入时间和三项统计。自己频道仅“管理投稿”为主操作，“编辑资料”“创作者中心”为次操作。作品复用 standard VideoCard，网格桌面三列、平板两列、<=700px 单列；无投稿复用 WATCH editorial empty state，不新增排序、Tabs 或 banner 能力。编辑资料 Dialog 只通过页面 scoped CSS 对齐表面和边框。

Phase 3.1 将频道几何线框改为身份区右侧的绝对定位背景：透明、无横幅边框、不占独立横幅高度；头像与 identity 和几何共享同一段落。移动端隐藏背景签名文字，保留轻线框，统计与操作保持原顺序。

Related 保留原同分类 popular 请求、排除当前 ID 与 slice 逻辑，轻量条目复用 VideoCover 和 compact 占位密度，仅显示两行标题及作者/播放数。举报保持 inline，评论保留现有创建、计数及本人/视频作者删除权限，不新增回复或点赞。

Related compact 缺图保留 26px GVideo mark 与轻几何框，不显示 placeholder 文案；规则仅作用于播放页 Related，不改变 VideoCover 加载／失败状态或其他卡片。浅色播放页画布与输入表面沿用 Phase 2.1 的 `#eeede9` / `#faf9f6`；标题、meta、creator、简介、评论与 Related 继承 light token，播放器保持独立暗色媒体表面。视觉 E2E 检查 token、最终背景颜色与 dark/light 刷新恢复。

- 首页 Featured 与 Latest 去重；数据不足时减少 section，搜索/分类/分页保持真实结果。
- 热门使用当前 popular API；不伪造周月榜、涨跌或趋势。
- 播放页目标顺序为 PLAYER、TITLE、META、INTERACTION、CREATOR、DESCRIPTION、COMMENTS；Related 桌面在右侧，窄屏在后。
- 播放器业务不重写，保留 HLS、fallback、quality、speed、subtitle、volume、fullscreen、theater、PiP、resume、keyboard 与生命周期清理。
- 字幕创作与管理仅在投稿管理中，播放页只选择已有轨道。
- 作者头部使用抽象品牌背景和真实头像，禁止把视频封面冒充作者 banner，不添加 banner 字段。
- 统计只读取 CreatorStats。可见性与处理状态存在交叉，不能混成一个互斥分布。
- 投稿行操作逐步迁移到上下文菜单，保留弹窗、焦点返回、轮询、失败重试、分页与删除确认。
- 发布可采用视觉阶段，仍然一次 FormData 提交；保留全部字段、限制、进度、取消和 beforeunload。
- 不显示 AI、草稿、合集、会员、存储套餐、认证、直播等缺少真实能力的功能。

## 验收

各 Phase 执行 `npm run typecheck`、`npm test`、`npm run build`，保留 HLS bundle budget；最终执行 `npm run test:e2e`。有写入的验收使用隔离测试数据，禁止污染 Docker 持久业务卷。

检查 1536×960、1440×900、1180×820、920×900、768×1024、430×932、390×844、320×720。禁止页面横向滚动、文字按钮重叠、控件溢出、弹窗越界和桌面表格强制压缩。全站对比度、播放器移动控件及全部页面最终在 Phase 6 统一复核；Phase 1 不代表这项验收已经完成。
