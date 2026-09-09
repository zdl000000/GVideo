import type { Video } from "../../types";

export const visibilityLabels: Record<Video["visibility"], string> = {
  public: "公开",
  unlisted: "不公开列出",
  private: "仅自己可见"
};

export const visibilityHelp: Record<Video["visibility"], string> = {
  public: "会出现在首页、作者空间、搜索和关注动态中。",
  unlisted: "不会进入公共列表，但获得链接的人可以观看。",
  private: "只有你登录后可以访问视频和相关媒体。"
};
