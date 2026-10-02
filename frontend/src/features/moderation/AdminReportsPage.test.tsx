// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router-dom";
import { api } from "../../shared/api/client";
import type { VideoReport, VideoReportPage, VideoReportStatus } from "../../types";
import { AdminReportsPage, reportReasonLabels, reportStatusLabels } from "./AdminReportsPage";

vi.mock("../../shared/api/client", () => ({ api: { adminReports: vi.fn(), reviewReport: vi.fn() } }));

function report(overrides: Partial<VideoReport> = {}): VideoReport {
  return { id: 7, video_id: 3, video_title: "测试视频", video_author_id: 5, video_author: "创作者小张", user_id: 9, reporter_username: "举报人小王", reason: "spam", detail: "", status: "pending", created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-02T00:00:00Z", ...overrides };
}
function result(items: VideoReport[] = [report()], overrides: Partial<VideoReportPage> = {}): VideoReportPage {
  return { items, page: 1, page_size: 20, total: items.length, has_next: false, ...overrides };
}
function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
}
function LocationProbe() { const location = useLocation(); return <div data-testid="query">{location.pathname}{location.search}</div>; }
function mount(url = "/admin/reports") { return render(<MemoryRouter initialEntries={[url]}><AdminReportsPage /><LocationProbe /></MemoryRouter>); }
function actions() { return screen.getByRole("group", { name: "处理举报 #7" }); }

describe("AdminReportsPage", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(api.adminReports).mockResolvedValue(result());
    vi.mocked(api.reviewReport).mockImplementation(async (_id, status) => report({ status }));
  });
  afterEach(cleanup);

  it("渲染治理页面、原有五项状态筛选与真实总数", async () => {
    mount();
    expect(screen.getByRole("heading", { name: "举报审核" })).toBeTruthy();
    const filters = screen.getByRole("group", { name: "举报状态筛选" });
    expect(within(filters).getAllByRole("button").map((item) => item.textContent)).toEqual(["全部", "待处理", "审核中", "已处理", "已驳回"]);
    expect(within(filters).getByRole("button", { name: "全部" }).getAttribute("aria-pressed")).toBe("true");
    await screen.findByRole("article");
    expect(document.querySelector(".gv-governance-count strong")?.textContent).toBe("1");
  });

  it("保留视频、举报人与作者链接以及创建和更新时间", async () => {
    mount();
    const row = await screen.findByRole("article", { name: "测试视频" });
    expect(within(row).getByRole("link", { name: "测试视频" }).getAttribute("href")).toBe("/video/3");
    expect(within(row).getByRole("link", { name: "举报人小王" }).getAttribute("href")).toBe("/users/9");
    expect(within(row).getByRole("link", { name: "创作者小张" }).getAttribute("href")).toBe("/users/5");
    expect(Array.from(row.querySelectorAll("time")).map((item) => item.dateTime)).toEqual(["2026-01-01T00:00:00Z", "2026-01-02T00:00:00Z"]);
    expect(within(row).getByText("举报人未填写补充说明")).toBeTruthy();
    expect(within(actions()).getByRole("button", { name: "处理完成" })).toBeTruthy();
    expect(within(actions()).getByRole("button", { name: "驳回" })).toBeTruthy();
  });

  it("四个状态均有文字、独立图标和状态 class，四个真实 reason 使用原映射", async () => {
    const statuses: VideoReportStatus[] = ["pending", "reviewed", "resolved", "dismissed"];
    const reasons = ["spam", "inappropriate", "copyright", "other"];
    vi.mocked(api.adminReports).mockResolvedValue(result(statuses.map((status, index) => report({ id: index + 1, status, reason: reasons[index] }))));
    mount();
    const rows = await screen.findAllByRole("article");
    const icons = new Set<string>();
    rows.forEach((row, index) => {
      const state = row.querySelector(".gv-report-status")!;
      expect(state.textContent).toBe(reportStatusLabels[statuses[index]]);
      expect(state.classList.contains(`gv-report-status--${statuses[index]}`)).toBe(true);
      const icon = state.querySelector("svg")!;
      expect(icon.getAttribute("aria-hidden")).toBe("true");
      icons.add(icon.innerHTML);
      expect(within(row).getByText(reportReasonLabels[reasons[index]])).toBeTruthy();
    });
    expect(icons.size).toBe(4);
  });

  it("无标题/用户名仍使用原 ID fallback，未知 reason 与长 detail 完整保留", async () => {
    const detail = "ABCDEFGHIJKLMNOPQRSTUVWXYZ_0123456789_".repeat(50);
    vi.mocked(api.adminReports).mockResolvedValue(result([report({ video_title: "", video_author: "", reporter_username: "", reason: "future_reason", detail })]));
    mount();
    const row = await screen.findByRole("article", { name: "视频 #3" });
    expect(within(row).getByRole("link", { name: "用户 #9" })).toBeTruthy();
    expect(within(row).getByRole("link", { name: "用户 #5" })).toBeTruthy();
    expect(within(row).getByText("future_reason")).toBeTruthy();
    expect(row.querySelector(".gv-report-detail")?.textContent).toBe(detail);
  });

  it("加载期间显示可读 status，并向读取 API 传递 AbortSignal", async () => {
    const request = deferred<VideoReportPage>();
    vi.mocked(api.adminReports).mockReturnValue(request.promise);
    const view = mount();
    expect(screen.getByRole("status").textContent).toContain("正在读取举报记录");
    const [query, signal] = vi.mocked(api.adminReports).mock.calls[0];
    expect(query?.get("page")).toBe("1"); expect(query?.get("page_size")).toBe("20");
    expect(signal).toBeInstanceOf(AbortSignal);
    view.unmount(); expect(signal?.aborted).toBe(true);
    await act(async () => request.resolve(result()));
  });

  it("从 URL 读取合法 status/page，并在切换筛选时重置 page 和取消旧读取", async () => {
    mount("/admin/reports?status=pending&page=4"); await screen.findByRole("article");
    const [query, signal] = vi.mocked(api.adminReports).mock.calls[0];
    expect(query?.get("status")).toBe("pending"); expect(query?.get("page")).toBe("4");
    fireEvent.click(within(screen.getByRole("group", { name: "举报状态筛选" })).getByRole("button", { name: "审核中" }));
    await waitFor(() => expect(api.adminReports).toHaveBeenCalledTimes(2));
    expect(signal?.aborted).toBe(true);
    const next = vi.mocked(api.adminReports).mock.calls[1][0];
    expect(next?.get("status")).toBe("reviewed"); expect(next?.get("page")).toBe("1");
    expect(screen.getByTestId("query").textContent).toBe("/admin/reports?status=reviewed");
  });

  it("非法 status 忽略、非法 page 回到第一页；全部筛选删除查询", async () => {
    mount("/admin/reports?status=invalid&page=-2"); await screen.findByRole("article");
    const query = vi.mocked(api.adminReports).mock.calls[0][0];
    expect(query?.has("status")).toBe(false); expect(query?.get("page")).toBe("1");
    fireEvent.click(within(screen.getByRole("group", { name: "举报状态筛选" })).getByRole("button", { name: "全部" }));
    await waitFor(() => expect(screen.getByTestId("query").textContent).toBe("/admin/reports"));
  });

  it.each(["reviewed", "resolved", "dismissed"] as const)("%s 调用原审核 API，当前筛选中的行原地保留并合并服务端返回值", async (status) => {
    vi.mocked(api.reviewReport).mockResolvedValue(report({ status, detail: "服务端返回的说明", updated_at: "2026-01-03T00:00:00Z" }));
    mount("/admin/reports?status=pending"); const row = await screen.findByRole("article");
    const labels = { reviewed: "审核中", resolved: "处理完成", dismissed: "驳回" };
    fireEvent.click(within(actions()).getByRole("button", { name: labels[status] }));
    await waitFor(() => expect(row.getAttribute("data-status")).toBe(status));
    expect(api.reviewReport).toHaveBeenCalledExactlyOnceWith(7, status);
    expect(screen.getAllByRole("article")).toHaveLength(1);
    expect(within(row).getByText("服务端返回的说明")).toBeTruthy();
    expect(screen.getByTestId("query").textContent).toBe("/admin/reports?status=pending");
    expect((within(actions()).getByRole("button", { name: labels[status] }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("更新期间有 busy 提示，禁用全部行操作且不提前修改状态", async () => {
    const request = deferred<VideoReport>(); vi.mocked(api.reviewReport).mockReturnValue(request.promise);
    vi.mocked(api.adminReports).mockResolvedValue(result([report(), report({ id: 8, video_title: "第二条视频" })]));
    mount(); const rows = await screen.findAllByRole("article");
    fireEvent.click(within(actions()).getByRole("button", { name: "处理完成" }));
    expect(rows[0].getAttribute("aria-busy")).toBe("true");
    expect(rows[0].getAttribute("data-status")).toBe("pending");
    expect(screen.getByRole("status").textContent).toBe("正在更新举报状态…");
    screen.getAllByRole("group", { name: /处理举报/ }).forEach((group) => within(group).getAllByRole("button").forEach((button) => expect((button as HTMLButtonElement).disabled).toBe(true)));
    fireEvent.click(within(actions()).getByRole("button", { name: "驳回" })); expect(api.reviewReport).toHaveBeenCalledTimes(1);
    await act(async () => request.resolve(report({ status: "resolved" })));
    expect(rows[0].getAttribute("aria-busy")).toBe("false");
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("更新失败显示 alert、保留原行状态并恢复操作", async () => {
    vi.mocked(api.reviewReport).mockRejectedValue(new Error("审核请求失败"));
    mount(); const row = await screen.findByRole("article");
    fireEvent.click(within(actions()).getByRole("button", { name: "驳回" }));
    expect((await screen.findByRole("alert")).textContent).toBe("审核请求失败");
    expect(row.getAttribute("data-status")).toBe("pending");
    expect((within(actions()).getByRole("button", { name: "驳回" }) as HTMLButtonElement).disabled).toBe(false);
  });

  it("列表读取失败有 alert，AbortError 不显示错误", async () => {
    vi.mocked(api.adminReports).mockRejectedValueOnce(new Error("读取举报失败"));
    const view = mount(); expect((await screen.findByRole("alert")).textContent).toBe("读取举报失败");
    view.unmount(); vi.mocked(api.adminReports).mockRejectedValueOnce(new DOMException("取消", "AbortError"));
    mount(); await screen.findByRole("heading", { name: "当前筛选下没有举报" });
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("筛选空状态无虚构 CTA；分页仍保留 status 并使用 page query", async () => {
    vi.mocked(api.adminReports).mockImplementation(async (query) => result([], { page: Number(query?.get("page") || 1), total: 41, has_next: Number(query?.get("page") || 1) < 3 }));
    mount("/admin/reports?status=reviewed");
    expect(await screen.findByRole("heading", { name: "当前筛选下没有举报" })).toBeTruthy();
    expect(document.querySelector(".gv-governance-empty a")).toBeNull();
    fireEvent.click(within(screen.getByRole("navigation", { name: "举报记录分页" })).getByRole("button", { name: "下一页" }));
    await waitFor(() => expect(screen.getByTestId("query").textContent).toBe("/admin/reports?status=reviewed&page=2"));
    await waitFor(() => expect(api.adminReports).toHaveBeenCalledTimes(2));
    const query = vi.mocked(api.adminReports).mock.calls[1][0];
    expect(query?.get("status")).toBe("reviewed"); expect(query?.get("page")).toBe("2"); expect(query?.get("page_size")).toBe("20");
  });
});
