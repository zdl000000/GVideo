// @vitest-environment jsdom
import { afterAll, afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router-dom";
import { api } from "../../shared/api/client";
import { videoFixture } from "../../shared/test/videoFixtures";
import { UploadPage } from "./UploadPage";

vi.hoisted(() => vi.resetModules());
vi.mock("../../shared/api/client", () => ({
  api: { categories: vi.fn(), upload: vi.fn() }
}));
const NativeURL = URL;
const video = new File(["isolated video"], "作品.mp4", { type: "video/mp4" });
const cover = new File(["isolated image"], "封面.png", { type: "image/png" });
const subtitle = new File(["WEBVTT\n\n"], "中文.vtt", { type: "text/vtt" });
const createURL = vi.fn(); const revokeURL = vi.fn();
function Location() { return <output data-testid="location">{useLocation().pathname}</output>; }
async function mount() {
  const result = render(<MemoryRouter initialEntries={["/upload"]}><UploadPage /><Location /></MemoryRouter>);
  await screen.findByRole("option", { name: "音乐" }); return result;
}
function choose(label: string, file: File) { fireEvent.change(screen.getByLabelText(label, { selector: "input" }), { target: { files: [file] } }); }
function fill() {
  choose("视频文件", video); choose("封面图片", cover); choose("字幕文件", subtitle);
  fireEvent.change(screen.getByLabelText(/^标题/), { target: { value: "真实作品标题" } });
  fireEvent.change(screen.getByLabelText("简介"), { target: { value: "创作简介" } });
  fireEvent.change(screen.getByLabelText("分区"), { target: { value: "生活" } });
  fireEvent.click(screen.getByRole("radio", { name: /^仅自己/ }));
}
function uploadRequest() {
  let resolve!: (value: ReturnType<typeof videoFixture>) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<ReturnType<typeof videoFixture>>((done, fail) => { resolve = done; reject = fail; });
  return { promise, resolve, reject };
}
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.categories).mockResolvedValue(["音乐", "生活"]);
  vi.mocked(api.upload).mockResolvedValue(videoFixture(1));
  createURL.mockReturnValue("blob:selected-cover");
  vi.stubGlobal("URL", class extends NativeURL { static createObjectURL = createURL; static revokeObjectURL = revokeURL; });
});
afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.unstubAllGlobals(); });
afterAll(() => vi.resetModules());

describe("UploadPage", () => {
  it("渲染发布表单骨架", async () => {
    await mount();
    expect(await screen.findByRole("heading", { name: "发布新作品" })).toBeTruthy();
    expect(await screen.findByText("音乐")).toBeTruthy();
    expect(screen.getByText("会出现在首页、作者空间、搜索和关注动态中。")).toBeTruthy();
    expect(screen.getByRole("button", { name: "发布作品" }).hasAttribute("disabled")).toBe(true);
    expect(screen.getByText("未选择封面时会自动截取视频画面")).toBeTruthy();
    expect(screen.getByText(/单条初始中文字幕/)).toBeTruthy();
  });
  it("requires video and preserves native title and file constraints", async () => {
    await mount(); fireEvent.submit(screen.getByRole("form", { name: "发布新作品" }));
    expect(await screen.findByRole("alert")).toHaveProperty("textContent", "请选择视频文件");
    expect(api.upload).not.toHaveBeenCalled();
    const title = screen.getByLabelText(/^标题/) as HTMLInputElement;
    expect(title.required).toBe(true); expect(title.minLength).toBe(2); expect(title.maxLength).toBe(80);
    expect((screen.getByLabelText("简介") as HTMLTextAreaElement).maxLength).toBe(2000);
    expect(screen.getByLabelText("视频文件", { selector: "input" }).getAttribute("accept")).toBe("video/mp4,video/webm,video/ogg");
    expect(screen.getByLabelText("封面图片").getAttribute("accept")).toBe("image/jpeg,image/png,image/webp");
    expect(screen.getByLabelText("字幕文件").getAttribute("accept")).toBe(".vtt,.srt,text/vtt,application/x-subrip");
  });
  it("selected file names and media type replace empty selectors", async () => {
    await mount(); choose("视频文件", video); choose("字幕文件", subtitle);
    expect(screen.getByText(video.name)).toBeTruthy(); expect(screen.getByText(/video\/mp4/)).toBeTruthy();
    expect(screen.queryByText("选择视频文件")).toBeNull(); expect(screen.getByText(subtitle.name)).toBeTruthy();
    expect(screen.getByRole("button", { name: "发布作品" }).hasAttribute("disabled")).toBe(false);
  });
  it("cover preview uses selected file and revokes replaced and unmounted URLs", async () => {
    const view = await mount(); choose("封面图片", cover);
    expect(createURL).toHaveBeenCalledWith(cover); expect(screen.getByAltText("封面预览").getAttribute("src")).toBe("blob:selected-cover");
    createURL.mockReturnValue("blob:replacement"); const replacement = new File(["replacement"], "新封面.webp", { type: "image/webp" }); choose("封面图片", replacement);
    expect(revokeURL).toHaveBeenCalledWith("blob:selected-cover"); expect(screen.getByAltText("封面预览").getAttribute("src")).toBe("blob:replacement");
    view.unmount(); expect(revokeURL).toHaveBeenCalledWith("blob:replacement");
  });
  it("submits exactly one FormData with the existing initial Chinese subtitle and navigates", async () => {
    await mount(); fill(); fireEvent.click(screen.getByRole("button", { name: "发布作品" }));
    await waitFor(() => expect(api.upload).toHaveBeenCalledOnce());
    const [form, progress, signal] = vi.mocked(api.upload).mock.calls[0];
    expect([...form.keys()]).toEqual(["title", "description", "category", "visibility", "video", "cover", "subtitle", "subtitle_language", "subtitle_label"]);
    expect(form.get("title")).toBe("真实作品标题"); expect(form.get("description")).toBe("创作简介"); expect(form.get("category")).toBe("生活"); expect(form.get("visibility")).toBe("private");
    expect(form.get("video")).toBe(video); expect(form.get("cover")).toBe(cover); expect(form.get("subtitle")).toBe(subtitle);
    expect(form.get("subtitle_language")).toBe("zh-CN"); expect(form.get("subtitle_label")).toBe("中文");
    expect(progress).toBeTypeOf("function"); expect(signal).toBeInstanceOf(AbortSignal);
    await waitFor(() => expect(screen.getByTestId("location").textContent).toBe("/video/1"));
  });
  it("omits optional cover and subtitle fields", async () => {
    await mount(); choose("视频文件", video); fireEvent.change(screen.getByLabelText(/^标题/), { target: { value: "最小投稿" } });
    fireEvent.click(screen.getByRole("button", { name: "发布作品" })); await waitFor(() => expect(api.upload).toHaveBeenCalledOnce());
    const form = vi.mocked(api.upload).mock.calls[0][0]; expect(form.get("visibility")).toBe("public"); expect(form.has("cover")).toBe(false); expect(form.has("subtitle")).toBe(false); expect(form.has("subtitle_language")).toBe(false);
  });
  it("shows actual upload progress and indeterminate unknown total without processing progress", async () => {
    vi.mocked(api.upload).mockImplementation(() => new Promise(() => {})); await mount(); fill();
    fireEvent.click(screen.getByRole("button", { name: "发布作品" }));
    const progress = vi.mocked(api.upload).mock.calls[0][1]!;
    expect(screen.getByRole("progressbar").getAttribute("aria-valuenow")).toBe("0");
    act(() => progress(45, 100)); expect(screen.getByRole("progressbar").getAttribute("aria-valuenow")).toBe("45");
    act(() => progress(100, 100)); expect(screen.getByText("100%")).toBeTruthy();
    act(() => progress(23, 0)); expect(screen.getByText("计算中")).toBeTruthy(); expect(screen.getByRole("progressbar").hasAttribute("aria-valuenow")).toBe(false);
    expect(screen.getByText("上传完成后会进入后台处理阶段")).toBeTruthy(); expect(screen.getByRole("button", { name: "正在上传..." }).hasAttribute("disabled")).toBe(true);
  });
  it("cancel aborts only the upload and keeps every selected file and form value", async () => {
    vi.mocked(api.upload).mockImplementation((_form, _progress, signal) => new Promise((_resolve, reject) => signal!.addEventListener("abort", () => reject(new DOMException("cancel", "AbortError")), { once: true })));
    await mount(); fill(); fireEvent.click(screen.getByRole("button", { name: "发布作品" }));
    const signal = vi.mocked(api.upload).mock.calls[0][2]!; fireEvent.click(screen.getByRole("button", { name: "取消上传" }));
    expect(signal.aborted).toBe(true); expect(await screen.findByRole("alert")).toHaveProperty("textContent", "上传已取消，文件和表单内容已保留");
    expect((screen.getByLabelText(/^标题/) as HTMLInputElement).value).toBe("真实作品标题"); expect((screen.getByLabelText("简介") as HTMLTextAreaElement).value).toBe("创作简介"); expect((screen.getByLabelText("分区") as HTMLSelectElement).value).toBe("生活");
    expect((screen.getByRole("radio", { name: /^仅自己/ }) as HTMLInputElement).checked).toBe(true);
    expect(screen.getByText(video.name)).toBeTruthy(); expect(screen.getByText(cover.name)).toBeTruthy(); expect(screen.getByText(subtitle.name)).toBeTruthy(); expect(screen.getByAltText("封面预览")).toBeTruthy();
    expect(screen.getByRole("button", { name: "发布作品" }).hasAttribute("disabled")).toBe(false);
  });
  it("beforeunload is registered only during busy and cleaned after cancel", async () => {
    const add = vi.spyOn(window, "addEventListener"); const remove = vi.spyOn(window, "removeEventListener");
    vi.mocked(api.upload).mockImplementation((_form, _progress, signal) => new Promise((_resolve, reject) => signal!.addEventListener("abort", () => reject(new DOMException("cancel", "AbortError")))));
    await mount(); fill(); expect(add.mock.calls.some(([type]) => type === "beforeunload")).toBe(false);
    fireEvent.click(screen.getByRole("button", { name: "发布作品" })); expect(add.mock.calls.some(([type]) => type === "beforeunload")).toBe(true);
    const event = new Event("beforeunload", { cancelable: true }); window.dispatchEvent(event); expect(event.defaultPrevented).toBe(true);
    fireEvent.click(screen.getByRole("button", { name: "取消上传" })); await screen.findByRole("alert");
    await waitFor(() => expect(remove.mock.calls.some(([type]) => type === "beforeunload")).toBe(true));
    const idle = new Event("beforeunload", { cancelable: true }); window.dispatchEvent(idle); expect(idle.defaultPrevented).toBe(false);
  });
  it("unmount aborts an active request", async () => {
    vi.mocked(api.upload).mockImplementation(() => new Promise(() => {})); const view = await mount(); fill(); fireEvent.click(screen.getByRole("button", { name: "发布作品" }));
    const signal = vi.mocked(api.upload).mock.calls[0][2]!; view.unmount(); expect(signal.aborted).toBe(true);
  });
  it("category and upload failures remain visible without clearing state", async () => {
    vi.mocked(api.categories).mockRejectedValue(new Error("分区不可用")); render(<MemoryRouter><UploadPage /></MemoryRouter>);
    expect(await screen.findByRole("alert")).toHaveProperty("textContent", "分区不可用"); cleanup();
    vi.mocked(api.categories).mockResolvedValue(["音乐", "生活"]); vi.mocked(api.upload).mockRejectedValue(new Error("上传失败")); await mount(); fill(); fireEvent.click(screen.getByRole("button", { name: "发布作品" }));
    expect(await screen.findByRole("alert")).toHaveProperty("textContent", "上传失败"); expect(screen.getByText(video.name)).toBeTruthy();
  });
  it("focused submit keeps keyboard context on cancel while busy and returns after cancellation", async () => {
    vi.mocked(api.upload).mockImplementation((_form, _progress, signal) => new Promise((_resolve, reject) => signal!.addEventListener("abort", () => reject(new DOMException("cancel", "AbortError")), { once: true })));
    await mount(); fill(); const submit = screen.getByRole("button", { name: "发布作品" });
    submit.focus(); fireEvent.click(submit);
    const cancel = screen.getByRole("button", { name: "取消上传" }); expect(document.activeElement).toBe(cancel);
    fireEvent.click(cancel); await screen.findByRole("alert");
    expect(document.activeElement).toBe(submit); expect(screen.getByText(video.name)).toBeTruthy();
  });
  it("upload failure restores focused submit from the removed cancel action", async () => {
    const request = uploadRequest(); vi.mocked(api.upload).mockReturnValue(request.promise);
    await mount(); fill(); const submit = screen.getByRole("button", { name: "发布作品" });
    submit.focus(); fireEvent.click(submit); expect(document.activeElement).toBe(screen.getByRole("button", { name: "取消上传" }));
    await act(async () => request.reject(new Error("焦点验收上传失败")));
    expect(screen.getByRole("alert").textContent).toBe("焦点验收上传失败"); expect(document.activeElement).toBe(submit);
  });
  it("implicit form submission from a focused title field keeps focus on the busy cancel action", async () => {
    vi.mocked(api.upload).mockImplementation((_form, _progress, signal) => new Promise((_resolve, reject) => signal!.addEventListener("abort", () => reject(new DOMException("cancel", "AbortError")), { once: true })));
    await mount(); fill(); const title = screen.getByLabelText(/^标题/); title.focus();
    fireEvent.submit(screen.getByRole("form", { name: "发布新作品" }));
    const cancel = screen.getByRole("button", { name: "取消上传" }); expect(document.activeElement).toBe(cancel);
    fireEvent.click(cancel); await screen.findByRole("alert");
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "发布作品" }));
  });
  it.each(["success", "error"] as const)("%s never steals focus after the user moves to another action", async (outcome) => {
    const request = uploadRequest(); vi.mocked(api.upload).mockReturnValue(request.promise);
    await mount(); fill(); render(<button>其他操作</button>);
    const submit = screen.getByRole("button", { name: "发布作品" }); submit.focus(); fireEvent.click(submit);
    const other = screen.getByRole("button", { name: "其他操作" }); other.focus();
    await act(async () => outcome === "success" ? request.resolve(videoFixture(1)) : request.reject(new Error("上传失败")));
    expect(document.activeElement).toBe(other);
    if (outcome === "success") expect(screen.getByTestId("location").textContent).toBe("/video/1");
    else expect(screen.getByRole("alert").textContent).toBe("上传失败");
  });
});
