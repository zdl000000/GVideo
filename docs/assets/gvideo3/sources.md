# GVideo 3.0 README 展示素材

采集日期：2026-10-03。UI RC 基线：[`e2c3a63`](https://github.com/zdl000000/GVideo/commit/e2c3a63fb84faea647d396b5e0ff622d535e09ba)，Phase 6 已通过 [PR #43](https://github.com/zdl000000/GVideo/pull/43) 合并。当前界面与完整验收边界分别见 [README](../../../README.md)、[设计系统](../../design-system.md) 和 [Phase 6 验收报告](../../gvideo3-phase6-acceptance.md)。

## 实际采集方式

- 运行已验收的 RC 前端 Nginx 镜像 `gvideo-phase6-rc-frontend:20261003`，image ID 为 `sha256:e205d52d6e961049ed0c8135b40958c9e45d3dd326aea8f6df623f918b81eb93`。服务中的入口、CSS、JS 与既有 RC 构建资源核对一致；没有为截图修改产品代码、CSS、DOM 或测试断言。
- 使用独立 Compose 项目 `gvideo-docs-showcase-20261003`，前端/后端端口分别为 `19088` / `19080`；数据库卷为 `gvideo-docs-showcase-20261003-data`，媒体卷为 `gvideo-docs-showcase-20261003-media`。注册和上传前核对项目标签、卷名、端口及空的视频列表，没有挂载或写入原开发数据库与媒体卷。
- 在独立库通过实际 API 注册演示账号 `gvideo_showcase`，上传六段既有开放影片样片，等待实际 FFprobe/FFmpeg 处理完成。用户名、投稿、时间、通知与播放统计均为该演示环境的实际状态，不是运营数据；没有模拟 API 返回或伪造互动统计。投稿标题中的“演示片段”和简介均标明用途及署名。
- 使用 Playwright 的 Chromium 采集当前视口 JPEG，质量 86，像素比例 1。等待字体、实际封面与加载过渡完成；检查八张图片均无横向溢出或失败图片，并逐张视觉审阅。没有拼接、修图、AI 生成或设计参考图替换。
- Light 首页来自没有保存主题偏好的新浏览器上下文。Dark 首页通过实际主题按钮切换并刷新后采集。移动图为 Chromium 的 390px 触控视口模拟，**不是实体手机、Safari 或 native HLS 实测**。
- 播放图显示实际 Sintel 媒体的暂停帧；发布图为选择真实媒体和封面后的未提交表单，截取首屏，字幕、可见范围与最终提交区域位于下方。其他图片同样只截取视口，不是完整长页面。
- 这里只做展示采集，没有在原开发环境运行完整 acceptance/E2E。RC 测试数字属于 Phase 6 原始验收，不是本次截图采集重新运行的结果。
- 采集完成后仅停止本次独立 showcase 项目的两个容器，演示卷保留；没有删除或清理原开发项目、Phase 6 项目及其数据。

## 正式图片

八张 JPEG 合计约 0.84 MB（842,094 字节）。文件名和路径区分大小写，README 全部使用相对路径；点击下表可查看原图。

| 图片 | 页面与主题 | 视口 | 文件大小 |
| --- | --- | --- | --- |
| [home-light.jpg](home-light.jpg) | 首页 `/`，Light | 1440 × 960 | 167,548 B |
| [home-dark.jpg](home-dark.jpg) | 首页 `/`，Dark | 1440 × 960 | 161,366 B |
| [watch-light.jpg](watch-light.jpg) | `/video/2`，Light，深色播放器 | 1440 × 960 | 109,799 B |
| [studio-light.jpg](studio-light.jpg) | `/creator`，Light | 1440 × 960 | 87,564 B |
| [content-light.jpg](content-light.jpg) | `/me/videos`，Light | 1440 × 960 | 125,793 B |
| [publish-light.jpg](publish-light.jpg) | `/upload`，Light，未提交表单首屏 | 1440 × 1080 | 78,763 B |
| [auth-light.jpg](auth-light.jpg) | `/auth`，Light，匿名登录表单 | 1440 × 960 | 57,044 B |
| [mobile-home-light.jpg](mobile-home-light.jpg) | 首页 `/`，Light，移动视口 | 390 × 844 | 54,217 B |

## 媒体来源与署名

复用仓库既有的本地开放影片演示样片；本次没有下载新媒体。历史展示采集记录见 [交付归档](../../handoff-archive.md)。仅使用可核对许可的 Big Buck Bunny 与 Sintel，不使用来源未确认的 Jellyfish 样片。截图不是完整影片的再发布，原始媒体不加入本次 Git 变更。

| 作品 | 权利人及署名 | 来源与许可 |
| --- | --- | --- |
| Big Buck Bunny | © copyright 2008, Blender Foundation / www.bigbuckbunny.org | [项目下载页](https://peach.blender.org/download/)、[Creative Commons 作品条目](https://wiki.creativecommons.org/wiki/Big_Buck_Bunny)；[CC BY 3.0](https://creativecommons.org/licenses/by/3.0/) |
| Sintel | © copyright Blender Foundation / durian.blender.org | [项目许可与署名说明](https://durian.blender.org/sharing/)；[CC BY 3.0](https://creativecommons.org/licenses/by/3.0/) |

既有样片已经截取、重新编码；具体在原片中的起止时间没有可靠记录，因此不补造时间码。GVideo 从样片的实际画面自动提取封面、生成 HLS；发布表单中的自定义封面也复用实际提取的 Sintel 帧。截图保留产品原有媒体裁切、阴影与 UI 覆盖，输出为 JPEG。Blender 项目标志及商标不因影片许可而成为 GVideo 品牌资产；本说明不暗示权利人认可本项目。

以下 SHA-256 记录本次实际上传的本地样片，便于与历史材料核对；不是公共媒体下载链接：

| 本地样片名 | SHA-256 |
| --- | --- |
| `bunny-1.mp4` | `7ae19316c13046940250e6deaa04a1a5b62df1fe107914eb108bd1cad4805cd1` |
| `bunny-2.mp4` | `af28909e6165ea2b48e1499b1fa31d7c20e2b75119eb699f6b8e1bea1faaf420` |
| `bunny-3.mp4` | `780f8768df25c8f762b5a63a6c53f28068d81d16917891fc5618861887cf4d57` |
| `sintel-1.mp4` | `dd04ce20e0f694bf3e8c5230f32bdfd12d5eae08a939eaca09189b09a272c826` |
| `sintel-2.mp4` | `ae87865d6b70327e35f64636f9bd4e00a906e819c5478ada12649c10577fd5dd` |
| `sintel-3.mp4` | `5e5ae0b4d30f51b38a4c957e75f3237d2df19e1d3c36e8289119018f0e64e6df` |

## 历史图片

README 已替换原来的五张展示图片。旧 `docs/assets/` 图片继续保留，供历史版本和交付记录追溯，不作为当前 UI 展示入口；没有删除或批量清理历史素材。Phase 验收报告中的本机证据路径保持为历史本地记录，正式公共展示入口使用本目录的仓库相对路径。
