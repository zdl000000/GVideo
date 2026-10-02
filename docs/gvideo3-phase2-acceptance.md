# GVideo 3.0 Phase 2 验收记录

日期：2026-10-02。基线：GitHub main 的 `7ac6b94`，工作分支：`codex/gvideo3-watch-discovery`。尚未 commit/push，没有进入 Phase 3。

当前状态：Phase 2 开发与本地验收已完成。typecheck、106 项单元测试、build/HLS 预算和最终相关 E2E 通过；实际浏览器八视口检查及指定截图已完成，停止并等待用户确认。

## 1. 新增文件（9 个）

- `frontend/src/features/videos/useDiscoveryFeed.ts`
- `frontend/src/features/videos/DiscoveryFeeds.test.tsx`
- `frontend/src/shared/components/DiscoveryControls.tsx`
- `frontend/src/shared/components/VideoCard.test.tsx`
- `frontend/src/shared/components/VideoCover.tsx`
- `frontend/src/shared/components/VideoHero.tsx`
- `frontend/src/shared/test/videoFixtures.ts`
- `frontend/e2e/discovery.spec.ts`
- `docs/gvideo3-phase2-acceptance.md`

## 2. 修改文件（13 个）

- `frontend/src/features/videos/HomePage.tsx`
- `frontend/src/features/videos/LatestPage.tsx`
- `frontend/src/features/videos/PopularPage.tsx`
- `frontend/src/features/videos/FollowingPage.tsx`
- `frontend/src/features/videos/FavoritesPage.tsx`
- `frontend/src/features/videos/HomePage.test.tsx`
- `frontend/src/features/videos/FavoritesPage.test.tsx`
- `frontend/src/shared/components/VideoCard.tsx`
- `frontend/src/styles/watch.css`
- `frontend/src/styles/responsive.css`
- `frontend/src/styles/motion.css`
- `docs/design-system.md`
- `docs/gvideo3-progress.md`

App、Shell、styles.css 入口、theme-init.js、index.html、package.json 和 lockfile、后端、VideoPage/VideoPlayer、CreatorPage/CreatorDashboard、MyVideosPage、UploadPage、Notifications、Admin、Auth 均没有修改。其他页面原有 VideoCard 调用保持旧信息密度，同时获得共享封面失败回退。

## 3. 首页数据组织

主列表读取现有 latest API，第一页 36 条，先按 Video ID 去重。从 ready 视频中取前 4 条：第 1 条 Hero，第 2–4 条 compact secondary story。Latest 使用当前主列表排除 Featured ID 的结果，首条用 editorial，其余 standard。数据不足不复制作品，不以测试夹具填充产品。

独立热门请求为 sort=popular、page=1、page_size=12。保留原始 API 排名，排除主列表已展示 ID 和热门列表内部重复 ID，最多显示 3 条。没有剩余内容则省略区域。请求使用 AbortController、独立 loading/error 和重试，不影响主列表恢复；首页主列表失败时不展示预览。

## 4. VideoCard 变体

| 变体 | 使用位置与层级 |
| --- | --- |
| standard | 最新、搜索/分类、收藏常规列表：封面、时长、标题、作者、播放量与日期 |
| editorial | 首页 Latest 首条、关注时间流：更强标题与可用的真实描述；关注优先展示作者身份 |
| compact | Hero 次要作品：小媒体与简化信息横向排列 |
| ranked | 热门与首页热门预览：显示传入的真实排名；榜首区域调整媒体尺度 |

共用同一个 Video 类型、封面、链接和格式化逻辑；未增加 saved/folder/playlist 变体。取消收藏由 FavoritesPage 管理，通过 action slot 提供按钮。

## 5. 封面回退

VideoCover 处理空 URL、loading、404。回退为中性深色背景、细微品牌渐变、已有 GVideo SVG 和 Play 图标。失败后移除远程 img，不用 onError 循环替换资源；换 URL 时重置状态。普通卡片 lazy/async，Hero eager/high priority。尺寸保持稳定。

## 6. 热门排名

保持真实 popular API 顺序，使用 `(result.page - 1) * result.page_size + index + 1`，第二页 36 条规格下从 37 开始。01–03 采用不等媒体尺度与 Rose 数字，4+ 紧凑排列；不增加周榜、月榜、涨跌或实时观看量。

## 7. 搜索与分类

全局搜索仍进入 `/?q=`。q/category 存在时显示结果标题、真实计数、分类与普通列表，不展示 Hero 或 unrelated 热门预览；第二页也是常规列表。分类来自 API；切换分类保留 q 并清除 page，分页保留 q/category；latest/popular 切换保留条件并返回第一页。

## 8. 响应式与状态

发现网格桌面 3 列、平板 2 列、<=520px 单列。Hero <=991px 改为纵向，次要内容在窄屏逐条排列；移动 Hero 最多 410px，单条长标题限制行数并保留 CTA。分类横向滚动，原生按钮支持键盘操作；移动排序、取消收藏、分页等控件为 44px。Phase 1 Shell 断点不变。

关注保留 Protected，强调真实作者与发布时间；空状态提供发现入口。收藏取消防重复提交，失败保留条目并提供重试；成功保留原位置，隐藏链接 inert，焦点进入成功状态。点击“更新列表”才重新读取实际数据并排布，末页失效时回到有效页码。

## 9–11. Typecheck、unit、build

按顺序在 frontend 执行：

```sh
npm run typecheck
npm test
npm run build
```

- typecheck：退出码 0。
- unit：18 个文件，106 项全部通过，无失败；22.09s。含原 App 守卫、主题、播放器等既有测试，没有删除旧业务断言、修改测试配置或放宽等待时间。此前 worker 启动/等待超时在此次恢复环境后未复现，不能据此确定 Docker 是唯一原因。
- build：退出码 0；1917 modules，7.11s；CSS 84.42 kB / gzip 15.92 kB。
- HLS 预算：raw 360.41 kB，gzip 113.16 kB，低于 550000 / 170000 字节限制。预算脚本和 Vite 展示的 gzip 计算结果可能不同，验收以现有预算脚本输出为准。

## 12. E2E 和实际截图

最终相关 E2E：19 项通过、1 项跳过、0 失败，退出码 0，1.6m。跳过仅为 desktop-chromium 下的移动导航用例；mobile-chromium 的移动导航通过。命令：

```powershell
$env:E2E_BASE_URL = 'http://127.0.0.1:5173'
npm run test:e2e -- e2e/discovery.spec.ts e2e/app.spec.ts --grep 'discovery|home renders|mobile navigation|theme selection' --workers=1
```

覆盖真实首页、搜索、分类、排序、连续排名与分页、匿名个人页守卫、主题和移动导航；个人页交互用独立 API 夹具测试作者身份、CSRF、取消收藏后的稳定位置及主动更新。边界夹具覆盖 0/1/40 条、长中文标题、超长用户名、24 个分类、404，以及八个指定视口。

夹具只存在测试文件，不进入产品请求。当前持久卷的关注/收藏写入流程未作实际 mutation 集成验收，不注册用户、不上传新视频、不更改用户关注收藏数据。全量 E2E 留至 Phase 6；本次为相关用例。

最后一轮边界检查发现单条长标题 Hero 在 920px 桌面视口横向溢出：aspect-ratio 与 430px 最小高度使自动宽度超过容器。已在单条 Hero 添加 `width: 100%`，保持高度与 CTA 约束，不改变其他页面。修正后按指定顺序重跑全部检查和相关 E2E，上述结果均为修正后的最终结果。

最终实际浏览器检查：

| 视口 | 文档宽度 | 真实首页 Hero 高度 |
| --- | --- | --- |
| 1536×960 | 1526 | 469.02 |
| 1440×900 | 1430 | 440.80 |
| 1180×820 | 1170 | 390.78 |
| 920×900 | 910 | 360 |
| 768×1024 | 758 | 360 |
| 430×932 | 420 | 300 |
| 390×844 | 380 | 300 |
| 320×720 | 320 | 300 |

均无页面横向溢出。9 张实际截图位于忽略的 `tmp/`：`gvideo3-phase2-home-dark.jpg`、`gvideo3-phase2-home-light.jpg`、`gvideo3-phase2-home-mobile.jpg`、`gvideo3-phase2-search.jpg`、`gvideo3-phase2-popular-desktop.jpg`、`gvideo3-phase2-popular-mobile.jpg`、`gvideo3-phase2-latest.jpg`、`gvideo3-phase2-following.jpg`、`gvideo3-phase2-favorites.jpg`。最新、搜索、热门等来自真实 API；已有测试账户的关注/收藏当前均为空，实际浏览器验证了登录后 0 条内容的正常页面。非空列表、取消收藏及稳定位置由单元/E2E 夹具覆盖。截图不进入源码提交；验收后退出测试账户、恢复匿名浅色与默认视口，保留首页预览。

## 13. 已知问题

- 当前持久数据存在旧封面 404，包括 `/media/covers/ca8acc832e381b50aa7d5bdf.jpg` 与 `/media/covers/60ac3d348e3fdf0300fd8d87.jpg`。Phase 1 验收已记录这些 URL 的 404，因此在 Phase 2 前存在；本阶段仅提供 UI 回退，没有修改媒体 URL、API、数据库和持久卷。
- 真实首页最新内容含既有 E2E 视频记录；产品没有用参考图素材或虚构内容替换它们。缺失媒体仍需另行恢复，封面回退不能修复文件本身。
- 本次不是全站对比度、全部个人写入集成流程或全量 E2E 验收。

## 14. 未照搬参考图的部分

没有参考图中的虚构人物/频道、专用摄影封面、fake 计数/推荐原因、编辑精选、周月榜、涨跌趋势、在线人数、creator post、收藏文件夹或 playlist。未增加复杂 carousel、parallax、WebGL 或自动播放 thumbnail。页面沿用系统字体，不新增字体或依赖。

## 15. Phase 3 前确认

Phase 2 全部验收完成后停止，等待确认视觉层级、三列/单列浏览密度、热门排名层级、收藏稳定占位交互，以及既有缺失媒体如何处理。Phase 3 的播放页、播放器呈现与作者空间需用户明确开始；播放器现有 HLS、质量、字幕、键盘、续播等行为必须保留。未来默认深色继续留到 Phase 6 决定，当前默认浅色和已保存偏好不变。
