// @vitest-environment jsdom
import { afterAll, afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router-dom";
import { api } from "../../shared/api/client";
import { MyVideosPage, EditVideoDialog, SubtitleDialog, DeleteVideoDialog } from "./MyVideosPage";
import { pageFixture, videoFixture } from "../../shared/test/videoFixtures";

vi.hoisted(() => vi.resetModules());
vi.mock("../../shared/api/client", () => ({ api: { myVideos: vi.fn(), categories: vi.fn(), updateVideo: vi.fn(), deleteVideo: vi.fn(), retryVideo: vi.fn(), uploadSubtitle: vi.fn(), setDefaultSubtitle: vi.fn(), deleteSubtitle: vi.fn() } }));
const video = videoFixture(3, { title: "我的测试投稿", cover_url: "/broken.jpg" });
function Location() { return <output data-testid="location">{useLocation().search}</output>; }
const mount = (path = "/me/videos") => render(<MemoryRouter initialEntries={[path]}><Location /><MyVideosPage /></MemoryRouter>);
function capturePolls() {
  const callbacks: (() => void)[] = [];
  const original = window.setTimeout.bind(window);
  vi.spyOn(window, "clearTimeout");
  vi.spyOn(window, "setTimeout").mockImplementation(((callback: TimerHandler, delay?: number) => {
    if (delay === 2500 && typeof callback === "function") { callbacks.push(callback as () => void); return 123; }
    return original(callback, delay);
  }) as typeof window.setTimeout);
  return callbacks;
}
async function menu(label: string) { const trigger = await screen.findByRole("button", { name: "我的测试投稿的操作菜单" }); fireEvent.click(trigger); fireEvent.click(screen.getByRole("menuitem", { name: label })); return trigger; }
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.myVideos).mockResolvedValue(pageFixture([video], { page_size: 12 }));
  vi.mocked(api.categories).mockResolvedValue(["音乐", "游戏"]);
  vi.mocked(api.updateVideo).mockResolvedValue(video);
  vi.mocked(api.deleteVideo).mockResolvedValue({ deleted: true });
  vi.mocked(api.retryVideo).mockResolvedValue({ processing_status: "pending" });
  vi.stubGlobal("scrollTo", vi.fn());
});
afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.useRealTimers(); vi.unstubAllGlobals(); });
afterAll(() => vi.resetModules());

describe("MyVideosPage workflow", () => {
  it("renders content hierarchy and shared broken cover", async () => {
    mount(); expect(await screen.findByRole("link", { name: video.title })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "我的投稿" })).toBeTruthy();
    expect(screen.getByText("条投稿")).toBeTruthy();
    fireEvent.error(document.querySelector(".gv-cover-image")!);
    expect(document.querySelector(".gv-video-cover--failed")).toBeTruthy();
    expect(screen.getByText("公开")).toBeTruthy();
    expect(screen.queryByRole("menu")).toBeNull();
  });
  it("loading and error feedback are explicit", async () => {
    vi.mocked(api.myVideos).mockRejectedValue(new Error("列表不可用")); mount();
    expect(screen.getByRole("status", { name: "正在加载投稿" })).toBeTruthy();
    expect(await screen.findByText("列表不可用")).toBeTruthy();
  });
  it("empty content uses a tool-style publish entry", async () => {
    vi.mocked(api.myVideos).mockResolvedValue(pageFixture([], { page_size: 12 })); mount();
    expect(await screen.findByRole("heading", { name: "还没有投稿" })).toBeTruthy();
    expect(screen.getByRole("link", { name: "发布第一条作品" })).toBeTruthy();
  });
  it("menu arrows, Home/End, Escape, outside click and disabled delete work", async () => {
    vi.mocked(api.myVideos).mockResolvedValue(pageFixture([{ ...video, processing_status: "processing" }], { page_size: 12 })); mount();
    const trigger = await screen.findByRole("button", { name: `${video.title}的操作菜单` });
    fireEvent.keyDown(trigger, { key: "ArrowDown" });
    const edit = screen.getByRole("menuitem", { name: "编辑视频" });
    expect(document.activeElement).toBe(edit);
    fireEvent.keyDown(edit, { key: "End" }); expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "字幕管理" }));
    expect((screen.getByRole("menuitem", { name: "删除" }) as HTMLButtonElement).disabled).toBe(true);
    fireEvent.keyDown(document.activeElement!, { key: "Home" }); expect(document.activeElement).toBe(edit);
    fireEvent.keyDown(edit, { key: "ArrowDown" }); expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "字幕管理" }));
    fireEvent.keyDown(window, { key: "Escape" }); expect(document.activeElement).toBe(trigger); expect(screen.queryByRole("menu")).toBeNull();
    fireEvent.click(trigger); fireEvent.pointerDown(document.body); expect(screen.queryByRole("menu")).toBeNull(); expect(document.activeElement).toBe(trigger);
  });
  for (const [label, role, closeName] of [["编辑视频", "dialog", "关闭编辑窗口"], ["字幕管理", "dialog", "关闭字幕管理"], ["删除", "alertdialog", "取消"]] as const) {
    it(`${label}: dialog cancellation returns to the persistent row trigger`, async () => {
      mount(); const trigger = await menu(label); const dialog = screen.getByRole(role);
      expect(dialog.contains(document.activeElement)).toBe(true);
      fireEvent.click(within(dialog).getByRole("button", { name: closeName }));
      await waitFor(() => expect(document.activeElement).toBe(trigger));
      expect(screen.queryByRole(role)).toBeNull();
    });
  }
  it("edit sends the same FormData fields and reports success", async () => {
    mount(); const trigger = await menu("编辑视频");
    fireEvent.change(screen.getByLabelText(/^标题/), { target: { value: "修改后的标题" } });
    vi.mocked(api.updateVideo).mockResolvedValue({ ...video, title: "修改后的标题" });
    fireEvent.click(screen.getByRole("button", { name: "保存修改" }));
    expect(await screen.findByText("投稿信息已保存")).toBeTruthy();
    const form = vi.mocked(api.updateVideo).mock.calls[0][1] as FormData;
    expect(form.get("title")).toBe("修改后的标题"); expect(form.get("visibility")).toBe("public");
    await waitFor(() => expect(document.activeElement).toBe(trigger));
  });
  it("pagination still requests page_size=12", async () => {
    vi.mocked(api.myVideos).mockResolvedValue(pageFixture([video], { page_size: 12, total: 13, has_next: true })); mount();
    fireEvent.click(await screen.findByRole("button", { name: "下一页" }));
    await waitFor(() => expect(api.myVideos).toHaveBeenCalledTimes(2));
    expect(vi.mocked(api.myVideos).mock.calls[1][0]?.toString()).toBe("page=2&page_size=12");
  });
  it("deleting the last row on page 2 returns to page 1", async () => {
    vi.mocked(api.myVideos).mockResolvedValue(pageFixture([video], { page: 2, page_size: 12, total: 13 }));
    mount("/me/videos?page=2"); await menu("删除");
    expect(screen.getByRole("alertdialog").getAttribute("aria-describedby")).toBe("delete-video-description");
    fireEvent.click(screen.getByRole("button", { name: "确认删除" }));
    expect(await screen.findByText("投稿及其媒体文件已删除")).toBeTruthy();
    await waitFor(() => expect(screen.getByTestId("location").textContent).toBe(""));
    expect(vi.mocked(api.myVideos).mock.calls.at(-1)?.[0]?.get("page")).toBe("1");
  });
  it("2500ms polling updates 20 -> 60 -> ready without replacing rows or losing a dialog", async () => {
    const processing = { ...video, processing_status: "processing" as const, processing_stage: "transcoding", processing_progress: 20 };
    vi.mocked(api.myVideos).mockResolvedValueOnce(pageFixture([processing], { page_size: 12 })).mockResolvedValueOnce(pageFixture([{ ...processing, processing_progress: 60 }], { page_size: 12 })).mockResolvedValueOnce(pageFixture([video], { page_size: 12 }));
    const polls = capturePolls(); mount(); await screen.findByRole("progressbar"); const row = document.querySelector(".gv-content-row");
    await menu("编辑视频");
    await act(async () => { polls.shift()!(); });
    expect(document.querySelector(".gv-content-row")).toBe(row); expect(document.querySelector('[role="progressbar"]')?.getAttribute("aria-valuenow")).toBe("60");
    expect(screen.getByRole("dialog")).toBeTruthy();
    await act(async () => { polls.shift()!(); });
    expect(document.querySelector(".gv-content-row")).toBe(row); expect(document.querySelector('[role="progressbar"]')).toBeNull();
    expect(window.clearTimeout).toHaveBeenCalledWith(123); expect(api.myVideos).toHaveBeenCalledTimes(3);
  });
  it("failed retry becomes pending and resumes polling, never fake ready", async () => {
    vi.mocked(api.myVideos).mockResolvedValue(pageFixture([{ ...video, processing_status: "failed", processing_stage: "failed", processing_error: "编码失败" }], { page_size: 12 }));
    const polls = capturePolls(); mount(); await menu("重新处理"); expect(await screen.findByText("已重新加入转码队列")).toBeTruthy();
    expect(document.querySelector(".gv-content-row--pending")).toBeTruthy();
    expect(api.retryVideo).toHaveBeenCalledWith(3);
    await act(async () => { polls.shift()!(); }); expect(api.myVideos).toHaveBeenCalledTimes(2);
  });
  it("retry errors remain visible and leave the failed row unchanged", async () => {
    vi.mocked(api.myVideos).mockResolvedValue(pageFixture([{ ...video, processing_status: "failed" }], { page_size: 12 }));
    vi.mocked(api.retryVideo).mockRejectedValue(new Error("重试失败")); mount(); await menu("重新处理");
    expect(await screen.findByText("重试失败")).toBeTruthy(); expect(document.querySelector(".gv-content-row--failed")).toBeTruthy();
  });
});

describe("Studio dialogs", () => {
  it("edit Escape/backdrop close, busy blocks both, and cover object URLs are revoked", async () => {
    const close = vi.fn(); const saved = vi.fn();
    vi.stubGlobal("URL", class extends URL { static createObjectURL = vi.fn(() => "blob:cover"); static revokeObjectURL = vi.fn(); });
    let finish!: (value: typeof video) => void; vi.mocked(api.updateVideo).mockReturnValue(new Promise((resolve) => { finish = resolve; }));
    const result = render(<EditVideoDialog video={video} categories={["音乐"]} onClose={close} onSaved={saved} />);
    fireEvent.keyDown(window, { key: "Escape" }); fireEvent.mouseDown(document.querySelector(".dialog-backdrop")!); expect(close).toHaveBeenCalledTimes(2);
    fireEvent.change(document.querySelector('input[type="file"]')!, { target: { files: [new File(["x"], "cover.png", { type: "image/png" })] } });
    fireEvent.click(screen.getByRole("button", { name: "保存修改" }));
    fireEvent.keyDown(window, { key: "Escape" }); fireEvent.mouseDown(document.querySelector(".dialog-backdrop")!); expect(close).toHaveBeenCalledTimes(2);
    expect((vi.mocked(api.updateVideo).mock.calls[0][1] as FormData).get("cover")).toBeInstanceOf(File);
    await act(async () => finish(video)); result.unmount(); expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:cover");
  });
  it("delete traps Tab and blocks cancellation while busy", () => {
    const close = vi.fn(); const confirm = vi.fn();
    const result = render(<DeleteVideoDialog video={video} busy={false} onClose={close} onConfirm={confirm} />);
    const cancel = screen.getByRole("button", { name: "取消" }); const remove = screen.getByRole("button", { name: "确认删除" });
    remove.focus(); fireEvent.keyDown(window, { key: "Tab" }); expect(document.activeElement).toBe(cancel);
    fireEvent.keyDown(window, { key: "Tab", shiftKey: true }); expect(document.activeElement).toBe(remove);
    fireEvent.keyDown(window, { key: "Escape" }); expect(close).toHaveBeenCalledTimes(1);
    result.rerender(<DeleteVideoDialog video={video} busy onClose={close} onConfirm={confirm} />);
    fireEvent.keyDown(window, { key: "Escape" }); fireEvent.mouseDown(document.querySelector(".dialog-backdrop")!); expect(close).toHaveBeenCalledTimes(1);
    expect(screen.getByText(/数据库记录、原视频、封面、字幕和转码文件/)).toBeTruthy();
  });
  it("subtitle upload busy prevents Escape/backdrop dismissal", async () => {
    const close = vi.fn(); let finish!: (value: { id: number; language: string; label: string; url: string; is_default: boolean }) => void;
    vi.mocked(api.uploadSubtitle).mockReturnValue(new Promise((resolve) => { finish = resolve; }));
    render(<SubtitleDialog video={video} onClose={close} onTracksChanged={vi.fn()} />);
    fireEvent.change(screen.getByLabelText("字幕文件"), { target: { files: [new File(["WEBVTT"], "test.vtt")] } });
    fireEvent.submit(document.querySelector(".subtitle-upload-form")!);
    await waitFor(() => expect((screen.getByRole("button", { name: "关闭字幕管理" }) as HTMLButtonElement).disabled).toBe(true));
    fireEvent.keyDown(window, { key: "Escape" }); fireEvent.mouseDown(document.querySelector(".dialog-backdrop")!); expect(close).not.toHaveBeenCalled();
    await act(async () => finish({ id: 1, language: "zh-CN", label: "中文", url: "/sub.vtt", is_default: true }));
    fireEvent.keyDown(window, { key: "Escape" }); expect(close).toHaveBeenCalledTimes(1);
  });
});
