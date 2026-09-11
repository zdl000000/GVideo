// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useNavigate } from "react-router-dom";
import { api } from "../../shared/api/client";
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

function RouteControls() {
  const navigate = useNavigate();
  return <button type="button" onClick={() => navigate("/video/2")}>切换视频</button>;
}

describe("VideoPage", () => {
  afterEach(() => cleanup());
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

  it("路由切换请求失败时不会在新地址显示旧视频", async () => {
    render(
      <MemoryRouter initialEntries={["/video/1"]}>
        <RouteControls />
        <Routes>
          <Route path="/video/:id" element={<VideoPage user={null} />} />
        </Routes>
      </MemoryRouter>
    );
    expect(await screen.findByRole("heading", { name: "测试视频标题" })).toBeTruthy();
    let rejectVideo!: (error: Error) => void;
    vi.mocked(api.video).mockImplementationOnce(() => new Promise((_, reject) => { rejectVideo = reject; }));

    fireEvent.click(screen.getByRole("button", { name: "切换视频" }));
    expect(await screen.findByText("正在准备播放器")).toBeTruthy();
    expect(screen.queryByRole("heading", { name: "测试视频标题" })).toBeNull();
    rejectVideo(new Error("新视频加载失败"));

    expect(await screen.findByRole("heading", { name: "内容加载失败" })).toBeTruthy();
    expect(screen.queryByRole("heading", { name: "测试视频标题" })).toBeNull();
    expect(screen.getByText("新视频加载失败")).toBeTruthy();
  });

});
