# 页面映射：现有 GVideo → GVideo 3.0

| 现有路由 | 新体验 | 视觉参考 | 业务行为 |
|---|---|---|---|
| `/` | Watch Home | `02_watch/watch_home.png` | 保留搜索、分类、分页/API；重新编排首页，避免 Featured 与下方视频重复 |
| `/latest` | Latest Feed | Home/Discover 系统 | 保留排序和分页，使用更克制的 feed |
| `/popular` | Discover / Trending | `02_watch/watch_discover.png` | 强化榜单和趋势视觉，保留现有 popular API |
| `/following` | Following Feed | Watch 系统 | 登录保护不变 |
| `/favorites` | Saved | Watch 系统 | 可从头像菜单/左栏进入 |
| `/users/:id` | Creator Channel | `02_watch/creator_channel.png` | 保留关注、编辑资料、分页 |
| `/video/:id` | Immersive Watch | `02_watch/watch_video.png` | HLS、字幕、断点续播、评论、举报全部保留 |
| `/auth` | Auth | Watch 品牌体系 | 保留 login/register/next 行为 |
| `/creator` | Studio Dashboard | `03_studio/studio_dashboard.png` | 只使用现有 stats；不要伪造后端不存在的趋势数据 |
| `/me/videos` | Studio Content | `03_studio/studio_content.png` | 编辑、字幕、重试、删除全部保留 |
| `/upload` | Studio Publish | `03_studio/studio_publish.png` | 当前 API 仍是一次提交，不要假装实现草稿/多步后端 |
| `/notifications` | Notification Center | Studio/Watch 共用组件 | 未读、全部已读、跳转逻辑不变 |
| `/admin/reports` | Governance | Studio Shell | admin guard 与审核 API 不变 |

## 导航信息架构

### Watch
- 首页
- 发现
- 最新
- 关注
- 收藏
- 创作中心（进入 Studio）

### Global utility
- Search
- Theme
- Notifications
- Publish
- Avatar Menu

Avatar Menu：
- 个人空间
- 收藏
- 创作中心
- 内容管理
- 退出

### Studio
- 概览
- 内容
- 发布
- 字幕（可进入内容管理的上下文，而不是新造后端路由）
- 评论（如后端尚无独立评论管理页，不要制造空页面）
- 数据分析（只展示已有 stats；未来数据用 disabled/coming soon，不假数据）
- 设置（若无功能，不建立虚假页面）

## 重要限制

视觉稿出现但后端当前没有的数据：
- 30 天趋势
- 新增粉丝趋势
- 存储空间套餐
- 草稿系统
- AI 优化
- 系列管理
- 商业升级计划

这些只是“未来视觉方向”，**当前实现不得伪造**。
可以：
- 不显示；
- 用已有数据重排；
- 留清晰扩展接口。
