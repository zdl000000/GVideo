# GVideo

GVideo 是面向视频创作者与观众的社区型全栈项目：创作者投稿视频，观众浏览、播放、评论与互动。本文件只定义项目自己的词汇（供人与 agent 共用），通用编程概念不在此列。

## Language

**视频（Video）**:
观众可观看的内容实体，对应 `videos` 表与 `/api/v1/videos`。
_Avoid_: 内容、片子

**投稿（Submission）**:
创作者上传文件并发布的动作与流程产物；「我的投稿」列表即该流程的管理入口。
_Avoid_: 作品、上传视频

**可见性（Visibility）**:
投稿的可见范围：`public`（公开，进入公共列表）、`unlisted`（不公开列出，凭链接可看）、`private`（仅自己可见）。
_Avoid_: 权限、公开度、私密

**处理状态（Processing Status）**:
投稿媒体处理的状态机：`pending` → `processing` → `ready`；失败进入 `failed`。
_Avoid_: 转码状态、视频状态

**媒体管线（Media Pipeline）**:
从探测（ffprobe）到封面生成与 HLS 转码的处理链，由媒体任务表 `transcoding_jobs` 驱动。
_Avoid_: 转码服务、处理流程

**创作者（Creator）**:
拥有已发布投稿的用户，拥有个人空间与创作者工作台。
_Avoid_: 作者、UP 主

**关注（Follow）**:
用户对创作者的单向订阅关系，构成关注动态。
_Avoid_: 订阅、好友

**互动（Interaction）**:
点赞（like）、收藏（favorite）、关注（follow）三类写操作及其通知事件的统称。
_Avoid_: 操作、行为

**通知（Notification）**:
站内消息，共六类：互动与评论事件（`comment`、`like`、`favorite`、`follow`，经事件总线写入）与媒体处理事件（`processing_ready`、`processing_failed`，由转码事务写入）。
_Avoid_: 消息、提醒

**举报（Report）**:
观众对投稿发起的违规申报，由管理员审核；状态：`pending`、`reviewed`、`resolved`、`dismissed`。
_Avoid_: 投诉、反馈

**字幕轨道（Subtitle Track）**:
投稿附带的一条 VTT/SRT 字幕，可切换默认轨道。
_Avoid_: 字幕文件、caption

**存储配额（Storage Quota）**:
每用户上传占用的计量额度 = 源视频 + 封面 + HLS 产出的实际字节。
_Avoid_: 空间、容量
