// @vitest-environment jsdom
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterAll, afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, useLocation } from "react-router-dom";
import { ApiError, api, setCSRFToken } from "../shared/api/client";
import type { AuthPayload } from "../types";
import App from "./App";

vi.hoisted(() => vi.resetModules());
vi.mock("../shared/api/client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../shared/api/client")>();
  return { ...actual, setCSRFToken: vi.fn(), api: { me: vi.fn(), notifications: vi.fn(), categories: vi.fn(), videos: vi.fn(), adminReports: vi.fn() } };
});

const user = { id: 1, username: "测试用户", avatar_url: "", bio: "", is_admin: false, created_at: "2026-01-01T00:00:00Z" };
function LocationProbe() {
  const location = useLocation();
  return <output data-testid="route-location">{location.pathname}{location.search}</output>;
}
function renderApp(path: string) {
  return render(<MemoryRouter initialEntries={[path]}><LocationProbe /><App /></MemoryRouter>);
}

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  vi.stubGlobal("matchMedia", vi.fn((media: string) => ({ media, matches: false, addEventListener: vi.fn(), removeEventListener: vi.fn() })));
  vi.mocked(api.me).mockRejectedValue(new ApiError("未登录", 401));
  vi.mocked(api.notifications).mockResolvedValue({ items: [], page: 1, page_size: 1, total: 0, has_next: false, unread_count: 0 });
  vi.mocked(api.categories).mockResolvedValue([]);
  vi.mocked(api.videos).mockResolvedValue({ items: [], page: 1, page_size: 24, total: 0, has_next: false });
  vi.mocked(api.adminReports).mockResolvedValue({ items: [], page: 1, page_size: 20, total: 0, has_next: false });
});
afterEach(cleanup);
// This suite loads lazy pages with its own API mock. The project reuses workers,
// so clear those page modules before another suite supplies its API fixtures.
afterAll(() => vi.resetModules());

describe("App guards and theme across shells", () => {
  it("匿名访问 Studio 仍保留完整 next 目标", async () => {
    renderApp("/upload?category=%E9%9F%B3%E4%B9%90");
    await waitFor(() => expect(screen.getByTestId("route-location").textContent).toBe("/auth?next=%2Fupload%3Fcategory%3D%25E9%259F%25B3%25E4%25B9%2590"));
    expect(await screen.findByLabelText("用户名")).toBeTruthy();
    expect(document.querySelector(".gv-auth-shell")).toBeTruthy();
  });

  it("非管理员不能访问审核页", async () => {
    vi.mocked(api.me).mockResolvedValue({ user, csrf_token: "test-csrf" });
    renderApp("/admin/reports");
    await waitFor(() => expect(screen.getByTestId("route-location").textContent).toBe("/"));
    expect(api.adminReports).not.toHaveBeenCalled();
    expect(setCSRFToken).toHaveBeenCalledWith("test-csrf");
    expect(await screen.findByRole("heading", { name: "这里还没有视频" })).toBeTruthy();
  });

  it("管理员使用 Studio 访问真实审核页面", async () => {
    vi.mocked(api.me).mockResolvedValue({ user: { ...user, is_admin: true }, csrf_token: "test-csrf" });
    renderApp("/admin/reports");
    expect(await screen.findByRole("heading", { name: "举报审核" })).toBeTruthy();
    expect(document.querySelector(".gv-studio-shell")).toBeTruthy();
    expect(api.adminReports).toHaveBeenCalledOnce();
  });

  it("登录过期清空 CSRF，并从 Studio 回到带 next 的 Auth", async () => {
    vi.mocked(api.me).mockResolvedValue({ user, csrf_token: "test-csrf" });
    renderApp("/upload");
    expect(await screen.findByRole("heading", { name: "发布新作品" })).toBeTruthy();
    act(() => { window.dispatchEvent(new Event("gvideo-auth-expired")); });
    await waitFor(() => expect(screen.getByTestId("route-location").textContent).toBe("/auth?next=%2Fupload"));
    expect(setCSRFToken).toHaveBeenLastCalledWith("");
    expect(await screen.findByLabelText("用户名")).toBeTruthy();
  });

  it("迟到的 me 请求不能恢复已经过期的登录状态", async () => {
    let resolve!: (payload: AuthPayload) => void;
    vi.mocked(api.me).mockImplementation(() => new Promise((done) => { resolve = done; }));
    renderApp("/upload");
    act(() => { window.dispatchEvent(new Event("gvideo-auth-expired")); });
    await act(async () => { resolve({ user, csrf_token: "stale-csrf" }); });
    await waitFor(() => expect(screen.getByTestId("route-location").textContent).toBe("/auth?next=%2Fupload"));
    expect(setCSRFToken).not.toHaveBeenCalledWith("stale-csrf");
  });

  it.each([
    [null, "light"],
    ["invalid", "light"],
    ["dark", "dark"],
    ["light", "light"]
  ])("保存值 %s 启动为 %s，无合法偏好默认浅色", async (saved, expected) => {
    if (saved !== null) localStorage.setItem("gvideo-theme", saved);
    renderApp("/auth");
    expect(document.documentElement.dataset.theme).toBe(expected);
    expect(document.documentElement.style.colorScheme).toBe(expected);
    expect(localStorage.getItem("gvideo-theme")).toBe(expected);
    expect(await screen.findByLabelText("用户名")).toBeTruthy();
  });
});
