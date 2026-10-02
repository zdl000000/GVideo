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
