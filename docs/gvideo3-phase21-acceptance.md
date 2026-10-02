# GVideo 3.0 Phase 2.1 Visual Polish 验收

日期：2026-10-02。Phase 2 已获用户验收，本记录只描述其后的视觉精修增量。工作分支 `codex/gvideo3-watch-discovery`；未 commit/push，未进入 Phase 3。

## 修改范围

修改 7 个已有工作区文件：

- `frontend/src/shared/components/VideoCover.tsx`：只替换回退内容 markup，保留空 URL、加载、失败、URL 重置与图片请求策略。
- `frontend/src/styles/watch.css`：统一抽象占位系统、发现页浅色局部表面、Hero 标题边界和编辑式空状态。
- `frontend/src/styles/responsive.css`：移动占位密度、空状态纵向布局、Hero 描述及间距收紧；标题移动端仍最多 2 行。
- `frontend/src/features/videos/FollowingPage.tsx`：仅空状态组件与 import；原文案、CTA 和业务逻辑保留。
- `frontend/src/features/videos/FavoritesPage.tsx`：仅空状态组件与 import；请求、分页、取消收藏及稳定位置逻辑保留。
- `docs/design-system.md`：补充精修规范。
- `docs/gvideo3-progress.md`：更新完成状态与验证记录。

新增 3 个源码/文档文件：

- `frontend/src/shared/components/WatchEmptyState.tsx`：语义 section、编号 eyebrow、标题、原说明/CTA、aria-hidden 的弱装饰。
- `frontend/e2e/visual-polish.spec.ts`：极长中文/ASCII 标题及个人空状态回归。
- `docs/gvideo3-phase21-acceptance.md`：本记录。

App、主题初始化、根主题令牌、package.json、API、数据逻辑、分页、搜索、VideoHero TSX、VideoPage、VideoPlayer、Studio 均未在 Phase 2.1 修改。浅色发现页样式仅作用于包含 `.gv-discovery-page` 的 WatchShell。共享 VideoCover 的抽象占位外观也适用于原有共享卡片调用，图片状态逻辑不变。

## 四项结果

1. 封面回退为 charcoal 基底、已有 GVideo mark、极轻双色 glow、细纹与几何线框的抽象占位；常规尺寸明确标注“封面占位”，紧凑/移动尺寸按空间减少标签。没有虚假摄影、视频帧或参考人物。既有真实 404 仍真实失败，未改 URL 或媒体文件。
2. WATCH 浅色使用暖灰画布、浅纸色 surface、细边框和媒体 framing；深色沿用同一品牌。无保存偏好默认 light，以及 dark/light 保存恢复行为均保持不变。
3. 桌面 Hero 最多 3 行，单条 Hero 与移动端最多 2 行。安全换行、禁止 flex 缩小文本区、收紧窄屏描述与间距，元信息/CTA 不挤出边界；继续验证 920px。
4. Following/Favorites 空状态保留业务说明、原 CTA 名称及 `/popular` 目标，仅增加编号、排版留白和弱线框，未新增功能或大卡片。

## 实际检查结果

在 `frontend/` 按顺序执行：

| 检查 | 结果 |
| --- | --- |
| `npm run typecheck` | 退出码 0 |
| `npm test` | 18 文件、106 项全部通过，退出码 0 |
| `npm run build` | 1918 modules；CSS 88.60 kB / gzip 16.76 kB；退出码 0 |
| 相关 E2E | 25 通过、1 跳过、0 失败，2.2m，退出码 0 |
| HLS budget | raw 360.41 kB / gzip 113.16 kB；低于 550000 / 170000 字节上限 |
| `git diff --check` | 通过 |

E2E 命令：

```powershell
$env:E2E_BASE_URL = 'http://127.0.0.1:5173'
npm run test:e2e -- e2e/discovery.spec.ts e2e/visual-polish.spec.ts e2e/app.spec.ts --grep 'discovery|home renders|mobile navigation|theme selection' --workers=1
```

跳过为 desktop-chromium 的移动菜单用例，mobile-chromium 对应用例通过。新增 3 个用例在两个项目各执行一次，共 6 项：中文极长标题、连续 ASCII/数字/下划线标题、关注/收藏空状态。长标题覆盖 1440、920、768、430、390、320px，检查最大行数、页面横向溢出、meta/CTA 顺序与 Hero 高度；原 discovery 用例继续覆盖八个指定视口及 0/1/40 条、404、搜索、排序、分页和个人页行为。主题选择与刷新恢复用例在桌面和移动均通过。本次为相关 E2E，全站验收仍留至 Phase 6。

## 七组截图

截图位于忽略的 `tmp/`，不暂存：

| 场景 | 文件 |
| --- | --- |
| desktop dark home，1440×900 | `tmp/gvideo3-phase21-home-dark.jpg` |
| desktop light home，1440×900 | `tmp/gvideo3-phase21-home-light.jpg` |
| mobile light home，390×844 | `tmp/gvideo3-phase21-home-mobile.jpg` |
| following empty，1440×900 | `tmp/gvideo3-phase21-following.jpg` |
| favorites empty，1440×900 | `tmp/gvideo3-phase21-favorites.jpg` |
| 超长中文 Hero，1440×900 | `tmp/gvideo3-phase21-title-chinese.jpg` |
| 超长 ASCII/underscore Hero，1440×900 | `tmp/gvideo3-phase21-title-ascii.jpg` |

首页和个人空状态截图来自实际页面与真实 API，个人页使用已有空内容测试账户，未写入关注/收藏。两张极长标题图来自忽略的 `frontend/tmp/phase21-title-preview.html`，明确标注验收夹具，直接复用 VideoHero 与产品 CSS，不写 API/数据库，不进入 dist。真实 Home 的标题边界另由 E2E 拦截数据完整验证。截图完成后退出测试账户，恢复匿名、浅色和默认视口。

既有封面 404 在 Phase 1 前已存在，本阶段仅升级其呈现；没有修复或替换缺失媒体。本轮到此停止，不进入 Phase 3，不 commit、不 push。
