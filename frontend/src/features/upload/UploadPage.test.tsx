// @vitest-environment jsdom
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { UploadPage } from "./UploadPage";

vi.mock("../../shared/api/client", () => ({
  api: {
    categories: vi.fn().mockResolvedValue(["音乐", "生活"]),
    upload: vi.fn().mockResolvedValue({ id: 1 })
  }
}));

describe("UploadPage", () => {
  it("渲染发布表单骨架", async () => {
    render(
      <MemoryRouter>
        <UploadPage />
      </MemoryRouter>
    );
    expect(await screen.findByRole("heading", { name: "发布新作品" })).toBeTruthy();
    // 分区列表来自 api.categories
    expect(await screen.findByText("音乐")).toBeTruthy();
    // 可见范围帮助文案来自本地 visibilityHelp 常量
    expect(screen.getByText("会出现在首页、作者空间、搜索和关注动态中。")).toBeTruthy();
  });
});
