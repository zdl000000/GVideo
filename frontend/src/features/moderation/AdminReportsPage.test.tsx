// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { AdminReportsPage } from "./AdminReportsPage";

vi.mock("../../shared/api/client", () => ({
  api: {
    adminReports: vi.fn().mockResolvedValue({
      items: [
        {
          id: 7,
          video_id: 3,
          video_title: "测试视频",
          video_author_id: 5,
          video_author: "创作者小张",
          user_id: 9,
          reporter_username: "举报人小王",
          reason: "spam",
          detail: "",
          status: "pending",
          created_at: "2026-01-01T00:00:00Z",
          updated_at: "2026-01-02T00:00:00Z"
        }
      ],
      page: 1,
      page_size: 20,
      total: 1,
      has_next: false
    }),
    reviewReport: vi.fn().mockResolvedValue({
      id: 7, video_id: 3, video_title: "测试视频", video_author_id: 5, video_author: "创作者小张",
      user_id: 9, reporter_username: "举报人小王", reason: "spam", detail: "", status: "reviewed",
      created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-02T00:00:00Z"
    })
  }
}));

describe("AdminReportsPage", () => {
  afterEach(cleanup);

  it("渲染页面骨架与状态筛选", async () => {
    render(<MemoryRouter><AdminReportsPage /></MemoryRouter>);
    expect(await screen.findByRole("heading", { name: "举报审核" })).toBeTruthy();
    // 工具栏是带 aria-label 的 div，用 getByLabelText 查询
    expect(screen.getByLabelText("举报状态筛选")).toBeTruthy();
    expect(screen.getByRole("button", { name: "全部" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "待处理" })).toBeTruthy();
  });

  it("渲染举报记录行", async () => {
    render(<MemoryRouter><AdminReportsPage /></MemoryRouter>);
    // reportReasonLabels 把 spam 翻译成 垃圾信息
    expect(await screen.findByText("垃圾信息")).toBeTruthy();
    expect(screen.getByRole("link", { name: "测试视频" })).toBeTruthy();
    expect(screen.getByLabelText("处理举报 #7")).toBeTruthy();
    expect(screen.getByRole("button", { name: "处理完成" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "驳回" })).toBeTruthy();
  });
});
