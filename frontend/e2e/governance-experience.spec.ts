import { expect, test, type Page } from "@playwright/test";
import type { VideoReport, VideoReportPage, VideoReportStatus } from "../src/types";

const viewports = [[1536, 960], [1440, 900], [1180, 820], [920, 900], [768, 1024], [430, 932], [390, 844], [320, 720]];
const statuses: VideoReportStatus[] = ["pending", "reviewed", "resolved", "dismissed"];
const reasons = ["spam", "inappropriate", "copyright", "other"];
const statusLabels = ["待处理", "审核中", "已处理", "已驳回"];

// Every API request is fulfilled locally, including moderation mutations and
// route destinations. No account registration, upload or real report is changed.
async function governanceFixture(page: Page, options: { guest?: boolean; nonAdmin?: boolean; empty?: boolean; long?: boolean; paginate?: boolean; readError?: boolean; updateError?: boolean; slowRead?: boolean; holdUpdate?: boolean } = {}) {
  let items: VideoReport[] = Array.from({ length: options.paginate ? 25 : 4 }, (_, index) => ({
    id: 7 + index, video_id: 30 + index, video_title: options.long && index === 0 ? "超长视频标题_ABCDEFGHIJKLMNOPQRSTUVWXYZ_0123456789_".repeat(8) : `隔离治理作品 ${index + 1}`,
    video_author_id: 50 + index, video_author: `内容作者 ${index + 1}`, user_id: 90 + index, reporter_username: options.long && index === 0 ? "Reporter_ABCDEFGHIJKLMNOPQRSTUVWXYZ_0123456789" : `举报人 ${index + 1}`,
    reason: reasons[index % 4], detail: options.long && index === 0 ? "连续文本ABCDEFGHIJKLMNOPQRSTUVWXYZ_0123456789_".repeat(12) : index === 1 ? "请核查视频中的相关内容。此说明仅用于隔离界面验收。" : "",
    status: statuses[index % 4], created_at: "2026-01-01T09:30:00Z", updated_at: "2026-01-02T12:15:00Z"
  }));
  if (options.empty) items = [];
  const mutations: Array<{ id: number; status: VideoReportStatus }> = [];
  const reads: string[] = [];
  let finishUpdate: (() => void) | undefined;
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request(); const url = new URL(request.url()); const path = url.pathname;
    let data: unknown;
    if (request.method() !== "GET") expect(request.headers()["x-csrf-token"]).toBe("governance-test-csrf");
    if (path === "/api/v1/auth/me") {
      if (options.guest) { await route.fulfill({ status: 401, json: { error: "请先登录" } }); return; }
      data = { user: { id: 42, username: "隔离治理管理员", avatar_url: "", bio: "", is_admin: !options.nonAdmin, created_at: "2026-01-01T00:00:00Z" }, csrf_token: "governance-test-csrf" };
    } else if (path === "/api/v1/me/notifications") data = { items: [], page: 1, page_size: 1, total: 0, has_next: false, unread_count: 0 };
    else if (path === "/api/v1/categories") data = ["音乐", "游戏"];
    else if (path === "/api/v1/videos") data = { items: [], page: 1, page_size: 24, total: 0, has_next: false };
    else if (path === "/api/v1/admin/reports" && request.method() === "GET") {
      reads.push(url.search);
      expect(url.searchParams.get("page_size")).toBe("20");
      if (options.slowRead) await new Promise((resolve) => setTimeout(resolve, 800));
      if (options.readError) { await route.fulfill({ status: 500, json: { error: "隔离验收：举报列表读取失败" } }); return; }
      const requestedStatus = url.searchParams.get("status"); const currentPage = Number(url.searchParams.get("page") || 1);
      const filtered = requestedStatus ? items.filter((item) => item.status === requestedStatus) : items;
      const result: VideoReportPage = { items: filtered.slice((currentPage - 1) * 20, currentPage * 20), page: currentPage, page_size: 20, total: filtered.length, has_next: currentPage * 20 < filtered.length };
      data = result;
    } else if (/^\/api\/v1\/admin\/reports\/\d+$/.test(path) && request.method() === "PATCH") {
      const id = Number(path.split("/").at(-1)); const status = request.postDataJSON().status as VideoReportStatus;
      expect(statuses).toContain(status); mutations.push({ id, status });
      if (options.holdUpdate) await new Promise<void>((resolve) => { finishUpdate = resolve; });
      else await new Promise((resolve) => setTimeout(resolve, 600));
      if (options.updateError) { await route.fulfill({ status: 500, json: { error: "隔离验收：审核更新失败" } }); return; }
      items = items.map((item) => item.id === id ? { ...item, status, updated_at: "2026-01-03T12:15:00Z" } : item);
      data = items.find((item) => item.id === id);
    } else { await route.fulfill({ status: 404, json: { error: "unexpected isolated API request" } }); return; }
    await route.fulfill({ json: { data } });
  });
  return { mutations, reads, releaseUpdate: () => { expect(finishUpdate).toBeDefined(); finishUpdate!(); finishUpdate = undefined; } };
}

async function noOverflow(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(await page.evaluate(() => innerWidth + 1));
}
async function theme(page: Page, value: "light" | "dark") {
  if (await page.locator("html").getAttribute("data-theme") === value) return;
  if (await page.locator(".mobile-menu-button").isVisible()) {
    await page.locator(".mobile-menu-button").click(); await page.locator(".mobile-account .theme-row").click();
    await page.getByRole("dialog", { name: "导航菜单" }).press("Escape");
  } else await page.getByRole("button", { name: `切换到${value === "light" ? "浅" : "深"}色主题`, exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", value);
}

test("Governance guard still rejects a signed-in non-admin without reading reports", async ({ page }) => {
  const state = await governanceFixture(page, { nonAdmin: true }); await page.goto("/admin/reports");
  await expect(page).toHaveURL(/\/$/); await expect(page.locator(".gv-governance-page")).toHaveCount(0); expect(state.reads).toEqual([]);
});

test("Governance guest redirects through the original protected next URL", async ({ page }) => {
  const state = await governanceFixture(page, { guest: true }); await page.goto("/admin/reports?status=pending&page=2");
  await expect(page).toHaveURL(/\/auth\?next=%2Fadmin%2Freports%3Fstatus%3Dpending%26page%3D2$/); expect(state.reads).toEqual([]);
});

test("Governance filters and pagination keep status/page URL contract", async ({ page }) => {
  const state = await governanceFixture(page, { paginate: true }); await page.goto("/admin/reports");
  await expect(page.locator(".gv-report-row")).toHaveCount(20, { timeout: 20_000 });
  await page.getByRole("navigation", { name: "举报记录分页" }).getByRole("button", { name: "下一页" }).click();
  await expect(page).toHaveURL(/\/admin\/reports\?page=2$/); await expect(page.locator(".gv-report-row")).toHaveCount(5);
  const filters = page.getByRole("group", { name: "举报状态筛选" });
  for (let index = 0; index < statuses.length; index++) {
    await filters.getByRole("button", { name: statusLabels[index], exact: true }).click();
    await expect(page).toHaveURL(new RegExp(`/admin/reports\\?status=${statuses[index]}$`));
    await expect(page.locator(".gv-report-status").first()).toHaveText(statusLabels[index]);
    await expect(filters.getByRole("button", { name: statusLabels[index], exact: true })).toHaveAttribute("aria-pressed", "true");
  }
  await filters.getByRole("button", { name: "全部", exact: true }).click(); await expect(page).toHaveURL(/\/admin\/reports$/);
  expect(state.reads.some((query) => new URLSearchParams(query).get("page") === "2")).toBe(true);
});

test("Governance keeps all status/reason labels and mutation rows, with busy and touch actions", async ({ page }) => {
  const state = await governanceFixture(page, { holdUpdate: true }); await page.goto("/admin/reports");
  await expect(page.locator(".gv-report-row")).toHaveCount(4, { timeout: 20_000 });
  await expect(page.locator(".gv-report-status")).toHaveText(statusLabels);
  await expect(page.locator(".gv-report-reason")).toHaveText(["垃圾信息", "不当内容", "版权问题", "其他"]);
  await expect(page.locator(".gv-report-status > svg")).toHaveCount(4);
  await expect(page.locator(".gv-report-row").first().getByRole("link", { name: "举报人 1" })).toHaveAttribute("href", "/users/90");
  await page.getByRole("group", { name: "举报状态筛选" }).getByRole("button", { name: "待处理", exact: true }).click();
  await expect(page.locator(".gv-report-row")).toHaveCount(1);
  const row = page.locator(".gv-report-row").first();
  const choices = [["审核中", "reviewed"], ["处理完成", "resolved"], ["驳回", "dismissed"]] as const;
  for (const [label, status] of choices) {
    const action = row.getByRole("button", { name: label, exact: true }); const box = await action.boundingBox();
    expect(box!.height).toBeGreaterThanOrEqual(44); expect(box!.width).toBeGreaterThanOrEqual(44);
    await action.click(); await expect(row).toHaveAttribute("aria-busy", "true");
    await expect(row.getByRole("status")).toHaveText("正在更新举报状态…");
    for (const button of await row.getByRole("button").all()) await expect(button).toBeDisabled();
    state.releaseUpdate();
    await expect(row).toHaveAttribute("data-status", status); await expect(row).toHaveAttribute("aria-busy", "false");
    await expect(page.locator(".gv-report-row")).toHaveCount(1); await expect(page).toHaveURL(/status=pending$/);
  }
  expect(state.mutations).toEqual([{ id: 7, status: "reviewed" }, { id: 7, status: "resolved" }, { id: 7, status: "dismissed" }]);
});

test("Governance loading, read errors and update errors preserve visible feedback", async ({ page }) => {
  await governanceFixture(page, { readError: true, slowRead: true }); await page.goto("/admin/reports");
  await expect(page.getByRole("status")).toContainText("正在读取举报记录");
  await expect(page.getByRole("alert")).toHaveText("隔离验收：举报列表读取失败");
  await page.unroute("**/api/v1/**"); await governanceFixture(page, { updateError: true }); await page.reload();
  const row = page.locator(".gv-report-row").first(); await row.getByRole("button", { name: "驳回", exact: true }).click();
  await expect(page.getByRole("alert")).toHaveText("隔离验收：审核更新失败");
  await expect(row).toHaveAttribute("data-status", "pending"); await expect(row.getByRole("button", { name: "驳回", exact: true })).toBeEnabled();
});

test("Governance empty filter is editorial with no invented action", async ({ page }) => {
  await governanceFixture(page, { empty: true }); await page.goto("/admin/reports?status=pending");
  await expect(page.getByRole("heading", { name: "当前筛选下没有举报" })).toBeVisible();
  await expect(page.locator(".gv-governance-empty")).toContainText("REPORTS / 00");
  await expect(page.locator(".gv-governance-empty a, .gv-governance-empty button")).toHaveCount(0);
  await noOverflow(page);
});

test("Governance eight viewports bound long titles/details and filters, with Creator OS themes", async ({ page }) => {
  test.setTimeout(180_000); await governanceFixture(page, { long: true }); await page.goto("/admin/reports");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  for (const value of ["light", "dark"] as const) {
    for (const [width, height] of viewports) {
      await page.setViewportSize({ width, height }); await theme(page, value);
      await expect(page.locator(".gv-studio-shell")).toHaveCSS("background-color", value === "light" ? "rgb(240, 241, 238)" : "rgb(8, 10, 13)");
      await noOverflow(page);
      const row = page.locator(".gv-report-row").first();
      for (const selector of [".gv-report-content", ".gv-report-detail", ".gv-governance-filters"]) {
        const box = await page.locator(selector).first().boundingBox(); expect(box!.x).toBeGreaterThanOrEqual(0); expect(box!.x + box!.width).toBeLessThanOrEqual(width + 1);
      }
      if (width <= 700) {
        await expect(page.locator(".gv-governance-columns")).toBeHidden();
        const parts = await row.locator(".gv-report-state, .gv-report-content, .gv-report-actions").all();
        const boxes = await Promise.all(parts.map((part) => part.boundingBox()));
        expect(boxes[0]!.y + boxes[0]!.height).toBeLessThanOrEqual(boxes[1]!.y); expect(boxes[1]!.y + boxes[1]!.height).toBeLessThanOrEqual(boxes[2]!.y);
      }
      for (const button of await row.getByRole("button").all()) { const box = await button.boundingBox(); expect(box!.height).toBeGreaterThanOrEqual(44); expect(box!.width).toBeGreaterThanOrEqual(44); }
      const filter = page.getByRole("group", { name: "举报状态筛选" }).getByRole("button", { name: "已驳回", exact: true });
      await filter.scrollIntoViewIfNeeded(); await noOverflow(page);
    }
    await page.reload(); await expect(page.locator("html")).toHaveAttribute("data-theme", value);
  }
});

if (process.env.PHASE5_SCREENSHOTS === "1") {
  test("Governance isolated full-page acceptance screenshots", async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== "desktop-chromium", "Acceptance captures use the desktop project with an explicit mobile viewport.");
    test.setTimeout(90_000);
    const capture = async (name: string) => {
      await page.evaluate(() => { window.scrollTo(0, 0); (document.activeElement as HTMLElement | null)?.blur(); });
      await page.mouse.move(0, 0);
      return page.screenshot({ path: `../tmp/phase5-screenshots/${name}.png`, fullPage: true });
    };
    await governanceFixture(page); await page.setViewportSize({ width: 1440, height: 900 }); await page.goto("/admin/reports");
    await expect(page.locator(".gv-report-row")).toHaveCount(4);
    await theme(page, "dark"); await capture("14-admin-dark");
    await theme(page, "light"); await capture("15-admin-light");
    await page.goto("/admin/reports?status=pending"); await expect(page.locator(".gv-report-row")).toHaveCount(1);
    await capture("16-admin-pending");
    await page.unroute("**/api/v1/**"); await governanceFixture(page, { empty: true });
    await page.goto("/admin/reports?status=reviewed"); await expect(page.getByRole("heading", { name: "当前筛选下没有举报" })).toBeVisible();
    await capture("17-admin-filtered-empty");
    await page.unroute("**/api/v1/**"); await governanceFixture(page); await page.setViewportSize({ width: 390, height: 844 }); await page.goto("/admin/reports");
    await expect(page.locator(".gv-report-row")).toHaveCount(4); await noOverflow(page); await capture("18-admin-mobile390");
  });
}
