// @vitest-environment jsdom
import { afterAll, afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { FavoritesPage } from "./FavoritesPage";
import { api } from "../../shared/api/client";
import { pageFixture, videoFixture } from "../../shared/test/videoFixtures";

vi.hoisted(() => vi.resetModules());
vi.mock("../../shared/api/client", () => ({ api: { favoriteVideos: vi.fn(), toggleFavorite: vi.fn() } }));
const saved = videoFixture(9, { title: "已收藏的视频", username: "收藏作者", favorited: true });
function show(path = "/favorites") { return render(<MemoryRouter initialEntries={[path]}><FavoritesPage /></MemoryRouter>); }
beforeEach(() => {
  vi.clearAllMocks();
  vi.stubGlobal("scrollTo", vi.fn());
  vi.mocked(api.favoriteVideos).mockResolvedValue(pageFixture([saved], { page_size: 24 }));
  vi.mocked(api.toggleFavorite).mockResolvedValue({ active: false });
});
afterEach(cleanup);
afterAll(() => vi.resetModules());

describe("FavoritesPage", () => {
  it("渲染页面骨架与收藏移除按钮", async () => {
    show();
    expect(await screen.findByRole("heading", { name: "我的收藏" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "收藏列表" })).toBeTruthy();
    expect(await screen.findByRole("button", { name: /取消收藏/ })).toBeTruthy();
    expect(screen.getByText("已收藏的视频")).toBeTruthy();
  });
  it("无收藏时渲染空状态", async () => {
    vi.mocked(api.favoriteVideos).mockResolvedValue(pageFixture([], { page_size: 24 }));
    show();
    expect(await screen.findByText("还没有收藏")).toBeTruthy();
  });
  it("成功取消更新数量、保留占位尺寸，焦点进入状态反馈", async () => {
    show();
    fireEvent.click(await screen.findByRole("button", { name: /取消收藏/ }));
    await screen.findByText("已取消收藏");
    expect(api.toggleFavorite).toHaveBeenCalledWith(9);
    expect(screen.getByText("0 条收藏")).toBeTruthy();
    expect(document.querySelector(".gv-saved-entry--removed .gv-saved-content")?.getAttribute("inert")).toBe("");
    expect(screen.queryByRole("link", { name: "已收藏的视频" })).toBeNull();
    expect(document.activeElement?.textContent).toContain("已取消收藏");
  });
  it("待完成时禁用重复点击，不会连续 toggle 回收藏状态", async () => {
    let resolve!: (value: { active: boolean }) => void;
    vi.mocked(api.toggleFavorite).mockImplementation(() => new Promise((done) => { resolve = done; }));
    show();
    const button = await screen.findByRole("button", { name: /取消收藏/ });
    fireEvent.click(button);
    fireEvent.click(button);
    expect(button).toHaveProperty("disabled", true);
    expect(api.toggleFavorite).toHaveBeenCalledOnce();
    await act(async () => resolve({ active: false }));
    await screen.findByText("已取消收藏");
  });
  it("取消失败只影响该条目，保留列表和重试按钮", async () => {
    vi.mocked(api.toggleFavorite).mockRejectedValueOnce(new Error("取消失败"));
    show();
    fireEvent.click(await screen.findByRole("button", { name: /取消收藏/ }));
    expect(await screen.findByRole("alert")).toHaveProperty("textContent", "取消失败");
    expect(screen.getByRole("link", { name: "已收藏的视频" })).toBeTruthy();
    expect(screen.getByText("1 条收藏")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: /取消收藏/ }));
    await screen.findByText("已取消收藏");
    expect(api.toggleFavorite).toHaveBeenCalledTimes(2);
  });
  it("服务端返回 active=true 时不假装取消成功", async () => {
    vi.mocked(api.toggleFavorite).mockResolvedValue({ active: true });
    show();
    fireEvent.click(await screen.findByRole("button", { name: /取消收藏/ }));
    await screen.findByText("收藏状态已变化，请刷新后重试。");
    expect(screen.queryByText("已取消收藏")).toBeNull();
    expect(screen.getByText("1 条收藏")).toBeTruthy();
  });
  it("分页仍调用原 API，旧页面迟到的取消结果不修改新页面数量", async () => {
    let resolve!: (value: { active: boolean }) => void;
    vi.mocked(api.toggleFavorite).mockImplementation(() => new Promise((done) => { resolve = done; }));
    vi.mocked(api.favoriteVideos).mockResolvedValueOnce(pageFixture([saved], { page_size: 24, total: 30, has_next: true })).mockResolvedValueOnce(pageFixture([videoFixture(10)], { page: 2, page_size: 24, total: 30 }));
    show();
    fireEvent.click(await screen.findByRole("button", { name: /取消收藏/ }));
    fireEvent.click(screen.getByRole("button", { name: "下一页" }));
    await screen.findByText("作品10");
    expect(vi.mocked(api.favoriteVideos).mock.calls.at(-1)![0]!.get("page")).toBe("2");
    await act(async () => resolve({ active: false }));
    expect(screen.getByText("30 条收藏")).toBeTruthy();
    expect(screen.queryByText("已取消收藏")).toBeNull();
  });
  it("加载失败保留原错误恢复", async () => {
    vi.mocked(api.favoriteVideos).mockRejectedValue(new Error("收藏加载失败"));
    show();
    await screen.findByRole("alert");
    expect(screen.getByRole("button", { name: "重新加载" })).toBeTruthy();
    await waitFor(() => expect(screen.queryByRole("button", { name: /取消收藏/ })).toBeNull());
  });
  it("主动更新列表后读取真实剩余内容，取消最后一条可以进入空状态", async () => {
    vi.mocked(api.favoriteVideos).mockResolvedValueOnce(pageFixture([saved], { page_size: 24 })).mockResolvedValueOnce(pageFixture([], { page_size: 24 }));
    show();
    fireEvent.click(await screen.findByRole("button", { name: /取消收藏/ }));
    await screen.findByText("已取消收藏");
    expect(api.favoriteVideos).toHaveBeenCalledOnce();
    fireEvent.click(screen.getByRole("button", { name: "更新列表" }));
    await screen.findByText("还没有收藏");
    expect(api.favoriteVideos).toHaveBeenCalledTimes(2);
  });
  it("末页最后一条取消后主动更新，会回到仍有内容的上一页", async () => {
    vi.mocked(api.favoriteVideos)
      .mockResolvedValueOnce(pageFixture([saved], { page: 2, page_size: 24, total: 25 }))
      .mockResolvedValueOnce(pageFixture([], { page: 2, page_size: 24, total: 24 }))
      .mockResolvedValueOnce(pageFixture([videoFixture(10)], { page_size: 24, total: 24 }));
    show("/favorites?page=2");
    fireEvent.click(await screen.findByRole("button", { name: /取消收藏/ }));
    await screen.findByText("已取消收藏");
    fireEvent.click(screen.getByRole("button", { name: "更新列表" }));
    await screen.findByText("作品10");
    expect(vi.mocked(api.favoriteVideos).mock.calls.at(-1)![0]!.get("page")).toBe("1");
    expect(screen.getByText("24 条收藏")).toBeTruthy();
  });
});
