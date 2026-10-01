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
