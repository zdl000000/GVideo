// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from "react-router-dom";
import { api } from "../../shared/api/client";
import { VideoPage } from "./VideoPage";
import { pageFixture, videoFixture } from "../../shared/test/videoFixtures";
import type { User } from "../../types";

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

const viewer: User = { id: 1, username: "测试用户", avatar_url: "", bio: "", is_admin: false, created_at: "2024-01-01T00:00:00Z" };
function Destination() { const location = useLocation(); return <p>{location.pathname + location.search}</p>; }
function openVideo(user: User | null = viewer) {
  return render(<MemoryRouter initialEntries={["/video/1"]}><RouteControls /><Routes><Route path="/video/:id" element={<VideoPage user={user} />} /><Route path="/auth" element={<Destination />} /></Routes></MemoryRouter>);
}

describe("VideoPage", () => {
  afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.useRealTimers(); vi.unstubAllGlobals(); });
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

  it("播放器先于标题，宽屏和互动更新保留同一个媒体节点与时间", async () => {
    openVideo();
    await screen.findByRole("heading", { name: "测试视频标题" });
    const media = document.querySelector("video")!;
    expect(document.querySelector(".watch-main")!.firstElementChild!.classList.contains("player-wrap")).toBe(true);
    media.currentTime = 30;
    fireEvent.click(screen.getByRole("button", { name: "宽屏模式" }));
    expect(document.querySelector("video")).toBe(media);
    expect(media.currentTime).toBe(30);
    fireEvent.click(screen.getByRole("button", { name: "宽屏模式" }));
    fireEvent.click(screen.getByRole("button", { name: "点赞" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "点赞" }).getAttribute("aria-pressed")).toBe("true"));
    expect(screen.getByRole("button", { name: "点赞" }).textContent).toBe("3");
    fireEvent.click(screen.getByRole("button", { name: "收藏" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "收藏" }).getAttribute("aria-pressed")).toBe("true"));
    expect(screen.getByRole("button", { name: "收藏" }).textContent).toBe("2");
    expect(document.querySelector("video")).toBe(media);
    expect(media.currentTime).toBe(30);
  });

  it.each(["点赞", "收藏", "关注", "举报"])("匿名 %s 保留当前视频登录回跳", async (name) => {
    openVideo(null);
    await screen.findByRole("heading", { name: "测试视频标题" });
    fireEvent.click(screen.getByRole("button", { name }));
    expect(await screen.findByText("/auth?next=%2Fvideo%2F1")).toBeTruthy();
  });

  it("关注更新真实作者状态和粉丝数", async () => {
    openVideo();
    await screen.findByText("3 粉丝");
    fireEvent.click(screen.getByRole("button", { name: "关注" }));
    expect(await screen.findByRole("button", { name: "已关注" })).toBeTruthy();
    expect(screen.getByText("4 粉丝")).toBeTruthy();
    expect(api.toggleFollow).toHaveBeenCalledWith(42);
  });

  it("评论作者只能删自己的评论，发布后保留原请求和计数", async () => {
    vi.mocked(api.video).mockResolvedValueOnce(videoFixture(1, { user_id: 42, comments_count: 2 }));
    vi.mocked(api.comments).mockResolvedValueOnce([
      { id: 11, video_id: 1, user_id: 2, username: "别人", avatar_url: "", content: "别人的评论", created_at: "2024-01-01T00:00:00Z" },
      { id: 12, video_id: 1, user_id: 1, username: "测试用户", avatar_url: "", content: "自己的评论", created_at: "2024-01-01T00:00:00Z" }
    ]);
    vi.spyOn(window, "confirm").mockReturnValue(true);
    openVideo();
    await screen.findByText("自己的评论");
    expect(screen.getAllByRole("button", { name: "删除" })).toHaveLength(1);
    fireEvent.click(screen.getByRole("button", { name: "删除" }));
    await waitFor(() => expect(screen.queryByText("自己的评论")).toBeNull());
    expect(screen.getByText("别人的评论")).toBeTruthy();
    expect(api.deleteComment).toHaveBeenCalledWith(1, 12);
    expect(document.querySelector(".comment-heading > span")?.textContent).toBe("1");
    fireEvent.change(screen.getByLabelText("评论内容"), { target: { value: " 评论内容 " } });
    fireEvent.click(screen.getByRole("button", { name: "发布" }));
    expect(await screen.findByText("评论内容")).toBeTruthy();
    expect(api.comment).toHaveBeenCalledWith("1", "评论内容");
    expect(document.querySelector(".comment-heading > span")?.textContent).toBe("2");
    expect((screen.getByLabelText("评论内容") as HTMLTextAreaElement).value).toBe("");
  });

  it.each([42, 99])("视频作者可删除他人评论，其他用户不可删除（用户 %i）", async (userID) => {
    vi.mocked(api.comments).mockResolvedValueOnce([{ id: 11, video_id: 1, user_id: 2, username: "别人", avatar_url: "", content: "别人的评论", created_at: "2024-01-01T00:00:00Z" }]);
    openVideo({ ...viewer, id: userID });
    await screen.findByText("别人的评论");
    expect(screen.queryAllByRole("button", { name: "删除" })).toHaveLength(userID === 42 ? 1 : 0);
  });

  it("保留 inline 举报类型、补充说明和成功状态", async () => {
    openVideo();
    await screen.findByRole("heading", { name: "测试视频标题" });
    fireEvent.click(screen.getByRole("button", { name: "举报" }));
    fireEvent.change(screen.getByLabelText("举报原因"), { target: { value: "copyright" } });
    fireEvent.change(screen.getByLabelText("补充说明"), { target: { value: "时间点 00:30" } });
    fireEvent.click(screen.getByRole("button", { name: "提交举报" }));
    expect(await screen.findByText("举报已提交")).toBeTruthy();
    expect(api.reportVideo).toHaveBeenCalledWith(1, "copyright", "时间点 00:30");
  });

  it("分享复制当前地址并反馈结果", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { clipboard: { writeText } });
    openVideo();
    await screen.findByRole("heading", { name: "测试视频标题" });
    fireEvent.click(screen.getByRole("button", { name: "分享" }));
    expect(await screen.findByText("链接已复制")).toBeTruthy();
    expect(writeText).toHaveBeenCalledWith(window.location.href);
  });

  it("related 排除当前视频并复用封面失败回退", async () => {
    vi.mocked(api.videos).mockResolvedValueOnce(pageFixture([videoFixture(1), videoFixture(2, { cover_url: "/broken.jpg" })]));
    openVideo();
    await screen.findByRole("link", { name: "作品2" });
    expect(document.querySelectorAll(".related-video")).toHaveLength(1);
    fireEvent.error(document.querySelector(".related-cover .gv-cover-image")!);
    expect(document.querySelector(".related-cover .gv-cover-image")).toBeNull();
    expect(document.querySelector(".related-cover .gv-media-caption")?.textContent).toBe("封面占位");
  });

  it("processing 轮询就绪后收起状态条并保留媒体节点", async () => {
    vi.useFakeTimers();
    vi.mocked(api.video).mockResolvedValueOnce(videoFixture(1, { processing_status: "processing", processing_progress: 35, processing_stage: "transcoding" })).mockResolvedValueOnce(videoFixture(1));
    await act(async () => { openVideo(); });
    const media = document.querySelector("video");
    expect(screen.getByRole("progressbar").getAttribute("aria-valuenow")).toBe("35");
    await act(async () => { await vi.advanceTimersByTimeAsync(2500); });
    expect(api.video).toHaveBeenCalledWith("1", false, expect.any(AbortSignal));
    expect(screen.queryByRole("progressbar")).toBeNull();
    expect(document.querySelector("video")).toBe(media);
  });

  it.each(["pending", "failed"] as const)("%s 保留播放器、真实状态和非 ready 元信息", async (processing_status) => {
    vi.mocked(api.video).mockResolvedValueOnce(videoFixture(1, { processing_status, processing_progress: 0, processing_error: "转码失败，请使用原始文件。" }));
    openVideo();
    await screen.findByRole("heading", { name: "作品1" });
    expect(document.querySelector(".watch-main")?.firstElementChild?.classList.contains("player-wrap")).toBe(true);
    expect(screen.queryByText("1920 × 1080")).toBeNull();
    if (processing_status === "failed") {
      expect(screen.getByRole("status").textContent).toContain("转码失败，请使用原始文件。");
      expect(screen.queryByRole("progressbar")).toBeNull();
    } else expect(screen.getByRole("progressbar").getAttribute("aria-valuenow")).toBe("0");
  });

  it("路由切换后迟到的旧请求不会覆盖新视频", async () => {
    let resolveOld!: (video: ReturnType<typeof videoFixture>) => void;
    vi.mocked(api.video).mockImplementationOnce(() => new Promise((resolve) => { resolveOld = resolve; })).mockResolvedValueOnce(videoFixture(2));
    openVideo();
    fireEvent.click(screen.getByRole("button", { name: "切换视频" }));
    await screen.findByRole("heading", { name: "作品2" });
    await act(async () => { resolveOld(videoFixture(1)); });
    expect(screen.getByRole("heading", { name: "作品2" })).toBeTruthy();
    expect(screen.queryByRole("heading", { name: "作品1" })).toBeNull();
  });
});
