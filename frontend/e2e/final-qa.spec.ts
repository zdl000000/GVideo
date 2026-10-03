import { expect, test, type Page, type Route, type TestInfo } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
import { finalQaBrokenCover, finalQaViewports, installFinalQaFixture, type FinalQaState } from "./final-qa.fixture";

type Theme = "light" | "dark";
const routes = [
  { name: "Home", path: "/", selector: ".gv-discovery-page", family: "watch" },
  { name: "Latest", path: "/latest", selector: ".gv-discovery-page", family: "watch" },
  { name: "Popular", path: "/popular", selector: ".gv-discovery-page", family: "watch" },
  { name: "Video", path: "/video/1", selector: ".gv-watch-page", family: "watch" },
  { name: "Creator Channel", path: "/users/42", selector: ".gv-channel-page", family: "watch" },
  { name: "Dashboard", path: "/creator", selector: ".gv-studio-dashboard", family: "studio" },
  { name: "Content Management", path: "/me/videos", selector: ".gv-content-page", family: "studio" },
  { name: "Publish", path: "/upload", selector: ".gvideo-publish-page", family: "studio" },
  { name: "Auth", path: "/auth", selector: ".gv-auth-experience", family: "auth" },
  { name: "Notifications", path: "/notifications", selector: ".gv-notification-center", family: "watch" },
  { name: "Governance", path: "/admin/reports", selector: ".gv-governance-page", family: "studio" }
] as const;

async function theme(page: Page, value: Theme) {
  if (await page.locator("html").getAttribute("data-theme") === value) return;
  if (await page.locator(".mobile-menu-button").isVisible()) {
    await page.locator(".mobile-menu-button").click();
    await page.locator(".mobile-account .theme-row").click();
    await page.getByRole("dialog", { name: "导航菜单" }).press("Escape");
  } else await page.getByRole("button", { name: `切换到${value === "light" ? "浅" : "深"}色主题`, exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", value);
}
async function noOverflow(page: Page) {
  const size = await page.evaluate(() => ({ viewport: innerWidth, document: document.documentElement.scrollWidth, body: document.body.scrollWidth }));
  expect(size.document, JSON.stringify(size)).toBeLessThanOrEqual(size.viewport + 1);
  expect(size.body, JSON.stringify(size)).toBeLessThanOrEqual(size.viewport + 1);
}
async function ready(page: Page, selector: string) {
  await expect(page.locator(selector)).toBeVisible({ timeout: 20_000 });
  await expect(page.getByText("正在加载页面", { exact: true })).toHaveCount(0);
  await expect(page.locator('[aria-label="正在加载视频"], [aria-label="正在加载投稿"]')).toHaveCount(0);
}

interface ExpectedHttpError { path: string; status: number }
async function audit(page: Page, allowedHttpErrors: ExpectedHttpError[] = []) {
  const consoleErrors: Array<{ message: string; url: string }> = [], pageErrors: string[] = [], requestFailures: Array<{ url: string; failure: string }> = [], responses: Array<{ url: string; status: number; resource: string }> = [];
  const unhandled: string[] = [];
  const expected = (url: string, message: string) => {
    let path: string; try { path = new URL(url).pathname; } catch { return false; }
    return [{ path: finalQaBrokenCover, status: 404 }, ...allowedHttpErrors].some((error) => path === error.path && new RegExp(`\\b${error.status}\\b`).test(message));
  };
  page.on("console", (message) => { if (message.type() === "error") consoleErrors.push({ message: message.text(), url: message.location().url }); });
  page.on("pageerror", (error) => pageErrors.push(error.message));
  page.on("requestfailed", (request) => requestFailures.push({ url: request.url(), failure: request.failure()?.errorText || "unknown request failure" }));
  page.on("response", (response) => responses.push({ url: response.url(), status: response.status(), resource: response.request().resourceType() }));
  await page.exposeFunction("phase6ReportUnhandled", (reason: string) => unhandled.push(reason));
  await page.addInitScript(() => {
    const target = window as Window & { phase6ReportUnhandled: (reason: string) => Promise<void> };
    window.addEventListener("unhandledrejection", (event) => { void target.phase6ReportUnhandled(String(event.reason)); });
  });
  return {
    consoleErrors, pageErrors, requestFailures,
    async assertClean(state: FinalQaState, testInfo?: TestInfo, artifactPath?: string) {
      const unexpectedConsole = consoleErrors.filter((item) => !expected(item.url, item.message));
      const unexpectedFailures = requestFailures.filter((item) => !(/ERR_ABORTED|NS_BINDING_ABORTED/.test(item.failure) && /^\/(api\/v1\/|media\/)/.test(new URL(item.url).pathname)));
      const unexpectedHttpErrors = responses.filter((item) => item.status >= 400 && !expected(item.url, String(item.status)));
      const report = JSON.stringify({ allowedHttpErrors, consoleErrors, pageErrors, requestFailures, responses, unhandled, unknownApi: state.unknownRequests, requests: state.requests }, null, 2);
      if (testInfo) await testInfo.attach("Phase 6 console-network audit", { contentType: "application/json", body: report });
      if (artifactPath) await writeFile(artifactPath, report, "utf8");
      expect(state.unknownRequests, "Unknown API paths are fail-closed and must be fixed in the fixture or diagnosed").toEqual([]);
      expect(unexpectedConsole, "Only the explicit expected HTTP error paths/statuses are allowed").toEqual([]);
      expect(unexpectedHttpErrors, "Unexpected HTTP errors must be diagnosed, including non-API assets").toEqual([]);
      expect(pageErrors).toEqual([]); expect(unhandled).toEqual([]); expect(unexpectedFailures).toEqual([]);
    }
  };
}

// All four role/shell families are exercised in an actual Chromium page. The
// fixture reads only the seed with count_view=false and intercepts every write.
for (const route of routes) {
  test(`Phase 6 ${route.name}: eight viewports, both themes, long text and console/network audit`, async ({ page }, testInfo) => {
    test.setTimeout(150_000);
    const diagnostics = await audit(page, route.family === "auth" ? [{ path: "/api/v1/auth/me", status: 401 }] : []);
    const state = await installFinalQaFixture(page, { role: route.family === "auth" ? "anonymous" : "admin", scenario: "long" });
    await page.goto(route.path); await ready(page, route.selector);
    const brandMark = page.locator('.gv-brand img').first();
    await expect.poll(() => brandMark.evaluate((element: HTMLImageElement) => element.naturalWidth)).toBeGreaterThan(0);
    expect(await brandMark.getAttribute('src'), 'The production CSP forbids data: images').not.toMatch(/^data:/);
    if (route.name === "Publish") {
      await page.locator('input[aria-label="视频文件"]').setInputFiles({ name: `${"LONG_ASCII_FILENAME_0123456789_".repeat(18)}.mp4`, mimeType: "video/mp4", buffer: Buffer.from("isolated file-state fixture") });
      await page.getByLabel(/^标题/).fill("中文标题_ABCDEFGHIJKLMNOPQRSTUVWXYZ_0123456789_".repeat(2).slice(0, 80));
      await page.getByLabel("简介", { exact: true }).fill("LONG_DESCRIPTION_中文_0123456789_".repeat(60).slice(0, 2000));
    }
    for (const value of ["light", "dark"] as const) {
      await theme(page, value);
      for (const [width, height] of finalQaViewports) {
        await page.setViewportSize({ width, height }); await noOverflow(page);
        const expectedCanvas = value === "dark" ? "#080a0d" : route.family === "watch" ? "#eeede9" : route.family === "studio" ? "#f0f1ee" : "#f7f8fa";
        const expectedSurface = value === "dark" ? "#11161d" : route.family === "watch" ? "#faf9f6" : route.family === "studio" ? "#fafbf8" : "#fff";
        const colors = await page.locator(route.selector).evaluate((element) => {
          const css = getComputedStyle(element); let ancestor: Element | null = element;
          while (ancestor && getComputedStyle(ancestor).backgroundColor === "rgba(0, 0, 0, 0)") ancestor = ancestor.parentElement;
          return { canvas: css.getPropertyValue("--gv-bg").trim(), surface: css.getPropertyValue("--gv-surface").trim(), visibleCanvas: ancestor ? getComputedStyle(ancestor).backgroundColor : "" };
        });
        const expectedVisibleCanvas = value === "dark" ? "rgb(8, 10, 13)" : route.family === "watch" ? "rgb(238, 237, 233)" : route.family === "studio" ? "rgb(240, 241, 238)" : "rgb(247, 248, 250)";
        expect(colors, `${route.name} ${value} ${width}`).toEqual({ canvas: expectedCanvas, surface: expectedSurface, visibleCanvas: expectedVisibleCanvas });
        await expect(page.locator("main h1").first()).toHaveCSS("color", value === "dark" ? "rgb(247, 248, 250)" : "rgb(17, 19, 24)");
        await expect(page.locator("main .error-state, main .management-error, main [role=alert]")).toHaveCount(0);
        if (route.name === "Video") { const bounds = await page.locator(".player-wrap").boundingBox(); expect(bounds!.x).toBeGreaterThanOrEqual(0); expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(width + 1); }
        if (route.name === "Content Management" && width <= 700) await expect(page.locator(".gv-content-columns")).toBeHidden();
        if (route.name === "Publish" && width <= 430) expect(await page.locator(".gv-publish-visibility").evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(" ").length)).toBe(1);
        if (route.name === "Auth" && width === 390) { const submit = await page.locator(".gv-auth-submit").boundingBox(); expect(submit!.y + submit!.height).toBeLessThanOrEqual(height); }
      }
    }
    await diagnostics.assertClean(state, testInfo);
  });
}

test("Phase 6 theme bootstrap runs before the app bundle on direct, protected and auth links", async ({ context }, testInfo) => {
  test.setTimeout(180_000);
  for (const preference of [null, "invalid", "light", "dark"] as const) for (const path of ["/", "/video/1", "/upload", "/auth"]) {
    // Keep the project's device settings and one framework-managed trace.
    // Each page has its own init script that resets the saved preference.
    const page = await context.newPage();
    try {
      const diagnostics = await audit(page, [{ path: "/api/v1/auth/me", status: 401 }]);
      await page.addInitScript((value) => { value === null ? localStorage.removeItem("gvideo-theme") : localStorage.setItem("gvideo-theme", value); }, preference);
      const state = await installFinalQaFixture(page, { role: "anonymous" });
      let release!: () => void; const gate = new Promise<void>((done) => { release = done; });
      const blockBundle = async (route: Route) => { await gate; await route.continue(); };
      await page.route("**/assets/index-*.js", blockBundle); await page.route("**/src/main.tsx", blockBundle);
      await page.goto(path, { waitUntil: "commit" });
      const expected = preference === "dark" ? "dark" : "light";
      await expect(page.locator("html")).toHaveAttribute("data-theme", expected);
      await expect(page.locator('meta[name="theme-color"]')).toHaveAttribute("content", expected === "dark" ? "#080a0d" : "#f7f8fa");
      await expect(page.locator("#root")).toBeEmpty();
      expect(await page.locator("html").evaluate((element) => getComputedStyle(element).colorScheme)).toBe(expected);
      release();
      if (path === "/upload") { await expect(page).toHaveURL(/\/auth\?next=%2Fupload$/); await ready(page, ".gv-auth-experience"); }
      else await ready(page, path === "/auth" ? ".gv-auth-experience" : path === "/video/1" ? ".gv-watch-page" : ".gv-discovery-page");
      await page.reload(); await expect(page.locator("html")).toHaveAttribute("data-theme", expected);
      await expect(page.locator('meta[name="theme-color"]')).toHaveAttribute("content", expected === "dark" ? "#080a0d" : "#f7f8fa");
      await ready(page, path === "/upload" || path === "/auth" ? ".gv-auth-experience" : path === "/video/1" ? ".gv-watch-page" : ".gv-discovery-page");
      expect(state.unknownRequests).toEqual([]);
      await diagnostics.assertClean(state, testInfo);
    } finally { await page.close(); }
  }
});

test("Phase 6 all thirteen routes retain anonymous, self, other, admin and auth-expiry semantics", async ({ page }, testInfo) => {
  test.setTimeout(120_000);
  const diagnostics = await audit(page, [{ path: "/api/v1/auth/me", status: 401 }, { path: "/api/v1/me/notifications", status: 401 }]);
  let state = await installFinalQaFixture(page, { role: "anonymous" });
  for (const path of ["/following", "/favorites", "/creator", "/me/videos", "/upload", "/notifications", "/admin/reports"]) {
    await page.goto(`${path}?page=2`);
    await expect(page).toHaveURL(new RegExp(`/auth\\?next=${encodeURIComponent(`${path}?page=2`)}$`)); await ready(page, ".gv-auth-experience");
  }
  for (const path of ["/", "/latest", "/popular", "/video/1", "/users/42", "/auth"]) { await page.goto(path); await expect(page.locator("main")).toBeVisible(); }
  expect(state.unknownRequests).toEqual([]);
  state = await installFinalQaFixture(page, { role: "self" }); await page.goto("/users/42");
  await expect(page.getByRole("button", { name: "编辑资料", exact: true })).toBeVisible(); await expect(page.getByRole("link", { name: "管理投稿", exact: true })).toBeVisible();
  await page.goto("/admin/reports"); await expect(page).toHaveURL(/\/$/); expect(state.reads("/api/v1/admin/reports")).toBe(0);
  expect(state.unknownRequests).toEqual([]);
  state = await installFinalQaFixture(page, { role: "other" }); await page.goto("/users/42");
  await expect(page.getByRole("button", { name: "关注", exact: true })).toBeVisible(); await expect(page.getByRole("button", { name: "编辑资料", exact: true })).toHaveCount(0);
  expect(state.unknownRequests).toEqual([]);
  state = await installFinalQaFixture(page, { role: "admin" }); await page.goto("/admin/reports"); await ready(page, ".gv-governance-page");
  await expect(page.locator(".gv-report-row")).toHaveCount(4);
  await page.goto("/notifications?page=2"); await ready(page, ".gv-notification-center"); state.expire();
  await page.evaluate(() => window.dispatchEvent(new Event("gvideo-notifications-changed")));
  await expect(page).toHaveURL(/\/auth\?next=%2Fnotifications%3Fpage%3D2$/); await ready(page, ".gv-auth-experience");
  expect(state.unknownRequests).toEqual([]);
  await diagnostics.assertClean(state, testInfo);
  // login/register/already-auth safe next rejection is intentionally covered by
  // auth-experience.spec.ts; no duplicate security implementation is introduced.
});

test("Phase 6 empty, zero and related/comment states remain visible in both themes", async ({ page }, testInfo) => {
  test.setTimeout(100_000); const diagnostics = await audit(page); const state = await installFinalQaFixture(page, { role: "admin", scenario: "empty" });
  const cases = [["/following", "关注动态还是空的"], ["/favorites", "还没有收藏"], ["/users/42", "还没有公开投稿"], ["/creator", "还没有投稿"], ["/me/videos", "还没有投稿"], ["/notifications", "暂时没有通知"], ["/admin/reports?status=pending", "当前筛选下没有举报"]];
  for (const value of ["light", "dark"] as const) for (const [path, heading] of cases) {
    await page.goto(path); await expect(page.getByRole("heading", { name: heading, exact: true })).toBeVisible(); await theme(page, value);
    await page.setViewportSize({ width: 320, height: 720 }); await noOverflow(page);
  }
  await page.goto("/video/1"); await ready(page, ".gv-watch-page"); await expect(page.getByText("暂无更多视频", { exact: true })).toBeVisible();
  await expect(page.locator(".comment-empty")).toBeVisible();
  await page.goto("/users/42"); await expect(page.locator(".creator-stats dd")).toHaveText(["0", "0", "0"]);
  expect(state.unknownRequests).toEqual([]);
  await diagnostics.assertClean(state, testInfo);
});

test("Phase 6 controlled loading and API errors preserve readable bounded feedback", async ({ page }, testInfo) => {
  test.setTimeout(120_000);
  const errorPaths = ["/api/v1/videos", "/api/v1/videos/1", "/api/v1/videos/1/comments", "/api/v1/users/42", "/api/v1/users/42/videos", "/api/v1/me/creator/stats", "/api/v1/me/videos", "/api/v1/me/notifications", "/api/v1/admin/reports"];
  const diagnostics = await audit(page, errorPaths.map((path) => ({ path, status: 500 })));
  let state = await installFinalQaFixture(page, { role: "admin" });
  const cases = [
    ["/", "/api/v1/videos", "正在加载视频"], ["/video/1", "/api/v1/videos/1", "正在准备播放器"], ["/users/42", "/api/v1/users/42", "正在打开作者空间"],
    ["/creator", "/api/v1/me/creator/stats", "正在整理创作数据"], ["/me/videos", "/api/v1/me/videos", "正在加载投稿"], ["/notifications", "/api/v1/me/notifications", "正在读取通知"], ["/admin/reports", "/api/v1/admin/reports", "正在读取举报记录"]
  ];
  for (const [path, endpoint, label] of cases) {
    const before = state.reads(endpoint); state.gateRead(endpoint); await page.goto(path);
    await expect.poll(() => state.reads(endpoint)).toBeGreaterThan(before);
    await expect(page.getByRole("status").filter({ hasText: label }).or(page.getByRole("status", { name: label, exact: true }))).toBeVisible();
    state.releaseRead(endpoint); await expect(page.getByText("正在加载页面", { exact: true })).toHaveCount(0);
    await page.getByRole("link", { name: "GVideo 首页", exact: true }).click();
  }
  // The most recently installed route takes precedence. Keep the previous
  // handler active while the next fixture reads its seed, so no API can escape.
  state = await installFinalQaFixture(page, { role: "admin", scenario: "error" });
  for (const path of ["/", "/video/1", "/users/42", "/creator", "/me/videos", "/notifications", "/admin/reports"]) {
    await page.goto(path); await expect(page.getByText(/隔离验收：读取失败/).first()).toBeVisible({ timeout: 20_000 });
    await page.setViewportSize({ width: 320, height: 720 }); await noOverflow(page);
  }
  expect(state.unknownRequests).toEqual([]);
  await diagnostics.assertClean(state, testInfo);
});

test("Phase 6 pending, processing, ready and failed playback states use actual status copy", async ({ page }, testInfo) => {
  const diagnostics = await audit(page); const state = await installFinalQaFixture(page); await page.goto("/video/1"); await ready(page, ".gv-watch-page");
  for (const status of ["pending", "processing", "failed", "ready"] as const) {
    state.setVideoStatus(status); await page.reload(); await ready(page, ".gv-watch-page");
    if (status === "ready") await expect(page.locator(".processing-notice")).toHaveCount(0);
    else { await expect(page.locator(".processing-notice")).toBeVisible(); await expect(page.locator(".processing-notice")).not.toHaveText(""); }
    await page.setViewportSize({ width: 320, height: 720 }); await noOverflow(page);
  }
  expect(state.unknownRequests).toEqual([]);
  await diagnostics.assertClean(state, testInfo);
});

test("Phase 6 processing polling stops after readiness and route unmount", async ({ page }, testInfo) => {
  test.setTimeout(60_000); const diagnostics = await audit(page); const state = await installFinalQaFixture(page, { videoStatus: "processing" }); await page.clock.install();
  await page.goto("/video/1"); await ready(page, ".gv-watch-page"); await expect(page.locator(".processing-notice")).toBeVisible();
  const initial = state.reads("/api/v1/videos/1"); await page.clock.runFor(2600);
  await expect.poll(() => state.reads("/api/v1/videos/1")).toBeGreaterThan(initial);
  expect(state.reads("/api/v1/videos/1")).toBeLessThanOrEqual(initial + 2);
  state.setVideoStatus("ready"); await page.clock.runFor(2600); await expect(page.locator(".processing-notice")).toHaveCount(0);
  const stopped = state.reads("/api/v1/videos/1"); await page.clock.runFor(5200); expect(state.reads("/api/v1/videos/1")).toBe(stopped);
  await page.goto("/me/videos"); await ready(page, ".gv-content-page"); await expect(page.locator(".gv-content-row")).toHaveCount(12);
  const before = state.reads("/api/v1/me/videos"); await page.clock.runFor(2600); await expect.poll(() => state.reads("/api/v1/me/videos")).toBeGreaterThan(before);
  await page.getByRole("link", { name: "GVideo 首页", exact: true }).click(); await ready(page, ".gv-discovery-page");
  const unmounted = state.reads("/api/v1/me/videos"); await page.clock.runFor(8000); expect(state.reads("/api/v1/me/videos")).toBe(unmounted);
  expect(state.unknownRequests).toEqual([]);
  await diagnostics.assertClean(state, testInfo);
});

test("Phase 6 delayed Discovery and Notifications responses cannot replace the newest query", async ({ page }, testInfo) => {
  test.setTimeout(80_000); const diagnostics = await audit(page); const state = await installFinalQaFixture(page, { role: "admin", scenario: "long" });
  await page.goto("/"); await ready(page, ".gv-discovery-page"); await expect(page.locator("[data-video-id]").first()).toBeVisible();
  const initial = state.requests.find((request) => request.path === "/api/v1/videos" && new URLSearchParams(request.query).get("sort") === "latest")!;
  const query = new URLSearchParams(initial.query); query.set("category", "音乐"); const heldPath = `/api/v1/videos?${query}`;
  state.gateRead(heldPath); await page.getByRole("group", { name: "视频分类" }).getByRole("button", { name: "音乐", exact: true }).click();
  await expect.poll(() => state.requests.some((request) => request.path === "/api/v1/videos" && new URLSearchParams(request.query).get("category") === "音乐")).toBe(true);
  await page.getByRole("group", { name: "视频分类" }).getByRole("button", { name: "生活", exact: true }).click();
  await expect(page.locator('[data-video-id="2"]').first()).toBeVisible(); state.releaseRead(heldPath);
  await expect(page.locator('[data-video-id="1"]')).toHaveCount(0); await expect(page.getByRole("button", { name: "生活", exact: true })).toHaveAttribute("aria-pressed", "true");
  await page.goto("/notifications"); await expect(page.locator(".gv-notification-row")).toHaveCount(20);
  const noticeGate = "/api/v1/me/notifications?page=2&page_size=20"; state.gateRead(noticeGate);
  const pagination = page.getByRole("navigation", { name: "通知分页" }); await pagination.getByRole("button", { name: "下一页" }).click();
  await expect.poll(() => state.requests.some((request) => request.path === "/api/v1/me/notifications" && new URLSearchParams(request.query).get("page") === "2")).toBe(true);
  // ShellContent remounts the page on a location-key change, so pagination
  // is absent during this pending load. Browser Back is the real route action.
  await page.goBack(); await expect(page).toHaveURL(/\/notifications$/);
  await expect(page.locator(".gv-notification-row")).toHaveCount(20); state.releaseRead(noticeGate);
  await expect(page.locator(".gv-notification-row")).toHaveCount(20); await expect(page.locator(".gv-notification-row").first().getByRole("link")).toHaveAttribute("href", "/users/9");
  await expect(page.locator("main [role=alert]")).toHaveCount(0); expect(state.unknownRequests).toEqual([]);
  await diagnostics.assertClean(state, testInfo);
});

test("Phase 6 delayed Video, Creator and Governance reads are aborted on route/filter changes", async ({ page }, testInfo) => {
  test.setTimeout(80_000); const diagnostics = await audit(page); const state = await installFinalQaFixture(page, { role: "admin" }); const aborted: string[] = [];
  page.on("requestfailed", (request) => { if (/ERR_ABORTED/.test(request.failure()?.errorText || "")) aborted.push(new URL(request.url()).pathname); });
  for (const [path, endpoint] of [["/video/1", "/api/v1/videos/1"], ["/users/42", "/api/v1/users/42"]]) {
    const before = state.reads(endpoint); state.gateRead(endpoint); await page.goto(path); await expect.poll(() => state.reads(endpoint)).toBeGreaterThan(before);
    await page.getByRole("link", { name: "GVideo 首页", exact: true }).click(); await ready(page, ".gv-discovery-page"); state.releaseRead(endpoint);
    await expect(page.locator(".gv-watch-page, .gv-channel-page")).toHaveCount(0); await expect.poll(() => aborted.includes(endpoint)).toBe(true);
  }
  const held = "/api/v1/admin/reports?page=1&page_size=20&status=pending"; state.gateRead(held); await page.goto("/admin/reports?status=pending");
  await expect.poll(() => state.requests.some((request) => request.path === "/api/v1/admin/reports" && new URLSearchParams(request.query).get("status") === "pending")).toBe(true);
  await page.getByRole("group", { name: "举报状态筛选" }).getByRole("button", { name: "审核中", exact: true }).click();
  await expect(page.locator(".gv-report-status")).toHaveText(["审核中"]); state.releaseRead(held);
  await expect(page.locator(".gv-report-status")).toHaveText(["审核中"]); await expect(page).toHaveURL(/status=reviewed$/);
  expect(state.unknownRequests).toEqual([]);
  await diagnostics.assertClean(state, testInfo);
});

test("Phase 6 Release Candidate: collect sixteen isolated desktop and mobile screenshots", async ({ page }, testInfo) => {
  test.skip(process.env.E2E_CAPTURE_PHASE6 !== "true" || testInfo.project.name !== "desktop-chromium", "Opt-in RC artifacts, collected only once by the desktop project");
  test.setTimeout(150_000); await mkdir("../tmp/phase6-screenshots", { recursive: true });
  const diagnostics = await audit(page, [{ path: "/api/v1/auth/me", status: 401 }]);
  let state = await installFinalQaFixture(page, { role: "admin" });
  const capture = async (path: string, value: Theme, width: number, name: string) => {
    await page.setViewportSize({ width, height: width === 390 ? 844 : 900 }); await page.goto(path);
    const route = routes.find((item) => item.path === path)!; await ready(page, route.selector); await theme(page, value);
    if (path === "/video/1") {
      const media = page.locator("video");
      await expect.poll(() => media.evaluate((element) => (element as HTMLVideoElement).readyState), { timeout: 20_000 }).toBeGreaterThanOrEqual(2);
      await media.evaluate((element) => { const video = element as HTMLVideoElement; video.pause(); video.currentTime = 2; });
      await expect.poll(() => media.evaluate((element) => { const video = element as HTMLVideoElement; return !video.seeking && Math.abs(video.currentTime - 2) < .1; })).toBe(true);
    }
    await page.evaluate(() => { window.scrollTo(0, 0); (document.activeElement as HTMLElement | null)?.blur(); }); await page.mouse.move(0, 0);
    await noOverflow(page); await page.screenshot({ path: `../tmp/phase6-screenshots/${name}.png`, fullPage: true, animations: "disabled" });
  };
  for (const [path, name] of [["/", "01-home-dark"], ["/video/1", "02-video-dark"], ["/creator", "03-dashboard-dark"], ["/upload", "04-publish-dark"]]) await capture(path, "dark", 1440, name);
  for (const [path, name] of [["/", "05-home-light"], ["/video/1", "06-video-light"], ["/creator", "07-dashboard-light"], ["/upload", "08-publish-light"]]) await capture(path, "light", 1440, name);
  for (const [path, name] of [["/", "09-mobile-home"], ["/video/1", "10-mobile-video"], ["/me/videos", "11-mobile-content"], ["/upload", "12-mobile-publish"], ["/notifications", "14-mobile-notifications"]]) await capture(path, "light", 390, name);
  await capture("/admin/reports", "dark", 1440, "15-admin-desktop"); await capture("/admin/reports", "light", 390, "16-admin-mobile");
  expect(state.unknownRequests).toEqual([]);
  state = await installFinalQaFixture(page, { role: "anonymous" }); await capture("/auth", "light", 390, "13-mobile-auth");
  await diagnostics.assertClean(state, testInfo, "../tmp/phase6-screenshots/console-network-audit.json");
  testInfo.annotations.push({ type: "fixture", description: "All RC content and business mutations are API fixtures; only the independent ready seed supplies actual playable media with count_view=false." });
});
