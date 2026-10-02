import { expect, test, type Page } from "@playwright/test";
import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import { pageFixture, videoFixture } from "../src/shared/test/videoFixtures";

const viewports = [[1536, 960], [1440, 900], [1180, 820], [920, 900], [768, 1024], [430, 932], [390, 844], [320, 720]];
const credentials = { username: "auth_fixture", password: "password123" };
const user = { id: 42, username: "隔离验收用户", avatar_url: "", bio: "", is_admin: false, created_at: "2026-01-01T00:00:00Z" };

// Every API request is isolated, including login/register. No real account is created.
async function fixture(page: Page, options: { signedIn?: boolean; slow?: boolean; fail?: boolean } = {}) {
  let signedIn = Boolean(options.signedIn);
  const submissions: { mode: string; username: string; password: string }[] = [];
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request(); const path = new URL(request.url()).pathname; let data: unknown;
    if (path === "/api/v1/auth/me") {
      if (!signedIn) { await route.fulfill({ status: 401, json: { error: "unauthorized" } }); return; }
      data = { user, csrf_token: "auth-fixture-csrf" };
    } else if (path === "/api/v1/auth/login" || path === "/api/v1/auth/register") {
      expect(request.method()).toBe("POST"); expect(request.postDataJSON()).toEqual(credentials);
      submissions.push({ mode: path.endsWith("login") ? "login" : "register", ...request.postDataJSON() });
      if (options.slow) await new Promise((resolve) => setTimeout(resolve, 1200));
      if (options.fail) { await route.fulfill({ status: 400, json: { error: "隔离验收：用户名或密码错误" } }); return; }
      signedIn = true; data = { user, csrf_token: "auth-fixture-csrf" };
    } else {
      expect(request.method(), "unexpected mutation must never reach a real API").toBe("GET");
      if (path === "/api/v1/me/notifications") data = { ...pageFixture([], { page_size: 1 }), unread_count: 0 };
      else if (path === "/api/v1/categories") data = ["音乐", "游戏"];
      else if (path === "/api/v1/videos") data = pageFixture([]);
      else if (path === "/api/v1/videos/1") data = videoFixture(1, { title: "登录后的站内作品", video_url: "", hls_url: "", processing_status: "pending", processing_stage: "queued", processing_progress: 0 });
      else if (path === "/api/v1/videos/1/comments") data = [];
      else { await route.fulfill({ status: 404, json: { error: "unexpected isolated API request" } }); return; }
    }
    await route.fulfill({ json: { data } });
  });
  return { submissions };
}

async function fill(page: Page) { await page.getByLabel("用户名", { exact: true }).fill(credentials.username); await page.getByLabel("密码", { exact: true }).fill(credentials.password); }
async function noOverflow(page: Page) { expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual((await page.evaluate(() => innerWidth)) + 1); }

test("Auth modes expose correct autocomplete and retain credentials with keyboard access", async ({ page }) => {
  await fixture(page); await page.goto("/auth");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  await expect(page.getByRole("heading", { name: "登录 GVideo" })).toBeVisible(); await fill(page);
  await expect(page.getByLabel("用户名", { exact: true })).toHaveAttribute("autocomplete", "username");
  await expect(page.getByLabel("密码", { exact: true })).toHaveAttribute("autocomplete", "current-password");
  await page.getByRole("button", { name: "注册", exact: true }).click();
  await expect(page.getByRole("heading", { name: "创建 GVideo 账号" })).toBeVisible();
  await expect(page.getByLabel("密码", { exact: true })).toHaveAttribute("autocomplete", "new-password");
  await expect(page.getByLabel("用户名", { exact: true })).toHaveValue(credentials.username); await expect(page.getByLabel("密码", { exact: true })).toHaveValue(credentials.password);
  await page.getByLabel("用户名", { exact: true }).focus(); await page.keyboard.press("Tab"); await expect(page.getByLabel("密码", { exact: true })).toBeFocused();
  await page.keyboard.press("Tab"); await expect(page.getByRole("button", { name: "创建账号", exact: true })).toBeFocused();
  await noOverflow(page);
});

for (const mode of ["login", "register"] as const) {
  test(`Auth ${mode} allows an internal next and rejects protocol-relative or absolute external next`, async ({ page }) => {
    for (const next of ["/video/1", "//evil.com", "http://evil.com"]) {
      const state = await fixture(page); await page.goto(`/auth?next=${encodeURIComponent(next)}`);
      if (mode === "register") await page.getByRole("button", { name: "注册", exact: true }).click();
      await fill(page); await page.locator(".gv-auth-form").getByRole("button", { name: mode === "login" ? "登录" : "创建账号", exact: true }).click();
      await expect(page).toHaveURL(new RegExp(next === "/video/1" ? "/video/1$" : "/$"));
      expect(new URL(page.url()).origin).not.toContain("evil.com"); expect(state.submissions).toEqual([{ mode, ...credentials }]);
    }
  });
}

test("Already-auth access uses the same internal-only next redirect boundary", async ({ page }) => {
  const state = await fixture(page, { signedIn: true });
  for (const next of ["/video/1", "//evil.com", "http://evil.com"]) {
    await page.goto(`/auth?next=${encodeURIComponent(next)}`);
    await expect(page).toHaveURL(new RegExp(next === "/video/1" ? "/video/1$" : "/$"));
    expect(new URL(page.url()).origin).not.toContain("evil.com"); await expect(page.locator(".gv-auth-form")).toHaveCount(0);
  }
  expect(state.submissions).toHaveLength(0);
});

test("Auth pending locks duplicate actions and preserves credentials after an API error", async ({ page }) => {
  const state = await fixture(page, { slow: true, fail: true }); await page.goto("/auth"); await fill(page);
  await page.locator(".gv-auth-form").getByRole("button", { name: "登录", exact: true }).click();
  await expect(page.locator(".gv-auth-form")).toHaveAttribute("aria-busy", "true");
  await expect(page.getByRole("button", { name: "处理中..." })).toBeDisabled(); await expect(page.getByRole("button", { name: "注册", exact: true })).toBeDisabled();
  await expect(page.locator(".gv-auth-form-note")).toContainText("正在登录");
  await expect(page.getByRole("alert")).toContainText("用户名或密码错误");
  await expect(page.getByLabel("用户名", { exact: true })).toHaveValue(credentials.username); await expect(page.getByLabel("密码", { exact: true })).toHaveValue(credentials.password);
  await expect(page.locator(".gv-auth-form")).toHaveAttribute("aria-busy", "false"); expect(state.submissions).toHaveLength(1); await expect(page).toHaveURL(/\/auth$/);
});

test("Auth login/register fits eight viewports in both themes with first-screen mobile form", async ({ page }) => {
  test.setTimeout(120_000); await fixture(page); await page.goto("/auth");
  for (const value of ["dark", "light"] as const) {
    if (await page.locator("html").getAttribute("data-theme") !== value) await page.getByRole("button", { name: `切换到${value === "light" ? "浅" : "深"}色主题`, exact: true }).click();
    await page.reload(); await expect(page.locator("html")).toHaveAttribute("data-theme", value);
    await expect(page.locator(".gv-auth-shell")).toHaveCSS("background-color", value === "light" ? "rgb(247, 248, 250)" : "rgb(8, 10, 13)");
    for (const [width, height] of viewports) {
      await page.setViewportSize({ width, height });
      for (const mode of ["login", "register"] as const) {
        await page.locator(".gv-auth-mode").getByRole("button", { name: mode === "login" ? "登录" : "注册", exact: true }).click();
        await expect(page.locator(".gv-auth-form")).toBeVisible(); await noOverflow(page);
        for (const selector of [".gv-auth-mode button", ".gv-auth-form input", ".gv-auth-submit"]) {
          for (const element of await page.locator(selector).all()) { const box = await element.boundingBox(); expect(box!.height).toBeGreaterThanOrEqual(44); expect(box!.width).toBeGreaterThanOrEqual(44); }
        }
        if (width === 390) { const form = await page.locator(".gv-auth-form").boundingBox(); const cta = await page.locator(".gv-auth-submit").boundingBox(); expect(form!.y).toBeLessThan(height); expect(cta!.y + cta!.height).toBeLessThanOrEqual(height); }
        if (width >= 768) { const narrative = await page.locator(".gv-auth-narrative").boundingBox(); const panel = await page.locator(".gv-auth-form-panel").boundingBox(); expect(panel!.x).toBeGreaterThan(narrative!.x + narrative!.width); }
      }
    }
  }
});

test("Phase 5 visual collection: Auth login, register and mobile", async ({ page }, testInfo) => {
  test.skip(process.env.PHASE5_SCREENSHOTS !== "1" || testInfo.project.name !== "desktop-chromium", "Opt-in desktop screenshot collection only");
  const directory = resolve(process.cwd(), "../tmp/phase5-screenshots"); await mkdir(directory, { recursive: true });
  const capture = async (name: string) => {
    await page.evaluate(() => { window.scrollTo(0, 0); (document.activeElement as HTMLElement | null)?.blur(); });
    await page.mouse.move(0, 0);
    await page.screenshot({ path: resolve(directory, `${name}.png`), fullPage: true, animations: "disabled" });
  };
  await fixture(page); await page.setViewportSize({ width: 1440, height: 900 }); await page.goto("/auth");
  await page.getByRole("button", { name: "切换到深色主题", exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect(page.getByRole("heading", { name: "登录 GVideo" })).toBeVisible();
  await capture("06-auth-login-desktop");
  await page.getByRole("button", { name: "注册", exact: true }).click();
  await expect(page.getByRole("heading", { name: "创建 GVideo 账号" })).toBeVisible();
  await capture("07-auth-register-desktop");
  await page.locator(".gv-auth-mode").getByRole("button", { name: "登录", exact: true }).click();
  await page.getByRole("button", { name: "切换到浅色主题", exact: true }).click(); await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light"); await expect(page.locator(".gv-auth-form")).toBeVisible();
  const cta = await page.locator(".gv-auth-submit").boundingBox(); expect(cta!.y + cta!.height).toBeLessThanOrEqual(844); await noOverflow(page);
  await capture("08-auth-mobile390");
});
