// @vitest-environment jsdom
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { CreatorDashboard } from "./CreatorDashboard";

vi.mock("../../shared/api/client", () => ({
  api: {
    creatorStats: vi.fn().mockResolvedValue({
      videos_count: 5,
      followers_count: 12,
      views_count: 100,
      likes_count: 20,
      favorites_count: 8,
      comments_count: 6,
      public_count: 4,
      unlisted_count: 1,
      private_count: 0,
      processing_count: 0,
      recent_videos: []
    })
  }
}));

describe("CreatorDashboard", () => {
  it("渲染创作数据概览", async () => {
    render(
      <MemoryRouter>
        <CreatorDashboard user={{ id: 1, username: "自己", bio: "", avatar_url: "", is_admin: false, created_at: "2024-01-01T00:00:00Z" }} />
      </MemoryRouter>
    );
    expect(await screen.findByRole("heading", { name: "作品与观众概览" })).toBeTruthy();
    expect(await screen.findByRole("heading", { name: "可见范围" })).toBeTruthy();
    expect(screen.getByText("还没有投稿")).toBeTruthy();
  });
});
