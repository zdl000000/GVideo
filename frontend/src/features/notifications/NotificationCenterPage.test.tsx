// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { NotificationCenterPage } from "./NotificationCenterPage";

vi.mock("../../shared/api/client", () => ({
  api: {
    notifications: vi.fn().mockResolvedValue({
      items: [
        {
          id: 1,
          type: "like",
          actor_id: 9,
          actor_username: "创作者小李",
          video_id: 3,
          video_title: "测试视频",
          created_at: "2026-01-01T00:00:00Z"
        }
      ],
      page: 1,
      page_size: 20,
      total: 1,
      has_next: false,
      unread_count: 1
    }),
    markNotificationRead: vi.fn().mockResolvedValue({ read: true }),
    markAllNotificationsRead: vi.fn().mockResolvedValue({ read: true })
  }
}));

describe("NotificationCenterPage", () => {
  afterEach(cleanup);

  it("渲染页面骨架与未读计数", async () => {
    render(<MemoryRouter><NotificationCenterPage /></MemoryRouter>);
    expect(await screen.findByRole("heading", { name: "通知中心" })).toBeTruthy();
    expect(screen.getByText("1 条未读通知")).toBeTruthy();
    expect(screen.getByRole("button", { name: "全部标记已读" })).toBeTruthy();
  });

  it("渲染通知条目文案", async () => {
    render(<MemoryRouter><NotificationCenterPage /></MemoryRouter>);
    // notificationCopy 生成的标题
    expect(await screen.findByRole("link", { name: "创作者小李 点赞了《测试视频》" })).toBeTruthy();
    expect(screen.getByText("你的作品收到了一个赞")).toBeTruthy();
  });
});
