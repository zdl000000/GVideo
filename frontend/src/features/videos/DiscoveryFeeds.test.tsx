// @vitest-environment jsdom
import { afterAll, afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router-dom";
import { LatestPage } from "./LatestPage";
import { PopularPage } from "./PopularPage";
import { FollowingPage } from "./FollowingPage";
import { api } from "../../shared/api/client";
import { pageFixture, videoFixture } from "../../shared/test/videoFixtures";

vi.hoisted(() => vi.resetModules());
vi.mock("../../shared/api/client", () => ({ api: { categories: vi.fn(), videos: vi.fn(), followingVideos: vi.fn() } }));
function LocationProbe() { const location = useLocation(); return <output data-testid="location">{location.search}</output>; }
function show(component: React.ReactNode, path: string) { return render(<MemoryRouter initialEntries={[path]}>{component}<LocationProbe /></MemoryRouter>); }
beforeEach(() => {
  vi.clearAllMocks();
  vi.stubGlobal("scrollTo", vi.fn());
  vi.mocked(api.categories).mockResolvedValue(["音乐", "游戏"]);
  vi.mocked(api.videos).mockResolvedValue(pageFixture(Array.from({ length: 5 }, (_, i) => videoFixture(i + 1)), { total: 40, has_next: true }));
  vi.mocked(api.followingVideos).mockResolvedValue(pageFixture([videoFixture(8)], { page_size: 24, total: 30, has_next: true }));
});
afterEach(cleanup);
afterAll(() => vi.resetModules());

describe("LatestPage", () => {
  it("完整时间 feed 保留 q、category、latest 和分页，不渲染 Hero", async () => {
    show(<LatestPage />, "/latest?q=演奏&category=音乐");
    await screen.findByText("作品1");
    const request = vi.mocked(api.videos).mock.calls[0][0];
    expect(request.get("sort")).toBe("latest");
    expect(request.get("q")).toBe("演奏");
    expect(request.get("category")).toBe("音乐");
    expect(document.querySelectorAll(".gv-video-card--standard")).toHaveLength(5);
    expect(document.querySelector(".gv-video-hero")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "下一页" }));
    await waitFor(() => expect(vi.mocked(api.videos).mock.calls.at(-1)![0]!.get("page")).toBe("2"));
    expect(screen.getByTestId("location").textContent).toContain("q=");
  });
  it("分类和排序切换保留关键词并重置分页", async () => {
    show(<LatestPage />, "/latest?q=演奏&page=2");
    fireEvent.click(await screen.findByRole("button", { name: "游戏" }));
    await waitFor(() => expect(vi.mocked(api.videos).mock.calls.at(-1)![0]!.get("category")).toBe("游戏"));
    expect(vi.mocked(api.videos).mock.calls.at(-1)![0]!.get("page")).toBe("1");
    const target = screen.getByRole("link", { name: "热门" }).getAttribute("href")!;
    expect(target.startsWith("/popular?")).toBe(true);
    expect(new URLSearchParams(target.split("?")[1]).get("q")).toBe("演奏");
    expect(target).not.toContain("page=");
  });
  it.each(["empty", "error"])("%s 状态仍可恢复", async (state) => {
    if (state === "empty") vi.mocked(api.videos).mockResolvedValue(pageFixture([]));
    else vi.mocked(api.videos).mockRejectedValue(new Error("最新加载失败"));
    show(<LatestPage />, "/latest");
    expect(await screen.findByRole("heading", { name: state === "empty" ? "暂无最新内容" : "内容加载失败" })).toBeTruthy();
  });
});

describe("PopularPage", () => {
  it("前三名使用更高媒体层级，之后紧凑排序，只有真实排名", async () => {
    show(<PopularPage />, "/popular");
    await screen.findByRole("region", { name: "榜单前三名" });
    expect(document.querySelectorAll(".gv-popular-leaders [data-video-id]")).toHaveLength(3);
    expect(document.querySelectorAll(".gv-ranked-list [data-video-id]")).toHaveLength(2);
    expect(screen.getByLabelText("第 1 名").textContent).toBe("01");
    expect(screen.getByLabelText("第 5 名").textContent).toBe("05");
    expect(vi.mocked(api.videos).mock.calls[0][0].get("sort")).toBe("popular");
    expect(screen.queryByText("本周")).toBeNull();
  });
  it("第二页使用 API page_size 连续排名 37、38，保留查询条件", async () => {
    vi.mocked(api.videos).mockResolvedValue(pageFixture([videoFixture(37), videoFixture(38)], { page: 2, page_size: 36, total: 38 }));
    show(<PopularPage />, "/popular?page=2&q=作品&category=音乐");
    expect(await screen.findByLabelText("第 37 名")).toHaveProperty("textContent", "37");
    expect(screen.getByLabelText("第 38 名").textContent).toBe("38");
    expect(screen.queryByRole("region", { name: "榜单前三名" })).toBeNull();
    const request = vi.mocked(api.videos).mock.calls[0][0];
    expect(request.get("q")).toBe("作品");
    expect(request.get("category")).toBe("音乐");
    fireEvent.click(screen.getByRole("button", { name: "上一页" }));
    await waitFor(() => expect(vi.mocked(api.videos).mock.calls.at(-1)![0]!.get("page")).toBe("1"));
  });
  it("只有一个视频不填充虚假名次", async () => {
    vi.mocked(api.videos).mockResolvedValue(pageFixture([videoFixture(1)]));
    show(<PopularPage />, "/popular");
    await screen.findByLabelText("第 1 名");
    expect(document.querySelectorAll("[data-video-id]")).toHaveLength(1);
    expect(screen.queryByLabelText("第 2 名")).toBeNull();
  });
  it.each(["empty", "error"])("%s 显示相应状态", async (state) => {
    if (state === "empty") vi.mocked(api.videos).mockResolvedValue(pageFixture([]));
    else vi.mocked(api.videos).mockRejectedValue(new Error("榜单失败"));
    show(<PopularPage />, "/popular");
    expect(await screen.findByRole("heading", { name: state === "empty" ? "暂无热门内容" : "内容加载失败" })).toBeTruthy();
  });
});

describe("FollowingPage", () => {
  it("时间流强调真实创作者及发布时间，并保留分页", async () => {
    show(<FollowingPage />, "/following");
    await screen.findByText("作品8");
    expect(screen.getByRole("link", { name: "创作者8" }).getAttribute("href")).toBe("/users/18");
    expect(document.querySelector(".gv-video-card--creator-first time")?.getAttribute("datetime")).toBe("2026-01-01T00:00:00Z");
    fireEvent.click(screen.getByRole("button", { name: "下一页" }));
    await waitFor(() => expect(vi.mocked(api.followingVideos).mock.calls.at(-1)![0]!.get("page")).toBe("2"));
    expect(vi.mocked(api.followingVideos).mock.calls.at(-1)![0]!.get("page_size")).toBe("24");
  });
  it("没有关注内容时仍可以发现创作者", async () => {
    vi.mocked(api.followingVideos).mockResolvedValue(pageFixture([]));
    show(<FollowingPage />, "/following");
    await screen.findByText("关注动态还是空的");
    expect(screen.getByRole("link", { name: "发现创作者" }).getAttribute("href")).toBe("/popular");
  });
  it("失败保留错误恢复，并在卸载时取消请求", async () => {
    vi.mocked(api.followingVideos).mockRejectedValue(new Error("关注失败"));
    const view = show(<FollowingPage />, "/following");
    await screen.findByRole("alert");
    expect(screen.getByRole("button", { name: "重新加载" })).toBeTruthy();
    const signal = vi.mocked(api.followingVideos).mock.calls[0][1]!;
    view.unmount();
    expect(signal.aborted).toBe(true);
  });
});
