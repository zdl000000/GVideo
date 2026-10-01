# Codex 执行 Prompt：GVideo 3.0 高还原 UI 重构

你正在重构仓库 `zdl000000/GVideo` 的前端 UI。

## 0. 任务性质

这是一次 **Presentation Layer / UX 重构**，不是业务重写。

必须保留：
- Go 后端
- 当前 API 契约
- 数据库
- Auth / CSRF
- React Router 路由路径
- HLS 播放逻辑
- 上传/处理/字幕/通知/审核等现有业务行为
- 当前测试覆盖表达的核心行为

不要先重写后端，不要创建新 API，不要用 mock 数据代替真实数据。

## 1. 开始前必须阅读

按顺序阅读：
1. `README.md`
2. `CONTEXT.md`
3. `docs/design-system.md`（作为旧版历史规范）
4. `frontend/package.json`
5. `frontend/src/app/App.tsx`
6. `frontend/src/styles.css`
7. `frontend/src/shared/components/*`
8. 各 `frontend/src/features/*` 页面
9. 本 UI Kit：
   - `GVideo_3_UI_Kit/README.md`
   - `GVideo_3_UI_Kit/docs/visual-spec.md`
   - `GVideo_3_UI_Kit/docs/page-map.md`
   - `GVideo_3_UI_Kit/docs/component-spec.md`
   - `GVideo_3_UI_Kit/docs/acceptance-checklist.md`

视觉参考：
- `00_direction/GVideo3_direction_board.png`
- `01_design_system/design_system_board.png`
- `02_watch/watch_home.png`
- `02_watch/watch_discover.png`
- `02_watch/watch_video.png`
- `02_watch/creator_channel.png`
- `03_studio/studio_dashboard.png`
- `03_studio/studio_content.png`
- `03_studio/studio_publish.png`
- `04_mobile/mobile_watch_video_studio.png`

注意：概念图中的文字和人物/封面只用于视觉参考，不是产品数据，不要硬编码进去。

## 2. 最终设计方向

### WATCH
融合：
- Kinetic Editorial
- Spatial Cinema

目标：
- cinematic
- editorial
- image-first
- asymmetrical but disciplined
- strong typography
- minimal surfaces

### STUDIO
采用：
- Radical Creator OS

目标：
- creator workflow
- data clarity
- content management efficiency
- dark professional workspace
- high-density but calm

不要把整个项目做成同一套“卡片 Dashboard”。

## 3. 技术约束

当前技术栈必须优先保留：
- React 19
- TypeScript
- Vite
- React Router
- Lucide React
- HLS.js
- 原生 CSS / CSS architecture

不要引入 Tailwind/shadcn 仅为了视觉重构。
不要引入大型 UI framework。
如果确实需要一个很小的可访问性依赖，先证明当前实现无法合理完成，否则不加。

## 4. CSS 重构

当前 `src/styles.css` 约 900+ 行，不继续无限堆叠。

将样式拆分，建议：

```text
src/styles/
  tokens.css
  base.css
  motion.css
  shells.css
  watch.css
  studio.css
  auth.css
  responsive.css
```

可以进一步按组件拆，但不要制造几十个 10 行 CSS 文件。

把 UI Kit 的 `05_assets/tokens.css` 作为起点，根据现有 light/dark 能力整合。

旧版 `--coral` 等颜色名逐步迁移到语义 token：
- `--gv-brand`
- `--gv-text`
- `--gv-text-2`
- `--gv-surface`
- `--gv-success`
等。

不要一次性做危险的机械全局替换；按组件迁移。

## 5. App architecture

把当前 `App.tsx` 中过多的 UI Shell 职责拆开。

目标结构示意：

```text
src/app/
  App.tsx
  routing.tsx
  shells/
    WatchShell.tsx
    StudioShell.tsx
    AuthShell.tsx

src/shared/ui/
  Button.tsx
  IconButton.tsx
  Badge.tsx
  Tabs.tsx
  DropdownMenu.tsx
  DialogFrame.tsx
  Field.tsx
  Progress.tsx
  ...

src/shared/components/
  VideoCard.tsx
  VideoHero.tsx
  CreatorIdentity.tsx
  ContentRow.tsx
  ...
```

不要过度抽象；只有至少两处有真实复用价值时才做抽象。

## 6. 路由分区

保留 URL。

WatchShell：
- `/`
- `/latest`
- `/popular`
- `/following`
- `/favorites`
- `/users/:id`
- `/video/:id`
- `/notifications` 可使用 Watch shell 或共享 compact shell

StudioShell：
- `/creator`
- `/me/videos`
- `/upload`
- `/admin/reports`

AuthShell：
- `/auth`

Protected / AdminProtected 行为保持一致。

## 7. Watch Home

参考 `watch_home.png`，但使用真实数据。

必须解决旧版问题：
首页 Featured 使用前几条视频后，下方“最新”不能立即重复同一批内容。

建议：
```ts
featured = first 4-5
latest = items excluding featured
```

如果当前一个 API 页不足以填充两个 section：
- 不额外伪造；
- 可以减少 section 数；
- 或合理发已有 API 的第二个请求；
- 保持 loading/error 可靠。

页面重点：
- hero = 强视觉核心
- 左侧品牌 rail
- search 放在内容顶部
- 最新发布 editorial grid
- 热门趋势分区
- 不要所有视频都用同一种 card

## 8. Discover / Popular

参考 `watch_discover.png`。

保留现有 popular 排序/API。
Top 1–3 应有明显视觉权重，但不要用浮夸金银铜渐变。
Top 4+ 可进入紧凑排行。

分类筛选保持可操作、可滚动、移动端友好。

## 9. Video Page

参考 `watch_video.png`。

必须保留 `VideoPlayer.tsx` 的播放能力，优先只改 presentation。

结构：
1. Player
2. Title / meta / tags
3. Interaction row
4. Creator identity
5. Description
6. Comments
7. Next-up / Related

桌面 >= 1180：
- 主播放器 + 右侧 next-up
移动：
- player full-bleed
- single column
- interaction buttons 触控友好

不把字幕管理放入播放页；这里只选择已有字幕。

## 10. Creator Channel

参考 `creator_channel.png`。

保留：
- Follow
- Edit profile
- Stats
- Creator videos
- Pagination

视觉：
- 创作者身份区可以更大胆，但不要依赖不存在的 banner 数据。
- 如果没有 banner，使用视频封面/中性色 editorial background，不新增后端字段。

## 11. Studio Dashboard

参考 `studio_dashboard.png`，但**只用真实后端当前提供的数据**。

现有 CreatorStats：
- videos
- followers
- views
- likes
- favorites
- comments
- visibility counts
- processing count
- recent videos

不要伪造：
- 月度趋势
- 新增粉丝趋势
- 商业套餐
- 存储套餐

如果图表没有真实时间序列数据，改为：
- 大数字
- proportional bars
- composition / status breakdown
- recent content
而不是假折线图。

## 12. Content Management

参考 `studio_content.png`。

把当前每行“编辑 / 字幕 / 删除”三个平铺按钮改成：
- 主行
- 状态
- 可见性
- 数据
- `...` context menu

Context menu：
- 编辑
- 字幕
- 失败时重试
- 删除

保持所有现有 Dialog 和 focus return 行为。

Processing row 要显示真实 progress。

## 13. Publish

参考 `studio_publish.png`，但当前后端提交是一次上传，因此：

不要为了视觉稿伪造真正“六步后端草稿流程”。

可以把 UI 做成视觉分区：
- Media
- Story
- Cover
- Subtitle
- Visibility
- Publish

但最终仍提交当前 FormData。

必须保留：
- MP4/WebM/Ogg
- 512MB 当前限制（以代码/后端为准）
- cover
- subtitle
- category
- visibility
- title/description
- upload progress
- abort
- beforeunload warning

## 14. Auth

保留左品牌、右表单思路，但重新用 GVideo 3.0 token 和 typography。

不要让登录页有巨大无意义空白。
保留 login/register segmented control 和安全 redirect。

## 15. Notifications

旧版每行横向过空。

新版：
- 更窄内容列或 split layout
- unread 只用微妙 surface/indicator
- 支持按视觉类别区分 icon，但不增加后端筛选 API
- 全部标记已读保留

## 16. Mobile

参考 `mobile_watch_video_studio.png`。

必须针对：
- 390x844
- 430x932

Watch：
- 不保留 desktop rail
- topbar compact
- hero / feed 单列优先
- player full width

Studio：
- compact topbar
- mobile bottom nav 或 drawer
- stat 2-column
- content management one-column rows

不要让桌面表格简单横向缩放到手机。

## 17. 品牌资源

可使用：
- `05_assets/gvideo-mark.svg`
- `05_assets/grain-tile.svg`

Logo wordmark 使用 HTML text + brand mark，不依赖图片文字。

grain 必须非常轻微（1–3% 视觉存在感），不要做脏旧滤镜。

## 18. 数据真实性

这是硬要求：

概念图中出现而当前 API 不存在的：
- 趋势百分比
- 30 天图表
- AI 功能
- storage plan
- 草稿
- 系列
- 直播
- 认证等级

都不得假装已实现。

UI 要从真实功能长出来，而不是为了还原概念图制造假的按钮。

## 19. 测试与回归

每完成一个大阶段执行：
```bash
npm run typecheck
npm test
npm run build
```

最后：
```bash
npm run test:e2e
```

不要为了通过测试删除断言。
如果 UI DOM 改变导致测试选择器失效，优先改成可访问性角色/label/data-testid 等稳定选择器。

## 20. 视觉验收 viewport

至少手工/Playwright 检查：
- 1536x960
- 1440x900
- 1180x820
- 920x900
- 768x1024
- 430x932
- 390x844
- 320x720

## 21. 实施阶段

### Phase 1
Tokens + WatchShell + StudioShell + shared UI primitives。

### Phase 2
Home + VideoCard variants + Discover/Latest/Popular。

### Phase 3
Video Page + Creator Channel。

### Phase 4
Studio Dashboard + Content Management + Dialog refresh。

### Phase 5
Publish + Auth + Notifications + Admin。

### Phase 6
Mobile / light theme / animation / accessibility / E2E。

每个 Phase 完成后保持应用可构建、可测试，不做一个长达几千行的一次性改动。

## 22. 完成定义

最终产品必须让人感觉这是：
“一个具有强烈影像品牌感的现代视频社区 + 一个专业创作者操作系统”

而不是：
- YouTube/B站克隆
- shadcn dashboard
- 霓虹赛博模板
- 把所有内容放进 Card 的后台模板

功能必须与改造前等价或更好，且不改变后端契约。

开始实施前，先输出：
1. 你理解的页面分区；
2. 计划修改/新增的文件列表；
3. 6 个 Phase 的执行顺序；
4. 你识别到的风险点。

然后从 Phase 1 开始，不要一次性改完整仓库。
