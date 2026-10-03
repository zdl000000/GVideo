// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { CreatorPage } from "./CreatorPage";
import { api } from "../../shared/api/client";
import { pageFixture, videoFixture } from "../../shared/test/videoFixtures";
import type { User } from "../../types";

afterEach(() => { cleanup(); vi.restoreAllMocks(); });

vi.mock("../../shared/api/client", () => ({
  api: {
    creator: vi.fn().mockResolvedValue({
      id: 42,
      username: "测试作者",
      bio: "",
      avatar_url: "",
      created_at: "2024-01-01T00:00:00Z",
      followers_count: 3,
      following_count: 4,
      videos_count: 0,
      followed: false
    }),
    creatorVideos: vi.fn().mockResolvedValue({ items: [], page: 1, page_size: 12, total: 0, has_next: false }),
    toggleFollow: vi.fn().mockResolvedValue({ active: true }),
    updateProfile: vi.fn().mockResolvedValue({ id: 42, username: "更新作者", bio: "更新简介", avatar_url: "", is_admin: false, created_at: "2024-01-01T00:00:00Z" })
  }
}));

const viewer: User = { id: 1, username: "测试用户", bio: "", avatar_url: "", is_admin: false, created_at: "2024-01-01T00:00:00Z" };
function Destination() { const location = useLocation(); return <p>{location.pathname + location.search}</p>; }
function openChannel(user: User | null = null, onUserUpdated = vi.fn()) {
  render(<MemoryRouter initialEntries={["/users/42"]}><Routes><Route path="/users/:id" element={<CreatorPage user={user} onUserUpdated={onUserUpdated} />} /><Route path="/auth" element={<Destination />} /></Routes></MemoryRouter>);
  return onUserUpdated;
}

describe("CreatorPage", () => {
  it("渲染作者空间骨架", async () => {
    render(
      <MemoryRouter initialEntries={["/users/42"]}>
        <Routes>
          <Route path="/users/:id" element={<CreatorPage user={null} onUserUpdated={() => {}} />} />
        </Routes>
      </MemoryRouter>
    );
    expect(await screen.findByRole("heading", { name: "测试作者" })).toBeTruthy();
    expect(await screen.findByRole("heading", { name: "测试作者 的作品" })).toBeTruthy();
  });
  it("他人频道保持无简介文案、真实统计和关注登录回跳", async () => {
    openChannel();
    await screen.findByRole("heading", { name: "测试作者" });
    expect(screen.getByText("这位创作者还没有填写个人简介。")).toBeTruthy();
    expect(screen.getByRole("heading", { name: "还没有公开投稿" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "编辑资料" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "关注" }));
    expect(await screen.findByText("/auth?next=%2Fusers%2F42")).toBeTruthy();
  });

  it("关注和取消关注更新 pressed 状态与粉丝计数", async () => {
    vi.mocked(api.toggleFollow).mockResolvedValueOnce({ active: true }).mockResolvedValueOnce({ active: false });
    openChannel(viewer);
    fireEvent.click(await screen.findByRole("button", { name: "关注" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "已关注" }).getAttribute("aria-pressed")).toBe("true"));
    expect(document.querySelector(".creator-stats div:nth-child(2) dd")?.textContent).toBe("4");
    fireEvent.click(screen.getByRole("button", { name: "已关注" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "关注" }).getAttribute("aria-pressed")).toBe("false"));
    expect(document.querySelector(".creator-stats div:nth-child(2) dd")?.textContent).toBe("3");
  });

  it("本人频道保留工作入口、资料保存契约和焦点返回", async () => {
    const updated = openChannel({ ...viewer, id: 42 });
    const edit = await screen.findByRole("button", { name: "编辑资料" });
    expect(screen.queryByRole("button", { name: "关注" })).toBeNull();
    expect(screen.getByRole("link", { name: "管理投稿" }).getAttribute("href")).toBe("/me/videos");
    expect(screen.getByRole("link", { name: "创作者中心" }).getAttribute("href")).toBe("/creator");
    fireEvent.click(edit);
    await screen.findByRole("dialog", { name: "编辑作者信息" });
    expect(document.querySelector<HTMLInputElement>('input[type="file"]')?.tabIndex).toBe(-1);
    fireEvent.change(screen.getByLabelText("用户名"), { target: { value: "更新作者" } });
    fireEvent.change(screen.getByRole("textbox", { name: /^个人简介/ }), { target: { value: "更新简介" } });
    fireEvent.click(screen.getByRole("button", { name: "保存资料" }));
    expect(await screen.findByRole("heading", { name: "更新作者" })).toBeTruthy();
    const sent = vi.mocked(api.updateProfile).mock.calls.at(-1)![0];
    expect(sent.get("username")).toBe("更新作者");
    expect(sent.get("bio")).toBe("更新简介");
    expect(updated).toHaveBeenCalled();
    await waitFor(() => expect(document.activeElement).toBe(edit));
    fireEvent.click(edit);
    await screen.findByRole("dialog");
    fireEvent.keyDown(document, { key: "Escape" });
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await waitFor(() => expect(document.activeElement).toBe(edit));
  });

  it("作者作品使用 standard 卡片并保留分页参数", async () => {
    vi.spyOn(window, "scrollTo").mockImplementation(() => undefined);
    vi.mocked(api.creatorVideos).mockResolvedValueOnce(pageFixture([videoFixture(1)], { total: 13, page_size: 12, has_next: true })).mockResolvedValueOnce(pageFixture([videoFixture(13)], { total: 13, page: 2, page_size: 12 }));
    openChannel();
    await screen.findByRole("link", { name: "作品1" });
    expect(document.querySelectorAll(".gv-video-card--standard")).toHaveLength(1);
    fireEvent.click(screen.getByRole("button", { name: "下一页" }));
    await screen.findByRole("link", { name: "作品13" });
    expect(vi.mocked(api.creatorVideos).mock.calls.at(-1)![1]!.get("page")).toBe("2");
    expect(vi.mocked(api.creatorVideos).mock.calls.at(-1)![1]!.get("page_size")).toBe("12");
  });
});
