// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { VideoPlayer } from "./VideoPlayer";
import type { Video } from "../../types";

const { loadHLSModule } = vi.hoisted(() => ({ loadHLSModule: vi.fn() }));
vi.mock("./hlsLoader", () => ({ loadHLSModule }));

const video: Video = {
  id: 1, user_id: 1, username: "测试用户", avatar_url: "", title: "测试视频", description: "", category: "科技",
  visibility: "public", video_url: "/media/video.mp4", hls_url: "", cover_url: "", mime_type: "video/mp4",
  duration_seconds: 120, size_bytes: 1024, processing_status: "ready", processing_progress: 100, processing_stage: "done",
  source_width: 1920, source_height: 1080, source_bitrate: 0, video_codec: "h264", audio_codec: "aac",
  views_count: 0, likes_count: 0, favorites_count: 0, comments_count: 0, liked: false, favorited: false,
  subtitle_tracks: [], created_at: "2026-01-01T00:00:00Z"
};
const hlsVideo = { ...video, hls_url: "/media/master.m3u8" };

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
}

function fakeHLSModule() {
  const handlers = new Map<string, (...args: unknown[]) => void>();
  const instance = { attachMedia: vi.fn(), loadSource: vi.fn(), destroy: vi.fn(), on: vi.fn((event: string, handler: (...args: unknown[]) => void) => handlers.set(event, handler)), currentLevel: -1 };
  class Constructor {
    static isSupported = () => true;
    static Events = { MEDIA_ATTACHED: "media", MANIFEST_PARSED: "manifest", ERROR: "error" };
    constructor() { return instance; }
  }
  return { module: { default: Constructor }, instance, handlers };
}

describe("VideoPlayer keyboard shortcuts", () => {
  beforeEach(() => {
    loadHLSModule.mockReset();
    vi.spyOn(HTMLMediaElement.prototype, "canPlayType").mockReturnValue("");
    vi.spyOn(HTMLMediaElement.prototype, "load").mockImplementation(() => undefined);
  });
  afterEach(() => { cleanup(); vi.restoreAllMocks(); });

  it.each([
    ["button", () => screen.getByRole("button", { name: "播放" })],
    ["link", () => screen.getByRole("link", { name: "播放器外链接" })],
    ["input", () => screen.getByRole("textbox", { name: "播放器外输入框" })],
    ["editable region", () => screen.getByRole("textbox", { name: "播放器外编辑区" })]
  ])("Space 不劫持 %s", (_name, getTarget) => {
    render(<><VideoPlayer video={video} /><a href="#outside">播放器外链接</a><input aria-label="播放器外输入框" /><div contentEditable role="textbox" aria-label="播放器外编辑区" /></>);
    const play = vi.spyOn(document.querySelector("video") as HTMLVideoElement, "play").mockResolvedValue(undefined);
    fireEvent.keyDown(getTarget(), { key: " ", code: "Space" });
    expect(play).not.toHaveBeenCalled();
  });

  it("Space 在非交互区域仍控制播放", () => {
    render(<><VideoPlayer video={video} /><div data-testid="page-background" /></>);
    const play = vi.spyOn(document.querySelector("video") as HTMLVideoElement, "play").mockResolvedValue(undefined);
    fireEvent.keyDown(screen.getByTestId("page-background"), { key: " ", code: "Space" });
    expect(play).toHaveBeenCalledOnce();
  });
});

describe("VideoPlayer HLS lifecycle", () => {
  beforeEach(() => {
    loadHLSModule.mockReset();
    vi.spyOn(HTMLMediaElement.prototype, "canPlayType").mockReturnValue("");
    vi.spyOn(HTMLMediaElement.prototype, "load").mockImplementation(() => undefined);
  });
  afterEach(() => { cleanup(); vi.restoreAllMocks(); });

  it("动态模块完成前卸载时不创建实例", async () => {
    const pending = deferred<ReturnType<typeof fakeHLSModule>["module"]>();
    loadHLSModule.mockReturnValue(pending.promise);
    const hls = fakeHLSModule();
    const view = render(<VideoPlayer video={hlsVideo} />);
    view.unmount();
    await act(async () => pending.resolve(hls.module));
    expect(hls.instance.attachMedia).not.toHaveBeenCalled();
  });

  it("卸载时销毁已创建的 HLS 实例", async () => {
    const hls = fakeHLSModule();
    loadHLSModule.mockResolvedValue(hls.module);
    const view = render(<VideoPlayer video={hlsVideo} />);
    await waitFor(() => expect(hls.instance.attachMedia).toHaveBeenCalled());
    view.unmount();
    expect(hls.instance.destroy).toHaveBeenCalledOnce();
  });

  it("致命错误回退原始视频并播报状态", async () => {
    const hls = fakeHLSModule();
    loadHLSModule.mockResolvedValue(hls.module);
    render(<VideoPlayer video={hlsVideo} />);
    await waitFor(() => expect(hls.instance.attachMedia).toHaveBeenCalled());
    vi.spyOn(document.querySelector("video") as HTMLVideoElement, "play").mockResolvedValue(undefined);
    act(() => hls.handlers.get("error")?.({}, { fatal: true }));
    expect((await screen.findByRole("status")).textContent).toContain("高清流不可用，已切换原始视频");
    expect(document.querySelector("video")?.getAttribute("src")).toContain("/media/video.mp4");
  });

  it("动态模块失败时回退，原始视频失败后提供重试", async () => {
    loadHLSModule.mockRejectedValue(new Error("chunk unavailable"));
    render(<VideoPlayer video={hlsVideo} />);
    expect(await screen.findByText("高清流不可用，已切换原始视频")).toBeTruthy();
    fireEvent.error(document.querySelector("video") as HTMLVideoElement);
    expect(await screen.findByRole("button", { name: "重试播放" })).toBeTruthy();
  });

  it("原生 HLS 请求在卸载时中止且不更新状态", async () => {
    vi.restoreAllMocks();
    vi.spyOn(HTMLMediaElement.prototype, "canPlayType").mockReturnValue("probably");
    const pending = deferred<Response>();
    let signal: AbortSignal | undefined;
    vi.stubGlobal("fetch", vi.fn((_url: string, init?: RequestInit) => { signal = init?.signal ?? undefined; return pending.promise; }));
    const view = render(<VideoPlayer video={hlsVideo} />);
    view.unmount();
    expect(signal?.aborted).toBe(true);
    await act(async () => pending.resolve(new Response("#EXTM3U")));
  });
});
