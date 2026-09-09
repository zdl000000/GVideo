// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { AuthPage } from "./AuthPage";

vi.mock("../../shared/api/client", () => ({
  api: {
    login: vi.fn().mockResolvedValue({ user: { id: 1, username: "测试用户", bio: "", avatar_url: "", is_admin: false, created_at: "2026-01-01T00:00:00Z" }, csrf_token: "token-1" }),
    register: vi.fn().mockResolvedValue({ user: { id: 1, username: "测试用户", bio: "", avatar_url: "", is_admin: false, created_at: "2026-01-01T00:00:00Z" }, csrf_token: "token-1" })
  }
}));

describe("AuthPage", () => {
  afterEach(cleanup);

  it("渲染页面骨架，含 auth-intro 与 auth-steps 标记", async () => {
    render(<MemoryRouter><AuthPage onAuth={vi.fn()} /></MemoryRouter>);
    expect(await screen.findByRole("heading", { name: "每一次上传，都是一段时间被认真留下。" })).toBeTruthy();
    expect(screen.getByText("加入片场")).toBeTruthy();
    // auth-steps 三个步骤
    expect(screen.getByText("发现创作")).toBeTruthy();
    expect(screen.getByText("分享作品")).toBeTruthy();
    expect(screen.getByText("参与讨论")).toBeTruthy();
    // 模式切换按钮与提交按钮的可访问名都是“登录”
    expect(screen.getAllByRole("button", { name: "登录" })).toHaveLength(2);
  });

  it("切换到注册模式后按钮文案变化", async () => {
    const { fireEvent } = await import("@testing-library/react");
    render(<MemoryRouter><AuthPage onAuth={vi.fn()} /></MemoryRouter>);
    await screen.findAllByRole("button", { name: "登录" });
    fireEvent.click(screen.getByRole("button", { name: "注册" }));
    expect(screen.getByRole("button", { name: "创建账号" })).toBeTruthy();
  });
});
