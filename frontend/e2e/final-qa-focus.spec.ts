import { expect, test, type Page } from "@playwright/test";
import { pageFixture, videoFixture } from "../src/shared/test/videoFixtures";

// Category A: every API read and mutation is intercepted; unknown API requests fail closed.
// FileChooser inputs are browser-only fixtures, never uploads to the development environment.
const image = { name: "focus-fixture.png", mimeType: "image/png", buffer: Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgQIAeJSM9QAAAABJRU5ErkJggg==", "base64") };
const user = { id: 42, username: "隔离键盘验收", avatar_url: "", bio: "", is_admin: false, created_at: "2026-01-01T00:00:00Z" };

async function fixture(page: Page, options: { anonymous?: boolean } = {}) {
  const items = Array.from({ length: 12 }, (_, index) => videoFixture(index + 1, { user_id: 42, username: user.username, title: `隔离焦点作品 ${index + 1}` }));
  let read = false;
  let finishDelete!: () => void, finishSubtitle!: () => void, finishRead!: (failure?: boolean) => void, finishAuth!: () => void;
  const deleting = new Promise<void>((resolve) => { finishDelete = resolve; });
  const subtitles = new Promise<void>((resolve) => { finishSubtitle = resolve; });
  const reading = new Promise<boolean | undefined>((resolve) => { finishRead = resolve; });
  const authenticating = new Promise<void>((resolve) => { finishAuth = resolve; });
  const mutations: string[] = [];
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request(), url = new URL(request.url()), path = url.pathname;
    let data: unknown;
    if (request.method() !== "GET") mutations.push(`${request.method()} ${path}`);
    if (path === "/api/v1/auth/me") {
      if (options.anonymous) { await route.fulfill({ status: 401, json: { error: "Unauthenticated isolated focus fixture" } }); return; }
      data = { user, csrf_token: "focus-test-csrf" };
    }
    else if (path === "/api/v1/auth/login" || path === "/api/v1/auth/register") { await authenticating; await route.fulfill({ status: 500, json: { error: "隔离认证失败" } }); return; }
    else if (path === "/api/v1/categories") data = ["音乐"];
    else if (path === "/api/v1/videos") data = pageFixture(items);
    else if (path === "/api/v1/me/videos" || path === "/api/v1/users/42/videos") data = pageFixture(items, { page: Number(url.searchParams.get("page") ?? 1), page_size: 12, total: 240, has_next: true });
    else if (path === "/api/v1/users/42") data = { ...user, followed: false, videos_count: 240, followers_count: 0, following_count: 0 };
    else if (path === "/api/v1/me/notifications") data = { items: [{ id: 1, type: "processing_ready", video_id: 1, video_title: "隔离焦点作品", read_at: read ? "2026-01-01T00:00:00Z" : undefined, created_at: "2026-01-01T00:00:00Z" }], page: 1, page_size: Number(url.searchParams.get("page_size") ?? 20), total: 1, has_next: false, unread_count: read ? 0 : 1 };
    else if (path === "/api/v1/me/notifications/1/read" && request.method() === "PATCH") {
      const failure = await reading;
      if (failure) { await route.fulfill({ status: 500, json: { error: "隔离已读失败" } }); return; }
      read = true; data = { read: true };
    } else if (path === "/api/v1/me/notifications/read-all" && request.method() === "POST") {
      const failure = await reading;
      if (failure) { await route.fulfill({ status: 500, json: { error: "隔离全部已读失败" } }); return; }
      read = true; data = { read: true };
    } else if (path === "/api/v1/videos/1" && request.method() === "DELETE") { await deleting; data = { deleted: true }; }
    else if (path === "/api/v1/videos/1/subtitles" && request.method() === "POST") { await subtitles; data = { id: 101, language: "zh-CN", label: "中文", url: "/isolated-focus.vtt", is_default: true }; }
    else { await route.fulfill({ status: 501, json: { error: "Blocked unexpected isolated focus request" } }); return; }
    await route.fulfill({ json: { data } });
  });
  return { finishDelete, finishSubtitle, finishRead, finishAuth, mutations };
}

async function bounded(page: Page, selector: string) {
  await expect.poll(() => page.locator(selector).evaluate((element) => {
    const box = element.getBoundingClientRect();
    return box.left >= -1 && box.top >= -1 && box.right <= innerWidth + 1 && box.bottom <= innerHeight + 1;
  })).toBe(true);
}

test("busy Delete and Subtitle keep dialog focus through Tab, Shift+Tab and Escape", async ({ page }) => {
  const state = await fixture(page); await page.goto("/me/videos");
  const trigger = page.locator(".gv-content-menu-trigger").first(); await trigger.click();
  await page.getByRole("menuitem", { name: "删除", exact: true }).click();
  const deleting = page.getByRole("alertdialog"); await deleting.getByRole("button", { name: "确认删除", exact: true }).press("Enter");
  await expect(deleting.getByRole("button", { name: "正在删除...", exact: true })).toBeDisabled(); await expect(deleting).toBeFocused();
  for (const key of ["Tab", "Shift+Tab", "Escape"]) { await page.keyboard.press(key); await expect(deleting).toBeVisible(); await expect(deleting).toBeFocused(); }
  await page.locator(".dialog-backdrop").click({ position: { x: 2, y: 2 } }); await expect(deleting).toBeVisible();
  state.finishDelete(); await expect(deleting).toBeHidden(); await expect(page.getByRole("heading", { name: "我的投稿", exact: true })).toBeFocused();
  await trigger.click(); await page.getByRole("menuitem", { name: "字幕管理", exact: true }).click();
  const subtitle = page.getByRole("dialog", { name: "字幕管理", exact: true });
  await subtitle.getByLabel("字幕文件").setInputFiles({ name: "focus.vtt", mimeType: "text/vtt", buffer: Buffer.from("WEBVTT\n\n00:00:00.000 --> 00:00:01.000\n焦点验收") });
  await subtitle.getByRole("button", { name: "上传轨道", exact: true }).press("Enter");
  await expect(subtitle.getByRole("button", { name: "关闭字幕管理", exact: true })).toBeDisabled(); await expect(subtitle).toBeFocused();
  for (const key of ["Tab", "Shift+Tab", "Escape"]) { await page.keyboard.press(key); await expect(subtitle).toBeVisible(); await expect(subtitle).toBeFocused(); }
  state.finishSubtitle(); await expect(subtitle.getByRole("status")).toContainText("字幕轨道已上传");
  await page.keyboard.press("Escape"); await expect(trigger).toBeFocused();
  expect(state.mutations).toEqual(["DELETE /api/v1/videos/1", "POST /api/v1/videos/1/subtitles"]);
});

test("Edit Video and Profile skip invisible files while keyboard buttons still open real FileChooser", async ({ page }) => {
  await fixture(page); await page.goto("/me/videos"); await page.locator(".gv-content-menu-trigger").first().click();
  await page.getByRole("menuitem", { name: "编辑视频", exact: true }).click();
  const editing = page.getByRole("dialog", { name: "编辑视频信息", exact: true });
  await expect(editing.locator('input[type="file"]')).toHaveAttribute("tabindex", "-1");
  const replace = editing.getByRole("button", { name: "替换封面", exact: true }); await replace.focus(); await page.keyboard.press("Tab");
  await expect(editing.getByLabel(/^标题/)).toBeFocused();
  const coverChooser = page.waitForEvent("filechooser"); await replace.press("Enter"); await (await coverChooser).setFiles(image);
  await expect(editing.getByText("已选择新封面", { exact: true })).toBeVisible();
  await page.keyboard.press("Escape"); await expect(page.locator(".gv-content-menu-trigger").first()).toBeFocused();
  await page.goto("/users/42"); const profileTrigger = page.getByRole("button", { name: "编辑资料", exact: true }); await profileTrigger.click();
  const profile = page.getByRole("dialog", { name: "编辑作者信息", exact: true });
  await expect(profile.locator('input[type="file"]')).toHaveAttribute("tabindex", "-1");
  const avatar = profile.getByRole("button", { name: "更换头像", exact: true }); await avatar.focus(); await page.keyboard.press("Tab");
  await expect(profile.getByLabel("用户名", { exact: true })).toBeFocused();
  const avatarChooser = page.waitForEvent("filechooser"); await avatar.press("Enter"); await (await avatarChooser).setFiles(image);
  expect(await profile.locator('input[type="file"]').evaluate((element) => (element as HTMLInputElement).files?.[0]?.name)).toBe(image.name);
  await page.keyboard.press("Escape"); await expect(profileTrigger).toBeFocused();
});

test("mobile Creator and Content pagination retains 44px targets and fits 320, 390 and 768", async ({ page }) => {
  await fixture(page);
  for (const width of [320, 390, 768]) {
    await page.setViewportSize({ width, height: width === 768 ? 1024 : 844 });
    for (const route of ["/me/videos?page=10", "/users/42?page=10"]) {
      await page.goto(route); await expect(page.locator(".pagination")).toBeVisible();
      for (const button of await page.locator(".pagination button").all()) {
        const box = await button.boundingBox(); expect(box!.width).toBeGreaterThanOrEqual(44); expect(box!.height).toBeGreaterThanOrEqual(44);
      }
      expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width + 1);
    }
  }
});

test("Content menu ArrowUp, wrap, Home, End, collision, resize, scroll and close restore focus", async ({ page }) => {
  await fixture(page); await page.setViewportSize({ width: 320, height: 720 }); await page.goto("/me/videos");
  const first = page.locator(".gv-content-menu-trigger").first(); await first.focus(); await first.press("ArrowUp");
  await expect(page.getByRole("menuitem", { name: "删除", exact: true })).toBeFocused();
  await page.keyboard.press("ArrowDown"); await expect(page.getByRole("menuitem", { name: "编辑视频", exact: true })).toBeFocused();
  await page.keyboard.press("End"); await expect(page.getByRole("menuitem", { name: "删除", exact: true })).toBeFocused();
  await page.keyboard.press("Home"); await expect(page.getByRole("menuitem", { name: "编辑视频", exact: true })).toBeFocused();
  await bounded(page, ".gv-content-menu"); await page.keyboard.press("Tab"); await expect(first).toBeFocused(); await expect(page.getByRole("menu")).toBeHidden();
  const last = page.locator(".gv-content-menu-trigger").last(); await last.scrollIntoViewIfNeeded(); await last.click(); await bounded(page, ".gv-content-menu");
  for (const width of [390, 920, 320]) { await page.setViewportSize({ width, height: 900 }); await bounded(page, ".gv-content-menu"); }
  await page.mouse.wheel(0, -200); await bounded(page, ".gv-content-menu");
  await page.keyboard.press("Escape"); await expect(last).toBeFocused();
  await last.click(); await page.locator(".gv-content-summary").click(); await expect(page.getByRole("menu")).toBeHidden();
});

test("keyboard notification read keeps focus on its link during busy and after button removal", async ({ page }) => {
  const state = await fixture(page); await page.goto("/notifications");
  const target = page.getByRole("link", { name: "《隔离焦点作品》已处理完成", exact: true });
  const button = page.getByRole("button", { name: "标记已读", exact: true }); await button.press("Enter");
  await expect(button).toBeDisabled(); await expect(target).toBeFocused(); state.finishRead();
  await expect(button).toBeHidden(); await expect(target).toBeFocused(); await expect(page.getByText("所有通知都已读", { exact: true })).toBeVisible();
  expect(state.mutations).toEqual(["PATCH /api/v1/me/notifications/1/read"]);
});

test("failed keyboard notification read returns focus to retry without losing its unread state", async ({ page }) => {
  const state = await fixture(page); await page.goto("/notifications");
  const button = page.getByRole("button", { name: "标记已读", exact: true }); await button.press("Enter");
  await expect(page.getByRole("link", { name: "《隔离焦点作品》已处理完成", exact: true })).toBeFocused();
  state.finishRead(true); await expect(page.getByRole("alert")).toHaveText("隔离已读失败");
  await expect(button).toBeFocused(); await expect(button).toBeEnabled(); await expect(page.getByText("1 条未读通知", { exact: true })).toBeVisible();
});

test("keyboard read-all keeps busy and success focus on the notification heading", async ({ page }) => {
  const state = await fixture(page); await page.goto("/notifications");
  const button = page.getByRole("button", { name: "全部标记已读", exact: true }); await button.press("Enter");
  const heading = page.getByRole("heading", { name: "通知中心", exact: true }); await expect(heading).toBeFocused();
  await expect(page.getByRole("button", { name: "正在标记全部已读", exact: true })).toBeDisabled(); state.finishRead();
  await expect(page.getByText("所有通知都已读", { exact: true })).toBeVisible(); await expect(button).toBeDisabled(); await expect(heading).toBeFocused();
  expect(state.mutations).toEqual(["POST /api/v1/me/notifications/read-all"]);
});

test("failed keyboard read-all restores retry-button focus with unread notifications intact", async ({ page }) => {
  const state = await fixture(page); await page.goto("/notifications");
  const button = page.getByRole("button", { name: "全部标记已读", exact: true }); await button.press("Enter");
  await expect(page.getByRole("heading", { name: "通知中心", exact: true })).toBeFocused(); state.finishRead(true);
  await expect(page.getByRole("alert")).toHaveText("隔离全部已读失败"); await expect(button).toBeFocused(); await expect(button).toBeEnabled();
  await expect(page.getByText("1 条未读通知", { exact: true })).toBeVisible();
});

for (const mode of ["login", "register"] as const) {
  test(`${mode} keyboard submission keeps busy focus and restores the original field on failure`, async ({ page }) => {
    const state = await fixture(page, { anonymous: true }); await page.goto("/auth?next=%2Fvideo%2F1");
    if (mode === "register") await page.getByRole("button", { name: "注册", exact: true }).click();
    await page.getByLabel("用户名", { exact: true }).fill("focus_test"); const password = page.getByLabel("密码", { exact: true }); await password.fill("password123"); await password.press("Enter");
    const form = page.getByRole("form"); await expect(form).toHaveAttribute("aria-busy", "true"); await expect(form).toBeFocused();
    await expect(page.getByRole("button", { name: "处理中...", exact: true })).toBeDisabled(); state.finishAuth();
    await expect(page.getByRole("alert")).toHaveText("隔离认证失败"); await expect(password).toBeFocused();
    await expect(password).toHaveValue("password123"); await expect(page.getByLabel("用户名", { exact: true })).toHaveValue("focus_test");
    expect(state.mutations).toEqual([`POST /api/v1/auth/${mode}`]);
  });
}

test("open mobile drawer traps keyboard then desktop resize returns focus to visible main", async ({ page }) => {
  await fixture(page); await page.setViewportSize({ width: 390, height: 844 }); await page.goto("/latest");
  await page.getByRole("button", { name: "打开菜单", exact: true }).press("Enter");
  const drawer = page.getByRole("dialog", { name: "导航菜单", exact: true });
  const first = drawer.getByRole("link", { name: "GVideo 首页", exact: true }); await expect(first).toBeFocused();
  const last = drawer.getByRole("button", { name: "退出登录", exact: true }); await page.keyboard.press("Shift+Tab"); await expect(last).toBeFocused();
  await page.keyboard.press("Tab"); await expect(first).toBeFocused();
  await page.setViewportSize({ width: 1440, height: 900 }); await expect(drawer).toBeHidden();
  await expect(page.getByRole("main")).toBeFocused(); expect(await page.getByRole("main").evaluate((element) => element.inert)).toBe(false);
  expect(await page.evaluate(() => document.body.style.overflow)).toBe("");
});
