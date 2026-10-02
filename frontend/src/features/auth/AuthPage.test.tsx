// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { AuthPage, AuthRedirect } from "./AuthPage";
import { api } from "../../shared/api/client";
import type { AuthPayload } from "../../types";

vi.mock("../../shared/api/client", () => ({ api: { login: vi.fn(), register: vi.fn() } }));

const payload: AuthPayload = { user: { id: 1, username: "测试用户", bio: "", avatar_url: "", is_admin: false, created_at: "2026-01-01T00:00:00Z" }, csrf_token: "token-1" };

function Destination() {
  const location = useLocation();
  return <p data-testid="destination">{location.pathname}{location.search}</p>;
}

function renderAuth(entry = "/auth", onAuth = vi.fn()) {
  return { onAuth, ...render(<MemoryRouter initialEntries={[entry]}><Routes><Route path="/auth" element={<AuthPage onAuth={onAuth} />} /><Route path="*" element={<Destination />} /></Routes></MemoryRouter>) };
}

function fillCredentials() {
  fireEvent.change(screen.getByLabelText("用户名"), { target: { value: "测试用户" } });
  fireEvent.change(screen.getByLabelText("密码"), { target: { value: "password123" } });
}

describe("AuthPage", () => {
  beforeEach(() => { vi.resetAllMocks(); vi.mocked(api.login).mockResolvedValue(payload); vi.mocked(api.register).mockResolvedValue(payload); });
  afterEach(cleanup);

  it("uses a brand narrative and compact form with accessible mode buttons", () => {
    renderAuth();
    expect(screen.getByRole("heading", { level: 1, name: "让内容连接更大的世界。" })).toBeTruthy();
    expect(screen.getByText("发现创作")).toBeTruthy();
    expect(screen.getByText("分享作品")).toBeTruthy();
    expect(screen.getByText("参与讨论")).toBeTruthy();
    expect(screen.getByRole("heading", { name: "登录 GVideo" })).toBeTruthy();
    const modes = screen.getByRole("group", { name: "登录或注册" });
    expect(within(modes).getByRole("button", { name: "登录" }).getAttribute("aria-pressed")).toBe("true");
    expect(within(modes).getByRole("button", { name: "注册" }).getAttribute("aria-pressed")).toBe("false");
    expect(screen.getAllByRole("button", { name: "登录" })).toHaveLength(2);
  });

  it("keeps username/password limits and correct autocomplete across mode switches", () => {
    renderAuth(); fillCredentials();
    const username = screen.getByLabelText("用户名") as HTMLInputElement;
    const password = screen.getByLabelText("密码") as HTMLInputElement;
    expect(username.autocomplete).toBe("username"); expect(username.minLength).toBe(3); expect(username.maxLength).toBe(24);
    expect(username.getAttribute("autocapitalize")).toBe("none");
    expect(password.autocomplete).toBe("current-password"); expect(password.minLength).toBe(8); expect(password.maxLength).toBe(72);
    fireEvent.click(screen.getByRole("button", { name: "注册" }));
    expect(screen.getByRole("heading", { name: "创建 GVideo 账号" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "创建账号" })).toBeTruthy();
    expect(password.autocomplete).toBe("new-password"); expect(username.value).toBe("测试用户"); expect(password.value).toBe("password123");
    fireEvent.click(within(screen.getByRole("group", { name: "登录或注册" })).getByRole("button", { name: "登录" }));
    expect(password.autocomplete).toBe("current-password");
  });

  it("logs in using the existing API and passes the complete auth payload", async () => {
    const { onAuth } = renderAuth(); fillCredentials(); fireEvent.submit(screen.getByRole("form"));
    await waitFor(() => expect(onAuth).toHaveBeenCalledWith(payload));
    expect(api.login).toHaveBeenCalledWith("测试用户", "password123"); expect(api.register).not.toHaveBeenCalled();
    expect(screen.getByTestId("destination").textContent).toBe("/");
  });

  it("registers using the existing API and callback", async () => {
    const { onAuth } = renderAuth(); fireEvent.click(screen.getByRole("button", { name: "注册" })); fillCredentials(); fireEvent.submit(screen.getByRole("form"));
    await waitFor(() => expect(onAuth).toHaveBeenCalledWith(payload));
    expect(api.register).toHaveBeenCalledWith("测试用户", "password123"); expect(api.login).not.toHaveBeenCalled();
  });

  it("announces busy and prevents duplicate submissions or mode changes while pending", async () => {
    let resolve!: (value: AuthPayload) => void;
    vi.mocked(api.login).mockImplementation(() => new Promise((done) => { resolve = done; }));
    renderAuth(); fillCredentials(); fireEvent.submit(screen.getByRole("form"));
    expect(screen.getByRole("form").getAttribute("aria-busy")).toBe("true");
    expect((screen.getByRole("button", { name: "处理中..." }) as HTMLButtonElement).disabled).toBe(true);
    expect((screen.getByRole("button", { name: "注册" }) as HTMLButtonElement).disabled).toBe(true);
    expect(screen.getByRole("status").textContent).toContain("正在登录");
    fireEvent.submit(screen.getByRole("form")); expect(api.login).toHaveBeenCalledTimes(1);
    await act(async () => resolve(payload));
    expect(screen.getByTestId("destination").textContent).toBe("/");
  });

  it("keeps credentials and exposes API failures in an alert without authenticating", async () => {
    vi.mocked(api.login).mockRejectedValueOnce(new Error("用户名或密码错误"));
    const { onAuth } = renderAuth(); fillCredentials(); fireEvent.submit(screen.getByRole("form"));
    expect((await screen.findByRole("alert")).textContent).toBe("用户名或密码错误"); expect(onAuth).not.toHaveBeenCalled();
    expect((screen.getByLabelText("用户名") as HTMLInputElement).value).toBe("测试用户");
    expect((screen.getByLabelText("密码") as HTMLInputElement).value).toBe("password123");
    expect(screen.getByRole("form").getAttribute("aria-busy")).toBe("false");
    expect((within(screen.getByRole("form")).getByRole("button", { name: "登录" }) as HTMLButtonElement).disabled).toBe(false);
    fireEvent.submit(screen.getByRole("form")); await screen.findByTestId("destination"); expect(api.login).toHaveBeenCalledTimes(2);
  });

  for (const mode of ["login", "register"] as const) {
    it.each([["/video/1", "/video/1"], ["/video/1?from=auth", "/video/1?from=auth"], ["//evil.com", "/"], ["http://evil.com", "/"]])(`${mode} accepts internal next and rejects external next: %s`, async (next, expected) => {
      renderAuth(`/auth?next=${encodeURIComponent(next)}`);
      if (mode === "register") fireEvent.click(screen.getByRole("button", { name: "注册" }));
      fillCredentials(); fireEvent.submit(screen.getByRole("form"));
      expect((await screen.findByTestId("destination")).textContent).toBe(expected);
    });
  }
});

// Keep the pre-existing signed-in redirect security assertions alongside successful auth.
describe("AuthRedirect", () => {
  afterEach(cleanup);
  it.each([["/upload", "/upload"], ["/video/1", "/video/1"], ["//evil.example", "/"], ["https://evil.example", "/"], ["//evil.com", "/"], ["http://evil.com", "/"]])("already-auth redirect validates next: %s", (next, expected) => {
    render(<MemoryRouter initialEntries={[`/auth?next=${encodeURIComponent(next)}`]}><Routes><Route path="/auth" element={<AuthRedirect />} /><Route path="*" element={<Destination />} /></Routes></MemoryRouter>);
    expect(screen.getByTestId("destination").textContent).toBe(expected);
  });
});
