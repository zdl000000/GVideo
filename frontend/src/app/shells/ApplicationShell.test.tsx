// @vitest-environment jsdom
import { afterAll, afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { ApplicationShell } from "./ApplicationShell";
import { api } from "../../shared/api/client";
import type { ShellProps } from "./shellTypes";

vi.hoisted(() => vi.resetModules());
vi.mock("../../shared/api/client", () => ({ api: { notifications: vi.fn(), logout: vi.fn() } }));

const user = { id: 1, username: "创作者", avatar_url: "", bio: "", is_admin: false, created_at: "2026-01-01T00:00:00Z" };
const defaults: ShellProps = { auth: { user, loading: false }, theme: "dark", onThemeChange: vi.fn(), onLogout: vi.fn() };
let mobile = false;

function RouteInfo() {
  const location = useLocation();
  return <h1>{location.pathname}{location.search}</h1>;
}

function renderShell(path: string, props: ShellProps = defaults) {
  return render(<MemoryRouter initialEntries={[path]}><Routes><Route element={<ApplicationShell {...props} />}><Route path="*" element={<RouteInfo />} /></Route></Routes></MemoryRouter>);
}

beforeEach(() => {
  mobile = false;
  vi.clearAllMocks();
  vi.mocked(api.notifications).mockResolvedValue({ items: [], page: 1, page_size: 1, total: 0, has_next: false, unread_count: 3 });
  vi.mocked(api.logout).mockResolvedValue({ logged_out: true });
  vi.stubGlobal("matchMedia", vi.fn((media: string) => ({ media, matches: mobile, addEventListener: vi.fn(), removeEventListener: vi.fn() })));
  vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => { callback(0); return 1; });
  vi.stubGlobal("cancelAnimationFrame", vi.fn());
});
afterEach(cleanup);
afterAll(() => vi.resetModules());

describe("ApplicationShell navigation", () => {
  it("Watch 导航与创作导航分开，搜索仍回到带查询参数的首页", async () => {
    renderShell("/latest");
    const rail = screen.getByRole("complementary", { name: "观看导航" });
    expect(within(rail).getByRole("link", { name: "最新发布" }).getAttribute("aria-current")).toBe("page");
    expect(within(rail).queryByRole("link", { name: "投稿管理" })).toBeNull();
    const search = screen.getByRole("textbox", { name: "搜索" });
    fireEvent.change(search, { target: { value: "  音乐 & 影像  " } });
    fireEvent.submit(search.closest("form")!);
    expect(await screen.findByRole("heading", { name: "/?q=%E9%9F%B3%E4%B9%90%20%26%20%E5%BD%B1%E5%83%8F" })).toBeTruthy();
    expect(await screen.findByRole("link", { name: "通知中心，3 条未读" })).toBeTruthy();
  });

  it("Studio 显示工作导航，管理员入口按真实角色显示", async () => {
    renderShell("/admin/reports", { ...defaults, auth: { user: { ...user, is_admin: true }, loading: false } });
    const rail = screen.getByRole("complementary", { name: "创作导航" });
    expect(within(rail).getByRole("link", { name: "举报审核" }).getAttribute("aria-current")).toBe("page");
    expect(within(rail).getByRole("link", { name: "发布视频" })).toBeTruthy();
    expect(within(rail).queryByRole("link", { name: "关注动态" })).toBeNull();
    await screen.findByRole("link", { name: "通知中心，3 条未读" });
  });

  it("Auth 不加载观看或工作导航，保留主题操作", async () => {
    renderShell("/auth?next=/upload", { ...defaults, auth: { user: null, loading: false } });
    expect(await screen.findByRole("heading", { name: "/auth?next=/upload" })).toBeTruthy();
    expect(screen.queryByRole("textbox", { name: "搜索" })).toBeNull();
    expect(screen.queryByRole("complementary", { name: "观看导航" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "切换到浅色主题" }));
    expect(defaults.onThemeChange).toHaveBeenCalledOnce();
  });

  it("移动抽屉约束焦点、隔离背景、Escape 恢复焦点和滚动", async () => {
    mobile = true;
    renderShell("/");
    const trigger = screen.getByRole("button", { name: "打开菜单" });
    fireEvent.click(trigger);
    const drawer = screen.getByRole("dialog", { name: "导航菜单" });
    const first = within(drawer).getByRole("link", { name: "GVideo 首页" });
    const last = within(drawer).getByRole("button", { name: "退出登录" });
    expect(document.activeElement).toBe(first);
    expect(document.querySelector("main")?.inert).toBe(true);
    expect(document.body.style.overflow).toBe("hidden");
    fireEvent.keyDown(first, { key: "Tab", shiftKey: true });
    expect(document.activeElement).toBe(last);
    fireEvent.keyDown(last, { key: "Tab" });
    expect(document.activeElement).toBe(first);
    fireEvent.keyDown(first, { key: "Escape" });
    expect(screen.queryByRole("dialog", { name: "导航菜单" })).toBeNull();
    expect(document.activeElement).toBe(trigger);
    expect(document.querySelector("main")?.inert).toBeFalsy();
    expect(document.body.style.overflow).toBe("");
    await screen.findByRole("link", { name: "通知中心，3 条未读" });
  });

  it("从移动导航进入 Studio 后关闭抽屉", async () => {
    mobile = true;
    renderShell("/");
    fireEvent.click(screen.getByRole("button", { name: "打开菜单" }));
    fireEvent.click(within(screen.getByRole("dialog", { name: "导航菜单" })).getByRole("link", { name: "投稿管理" }));
    expect(await screen.findByRole("heading", { name: "/me/videos" })).toBeTruthy();
    expect(screen.getByRole("complementary", { name: "创作导航" })).toBeTruthy();
    expect(screen.queryByRole("dialog", { name: "导航菜单" })).toBeNull();
    expect(document.body.style.overflow).toBe("");
  });

  it("切换用户时取消旧通知请求，旧结果不覆盖当前未读数", async () => {
    let resolveOld!: (result: Awaited<ReturnType<typeof api.notifications>>) => void;
    vi.mocked(api.notifications).mockImplementationOnce(() => new Promise((resolve) => { resolveOld = resolve; }));
    const view = renderShell("/latest");
    await waitFor(() => expect(api.notifications).toHaveBeenCalledOnce());
    const signal = vi.mocked(api.notifications).mock.calls[0][1];
    const nextProps = { ...defaults, auth: { user: { ...user, id: 2 }, loading: false } };
    view.rerender(<MemoryRouter initialEntries={["/latest"]}><Routes><Route element={<ApplicationShell {...nextProps} />}><Route path="*" element={<RouteInfo />} /></Route></Routes></MemoryRouter>);
    expect(signal?.aborted).toBe(true);
    await screen.findByRole("link", { name: "通知中心，3 条未读" });
    await act(async () => { resolveOld({ items: [], page: 1, page_size: 1, total: 0, has_next: false, unread_count: 99 }); });
    expect(screen.getByRole("link", { name: "通知中心，3 条未读" })).toBeTruthy();
    expect(screen.queryByRole("link", { name: "通知中心，99 条未读" })).toBeNull();
  });
});
