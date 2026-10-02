// @vitest-environment jsdom
import { afterAll, afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { api } from "../../shared/api/client";
import { CreatorDashboard } from "./CreatorDashboard";
import { videoFixture } from "../../shared/test/videoFixtures";
import type { CreatorStats } from "../../types";

vi.hoisted(() => vi.resetModules());
vi.mock("../../shared/api/client", () => ({ api: { creatorStats: vi.fn() } }));
const stats: CreatorStats = { videos_count: 5, followers_count: 12, views_count: 125430, likes_count: 20, favorites_count: 8, comments_count: 6, public_count: 3, unlisted_count: 1, private_count: 1, processing_count: 2, recent_videos: [videoFixture(3, { cover_url: "/broken.jpg" })] };
const user = { id: 1, username: "自己", bio: "", avatar_url: "", is_admin: false, created_at: "2024-01-01T00:00:00Z" };
const mount = () => render(<MemoryRouter><CreatorDashboard user={user} /></MemoryRouter>);
beforeEach(() => { vi.mocked(api.creatorStats).mockReset().mockResolvedValue(stats); });
afterEach(cleanup);
afterAll(() => vi.resetModules());
describe("CreatorDashboard", () => {
  it("displays exact aggregate numbers, real navigation and separate overlapping processing count", async () => {
    mount();
    expect(await screen.findByRole("heading", { name: "作品与观众概览" })).toBeTruthy();
    expect(screen.getByText("125,430")).toBeTruthy();
    expect(document.querySelector(".gv-studio-total strong")?.textContent).toBe("5");
    expect([...document.querySelectorAll(".gv-studio-status dd")].map((el) => el.textContent)).toEqual(["3", "1", "1"]);
    expect(document.querySelector(".gv-studio-processing")?.textContent).toContain("2 条投稿正在处理");
    expect(screen.getByRole("link", { name: "个人空间" }).getAttribute("href")).toBe("/users/1");
    expect(screen.getByRole("link", { name: "管理投稿" }).getAttribute("href")).toBe("/me/videos");
    expect(screen.queryByText(/30 天|增长|收入|存储空间/)).toBeNull();
  });
  it("renders recent content and switches a failed cover to the shared placeholder", async () => {
    mount(); await screen.findByRole("link", { name: "作品3" });
    fireEvent.error(document.querySelector(".gv-cover-image")!);
    expect(document.querySelector(".gv-video-cover--failed")).toBeTruthy();
    expect(document.querySelector(".gv-cover-image")).toBeNull();
    expect(screen.getByText("已完成")).toBeTruthy();
  });
  it("zero aggregates show onboarding without invented completion steps", async () => {
    vi.mocked(api.creatorStats).mockResolvedValue({ ...stats, videos_count: 0, followers_count: 0, views_count: 0, likes_count: 0, favorites_count: 0, comments_count: 0, public_count: 0, unlisted_count: 0, private_count: 0, processing_count: 0, recent_videos: [] });
    mount(); expect(await screen.findByRole("heading", { name: "还没有投稿" })).toBeTruthy();
    expect(screen.getByRole("link", { name: "发布第一条作品" })).toBeTruthy();
    expect(screen.getByText("当前没有正在处理的投稿")).toBeTruthy();
  });
  it("nonzero aggregate with no recent results stays truthful", async () => {
    vi.mocked(api.creatorStats).mockResolvedValue({ ...stats, recent_videos: [] }); mount();
    expect(await screen.findByText("暂无最近投稿。")).toBeTruthy();
    expect(screen.queryByText("还没有投稿")).toBeNull();
  });
  it("shows loading until the stats resolve", () => {
    vi.mocked(api.creatorStats).mockReturnValue(new Promise(() => {})); mount();
    expect(screen.getByText("正在整理创作数据")).toBeTruthy();
  });
  it("shows API errors without fabricated aggregate values", async () => {
    vi.mocked(api.creatorStats).mockRejectedValue(new Error("统计服务不可用")); mount();
    expect(await screen.findByText("统计服务不可用")).toBeTruthy();
    expect(document.querySelector(".gv-studio-performance")).toBeNull();
  });
});
