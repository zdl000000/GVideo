// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router-dom";
import { api } from "../../shared/api/client";
import type { Notification, NotificationPage } from "../../types";
import { NotificationCenterPage, notificationCopy } from "./NotificationCenterPage";

vi.mock("../../shared/api/client", () => ({ api: { notifications: vi.fn(), markNotificationRead: vi.fn(), markAllNotificationsRead: vi.fn() } }));

const item = (id: number, overrides: Partial<Notification> = {}): Notification => ({ id, type: "like", actor_id: 9, actor_username: "创作者小李", video_id: 3, video_title: "测试视频", created_at: "2026-01-01T00:00:00Z", ...overrides });
const data = (items = [item(1)], overrides: Partial<NotificationPage> = {}): NotificationPage => ({ items, page: 1, page_size: 20, total: items.length, has_next: false, unread_count: items.filter((notification) => !notification.read_at).length, ...overrides });
function Location() { const location = useLocation(); return <div aria-label="当前路径">{location.pathname}{location.search}</div>; }
function mount(path = "/notifications") { return render(<MemoryRouter initialEntries={[path]}><NotificationCenterPage /><Location /></MemoryRouter>); }
function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>((done) => { resolve = done; }); return { promise, resolve }; }

describe("NotificationCenterPage", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(api.notifications).mockResolvedValue(data());
    vi.mocked(api.markNotificationRead).mockResolvedValue({ read: true });
    vi.mocked(api.markAllNotificationsRead).mockResolvedValue({ read: true });
  });
  afterEach(() => { cleanup(); vi.restoreAllMocks(); });

  it("读取时显示加载状态且不能全部标记，仍请求每页20条", async () => {
    const pending = deferred<NotificationPage>(); vi.mocked(api.notifications).mockReturnValue(pending.promise);
    mount();
    expect(screen.getByRole("status").textContent).toContain("正在读取通知");
    expect((screen.getByRole("button", { name: "全部标记已读" }) as HTMLButtonElement).disabled).toBe(true);
    const query = vi.mocked(api.notifications).mock.calls[0][0]!;
    expect(query.get("page_size")).toBe("20"); expect(query.get("page")).toBe("1");
    await act(async () => pending.resolve(data()));
    expect(screen.getByText("1 条未读通知")).toBeTruthy();
  });

  it("加载错误使用可访问反馈，不伪称空消息流", async () => {
    vi.mocked(api.notifications).mockRejectedValue(new Error("读取通知失败")); mount();
    expect((await screen.findByRole("alert")).textContent).toBe("读取通知失败");
    expect(screen.queryByRole("heading", { name: "暂时没有通知" })).toBeNull();
  });

  it("空消息流使用editorial说明，无新增CTA", async () => {
    vi.mocked(api.notifications).mockResolvedValue(data([])); mount();
    expect(await screen.findByRole("heading", { name: "暂时没有通知" })).toBeTruthy();
    expect(screen.getByText("ACTIVITY / 00")).toBeTruthy();
    expect(screen.getByText("新的关注、互动和处理状态会显示在这里。")).toBeTruthy();
    expect(screen.queryAllByRole("link")).toHaveLength(0);
  });

  it("六类型同时显示文字标签与图标，并保持视频/用户目标优先级", async () => {
    const types: Notification["type"][] = ["follow", "like", "favorite", "comment", "processing_ready", "processing_failed"];
    const items = types.map((type, index) => item(index + 1, { type, ...(type === "follow" ? { video_id: undefined } : {}), comment_preview: type === "comment" ? "评论内容" : undefined }));
    vi.mocked(api.notifications).mockResolvedValue(data(items)); mount();
    expect(await screen.findByText("6 条未读通知")).toBeTruthy();
    const feed = screen.getByRole("region", { name: "通知列表" });
    for (const [index, label] of ["关注", "点赞", "收藏", "评论", "处理完成", "处理失败"].entries()) {
      const row = within(feed).getAllByRole("article")[index];
      expect(within(row).getByText(label)).toBeTruthy(); expect(row.querySelector(".gv-notification-icon svg")).toBeTruthy();
      expect(within(row).getByRole("link").getAttribute("href")).toBe(index === 0 ? "/users/9" : "/video/3");
      expect(row.querySelector("time")?.getAttribute("datetime")).toBe(items[index].created_at);
    }
    expect(screen.getByText("评论内容")).toBeTruthy();
  });

  it("notificationCopy仍提供缺少actor/title/comment时的原业务文案", () => {
    expect(notificationCopy(item(1, { type: "comment", actor_username: "", video_title: "", comment_preview: "" }))).toMatchObject({ title: "有用户 评论了《你的视频》", detail: "查看这条新评论" });
    expect(notificationCopy(item(2, { type: "processing_failed", comment_preview: "" })).detail).toBe("请检查视频并重新尝试");
  });

  it("单条标记期间反馈busy、禁用冲突操作，完成后同步全局bell", async () => {
    const pending = deferred<{ read: boolean }>(); vi.mocked(api.markNotificationRead).mockReturnValue(pending.promise);
    const changed = vi.fn(); window.addEventListener("gvideo-notifications-changed", changed);
    mount(); fireEvent.click(await screen.findByRole("button", { name: "标记已读" }));
    expect(screen.getByRole("status").textContent).toBe("正在标记已读");
    expect(screen.getByRole("article").getAttribute("aria-busy")).toBe("true");
    expect((screen.getByRole("button", { name: "标记已读" }) as HTMLButtonElement).disabled).toBe(true);
    expect((screen.getByRole("button", { name: "全部标记已读" }) as HTMLButtonElement).disabled).toBe(true);
    await act(async () => pending.resolve({ read: true }));
    expect(api.markNotificationRead).toHaveBeenCalledWith(1); expect(changed).toHaveBeenCalledOnce();
    expect(screen.getByText("所有通知都已读")).toBeTruthy(); expect(screen.queryByRole("button", { name: "标记已读" })).toBeNull();
    window.removeEventListener("gvideo-notifications-changed", changed);
  });

  it("点击未读通知继续目标跳转并标记，已读条目不重复请求", async () => {
    vi.mocked(api.notifications).mockResolvedValue(data([item(1), item(2, { type: "follow", video_id: undefined, read_at: "2026-01-02T00:00:00Z" })])); mount();
    fireEvent.click(await screen.findByRole("link", { name: "创作者小李 点赞了《测试视频》" }));
    await waitFor(() => expect(api.markNotificationRead).toHaveBeenCalledWith(1));
    expect(screen.getByLabelText("当前路径").textContent).toBe("/video/3");
    fireEvent.click(screen.getByRole("link", { name: "创作者小李 关注了你" }));
    expect(api.markNotificationRead).toHaveBeenCalledOnce(); expect(screen.getByLabelText("当前路径").textContent).toBe("/users/9");
  });

  it("全部标记期间可见busy，完成后所有当前条目已读且同步全局事件", async () => {
    const pending = deferred<{ read: boolean }>(); vi.mocked(api.markAllNotificationsRead).mockReturnValue(pending.promise);
    const changed = vi.fn(); window.addEventListener("gvideo-notifications-changed", changed);
    vi.mocked(api.notifications).mockResolvedValue(data([item(1), item(2)], { unread_count: 24 })); mount();
    await screen.findByText("24 条未读通知"); fireEvent.click(screen.getByRole("button", { name: "全部标记已读" }));
    expect((screen.getByRole("button", { name: "正在标记全部已读" }) as HTMLButtonElement).disabled).toBe(true);
    expect(screen.getAllByRole("button", { name: "标记已读" }).every((button) => (button as HTMLButtonElement).disabled)).toBe(true);
    await act(async () => pending.resolve({ read: true }));
    expect(api.markAllNotificationsRead).toHaveBeenCalledOnce(); expect(changed).toHaveBeenCalledOnce();
    expect(screen.queryAllByText("未读")).toHaveLength(0); expect(screen.getByText("所有通知都已读")).toBeTruthy();
    window.removeEventListener("gvideo-notifications-changed", changed);
  });

  it.each(["one", "all"] as const)("%s 标记失败保留未读并显示错误", async (operation) => {
    const method = operation === "one" ? api.markNotificationRead : api.markAllNotificationsRead;
    vi.mocked(method).mockRejectedValue(new Error("标记失败，请稍后重试")); mount();
    await screen.findByText("1 条未读通知"); fireEvent.click(screen.getByRole("button", { name: operation === "one" ? "标记已读" : "全部标记已读" }));
    expect((await screen.findByRole("alert")).textContent).toBe("标记失败，请稍后重试");
    expect(screen.getByText("1 条未读通知")).toBeTruthy();
    expect((screen.getByRole("button", { name: "标记已读" }) as HTMLButtonElement).disabled).toBe(false);
  });

  it("分页继续使用page URL及20条请求，返回第一页清除page参数", async () => {
    vi.mocked(api.notifications).mockImplementation(async (params) => data([item(1)], { page: Number(params?.get("page")), total: 21, has_next: params?.get("page") === "1" }));
    mount(); await screen.findByText("1 条未读通知");
    fireEvent.click(screen.getByRole("button", { name: "下一页" }));
    await waitFor(() => expect(screen.getByLabelText("当前路径").textContent).toBe("/notifications?page=2"));
    await waitFor(() => expect((screen.getByRole("button", { name: "下一页" }) as HTMLButtonElement).disabled).toBe(true));
    expect(vi.mocked(api.notifications).mock.calls.at(-1)![0]!.get("page_size")).toBe("20");
    fireEvent.click(screen.getByRole("button", { name: "上一页" }));
    await waitFor(() => expect(screen.getByLabelText("当前路径").textContent).toBe("/notifications"));
  });

  it("长标题及评论原文保留，没有内容裁切或伪造目标", async () => {
    const title = "中英文_ABCDEFGHIJKLMNOPQRSTUVWXYZ_0123456789_".repeat(20), comment = "这是评论预览_".repeat(80);
    const notification = item(1, { type: "comment", video_id: undefined, actor_id: undefined, video_title: title, comment_preview: comment });
    vi.mocked(api.notifications).mockResolvedValue(data([notification])); mount();
    const link = await screen.findByRole("link", { name: notificationCopy(notification).title });
    expect(link.getAttribute("href")).toBe("/notifications"); expect(screen.getByText(comment)).toBeTruthy();
  });
});
