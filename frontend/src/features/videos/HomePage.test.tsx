// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { HomePage } from "./HomePage";

vi.mock("../../shared/api/client", () => ({
  api: {
    categories: vi.fn().mockResolvedValue(["音乐", "游戏"]),
    videos: vi.fn().mockResolvedValue({
      items: [
        {
          id: 1, user_id: 7, username: "创作者小张", avatar_url: "", title: "首页头条视频", description: "",
          category: "音乐", visibility: "public", video_url: "", hls_url: "", cover_url: "", mime_type: "video/mp4",
          duration_seconds: 61, size_bytes: 1024, processing_status: "ready", processing_progress: 100,
          processing_stage: "done", source_width: 1920, source_height: 1080, source_bitrate: 0,
          video_codec: "h264", audio_codec: "aac", views_count: 12, likes_count: 3, favorites_count: 1,
          comments_count: 2, liked: false, favorited: false, subtitle_tracks: [], created_at: "2026-01-01T00:00:00Z"
        },
        {
          id: 2, user_id: 8, username: "创作者小李", avatar_url: "", title: "首页次条视频", description: "",
          category: "游戏", visibility: "public", video_url: "", hls_url: "", cover_url: "", mime_type: "video/mp4",
          duration_seconds: 122, size_bytes: 2048, processing_status: "ready", processing_progress: 100,
          processing_stage: "done", source_width: 1920, source_height: 1080, source_bitrate: 0,
          video_codec: "h264", audio_codec: "aac", views_count: 20, likes_count: 5, favorites_count: 2,
          comments_count: 4, liked: false, favorited: false, subtitle_tracks: [], created_at: "2026-01-02T00:00:00Z"
        }
      ],
      page: 1, page_size: 36, total: 2, has_next: false
    })
  }
}));

describe("HomePage", () => {
  afterEach(cleanup);

  it("渲染页面骨架与 featured 展示区", async () => {
    render(<MemoryRouter><HomePage /></MemoryRouter>);
    expect(await screen.findByRole("heading", { name: "最新视频" })).toBeTruthy();
    expect(screen.getByText("最新发布")).toBeTruthy();
    // 同名链接会同时出现在 featured 展示区与下方 VideoCard 列表中
    expect(screen.getAllByRole("link", { name: "首页头条视频" }).length).toBeGreaterThan(0);
    expect(screen.getAllByRole("link", { name: "首页次条视频" }).length).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: "全部" })).toBeTruthy();
  });

  it("无视频时渲染空状态", async () => {
    const { api } = await import("../../shared/api/client");
    vi.mocked(api.videos).mockResolvedValueOnce({ items: [], page: 1, page_size: 36, total: 0, has_next: false });
    render(<MemoryRouter><HomePage /></MemoryRouter>);
    expect(await screen.findByText("这里还没有视频")).toBeTruthy();
  });
});
