# GVideo 3.0 验收清单

## 视觉
- [ ] Dark 主题与参考图具有同一视觉语言，而非普通 shadcn/后台模板
- [ ] Rose 使用克制，只用于关键状态/CTA
- [ ] Watch 图片/播放器是第一视觉层级
- [ ] Studio 数据层级清楚，不是等权卡片堆叠
- [ ] Desktop 1440/1536 宽度无大面积无意义空白
- [ ] 中文长标题不会溢出
- [ ] 动态数据不会导致布局跳动

## 功能回归
- [ ] 注册 / 登录 / next redirect
- [ ] 搜索
- [ ] 首页 / 最新 / 热门 / 分类 / 分页
- [ ] Follow / Favorite
- [ ] Video like / favorite / share / report
- [ ] 评论新增/删除
- [ ] HLS / 原视频 / 清晰度 / 字幕 / resume
- [ ] Upload + progress + cancel
- [ ] Edit video
- [ ] Subtitle management
- [ ] Retry processing
- [ ] Delete video
- [ ] Notifications read / read all
- [ ] Admin reports
- [ ] Light / dark theme
- [ ] Auth expiry

## Responsive
Viewports:
- [ ] 1536 x 960
- [ ] 1440 x 900
- [ ] 1180 x 820
- [ ] 920 x 900
- [ ] 768 x 1024
- [ ] 430 x 932
- [ ] 390 x 844
- [ ] 320 x 720

任何尺寸：
- [ ] 无页面级横向滚动
- [ ] 控件不重叠
- [ ] player controls 可操作
- [ ] dialog 不溢出视口
- [ ] mobile touch target >= 44px

## Accessibility
- [ ] Tab 顺序合理
- [ ] Focus visible
- [ ] Drawer/Dialog Escape
- [ ] aria labels
- [ ] reduced motion
- [ ] 颜色不是状态唯一表达

## 工程
- [ ] `npm run typecheck`
- [ ] `npm test`
- [ ] `npm run build`
- [ ] `npm run test:e2e`
- [ ] 不破坏 bundle budget
- [ ] 不修改后端 API 契约


## Phase 4 — 工程验收范围

- CreatorStats 累计字段、零值/无最近作品、可见范围与独立处理数量、加载错误、封面失败。
- 内容列表 page_size=12、2500ms 轮询、AbortController、同一 DOM 行与弹窗/菜单保留、failed retry -> pending -> 真实轮询。
- 菜单箭头/Home/End/Escape/外部点击/禁用删除、持久 trigger；编辑/字幕/删除取消和保存后的焦点返回；busy 不能关闭、Tab 不离开 Dialog。
- FormData 与本地 cover URL lifecycle；字幕上传/默认/删除/错误与默认 badge；删除末页唯一条目返回上一页。
- 1536x960、1440x900、1180x820、920x900、768x1024、430x932、390x844、320x720，深浅主题；大数字、长标题/简介、12 行、404、菜单与三个 Dialog 边界。
- 隔离 API E2E 禁止写入真实开发数据；App/theme、Discovery、Playback 原用例回归；未进入 Phase 5。

实际结果与已知边界以 docs/gvideo3-phase4-acceptance.md 为准。截图及一次性预览放在忽略的 tmp/ 下，不属于发布源码。视觉验收等待用户确认，不由自动化通过替代。

## Phase 5 — 发布、账号、通知与举报验收范围

以下列出实现的验收范围，不以勾选或预填数字代替实际执行结果。最终命令、退出状态、通过/跳过数量、截图与已知问题见 [Phase 5 验收报告](../../docs/gvideo3-phase5-acceptance.md)；工程与视觉通过分别记录，用户视觉确认不能由 E2E 通过替代。

### 功能与边界

- Publish：点击式原生文件选择/更换；真实文件名/大小/类型；空封面 `VideoCover` 与本地预览替换/revoke；单条初始中文字幕；原分类与可见范围语义；原生标题/简介限制；一次 FormData 的必填、可选字段及中文 language/label。
- Upload lifecycle：真实回调 0/45/100 与 total<=0 状态；上传/后台处理区别；busy 禁用；AbortController 取消与卸载；取消/失败后文件及字段保留；仅 busy 时 beforeunload；成功跳转真实视频路由。API client 原有不可计算长度事件边界保持不变。
- Auth：login/register 模式、autocomplete、字段保留、原 login/register API 与 onAuth、busy 防重复、失败反馈、Tab 顺序；提交和已登录回跳均允许站内 `/video/1`，拒绝协议相对 `//evil.com` 与绝对外部 `http://evil.com`。
- Notifications：六类文案与图标、未读/已读、单条和全部标记 busy/失败；通知数与全局铃通过原事件同步；视频/作者目标；page_size=20 与 page URL；loading/read-error/empty，无虚构筛选、设置、实时功能或空状态动作。
- Governance：未登录/非管理员保护；四状态和四原因标签/未知原因回退；真实举报人、作者、视频、created_at/updated_at；全部/状态筛选与 page URL、page_size=20、加载/读取/更新错误；三审核动作、busy、防重复；更新行保持原 map 行为，无搜索、批量、封禁或删除。

### 主题、布局与截图

- 四页面均覆盖 1536x960、1440x900、1180x820、920x900、768x1024、430x932、390x844、320x720，dark/light 两主题；检查页面横向溢出、控件边界、长中文/连续 ASCII/下划线、长文件名、通知预览与举报详情。
- Light Creator OS 的 Publish/Admin、warm Light WATCH 的 Notifications、Light Auth 使用各自实际 canvas/surface；Dark 检查可见 canvas。无合法保存主题时仍 light，已保存 dark/light 切换刷新恢复。
- 移动操作热区 >=44px；390px Auth 表单/CTA 首屏；Publish 六章节单列与可见范围、Notifications 时间/已读操作、Admin 状态/内容/操作次序与筛选栏内部滚动；保留可见焦点、字段标签、状态/错误语义与 reduced-motion。
- 18 张隔离截图：Publish dark/light/selected files/uploading/mobile390；Auth desktop login/register/mobile390；Notifications dark/light/unread/empty/mobile390；Admin dark/light/pending/filter-empty/mobile390。截图和预览只放忽略的 tmp/，不进入生产源码。

### 测试与保护范围

- 页面单元测试与 `publish-experience.spec.ts`、`auth-experience.spec.ts`、`notifications-experience.spec.ts`、`governance-experience.spec.ts` 对应上述业务与展示边界；截图收集需显式 `PHASE5_SCREENSHOTS=1`，由 desktop 项目同时采集桌面与 390px 截图，条件跳过须在报告说明。
- 新 E2E 的全部 API 请求与写入使用隔离 route fixture；上传只发送隔离文件并通过受控 ProgressEvent 驱动原 XHR 回调。不得创建真实账号、上传真实媒体、标记真实通知或更新真实举报。隔离抽象 cover 明示 fixture，不伪装真实内容。
- 执行 typecheck、unit test、build 与 HLS bundle budget，再验证 App/theme、Discovery、Playback、Studio 的既有回归。环境可用时按项目要求运行综合检查，结果由最终报告记录。
- diff 自查：App/Shell、WATCH、VideoPage/VideoPlayer/hlsLoader、CreatorPage/CreatorDashboard/MyVideosPage/ContentRow/ContentActionMenu、已验收 workspace/watch CSS、API client、types、backend、依赖、全局 tokens、主题初始化没有混入变更。
- 不扩展草稿、autosave、wizard persistence、定时发布、AI、OAuth/reset、通知筛选/设置/realtime、Admin 搜索/批量/封禁/删除；不进入 Phase 6，不 commit、不 push。设计规范与验收报告保留，tmp/、截图、preview、夹具、日志、构建产物与敏感本地文件不进入提交。
