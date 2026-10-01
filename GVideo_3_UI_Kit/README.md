# GVideo 3.0 UI Kit

这是一套为现有 `zdl000000/GVideo` 项目准备的 **UI 重构视觉参考 + 实施规范 + Codex 开发提示词**。

## 最终视觉方向

不是把三套概念平均混合，而是：

- **WATCH（观众侧） = Kinetic Editorial + Spatial Cinema**
  - 首页、发现/热门、播放页、创作者主页
  - 强影像、编辑感、非对称构图、沉浸式深色体验
- **STUDIO（创作者侧） = Radical Creator OS**
  - 创作中心、内容管理、发布、字幕、数据、审核
  - 数据密度高、工作流清楚、操作效率优先

品牌底层统一：`GVideo Rose + Obsidian Black + Aqua Cyan + editorial typography`。

## 目录

- `00_direction/`：总方向板
- `01_design_system/`：设计系统视觉板
- `02_watch/`：Watch 关键页面概念图
- `03_studio/`：Studio 关键页面概念图
- `04_mobile/`：移动端概念图
- `05_assets/`：可直接使用的 SVG / CSS token 资产
- `docs/`：实施规范、页面映射、组件规格、验收清单、Codex Prompt

## 重要说明

这些 PNG 是**视觉构图参考**，不是生产素材，不要把整张概念图直接嵌入产品。
图片生成中的文字可能存在轻微错字或虚构内容，开发时应以：
1. 当前 GVideo 真实业务数据；
2. `CONTEXT.md` 中的领域词汇；
3. 本 UI Kit 的文字规范；
为准。

视频封面、作者头像、媒体内容优先使用现有项目真实数据；不要用概念图中的人物或场景替换用户内容。

## 建议实施顺序

1. Tokens + Shell
2. Watch Home / Discover
3. Video Card + Watch Page
4. Creator Channel
5. Studio Shell + Dashboard
6. Content Management
7. Publish Flow
8. Auth / Notifications / Admin
9. Mobile + Light Theme parity
10. E2E / responsive / accessibility 回归
