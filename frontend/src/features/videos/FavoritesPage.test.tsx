// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { FavoritesPage } from "./FavoritesPage";

vi.mock("../../shared/api/client", () => ({
  api: {
    favoriteVideos: vi.fn().mockResolvedValue({
      items: [
        {
          id: 9, user_id: 3, username: "收藏作者", avatar_url: "", title: "已收藏的视频", description: "",
          category: "生活", visibility: "public", video_url: "", hls_url: "", cover_url: "", mime_type: "video/mp4",
          duration_seconds: 300, size_bytes: 4096, processing_status: "ready", processing_progress: 100,
          processing_stage: "done", source_width: 1280, source_height: 720, source_bitrate: 0,
          video_codec: "h264", audio_codec: "aac", views_count: 8, likes_count: 1, favorites_count: 1,
          comments_count: 0, liked: false, favorited: true, subtitle_tracks: [], created_at: "2026-02-01T00:00:00Z"
        }
      ],
      page: 1, page_size: 24, total: 1, has_next: false
    }),
    toggleFavorite: vi.fn().mockResolvedValue({ active: false })
  }
}));

describe("FavoritesPage", () => {
  afterEach(cleanup);

  it("渲染页面骨架与收藏移除按钮", async () => {
    render(<MemoryRouter><FavoritesPage /></MemoryRouter>);
    expect(await screen.findByRole("heading", { name: "我的收藏" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "收藏列表" })).toBeTruthy();
    expect(screen.getByRole("button", { name: /取消收藏/ })).toBeTruthy();
    expect(screen.getByText("已收藏的视频")).toBeTruthy();
  });

  it("无收藏时渲染空状态", async () => {
    const { api } = await import("../../shared/api/client");
    vi.mocked(api.favoriteVideos).mockResolvedValueOnce({ items: [], page: 1, page_size: 24, total: 0, has_next: false });
    render(<MemoryRouter><FavoritesPage /></MemoryRouter>);
    expect(await screen.findByText("还没有收藏")).toBeTruthy();
  });
});
