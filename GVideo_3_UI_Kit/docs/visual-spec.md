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
