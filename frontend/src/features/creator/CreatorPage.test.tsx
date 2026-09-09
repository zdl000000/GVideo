// @vitest-environment jsdom
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { CreatorPage } from "./CreatorPage";

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
    toggleFollow: vi.fn().mockResolvedValue({ active: true })
  }
}));

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
});
