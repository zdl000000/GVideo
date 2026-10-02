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
