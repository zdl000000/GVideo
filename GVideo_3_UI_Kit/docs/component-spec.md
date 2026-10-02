# GVideo 3.0 组件规格

## Shell

### `WatchShell`
职责：
- Watch desktop brand rail
- global search
- theme / notification / publish / account
- mobile nav drawer
- content viewport

### `StudioShell`
职责：
- Studio rail
- Studio utility bar
- publish CTA
- active route
- mobile bottom / drawer navigation

### `AuthShell`
保持独立、简洁，不加载完整 Studio 导航。

## UI primitives

建议在 `src/shared/ui/` 新建：
- `Button`
- `IconButton`
- `Badge`
- `Tabs`
- `DropdownMenu`
- `DialogFrame`
- `Field`
- `SelectField`
- `TextareaField`
- `Tooltip`
- `Progress`
- `Skeleton`
- `EmptyState`
- `Toast`（如果当前没有全局 toast，可先保留现有 inline status，不必强加）

不要为了 UI 重构引入大型组件库。
继续使用 Lucide React。

## Product components

### `VideoCard`
Variants:
- `editorial`
- `compact`
- `ranked`
- `related`
- `management`

共用真实 `Video` type。
不要为每个页面复制一份 Card JSX。

### `VideoHero`
- 主封面
- category/eyebrow
- title
- description/excerpt
- primary action
- meta
- responsive overlay

### `CreatorIdentity`
- Avatar
- username
- bio
- stats
- follow/edit action

### `StatBlock`
不强制 card；
支持 `large`, `compact`, `inline`。

### `ProcessingStatus`
继续复用现有状态机：
`pending -> processing -> ready / failed`

### `ContentRow`
用于 `/me/videos`：
- thumbnail
- title/meta
- status
- visibility
- stats
- context menu

### `PublishDropzone`
只包装现有 file input 行为。
必须保留 accept/size/error/upload progress/cancel。

## 视觉规则

- Buttons 高度：40 / 44 / 48
- Icon hit-area >= 40 desktop, >= 44 mobile
- 内容圆角 10–14px；Hero 16–20px
- Badge pill
- Dialog 14–20px
- 表格行高 68–80px
- 16:9 media 为默认
- Avatar 不使用随机渐变；使用统一 fallback 体系

## Player

`VideoPlayer.tsx` 的播放逻辑不要重写。
只拆外观和 controls：
- playback timeline
- volume
- captions
- quality
- speed
- theater
- fullscreen

保持 HLS loader、resume、keyboard、安全逻辑。


## Phase 4 — 已实现的 Studio 内容工作流

- Dashboard 使用现有 CreatorStats 的累计数字：作品总数为主，播放/关注为第二层，获赞/收藏/评论为次层。可见范围三分类与 processing_count 分开；不构造曲线、增长率、套餐、草稿或存储数据。
- ContentRow 是真实 Video 的语义 article，复用 VideoCover、ProcessingBadge、ProcessingProgress 和 visibilityLabels。发布日期来自 created_at，不虚构更新时间。文件大小和简介为低优先级。
- ContentActionMenu 仅属于投稿管理，不建立通用 DataGrid/Popover 体系。44px trigger；menu/menuitem、上下箭头、Home/End、Escape/Tab、外部点击、禁用删除和 danger 项；测量真实菜单/按钮边界，自动翻转并约束到视口，滚动/缩放时更新位置。
- 编辑、字幕、删除由 MyVideosPage 持有。菜单选择传递持久的行 trigger；弹窗 cleanup 恢复背景 inert 后才恢复焦点。保存/取消回原 trigger，同页删除成功回 heading。
- 字幕管理只在 workspace 调用中启用新样式。onBusyChange 将既有上传/默认/删除请求状态告知 SubtitleDialog；业务 API/FormData/track list 不变。SubtitleTrack 不提供原始文件名/格式字段，所以不展示虚构 file/format。
- StudioEmptyState 只提供真实发布入口，没有 checklist/完成率。Loading 使用内容行同构骨架，沿用全局 reduced-motion。
- <=700px 用封面+标题、状态/可见范围、播放/互动、日期、完整行进度的阅读顺序，隐藏桌面列标题；标题两行并允许连续 ASCII 安全换行。

## Phase 5 — 已实现的发布、账号、通知与举报组件

### Publish

- `/upload` 保留一个表单与一次提交，以 MEDIA、STORY、COVER、SUBTITLE、VISIBILITY、PUBLISH 六个编号视觉章节组织阅读。章节不具备向导步骤、草稿或独立保存行为。
- 文件入口是可用键盘操作的原生 button，点击现有 file input 选择或更换文件；未实现拖放，不以 dropzone 文案暗示拖放能力。选择后显示真实文件名、大小与类型。
- 空封面复用 `VideoCover` 品牌占位；已选封面展示本地 object URL 预览，保留原 create/revoke 生命周期，不生成假视频内容。
- 字幕入口只接受单条初始中文字幕；同一次 FormData 继续发送 `subtitle_language=zh-CN`、`subtitle_label=中文`。更多轨道管理仍通过已有投稿管理字幕弹窗完成。
- 一次 FormData 保留 `title`、`description`、`category`、`visibility`、`video`，可选 `cover` 与 `subtitle`。标题/简介原生限制、分类加载、可见范围 map/help、上传回调、AbortController、取消后内容保留、busy 时 beforeunload 与成功跳转 `/video/:id` 保留。
- 主提交与取消操作复用 `Button`。busy 时文件选择、表单和可见范围不可改；进度使用真实回调与 `progressbar` 语义，明确区分文件上传和随后后台处理。总量未知的回调不提供 `aria-valuenow`；原 API client 对不可计算长度事件的处理保持不变，不承诺每次上传都有百分比。

### Auth

- `/auth` 使用品牌叙事与紧凑表单双列布局；移动端压缩装饰，优先显示登录/注册表单。已有 GVideo mark 与轻 frame geometry 属于品牌装饰，不作为用户内容。
- 登录与注册仍是同一个页面内的两个模式，复用 `Button` 与现有 login/register API、onAuth。原生 label、username/current-password/new-password autocomplete、busy 锁定、inline alert/status 保留明确反馈；无 OAuth、忘记密码或虚构认证入口。
- 本地 `authTarget` 统一登录/注册成功和已登录回跳：接受 `/video/1` 等站内路径，拒绝 `//evil.com`、`http://evil.com`。App 的保护路由和 Shell 不改。

### Activity

- `/notifications` 的本地 `NotificationRow` 使用语义 article、细分割线与紧凑信息层级；仍使用唯一 `notificationCopy` 映射，并为关注、点赞、收藏、评论、处理完成、处理失败提供文字类别和图标。
- 未读状态由背景、圆点和“未读”文字共同表达；单条已读按钮为 44px，busy 状态提供文本，全部已读沿用原 API。保留 page_size=20、page 查询、视频/作者链接和 `gvideo-notifications-changed` 事件，使全局通知铃继续更新。
- 空状态用 ACTIVITY / 00、标题、留白和弱几何构成；无新 CTA、筛选、设置或实时功能。复用现有 `Pagination`、`LoadingBlock`、错误与日期格式工具。

### Governance

- `/admin/reports` 的本地 `ReportRow` 使用状态/编号、举报内容、审核操作三列，移动端依次阅读状态、内容、操作，不建立通用 DataGrid 体系。
- 四个现有状态 pending/reviewed/resolved/dismissed 保留原语义，分别使用文字、图标和色彩。垃圾信息、不当内容、版权问题、其他及未知原因回退保留；标题、详情、举报人、作者、提交/更新时间使用真实字段。
- 审核中、处理完成、驳回仍调用原 review API；请求期间保留全局 busy 防重复操作。状态变更后沿用原本当前行更新行为，不擅自从当前筛选结果移除该行。
- 状态/page URL、page_size=20、请求取消、管理员权限保护、加载/错误/空状态与 `Pagination` 保留。没有搜索、批量操作、封禁或删除能力。

仅增加页面内组合与 scoped 样式；不改 App/Shell、WATCH、Playback/Player/HLS、已验收 Studio 页面、API client、types、backend、依赖或全局主题初始化。实际验证与边界见 [Phase 5 验收报告](../../docs/gvideo3-phase5-acceptance.md)。
