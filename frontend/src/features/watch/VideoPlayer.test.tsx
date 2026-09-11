// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useState } from "react";
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

function renderPlayer(playerVideo: Video = video) {
  function Harness() {
    const [theaterMode, setTheaterMode] = useState(false);
    return <VideoPlayer video={playerVideo} theaterMode={theaterMode} onTheaterModeChange={setTheaterMode} />;
  }
  return render(<Harness />);
}

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
    ["input", () => screen.getByRole("slider", { name: "播放进度" })],
    ["editable region", () => screen.getByRole("textbox", { name: "播放器内编辑区" })]
  ])("不劫持播放器内 %s 上的快捷键", (_name, getTarget) => {
    renderPlayer();
    const player = screen.getByRole("region", { name: "测试视频 视频播放器" });
    const editable = document.createElement("div");
    editable.contentEditable = "true";
    editable.setAttribute("role", "textbox");
    editable.setAttribute("aria-label", "播放器内编辑区");
    player.appendChild(editable);
    const element = document.querySelector("video") as HTMLVideoElement;
    const play = vi.spyOn(element, "play").mockResolvedValue(undefined);
    element.currentTime = 30;
    fireEvent.keyDown(getTarget(), { key: " ", code: "Space" });
    fireEvent.keyDown(getTarget(), { key: "k", code: "KeyK" });
    fireEvent.keyDown(getTarget(), { key: "ArrowRight", code: "ArrowRight" });
    expect(play).not.toHaveBeenCalled();
    expect(element.currentTime).toBe(30);
  });

  it("播放器外按键不触发快捷键", () => {
    render(<><VideoPlayer video={video} theaterMode={false} onTheaterModeChange={() => undefined} /><div data-testid="page-background" tabIndex={0} /></>);
    const element = document.querySelector("video") as HTMLVideoElement;
    const play = vi.spyOn(element, "play").mockResolvedValue(undefined);
    element.currentTime = 30;
    fireEvent.keyDown(screen.getByTestId("page-background"), { key: " ", code: "Space" });
    fireEvent.keyDown(screen.getByTestId("page-background"), { key: "ArrowRight", code: "ArrowRight" });
    expect(play).not.toHaveBeenCalled();
    expect(element.currentTime).toBe(30);
  });

  it("提供可聚焦播放器区域和可发现的快捷键信息", () => {
    renderPlayer();
    const player = screen.getByRole("region", { name: "测试视频 视频播放器" });
    expect(player.getAttribute("tabindex")).toBe("0");
    expect(player.getAttribute("aria-describedby")).toBe("player-keyboard-help-1");
    expect(player.getAttribute("aria-keyshortcuts")).toBe("Space K ArrowLeft ArrowRight J L Home End ArrowUp ArrowDown M C T F");
    expect(screen.getByRole("button", { name: "播放" }).hasAttribute("aria-keyshortcuts")).toBe(false);
  });

  it.each(["Space", "KeyK"])("%s 在播放器区域切换播放", (code) => {
    renderPlayer();
    const play = vi.spyOn(document.querySelector("video") as HTMLVideoElement, "play").mockResolvedValue(undefined);
    fireEvent.keyDown(screen.getByRole("region", { name: "测试视频 视频播放器" }), { key: code === "Space" ? " " : "k", code });
    expect(play).toHaveBeenCalledOnce();
  });

  it("忽略组合键和切换键长按重复事件", () => {
    renderPlayer();
    const play = vi.spyOn(document.querySelector("video") as HTMLVideoElement, "play").mockResolvedValue(undefined);
    const target = screen.getByRole("region", { name: "测试视频 视频播放器" });
    fireEvent.keyDown(target, { key: "k", code: "KeyK", ctrlKey: true });
    fireEvent.keyDown(target, { key: "K", code: "KeyK", shiftKey: true });
    fireEvent.keyDown(target, { key: "k", code: "KeyK", repeat: true });
    expect(play).not.toHaveBeenCalled();
  });

  it("方向键、J/L、Home/End 在媒体边界内跳转", () => {
    renderPlayer();
    const element = document.querySelector("video") as HTMLVideoElement;
    Object.defineProperty(element, "duration", { configurable: true, value: 120 });
    const target = screen.getByRole("region", { name: "测试视频 视频播放器" });

    element.currentTime = 50;
    fireEvent.keyDown(target, { key: "ArrowLeft", code: "ArrowLeft" });
    expect(element.currentTime).toBe(45);
    fireEvent.keyDown(target, { key: "l", code: "KeyL" });
    expect(element.currentTime).toBe(55);
    fireEvent.keyDown(target, { key: "j", code: "KeyJ" });
    expect(element.currentTime).toBe(45);
    fireEvent.keyDown(target, { key: "Home", code: "Home" });
    expect(element.currentTime).toBe(0);
    fireEvent.keyDown(target, { key: "ArrowLeft", code: "ArrowLeft" });
    expect(element.currentTime).toBe(0);
    fireEvent.keyDown(target, { key: "End", code: "End" });
    expect(element.currentTime).toBe(120);
    fireEvent.keyDown(target, { key: "ArrowRight", code: "ArrowRight" });
    expect(element.currentTime).toBe(120);
  });

  it("上下方向键调节音量并由 M 切换静音", () => {
    renderPlayer();
    const element = document.querySelector("video") as HTMLVideoElement;
    const target = screen.getByRole("region", { name: "测试视频 视频播放器" });
    element.volume = 0.5;
    fireEvent.keyDown(target, { key: "ArrowUp", code: "ArrowUp" });
    expect(element.volume).toBeCloseTo(0.55);
    fireEvent.keyDown(target, { key: "ArrowDown", code: "ArrowDown" });
    expect(element.volume).toBeCloseTo(0.5);
    fireEvent.keyDown(target, { key: "m", code: "KeyM" });
    expect(element.muted).toBe(true);
    fireEvent.keyDown(target, { key: "m", code: "KeyM" });
    expect(element.muted).toBe(false);
    element.volume = 0;
    element.muted = true;
    fireEvent.volumeChange(element);
    fireEvent.click(screen.getByRole("button", { name: "取消静音" }));
    expect(element.muted).toBe(false);
    expect(element.volume).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: "静音" })).toBeTruthy();
  });


  it("T 或宽屏按钮切换受控宽屏状态", () => {
    renderPlayer();
    const player = screen.getByRole("region", { name: "测试视频 视频播放器" });
    const button = screen.getByRole("button", { name: "宽屏模式" });
    expect(player.classList.contains("theater-mode")).toBe(false);
    expect(button.getAttribute("aria-pressed")).toBe("false");

    fireEvent.keyDown(player, { key: "t", code: "KeyT" });
    expect(player.classList.contains("theater-mode")).toBe(true);
    expect(screen.getByRole("button", { name: "宽屏模式" }).getAttribute("aria-pressed")).toBe("true");
    expect(screen.getByTestId("player-shortcut-status").textContent).toBe("已进入宽屏模式");

    fireEvent.click(screen.getByRole("button", { name: "宽屏模式" }));
    expect(player.classList.contains("theater-mode")).toBe(false);
    expect(screen.getByRole("button", { name: "宽屏模式" }).getAttribute("aria-pressed")).toBe("false");
  });
  it("C 切换并恢复最近选择的字幕，没有字幕时播报状态", async () => {
    const withSubtitle = { ...video, subtitle_tracks: [
      { id: 7, video_id: 1, language: "zh", label: "中文", url: "/media/subtitle.vtt", is_default: true, created_at: "2026-01-01T00:00:00Z" },
      { id: 8, video_id: 1, language: "en", label: "English", url: "/media/subtitle-en.vtt", is_default: false, created_at: "2026-01-01T00:00:00Z" }
    ] };
    const view = renderPlayer(withSubtitle);
    const target = screen.getByRole("region", { name: "测试视频 视频播放器" });
    const subtitleButton = screen.getByRole("button", { name: "选择字幕" });
    expect(subtitleButton.getAttribute("aria-pressed")).toBe("true");
    fireEvent.click(subtitleButton);
    const englishSubtitle = screen.getByRole("button", { name: "English" });
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("button", { name: "关闭" })));
    fireEvent.click(englishSubtitle);
    expect(subtitleButton.getAttribute("aria-pressed")).toBe("true");
    expect(screen.queryByRole("button", { name: "English" })).toBeNull();
    fireEvent.click(subtitleButton);
    expect(screen.getByRole("button", { name: "English" }).getAttribute("aria-pressed")).toBe("true");
    fireEvent.click(subtitleButton);
    fireEvent.keyDown(target, { key: "c", code: "KeyC" });
    expect(subtitleButton.getAttribute("aria-pressed")).toBe("false");
    fireEvent.keyDown(target, { key: "c", code: "KeyC" });
    expect(subtitleButton.getAttribute("aria-pressed")).toBe("true");
    fireEvent.click(subtitleButton);
    expect(screen.getByRole("button", { name: "English" }).classList.contains("active")).toBe(true);

    view.unmount();
    renderPlayer();
    fireEvent.keyDown(screen.getByRole("region", { name: "测试视频 视频播放器" }), { key: "c", code: "KeyC" });
    expect(screen.getByTestId("player-shortcut-status").textContent).toContain("此视频暂无字幕");
  });


  it("从菜单选择或按 Escape 关闭后恢复触发器焦点", async () => {
    const withSubtitle = { ...video, subtitle_tracks: [{ id: 7, video_id: 1, language: "zh", label: "中文", url: "/media/subtitle.vtt", is_default: true, created_at: "2026-01-01T00:00:00Z" }] };
    renderPlayer(withSubtitle);
    const trigger = screen.getByRole("button", { name: "选择字幕" });
    fireEvent.click(trigger);
    fireEvent.click(screen.getByRole("button", { name: "关闭" }));
    await waitFor(() => expect(document.activeElement).toBe(trigger));

    fireEvent.click(trigger);
    const option = screen.getByRole("button", { name: "中文" });
    option.focus();
    fireEvent.keyDown(option, { key: "Escape", code: "Escape" });
    await waitFor(() => expect(document.activeElement).toBe(trigger));
  });

  it("播放被浏览器拒绝时会处理 Promise 拒绝", async () => {
    renderPlayer();
    const rejection = new Error("denied");
    const play = vi.spyOn(document.querySelector("video") as HTMLVideoElement, "play").mockRejectedValue(rejection);
    fireEvent.keyDown(screen.getByRole("region", { name: "测试视频 视频播放器" }), { key: "k", code: "KeyK" });
    expect(play).toHaveBeenCalledOnce();
    await act(async () => Promise.resolve());
    expect(screen.getByTestId("player-shortcut-status").textContent).toBe("无法播放");
  });

  it("F 播报全屏进入、退出、不支持和拒绝结果", async () => {
    renderPlayer();
    const player = screen.getByRole("region", { name: "测试视频 视频播放器" }) as HTMLDivElement;
    const requestFullscreen = vi.fn().mockResolvedValue(undefined);
    player.requestFullscreen = requestFullscreen;
    fireEvent.keyDown(player, { key: "f", code: "KeyF" });
    await waitFor(() => expect(screen.getByTestId("player-shortcut-status").textContent).toBe("已进入全屏"));

    const exitFullscreen = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(document, "exitFullscreen", { configurable: true, value: exitFullscreen });
    Object.defineProperty(document, "fullscreenElement", { configurable: true, value: player });
    fireEvent.keyDown(player, { key: "f", code: "KeyF" });
    await waitFor(() => expect(screen.getByTestId("player-shortcut-status").textContent).toBe("已退出全屏"));

    Object.defineProperty(document, "fullscreenElement", { configurable: true, value: null });
    Object.defineProperty(player, "requestFullscreen", { configurable: true, value: undefined });
    fireEvent.keyDown(player, { key: "f", code: "KeyF" });
    await waitFor(() => expect(screen.getByTestId("player-shortcut-status").textContent).toBe("当前无法切换全屏"));

    Object.defineProperty(player, "requestFullscreen", { configurable: true, value: vi.fn().mockRejectedValue(new Error("denied")) });
    fireEvent.keyDown(player, { key: "f", code: "KeyF" });
    await waitFor(() => expect(screen.getByTestId("player-shortcut-status").textContent).toBe("全屏切换失败"));
  });

  it("画中画 Promise 被拒绝时不会泄漏未处理异常", async () => {
    Object.defineProperty(document, "pictureInPictureEnabled", { configurable: true, value: true });
    renderPlayer();
    const element = document.querySelector("video") as HTMLVideoElement & { requestPictureInPicture?: () => Promise<unknown> };
    const requestPictureInPicture = vi.fn().mockRejectedValue(new Error("denied"));
    element.requestPictureInPicture = requestPictureInPicture;
    fireEvent.click(screen.getByRole("button", { name: "画中画" }));
    expect(requestPictureInPicture).toHaveBeenCalledOnce();
    await act(async () => Promise.resolve());

    const exitPictureInPicture = vi.fn().mockRejectedValue(new Error("denied"));
    Object.defineProperty(document, "pictureInPictureElement", { configurable: true, value: element });
    Object.defineProperty(document, "exitPictureInPicture", { configurable: true, value: exitPictureInPicture });
    fireEvent.click(screen.getByRole("button", { name: "画中画" }));
    expect(exitPictureInPicture).toHaveBeenCalledOnce();
    await act(async () => Promise.resolve());
    Object.defineProperty(document, "pictureInPictureElement", { configurable: true, value: null });
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
    const view = renderPlayer(hlsVideo);
    view.unmount();
    await act(async () => pending.resolve(hls.module));
    expect(hls.instance.attachMedia).not.toHaveBeenCalled();
  });

  it("卸载时销毁已创建的 HLS 实例", async () => {
    const hls = fakeHLSModule();
    loadHLSModule.mockResolvedValue(hls.module);
    const view = renderPlayer(hlsVideo);
    await waitFor(() => expect(hls.instance.attachMedia).toHaveBeenCalled());
    view.unmount();
    expect(hls.instance.destroy).toHaveBeenCalledOnce();
  });

  it("致命错误回退原始视频并播报状态", async () => {
    const hls = fakeHLSModule();
    loadHLSModule.mockResolvedValue(hls.module);
    renderPlayer(hlsVideo);
    await waitFor(() => expect(hls.instance.attachMedia).toHaveBeenCalled());
    vi.spyOn(document.querySelector("video") as HTMLVideoElement, "play").mockResolvedValue(undefined);
    act(() => hls.handlers.get("error")?.({}, { fatal: true }));
    expect((await screen.findByRole("status")).textContent).toContain("高清流不可用，已切换原始视频");
    expect(document.querySelector("video")?.getAttribute("src")).toContain("/media/video.mp4");
  });

  it("动态模块失败时回退，原始视频失败后提供重试", async () => {
    loadHLSModule.mockRejectedValue(new Error("chunk unavailable"));
    renderPlayer(hlsVideo);
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
    const view = renderPlayer(hlsVideo);
    view.unmount();
    expect(signal?.aborted).toBe(true);
    await act(async () => pending.resolve(new Response("#EXTM3U")));
  });
});
