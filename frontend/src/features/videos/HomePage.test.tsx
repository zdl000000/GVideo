// @vitest-environment jsdom
import { afterAll, afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router-dom";
import { HomePage } from "./HomePage";
import { api } from "../../shared/api/client";
import { pageFixture, videoFixture } from "../../shared/test/videoFixtures";
import type { VideoPage } from "../../types";

vi.hoisted(() => vi.resetModules());
vi.mock("../../shared/api/client", () => ({ api: { categories: vi.fn(), videos: vi.fn() } }));
const lead = videoFixture(1, { title: "首页头条视频", username: "创作者小张" });
const secondary = videoFixture(2, { title: "首页次条视频", username: "创作者小李" });
const defaults = [lead, secondary];
function LocationProbe() { const location = useLocation(); return <output data-testid="location">{location.search}</output>; }
function show(path = "/") { return render(<MemoryRouter initialEntries={[path]}><HomePage /><LocationProbe /></MemoryRouter>); }
beforeEach(() => {
  vi.clearAllMocks();
  vi.stubGlobal("scrollTo", vi.fn());
  vi.mocked(api.categories).mockResolvedValue(["音乐", "游戏"]);
  vi.mocked(api.videos).mockImplementation(async (params) => pageFixture(params.get("sort") === "popular" ? [] : defaults));
});
afterEach(cleanup);
afterAll(() => vi.resetModules());

describe("HomePage discovery", () => {
  it("渲染 featured、真实标题、分类，数据少时不复制 Latest", async () => {
    show();
    const stories = await screen.findByRole("region", { name: "最新发布视频" });
    expect(within(stories).getByText((_, node) => node?.tagName === "P" && node.textContent === "最新发布 / 音乐")).toBeTruthy();
    expect(screen.getByRole("link", { name: "首页头条视频" })).toBeTruthy();
    expect(screen.getByRole("link", { name: "首页次条视频" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "全部" })).toBeTruthy();
    expect(document.querySelectorAll('[data-video-id="1"]')).toHaveLength(1);
    expect(document.querySelectorAll('[data-video-id="2"]')).toHaveLength(1);
    expect(screen.queryByRole("region", { name: "最新视频" })).toBeNull();
  });
  it("无视频时渲染原空状态", async () => {
    vi.mocked(api.videos).mockResolvedValue(pageFixture([]));
    show();
    expect(await screen.findByText("这里还没有视频")).toBeTruthy();
  });
  it("只一个视频时收起次级内容和其他区域", async () => {
    vi.mocked(api.videos).mockResolvedValue(pageFixture([lead]));
    show();
    await screen.findByRole("region", { name: "最新发布视频" });
    expect(document.querySelector(".gv-secondary-stories")).toBeNull();
    expect(document.querySelectorAll("[data-video-id]")).toHaveLength(1);
    expect(screen.queryByRole("region", { name: "热门预览" })).toBeNull();
  });
  it("Featured、Latest、Popular 跨区域按 ID 去重，保留真实热门名次", async () => {
    const items = Array.from({ length: 8 }, (_, i) => videoFixture(i + 1));
    vi.mocked(api.videos).mockImplementation(async (params) => pageFixture(params.get("sort") === "popular" ? [items[0], videoFixture(20), items[5], videoFixture(21)] : [...items, items[0]]));
    show();
    await screen.findByRole("region", { name: "热门预览" });
    expect(document.querySelectorAll(".gv-home-stories [data-video-id]")).toHaveLength(4);
    expect(document.querySelectorAll(".gv-latest-mosaic [data-video-id]")).toHaveLength(4);
    const ids = Array.from(document.querySelectorAll("[data-video-id]"), (node) => node.getAttribute("data-video-id"));
    expect(new Set(ids).size).toBe(ids.length);
    const preview = screen.getByRole("region", { name: "热门预览" });
    expect(within(preview).getByLabelText("第 2 名")).toBeTruthy();
    expect(within(preview).getByLabelText("第 4 名")).toBeTruthy();
    expect(api.videos).toHaveBeenCalledWith(expect.any(URLSearchParams), expect.any(AbortSignal));
  });
  it.each(["/?q=音乐", "/?category=音乐"])("%s 使用筛选模式，不显示或请求 unrelated Hero/热门", async (path) => {
    show(path);
    expect(await screen.findByRole("heading", { name: "筛选结果" })).toBeTruthy();
    expect(screen.getByText("共 2 条视频")).toBeTruthy();
    expect(document.querySelector(".gv-video-hero")).toBeNull();
    expect(api.videos).toHaveBeenCalledOnce();
    expect(vi.mocked(api.videos).mock.calls[0][0].get(path.includes("q=") ? "q" : "category")).toBe("音乐");
  });
  it("分类保持 q、移除 page，键盘可达并暴露选中状态", async () => {
    show("/?q=作品&page=2");
    const button = await screen.findByRole("button", { name: "游戏" });
    fireEvent.click(button);
    await waitFor(() => expect(new URLSearchParams(screen.getByTestId("location").textContent || "").get("category")).toBe("游戏"));
    const request = vi.mocked(api.videos).mock.calls.at(-1)![0];
    expect(request.get("q")).toBe("作品");
    expect(request.get("page")).toBe("1");
    expect(button.getAttribute("aria-pressed")).toBe("true");
  });
  it("分页保留过滤条件、第一页之外不显示 Hero", async () => {
    vi.mocked(api.videos).mockResolvedValue(pageFixture(defaults, { total: 40, has_next: true }));
    show("/?q=作品&category=音乐");
    fireEvent.click(await screen.findByRole("button", { name: "下一页" }));
    await waitFor(() => expect(vi.mocked(api.videos).mock.calls.at(-1)![0]!.get("page")).toBe("2"));
    expect(screen.getByTestId("location").textContent).toContain("category=");
    expect(screen.getByTestId("location").textContent).toContain("q=");
    expect(document.querySelector(".gv-video-hero")).toBeNull();
  });
  it("热门请求失败只影响预览，且可独立重试", async () => {
    vi.mocked(api.videos).mockImplementation(async (params) => { if (params.get("sort") === "popular") throw new Error("热门网络错误"); return pageFixture(defaults); });
    show();
    await screen.findByText(/热门网络错误/);
    expect(screen.getByRole("link", { name: "首页头条视频" })).toBeTruthy();
    expect(screen.queryByRole("alert")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "重试热门内容" }));
    await waitFor(() => expect(vi.mocked(api.videos).mock.calls.filter(([params]) => params.get("sort") === "popular")).toHaveLength(2));
    expect(vi.mocked(api.videos).mock.calls.filter(([params]) => params.get("sort") === "latest")).toHaveLength(1);
  });
  it("主请求失败保留错误恢复，不渲染热门替代主列表", async () => {
    vi.mocked(api.videos).mockImplementation(async (params) => { if (params.get("sort") === "latest") throw new Error("主列表失败"); return pageFixture([videoFixture(20)]); });
    show();
    expect(await screen.findByRole("alert")).toHaveProperty("textContent", expect.stringContaining("主列表失败"));
    expect(screen.getByRole("button", { name: "重新加载" })).toBeTruthy();
    expect(document.querySelectorAll("[data-video-id]")).toHaveLength(0);
  });
  it("卸载中止两个请求，迟到的响应不能覆盖下一次搜索", async () => {
    let resolve!: (value: VideoPage) => void;
    vi.mocked(api.videos).mockImplementation((params) => params.get("q") ? Promise.resolve(pageFixture([videoFixture(30)])) : new Promise((done) => { resolve = done; }));
    const view = show();
    const signals = vi.mocked(api.videos).mock.calls.map(([, signal]) => signal!);
    view.unmount();
    expect(signals.every((signal) => signal.aborted)).toBe(true);
    show("/?q=新的");
    await screen.findByText("作品30");
    await act(async () => resolve(pageFixture([lead])));
    expect(screen.queryByText("首页头条视频")).toBeNull();
  });
  it("categories 失败不影响主列表", async () => {
    vi.mocked(api.categories).mockRejectedValue(new Error("分类失败"));
    show("/?category=现有分类");
    await screen.findByText("首页头条视频");
    expect(screen.getByRole("button", { name: "现有分类" }).getAttribute("aria-pressed")).toBe("true");
    expect(screen.getByText(/分类暂时无法加载/)).toBeTruthy();
  });
});
