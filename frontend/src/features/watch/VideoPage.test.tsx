// @vitest-environment jsdom
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { VideoPage } from "./VideoPage";

vi.mock("../../shared/api/client", () => ({
  api: {
    video: vi.fn().mockResolvedValue({
      id: 1,
      user_id: 42,
      username: "测试作者",
      avatar_url: "",
      title: "测试视频标题",
      description: "这是一段视频简介",
      category: "科技",
      visibility: "public",
      video_url: "/media/video.mp4",
      hls_url: "",
      cover_url: "",
      mime_type: "video/mp4",
      duration_seconds: 120,
      size_bytes: 1024,
      processing_status: "ready",
      processing_progress: 100,
      processing_stage: "done",
      source_width: 1920,
      source_height: 1080,
      source_bitrate: 0,
      video_codec: "h264",
      audio_codec: "aac",
      views_count: 10,
      likes_count: 2,
      favorites_count: 1,
      comments_count: 0,
      liked: false,
      favorited: false,
      subtitle_tracks: [],
      created_at: "2024-01-01T00:00:00Z"
    }),
    comments: vi.fn().mockResolvedValue([]),
    creator: vi.fn().mockResolvedValue({
      id: 42,
      username: "测试作者",
      bio: "",
      avatar_url: "",
      created_at: "2024-01-01T00:00:00Z",
      followers_count: 3,
      following_count: 4,
      videos_count: 1,
      followed: false
    }),
    videos: vi.fn().mockResolvedValue({ items: [], page: 1, page_size: 12, total: 0, has_next: false }),
    toggleLike: vi.fn().mockResolvedValue({ active: true }),
    toggleFavorite: vi.fn().mockResolvedValue({ active: true }),
    toggleFollow: vi.fn().mockResolvedValue({ active: true }),
    comment: vi.fn().mockResolvedValue({ id: 1, video_id: 1, user_id: 1, username: "测试用户", avatar_url: "", content: "评论内容", created_at: "2024-01-01T00:00:00Z" }),
    deleteComment: vi.fn().mockResolvedValue({ deleted: true }),
    reportVideo: vi.fn().mockResolvedValue({ id: 1, status: "pending" })
  }
}));

describe("VideoPage", () => {
  it("渲染播放页骨架", async () => {
    render(
      <MemoryRouter initialEntries={["/video/1"]}>
        <Routes>
          <Route path="/video/:id" element={<VideoPage user={null} />} />
        </Routes>
      </MemoryRouter>
    );
    expect(await screen.findByRole("heading", { name: "测试视频标题" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "相关推荐" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "评论" })).toBeTruthy();
    expect(screen.getByText("还没有评论，来聊第一句。")).toBeTruthy();
    expect(screen.getByPlaceholderText("登录后参与讨论")).toBeTruthy();
  });
});
