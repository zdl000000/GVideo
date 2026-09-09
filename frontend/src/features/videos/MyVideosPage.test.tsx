// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { MyVideosPage } from "./MyVideosPage";

vi.mock("../../shared/api/client", () => ({
  api: {
    myVideos: vi.fn().mockResolvedValue({
      items: [
        {
          id: 3, user_id: 1, username: "测试用户", avatar_url: "", title: "我的测试投稿", description: "",
          category: "音乐", visibility: "public", video_url: "", hls_url: "", cover_url: "", mime_type: "video/mp4",
          duration_seconds: 61, size_bytes: 1024, processing_status: "ready", processing_progress: 100,
          processing_stage: "done", source_width: 1920, source_height: 1080, source_bitrate: 0,
          video_codec: "h264", audio_codec: "aac", views_count: 12, likes_count: 3, favorites_count: 1,
          comments_count: 2, liked: false, favorited: false, subtitle_tracks: [], created_at: "2026-01-01T00:00:00Z"
        }
      ],
      page: 1, page_size: 12, total: 1, has_next: false
    }),
    categories: vi.fn().mockResolvedValue(["音乐", "游戏"]),
    updateVideo: vi.fn(),
    deleteVideo: vi.fn().mockResolvedValue({ deleted: true }),
    retryVideo: vi.fn().mockResolvedValue({ processing_status: "pending" })
  }
}));

describe("MyVideosPage", () => {
  afterEach(cleanup);

  it("渲染页面骨架与投稿统计", async () => {
    render(<MemoryRouter><MyVideosPage /></MemoryRouter>);
    expect(await screen.findByRole("heading", { name: "我的投稿" })).toBeTruthy();
    // "1" 在 <strong> 内，文本被拆分，匹配外层文案
    expect(screen.getByText("条投稿")).toBeTruthy();
    expect(screen.getByRole("link", { name: "发布视频" })).toBeTruthy();
  });

  it("渲染投稿行与管理操作", async () => {
    render(<MemoryRouter><MyVideosPage /></MemoryRouter>);
    expect(await screen.findByRole("link", { name: "我的测试投稿" })).toBeTruthy();
    // 操作区是带 aria-label 的 div，用 getByLabelText 查询；按钮可访问名取自内容而非 title
    expect(screen.getByLabelText("我的测试投稿的管理操作")).toBeTruthy();
    expect(screen.getByRole("button", { name: "编辑" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "字幕" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "删除" })).toBeTruthy();
    // visibilityLabels 文案
    expect(screen.getByText("公开")).toBeTruthy();
  });
});
