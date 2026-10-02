# GVideo 3.0 视觉规范

## 1. 品牌定位

GVideo 3.0 不应被实现成普通“视频网站模板”或普通“后台管理模板”。

目标气质：
- Cinematic：影像优先，封面/播放器是第一视觉层级。
- Editorial：像数字杂志一样有大小、留白、错位与节奏。
- Precise：Creator Studio 像专业创作操作系统，不堆装饰。
- Human：中文信息层级清楚，避免纯炫技导致可用性下降。

品牌句可用于装饰性位置，但不是产品功能文案：
- WATCH · CREATE · SHARE
- A BRIGHTER WORLD THROUGH VIDEO
- 让内容连接更大的世界

## 2. 主题

视觉基线采用 dark-first。必须保留现有 light/dark 能力。
不要强制覆盖用户已有主题偏好。

深色：
- Canvas `#080A0D`
- Elevated `#0D1015`
- Surface `#11161D`
- Surface 2 `#171D26`
- Text `#F7F8FA`
- Secondary `#B4BBC6`
- Tertiary `#7B8492`
- Brand `#FF2F62`
- Cyan `#00D4D3`

浅色使用 `05_assets/tokens.css` 中的派生令牌。

## 3. 色彩原则

Rose 不是“到处粉红”，而是视觉事件：
- 主 CTA
- 当前导航
- 高价值数字/趋势高亮
- 播放进度
- 核心焦点状态

Cyan：
- 媒体处理成功/进度
- 数据辅助线
- Studio 次级数据

不要使用页面级大面积粉色背景。
不要为每种分类分配随机颜色。

## 4. Typography

不捆绑专有字体文件。默认使用系统栈：

```css
font-family:
  Inter, ui-sans-serif, -apple-system, BlinkMacSystemFont,
  "PingFang SC", "Microsoft YaHei", "Noto Sans SC", sans-serif;
```

Display 中文大标题：
- Desktop Hero: `clamp(52px, 6vw, 92px)`, 800/900
- Page H1: `clamp(36px, 4vw, 58px)`, 800
- Section H2: 26–32px, 800
- Video title: 15–18px, 650/700
- Body: 14–16px, line-height 1.6

数据与状态：
```css
font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
font-variant-numeric: tabular-nums;
```

原则：
- 巨型字体用来建立品牌节奏，不要每个页面都有巨型标题。
- Studio 用数字做视觉焦点，而不是让所有数据都进入独立彩色 Card。

## 5. Grid / Layout

### Watch Desktop >= 1280
- 固定左侧 Brand Rail：176px
- 内容区域：`minmax(0, 1fr)`
- 最大内容宽：1600px
- gutter: 24–32px
- Hero 允许非对称 8/4 或 7/5 布局

### 992–1279
- Brand Rail 压缩成 72px icon rail
- 隐藏装饰性品牌文案
- Hero 仍保持非对称

### < 992
- 左栏消失
- 顶部紧凑导航 + drawer
- 页面单列化

### Studio Desktop
- 左栏 184px
- 顶部 utility bar 64px
- 主内容采用 12-column grid
- 右侧辅助栏只在 >= 1280 出现

### Mobile <= 520
- 首页单列优先；不要强行维持双列小卡片
- 播放器全宽贴边
- 触控热区 >= 44px
- Studio 数据可 2 列，但操作列表单列

## 6. Surface

少用 Card。
优先层级：
1. Typography
2. Whitespace
3. Divider
4. Image
5. Surface/Card（只有需要独立操作边界时使用）

允许：
- 玻璃搜索栏
- Player / Dialog / Studio widget 使用轻 Surface
- 低对比边框

禁止：
- 每块内容都套卡片
- 大面积渐变背景
- 同屏十几种圆角/阴影
- “Dashboard = 5 张一样的 KPI 卡”的机械模板感

## 7. Motion

- Hover: 140–220ms
- Layout/Drawer: 220–320ms
- Hero / editorial transition: 320–520ms
- 只动画 `opacity`, `transform`, 明确必要属性
- `prefers-reduced-motion` 必须禁用大幅运动

Watch:
- 封面 hover scale 1.02–1.035
- Hero 文案轻微 translate + fade
- 视频预览必须可关闭，移动端不自动 hover preview

Studio:
- 数字/图表动画只首次进入触发
- Progress 精准，不弹跳
- 状态切换以 opacity + border/color 为主

## 8. Accessibility

- 所有 icon button 有 aria-label/title
- keyboard focus 清楚
- Dialog focus trap 保留
- 颜色对比至少 WCAG AA
- 不用颜色作为状态唯一线索
- 动态背景必须有足够 overlay 保证文字可读


## Phase 4 — Studio workspace 应用范围

仅 `.gv-studio-dashboard`、`.gv-content-page`、`.gv-studio-dialog` 和 `.gv-subtitle-workspace` 使用 studio-workspace.css。原 studio.css 继续服务 Upload/Admin；不整体搬运旧样式。

- Dark 沿用品牌语义令牌。Light 在含上述两个页面的 StudioShell 下使用中性工作区 canvas #f0f1ee、surface #fafbf8、field #e6e8e4 和精确边框。WATCH、Upload、Admin 的令牌不受这些覆盖影响。
- 数字使用 mono/tabular、真实千分位，分割线与留白建立层级，避免等尺寸 KPI 卡、伪图表和大面积装饰渐变。
- 内容行普通桌面最小高度 110px；实际处理进度或错误占完整附加行。封面保持 16:9，共享品牌占位状态，不伪造视频内容。
- Dialog 保留原焦点隔离、忙碌保护、确认警告和请求反馈；深浅主题都使用所在 workspace 的 surface/field，不固定成深色卡。
- 产品默认行为仍是无合法保存值时 light；已保存 dark/light 仍恢复。视觉 dark-first 不代表默认行为变更。

## Phase 5 — 四个页面的视觉应用范围

Phase 5 使用四份新增 CSS，不整体复制或重写旧样式：

| CSS | 作用域 | 视觉职责 |
| --- | --- | --- |
| `publish.css` | `.gvideo-publish-page`，及包含此页的 StudioShell | 六个编号视觉章节、选择文件、真实封面预览、字幕入口、可见范围、上传进度和提交操作 |
| `auth-experience.css` | `.gv-auth-experience`，及包含此页的 AuthShell | 桌面品牌叙事/表单双列、轻几何品牌图形、移动首屏表单、模式/错误/busy |
| `notifications.css` | `.gv-notification-center`，及此页的浅色 WatchShell | 紧凑活动行、未读层级、类别图标、标记反馈、editorial empty state |
| `governance.css` | `.gv-governance-page`，及包含此页的 StudioShell | 状态/编号与内容/操作列、文字+图标状态、筛选栏、紧凑记录、移动阅读顺序 |

- Dark 延续品牌语义令牌，canvas 为 `#080a0d`。通知的 dark WATCH Shell 允许透明并继承 body canvas；验收须检查实际可见 canvas，不能把透明容器误判为主题未生效。
- Publish 与 Governance 的 Light Creator OS 在所在 Shell 局部使用 canvas `#f0f1ee`、surface `#fafbf8`、field `#e6e8e4` 与中性边框。Notifications 的 Light WATCH 使用 canvas `#eeede9`、surface `#faf9f6`、secondary surface `#e4e2dc`。Auth 使用既有浅色 canvas `#f7f8fa` 与语义 surface，不改变全局 tokens。
- 视觉 dark-first 仍是品牌基线；产品行为保持无合法保存偏好时 light，保存 dark/light 仍恢复，切换和刷新逻辑不改。
- Publish 通过编号、字级、留白和细线建立六个章节，桌面使用窄标签列与宽表单列，<=700px 顺序单列。可见范围 <=850px 单列，提交操作在移动端全宽。选择文件界面明确表达点击选择，不暗示拖放、草稿、定时或多步保存。
- Auth 的桌面视觉重点为品牌句与现有 mark/frame geometry，表单通过细竖线与叙事分开，避免大 Card。<=700px 隐藏次要品牌条目和几何图形，缩短叙事，使 390px 登录/注册 CTA 位于首屏。
- Notifications 用类别、时间、标题/预览、已读操作构成行，已读降低字重；未读文字与圆点保留，处理失败有文字与图标，不只靠颜色。移动端时间独立一行，操作保持 44px。空状态不新增动作。
- Governance 的桌面操作窄列纵向排列，<=700px 记录单列并隐藏列标题；移动操作三列，<=360px 为两列加完整驳回行。筛选栏允许自身横向滚动，页面不得横向溢出。长标题、ASCII/下划线、详情、文件名使用安全换行。
- Rose 用于主 CTA、当前模式/关键提示；Cyan 用于真实上传进度和选择反馈。不使用大面积渐变、虚构摄影图或伪造视频帧。通知忙碌图标和发布进度遵守 reduced-motion；焦点延续共享体系与页面局部可见样式。

仅四个页面的视觉范围扩展；已验收的 App/Shell、WATCH、Playback、Creator Channel 与 Studio workspace 保持原范围。截图为隔离验收夹具，不作为生产图片资产；实际结果与视觉确认见 [Phase 5 验收报告](../../docs/gvideo3-phase5-acceptance.md)。
