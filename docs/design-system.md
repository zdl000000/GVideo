# GVideo 3.0 前端设计系统

## 当前实施状态

2026-10-02：Phase 1 建立主题令牌、CSS 分层、WATCH / STUDIO / Auth 外壳及首批共享 UI。具体页面构图按 Phase 2–5 逐项迁移；Phase 6 完成全部视口、浅色、动效与无障碍验收。未迁移页面暂时通过集中变量别名复用旧样式，不能据此声称整个 3.0 已完成。

本规范以 `GVideo_3_UI_Kit/` 与本次用户提示词为准，替代旧版本横向频道条和桌面无品牌轨的规则。视觉系统保持 dark-first，默认产品行为仍为浅色。界面使用真实 API；参考 PNG 只提供视觉构图，不能成为数据来源。

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
| `--gv-text-2` | `#B4BBC6` | `#5D6674` |
| `--gv-text-3` | `#8E98A7` | `#626D7D` |
| `--gv-brand` | `#FF2F62` | `#FF2F62` |
| `--gv-brand-text` | `#FF7191` | `#B90E39` |
| `--gv-cyan` | `#00D4D3` | `#00D4D3` |
| `--gv-cyan-text` | `#00D4D3` | `#087777` |

弱文本及彩色前景由参考色派生，以可读性优先。Rose 用于主 CTA、当前导航、播放进度和焦点；不得成为页面大面积底色。主 Rose 按钮使用 `--gv-on-brand` 深色文字以满足小字号对比度。危险操作使用独立 danger 语义，不混同品牌高亮。

旧变量 `--canvas`、`--ink`、`--coral` 等只作为 `tokens.css` 中的过渡别名。新样式使用 `--gv-*`。保留播放器局部深色表面，无论页面采用何种主题。

## CSS 与组件边界

`src/styles.css` 仅导入八个文件：

- `tokens.css`：主题、排版、间距、圆角、动效令牌和过渡别名。
- `base.css`：基础元素、共享 primitives、反馈、分页、处理状态和弹窗公共样式。
- `shells.css`：三种外壳、品牌轨、工具栏、账户导航和移动抽屉。
- `watch.css`：观看、发现、创作者空间及通知内容。
- `studio.css`：工作台、投稿管理、上传、字幕管理和审核内容。
- `auth.css`：登录注册内容布局。
- `responsive.css`：页面响应式规则；Shell 自身断点位于 shells。
- `motion.css`：交互动效、hover 与 reduced-motion。

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

## 后续页面约束

- 首页 Featured 与 Latest 去重；数据不足时减少 section，搜索/分类/分页保持真实结果。
- 热门使用当前 popular API；不伪造周月榜、涨跌或趋势。
- 播放页目标顺序为 PLAYER、TITLE、META、INTERACTION、CREATOR、DESCRIPTION、COMMENTS；Related 桌面在右侧，窄屏在后。
- 播放器业务不重写，保留 HLS、fallback、quality、speed、subtitle、volume、fullscreen、theater、PiP、resume、keyboard 与生命周期清理。
- 字幕创作与管理仅在投稿管理中，播放页只选择已有轨道。
- 作者头部使用已有封面或中性表面，不添加 banner 字段。
- 统计只读取 CreatorStats。可见性与处理状态存在交叉，不能混成一个互斥分布。
- 投稿行操作逐步迁移到上下文菜单，保留弹窗、焦点返回、轮询、失败重试、分页与删除确认。
- 发布可采用视觉阶段，仍然一次 FormData 提交；保留全部字段、限制、进度、取消和 beforeunload。
- 不显示 AI、草稿、合集、会员、存储套餐、认证、直播等缺少真实能力的功能。

## 验收

各 Phase 执行 `npm run typecheck`、`npm test`、`npm run build`，保留 HLS bundle budget；最终执行 `npm run test:e2e`。有写入的验收使用隔离测试数据，禁止污染 Docker 持久业务卷。

检查 1536×960、1440×900、1180×820、920×900、768×1024、430×932、390×844、320×720。禁止页面横向滚动、文字按钮重叠、控件溢出、弹窗越界和桌面表格强制压缩。全站对比度、播放器移动控件及全部页面最终在 Phase 6 统一复核；Phase 1 不代表这项验收已经完成。
