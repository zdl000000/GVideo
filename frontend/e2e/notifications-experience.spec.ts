import { expect, test, type Page } from "@playwright/test";
import { pageFixture, videoFixture } from "../src/shared/test/videoFixtures";
import type { Notification } from "../src/types";

const viewports = [[1536, 960], [1440, 900], [1180, 820], [920, 900], [768, 1024], [430, 932], [390, 844], [320, 720]];
const notificationTypes: Notification["type"][] = ["follow", "like", "favorite", "comment", "processing_ready", "processing_failed"];
const title = "长作品标题_ABCDEFGHIJKLMNOPQRSTUVWXYZ_0123456789_".repeat(8);

// Every API response and mutation is isolated. No real notification, account or video is changed.
async function fixture(page: Page, options: { empty?: boolean; error?: boolean; loading?: boolean; slowMutation?: boolean; mutationError?: boolean; long?: boolean; compact?: boolean } = {}) {
  let items: Notification[] = Array.from({ length: 21 }, (_, index) => ({
    id: index + 1, type: notificationTypes[index % 6], actor_id: 9, actor_username: "山间放映室", video_id: index % 6 === 0 ? undefined : 65,
    video_title: options.long && index < 6 ? title : "沿着海岸线，看见日常以外的风景", comment_preview: index % 6 === 3 ? (options.long ? "评论预览_中英文_ABCDEFGHIJKLMNOPQRSTUVWXYZ_".repeat(12) : "喜欢这里的光线和节奏，期待下一段旅程。") : undefined,
    read_at: index < 6 ? undefined : "2026-10-01T07:00:00Z", created_at: "2026-10-01T06:20:00Z"
  }));
  if (options.compact) items = items.slice(0, 6);
  if (options.empty) items = [];
  let marks = 0; let allMarks = 0; let bellReads = 0;
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request(), url = new URL(request.url()), path = url.pathname; let data: unknown;
    if (request.method() !== "GET") expect(request.headers()["x-csrf-token"]).toBe("notification-test-csrf");
    if (path === "/api/v1/auth/me") data = { user: { id: 42, username: "通知验收用户", avatar_url: "", bio: "", is_admin: false, created_at: "2026-01-01T00:00:00Z" }, csrf_token: "notification-test-csrf" };
    else if (path === "/api/v1/categories") data = ["旅行", "音乐"];
    else if (path === "/api/v1/me/notifications" && request.method() === "GET") {
      const size = Number(url.searchParams.get("page_size")), pageNumber = Number(url.searchParams.get("page") ?? 1);
      expect([1, 20]).toContain(size);
      if (size === 1) bellReads++;
      if (options.loading && size === 20) await new Promise((resolve) => setTimeout(resolve, 1000));
      if (options.error && size === 20) { await route.fulfill({ status: 500, json: { error: "隔离验收：通知读取失败" } }); return; }
      data = { items: items.slice((pageNumber - 1) * size, pageNumber * size), page: pageNumber, page_size: size, total: items.length, has_next: pageNumber * size < items.length, unread_count: items.filter((item) => !item.read_at).length };
    } else if (path === "/api/v1/me/notifications/read-all" && request.method() === "POST") {
      allMarks++;
      if (options.slowMutation) await new Promise((resolve) => setTimeout(resolve, 1000));
      if (options.mutationError) { await route.fulfill({ status: 500, json: { error: "隔离验收：标记失败" } }); return; }
      items = items.map((item) => ({ ...item, read_at: item.read_at || "2026-10-02T10:00:00Z" })); data = { read: true };
    } else if (/\/me\/notifications\/\d+\/read$/.test(path) && request.method() === "PATCH") {
      marks++;
      if (options.slowMutation) await new Promise((resolve) => setTimeout(resolve, 1000));
      if (options.mutationError) { await route.fulfill({ status: 500, json: { error: "隔离验收：标记失败" } }); return; }
      const id = Number(path.split("/")[5]); items = items.map((item) => item.id === id ? { ...item, read_at: "2026-10-02T10:00:00Z" } : item); data = { read: true };
    } else if (path === "/api/v1/videos/65") data = videoFixture(65, { title: "隔离通知目标视频", user_id: 9, username: "山间放映室", video_url: "", hls_url: "", cover_url: "", subtitle_tracks: [] });
    else if (path === "/api/v1/videos/65/comments") data = [];
    else if (path === "/api/v1/videos") data = pageFixture([]);
    else if (path === "/api/v1/users/9") data = { id: 9, username: "山间放映室", avatar_url: "", bio: "隔离通知目标作者", created_at: "2026-01-01T00:00:00Z", videos_count: 0, followers_count: 0, following_count: 0, followed: false };
    else if (path === "/api/v1/users/9/videos") data = pageFixture([]);
    else { await route.fulfill({ status: 404, json: { error: "unexpected isolated notification request" } }); return; }
    await route.fulfill({ json: { data } });
  });
  return { marks: () => marks, allMarks: () => allMarks, bellReads: () => bellReads };
}

async function theme(page: Page, value: "light" | "dark") {
  if (await page.locator("html").getAttribute("data-theme") === value) return;
  if (await page.locator(".mobile-menu-button").isVisible()) {
    await page.locator(".mobile-menu-button").click(); await page.locator(".mobile-account .theme-row").click(); await page.getByRole("dialog", { name: "导航菜单" }).press("Escape");
  } else await page.getByRole("button", { name: `切换到${value === "light" ? "浅" : "深"}色主题`, exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", value);
}

test("notification types, unread feed and global bell stay synchronized during single and all read", async ({ page }) => {
  const state = await fixture(page, { slowMutation: true }); await page.goto("/notifications");
  await expect(page.getByText("6 条未读通知", { exact: true })).toBeVisible();
  await expect(page.locator(".notification-button")).toHaveAttribute("aria-label", "通知中心，6 条未读");
  for (const label of ["关注", "点赞", "收藏", "评论", "处理完成", "处理失败"]) await expect(page.locator(".gv-notification-meta > span:first-child").filter({ hasText: new RegExp(`^${label}$`) }).first()).toBeVisible();
  const first = page.locator(".gv-notification-row").first(); await first.getByRole("button", { name: "标记已读" }).click();
  await expect(first).toHaveAttribute("aria-busy", "true"); await expect(first.getByRole("status")).toHaveText("正在标记已读");
  await expect(page.getByRole("button", { name: "全部标记已读" })).toBeDisabled();
  await expect(page.locator(".notification-button")).toHaveAttribute("aria-label", "通知中心，5 条未读");
  await expect(first).toHaveClass(/\bread\b/); expect(state.marks()).toBe(1);
  await page.getByRole("button", { name: "全部标记已读" }).click(); await expect(page.getByRole("button", { name: "正在标记全部已读" })).toBeDisabled();
  await expect(page.locator(".notification-button")).toHaveAttribute("aria-label", "通知中心，0 条未读");
  await expect(page.locator(".gv-notification-row.unread")).toHaveCount(0); await expect(page.getByText("所有通知都已读", { exact: true })).toBeVisible();
  expect(state.allMarks()).toBe(1); expect(state.bellReads()).toBeGreaterThanOrEqual(3);
});

test("notification target links retain video and creator routes and mark on click", async ({ page }) => {
  const state = await fixture(page); await page.goto("/notifications");
  const videoResponse = page.waitForResponse((response) => new URL(response.url()).pathname === "/api/v1/videos/65" && response.status() === 200);
  await page.locator(".gv-notification-row--like").first().getByRole("link").click(); await expect(page).toHaveURL(/\/video\/65$/);
  await videoResponse; await expect.poll(state.marks).toBe(1);
  await expect(page.getByRole("heading", { name: "隔离通知目标视频", exact: true })).toBeVisible({ timeout: 15_000 });
  await page.goto("/notifications"); await page.locator(".gv-notification-row--follow").first().getByRole("link").click();
  await expect(page).toHaveURL(/\/users\/9$/); await expect.poll(state.marks).toBe(2);
});

test("notification pagination retains 20 items and URL page query", async ({ page }) => {
  await fixture(page); await page.goto("/notifications"); await expect(page.locator(".gv-notification-row")).toHaveCount(20);
  const pagination = page.getByRole("navigation", { name: "通知分页" }); await pagination.getByRole("button", { name: "下一页" }).click();
  await expect(page).toHaveURL(/\/notifications\?page=2$/); await expect(page.locator(".gv-notification-row")).toHaveCount(1);
  await expect(pagination.getByRole("button", { name: "下一页" })).toBeDisabled(); await pagination.getByRole("button", { name: "上一页" }).click();
  await expect(page).toHaveURL(/\/notifications$/); await expect(page.locator(".gv-notification-row")).toHaveCount(20);
});

test("notification loading, read failure and compact empty feed provide real feedback", async ({ page }) => {
  await fixture(page, { loading: true, mutationError: true }); await page.goto("/notifications");
  await expect(page.getByRole("status")).toContainText("正在读取通知");
  await page.locator(".gv-notification-row").first().getByRole("button", { name: "标记已读" }).click();
  await expect(page.getByRole("alert")).toHaveText("隔离验收：标记失败"); await expect(page.getByText("6 条未读通知", { exact: true })).toBeVisible();
  await page.unroute("**/api/v1/**"); await fixture(page, { empty: true }); await page.goto("/notifications");
  await expect(page.getByRole("heading", { name: "暂时没有通知" })).toBeVisible();
  await expect(page.locator(".gv-notification-empty").getByRole("link")).toHaveCount(0);
  await expect(page.locator(".gv-notification-empty").getByRole("button")).toHaveCount(0);
  await page.unroute("**/api/v1/**"); await fixture(page, { error: true }); await page.goto("/notifications");
  await expect(page.getByRole("alert")).toHaveText("隔离验收：通知读取失败");
});

test("notification dark and warm light use saved themes and eight viewports safely wrap long content", async ({ page }) => {
  test.setTimeout(180_000); await fixture(page, { long: true });
  for (const value of ["light", "dark"] as const) {
    for (const [width, height] of viewports) {
      await page.setViewportSize({ width, height }); await page.goto("/notifications"); await theme(page, value);
      await expect(page.locator(".gv-notification-row")).toHaveCount(20);
      // Dark WATCH inherits its visible canvas from body; the warm light surface is page-scoped on the Shell.
      await expect(page.locator(value === "light" ? ".gv-watch-shell" : "body")).toHaveCSS("background-color", value === "light" ? "rgb(238, 237, 233)" : "rgb(8, 10, 13)");
      expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width + 1);
      const row = page.locator(".gv-notification-row").first(); const action = await row.getByRole("button", { name: "标记已读" }).boundingBox();
      expect(action!.width).toBeGreaterThanOrEqual(44); expect(action!.height).toBeGreaterThanOrEqual(44);
      const link = row.getByRole("link"); await link.focus(); await expect(link).toBeFocused(); await page.keyboard.press("Tab"); await expect(row.getByRole("button", { name: "标记已读" })).toBeFocused();
    }
    await page.reload(); await expect(page.locator("html")).toHaveAttribute("data-theme", value);
  }
});

test("capture Phase 5 notification acceptance screenshots using isolated API fixtures", async ({ page }, testInfo) => {
  test.skip(process.env.PHASE5_SCREENSHOTS !== "1" || testInfo.project.name !== "desktop-chromium", "Optional acceptance artifacts, collected only with PHASE5_SCREENSHOTS=1 on desktop-chromium");
  test.setTimeout(120_000);
  testInfo.annotations.push({ type: "fixture", description: "Screenshots use six isolated notification API fixtures; no real messages or account mutations." });
  const output = "../tmp/phase5-screenshots";
  const capture = async (name: string) => {
    await page.evaluate(() => { window.scrollTo(0, 0); (document.activeElement as HTMLElement | null)?.blur(); });
    await page.mouse.move(0, 0);
    await page.screenshot({ path: `${output}/${name}`, fullPage: true });
  };
  await page.setViewportSize({ width: 1440, height: 900 }); await fixture(page, { compact: true }); await page.goto("/notifications");
  await expect(page.locator(".gv-notification-row")).toHaveCount(6); await theme(page, "dark");
  // The overview shows acknowledged activity. The dedicated unread shot below shows all six types unread.
  await page.getByRole("button", { name: "全部标记已读" }).click(); await expect(page.locator(".gv-notification-row.unread")).toHaveCount(0);
  await capture("09-notifications-dark.png");
  await theme(page, "light"); await capture("10-notifications-light.png");
  await page.unroute("**/api/v1/**"); await fixture(page, { compact: true }); await page.goto("/notifications"); await theme(page, "dark");
  await expect(page.locator(".gv-notification-row.unread")).toHaveCount(6);
  await capture("11-notifications-unread.png");
  await page.setViewportSize({ width: 390, height: 844 }); await theme(page, "light");
  await capture("13-notifications-mobile390.png");
  await page.setViewportSize({ width: 1440, height: 900 }); await page.unroute("**/api/v1/**"); await fixture(page, { empty: true }); await page.goto("/notifications");
  await expect(page.getByRole("heading", { name: "暂时没有通知" })).toBeVisible();
  await capture("12-notifications-empty.png");
});
