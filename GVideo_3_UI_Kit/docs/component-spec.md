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
