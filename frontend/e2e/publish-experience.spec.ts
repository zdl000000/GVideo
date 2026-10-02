import { expect, test, type Page } from "@playwright/test";
import { mkdir } from "node:fs/promises";
import { pageFixture, videoFixture } from "../src/shared/test/videoFixtures";

declare global { interface Window { phase5Upload?: { xhr: XMLHttpRequest; fields: Record<string, string>; complete: () => void } } }
const viewports = [[1536, 960], [1440, 900], [1180, 820], [920, 900], [768, 1024], [430, 932], [390, 844], [320, 720]];
const videoName = `隔离验收_${"long_ASCII_filename_".repeat(12)}.mp4`;
const coverBytes = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jGuUAAAAASUVORK5CYII=", "base64");

// Hold only the fixture upload's send, dispatch real ProgressEvents and then use
// the original XHR through an intercepted API response. No real media is uploaded.
async function fixture(page: Page, options: { fail?: boolean } = {}) {
  let uploads = 0;
  await page.addInitScript(() => {
    const send = XMLHttpRequest.prototype.send;
    XMLHttpRequest.prototype.send = function (body) {
      if (body instanceof FormData && body.has("video")) {
        const xhr = this; const fields = Object.fromEntries([...body.entries()].map(([key, value]) => [key, value instanceof File ? value.name : value]));
        window.phase5Upload = { xhr, fields, complete: () => send.call(xhr, body) };
        xhr.upload.dispatchEvent(new ProgressEvent("progress", { lengthComputable: true, loaded: 0, total: 100 }));
      } else send.call(this, body);
    };
  });
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request(); const path = new URL(request.url()).pathname; let data: unknown;
    if (path === "/api/v1/auth/me") data = { user: { id: 42, username: "隔离验收创作者", avatar_url: "", bio: "", is_admin: false, created_at: "2026-01-01T00:00:00Z" }, csrf_token: "publish-fixture-csrf" };
    else if (path === "/api/v1/me/notifications") data = { ...pageFixture([], { page_size: 1 }), unread_count: 0 };
    else if (path === "/api/v1/categories") data = ["音乐", "生活"];
    else if (path === "/api/v1/videos" && request.method() === "POST") {
      uploads++; expect(request.headers()["x-csrf-token"]).toBe("publish-fixture-csrf");
      const body = request.postData()!;
      for (const field of ["title", "description", "category", "visibility", "video", "cover", "subtitle", "subtitle_language", "subtitle_label"]) expect(body).toContain(`name="${field}"`);
      if (options.fail) { await route.fulfill({ status: 500, json: { error: "隔离验收上传失败" } }); return; }
      data = videoFixture(901, { title: "隔离发布作品", video_url: "", hls_url: "", processing_status: "pending", processing_stage: "queued", processing_progress: 0 });
    } else {
      expect(request.method(), "unexpected mutation must never reach a real API").toBe("GET");
      if (path === "/api/v1/videos/901") data = videoFixture(901, { title: "隔离发布作品", video_url: "", hls_url: "", processing_status: "pending", processing_stage: "queued", processing_progress: 0 });
      else if (path === "/api/v1/videos/901/comments") data = [];
      else if (path === "/api/v1/videos") data = pageFixture([]);
      else { await route.fulfill({ status: 404, json: { error: "unexpected isolated API request" } }); return; }
    }
    await route.fulfill({ json: { data } });
  });
  return { uploads: () => uploads };
}
async function selectFiles(page: Page) {
  await page.locator('input[aria-label="视频文件"]').setInputFiles({ name: videoName, mimeType: "video/mp4", buffer: Buffer.from("isolated video fixture") });
  await page.getByLabel("封面图片", { exact: true }).setInputFiles({ name: "隔离封面.png", mimeType: "image/png", buffer: coverBytes });
  await page.getByLabel("字幕文件", { exact: true }).setInputFiles({ name: "隔离字幕.vtt", mimeType: "text/vtt", buffer: Buffer.from("WEBVTT\n\n00:00:00.000 --> 00:00:01.000\n隔离验收\n") });
  await page.getByLabel(/^标题/).fill("隔离发布作品"); await page.getByLabel("简介", { exact: true }).fill("真实流程，隔离文件与接口响应。");
  await page.getByLabel("分区", { exact: true }).selectOption("生活"); await page.getByRole("radio", { name: /^仅自己/ }).check();
}
async function progress(page: Page, loaded: number) { await page.evaluate((loaded) => window.phase5Upload!.xhr.upload.dispatchEvent(new ProgressEvent("progress", { lengthComputable: true, loaded, total: 100 })), loaded); }
async function theme(page: Page, value: "light" | "dark") {
  if (await page.locator("html").getAttribute("data-theme") === value) return;
  if (await page.locator(".mobile-menu-button").isVisible()) { await page.locator(".mobile-menu-button").click(); await page.locator(".mobile-account .theme-row").click(); await page.getByRole("dialog", { name: "导航菜单" }).press("Escape"); }
  else await page.getByRole("button", { name: `切换到${value === "light" ? "浅" : "深"}色主题`, exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", value);
}

test("Publish selected media, actual cover replacement, initial subtitle and unchanged visibility help", async ({ page }) => {
  await fixture(page); await page.goto("/upload");
  await expect(page.getByRole("button", { name: "发布作品", exact: true })).toBeDisabled(); await expect(page.getByText("未选择封面时会自动截取视频画面")).toBeVisible();
  await selectFiles(page); await expect(page.getByText(videoName, { exact: true })).toBeVisible(); await expect(page.getByText("选择视频文件", { exact: true })).toHaveCount(0);
  await page.getByLabel(/^标题/).fill("短"); await page.getByRole("button", { name: "发布作品", exact: true }).click();
  expect(await page.getByLabel(/^标题/).evaluate((input: HTMLInputElement) => input.validity.tooShort)).toBe(true); await expect(page.getByRole("progressbar")).toHaveCount(0); await page.getByLabel(/^标题/).fill("隔离发布作品");
  await expect(page.getByText("隔离字幕.vtt", { exact: true })).toBeVisible(); await expect(page.getByRole("radio", { name: /^仅自己/ })).toBeChecked();
  await expect(page.getByAltText("封面预览")).toHaveJSProperty("complete", true);
  const first = await page.getByAltText("封面预览").getAttribute("src");
  await page.getByLabel("封面图片", { exact: true }).setInputFiles({ name: "替换封面.png", mimeType: "image/png", buffer: coverBytes });
  expect(await page.getByAltText("封面预览").getAttribute("src")).not.toBe(first); await expect(page.getByText("替换封面.png", { exact: true })).toBeVisible();
  await expect(page.getByText("只有你登录后可以访问视频和相关媒体。")).toBeVisible();
});

test("Publish progress 0 -> 45 -> 100 submits one FormData and navigates to actual video route", async ({ page }) => {
  const state = await fixture(page); await page.goto("/upload"); await selectFiles(page); await page.getByRole("button", { name: "发布作品", exact: true }).click();
  const bar = page.getByRole("progressbar", { name: "文件上传进度" }); await expect(bar).toHaveAttribute("aria-valuenow", "0");
  await progress(page, 45); await expect(bar).toHaveAttribute("aria-valuenow", "45"); await expect(page.getByRole("button", { name: "正在上传..." })).toBeDisabled();
  await expect(page.getByText("上传完成后会进入后台处理阶段")).toBeVisible(); await progress(page, 100); await expect(bar).toHaveAttribute("aria-valuenow", "100");
  const fields = await page.evaluate(() => window.phase5Upload!.fields);
  expect(fields).toEqual({ title: "隔离发布作品", description: "真实流程，隔离文件与接口响应。", category: "生活", visibility: "private", video: videoName, cover: "隔离封面.png", subtitle: "隔离字幕.vtt", subtitle_language: "zh-CN", subtitle_label: "中文" });
  await page.evaluate(() => window.phase5Upload!.complete()); await expect(page).toHaveURL(/\/video\/901$/); expect(state.uploads()).toBe(1);
  await expect(page.getByRole("heading", { name: "隔离发布作品", exact: true })).toBeVisible();
});

test("Publish cancellation keeps all fields/files and beforeunload warns only while busy", async ({ page }) => {
  const state = await fixture(page); await page.goto("/upload"); await selectFiles(page);
  const warn = () => page.evaluate(() => { const event = new Event("beforeunload", { cancelable: true }); window.dispatchEvent(event); return event.defaultPrevented; });
  expect(await warn()).toBe(false); await page.getByRole("button", { name: "发布作品", exact: true }).click(); await progress(page, 45); expect(await warn()).toBe(true);
  await page.getByRole("button", { name: "取消上传", exact: true }).click(); await expect(page.getByRole("alert")).toHaveText("上传已取消，文件和表单内容已保留"); expect(await warn()).toBe(false);
  await expect(page.getByLabel(/^标题/)).toHaveValue("隔离发布作品"); await expect(page.getByLabel("简介", { exact: true })).toHaveValue("真实流程，隔离文件与接口响应。"); await expect(page.getByLabel("分区", { exact: true })).toHaveValue("生活"); await expect(page.getByRole("radio", { name: /^仅自己/ })).toBeChecked();
  for (const label of ["视频文件", "封面图片", "字幕文件"]) expect(await page.locator(`input[aria-label="${label}"]`).evaluate((input: HTMLInputElement) => input.files!.length)).toBe(1);
  await expect(page.getByAltText("封面预览")).toBeVisible(); expect(state.uploads()).toBe(0);
});

test("Publish upload API error retains selected media and permits retry", async ({ page }) => {
  const state = await fixture(page, { fail: true }); await page.goto("/upload"); await selectFiles(page); await page.getByRole("button", { name: "发布作品", exact: true }).click(); await page.evaluate(() => window.phase5Upload!.complete());
  await expect(page.getByRole("alert")).toHaveText("隔离验收上传失败"); await expect(page.getByRole("button", { name: "发布作品", exact: true })).toBeEnabled(); await expect(page.getByText(videoName, { exact: true })).toBeVisible(); expect(state.uploads()).toBe(1);
});

test("Publish eight viewports in both themes bound long file names, fields, visibility and upload state", async ({ page }) => {
  test.setTimeout(120_000); await fixture(page); await page.goto("/upload"); await selectFiles(page);
  await page.getByLabel(/^标题/).fill("连续中文标题".repeat(13).slice(0, 80)); await page.getByLabel("简介", { exact: true }).fill("ASCII_underscore_long_description_".repeat(55));
  for (const value of ["dark", "light"] as const) for (const [width, height] of viewports) {
    await page.setViewportSize({ width, height }); await theme(page, value);
    expect(await page.evaluate(() => document.documentElement.scrollWidth), `${value} ${width}`).toBeLessThanOrEqual(width + 1);
    await expect(page.locator(".gv-studio-shell")).toHaveCSS("background-color", value === "light" ? "rgb(240, 241, 238)" : "rgb(8, 10, 13)");
    for (const selector of [".gv-publish-file", ".gv-publish-fields", ".gv-publish-cover", ".gv-publish-subtitle", ".gv-publish-visibility"]) {
      const box = await page.locator(selector).boundingBox(); expect(box!.x).toBeGreaterThanOrEqual(0); expect(box!.x + box!.width).toBeLessThanOrEqual(width + 1);
    }
    const button = await page.getByRole("button", { name: "发布作品", exact: true }).boundingBox(); expect(button!.height).toBeGreaterThanOrEqual(44);
    if (width <= 430) expect(await page.locator(".gv-publish-visibility").evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(" ").length)).toBe(1);
  }
  await page.getByRole("button", { name: "发布作品", exact: true }).click(); await progress(page, 45); expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(321);
  await page.getByRole("button", { name: "取消上传", exact: true }).click();
});

test("Publish opt-in isolated visual acceptance screenshots", async ({ page }, testInfo) => {
  test.skip(process.env.PHASE5_SCREENSHOTS !== "1" || testInfo.project.name !== "desktop-chromium", "Local screenshot collection only");
  test.setTimeout(120_000); await mkdir("../tmp/phase5-screenshots", { recursive: true }); await fixture(page); await page.setViewportSize({ width: 1440, height: 900 }); await page.goto("/upload");
  await expect(page.getByRole("heading", { name: "发布新作品", exact: true })).toBeVisible();
  const capture = async (name: string) => { await page.evaluate(() => { window.scrollTo(0, 0); (document.activeElement as HTMLElement | null)?.blur(); }); await page.mouse.move(0, 0); return page.screenshot({ path: `../tmp/phase5-screenshots/${name}.png`, fullPage: true, animations: "disabled" }); };
  await theme(page, "dark"); await capture("01-publish-dark"); await theme(page, "light"); await capture("02-publish-light"); await selectFiles(page);
  // This explicitly abstract cover is a selected local file, not video content.
  const png = await page.evaluate(() => { const canvas = document.createElement("canvas"); canvas.width = 640; canvas.height = 360; const ctx = canvas.getContext("2d")!; ctx.fillStyle = "#11161d"; ctx.fillRect(0, 0, 640, 360); ctx.strokeStyle = "#00d4d3"; ctx.strokeRect(64, 50, 510, 260); ctx.strokeStyle = "#ff2f62"; ctx.strokeRect(100, 84, 440, 194); ctx.fillStyle = "#b4bbc6"; ctx.font = "20px monospace"; ctx.fillText("ISOLATED COVER FIXTURE", 166, 186); return canvas.toDataURL("image/png").split(",")[1]; });
  await page.getByLabel("封面图片", { exact: true }).setInputFiles({ name: "隔离抽象封面.png", mimeType: "image/png", buffer: Buffer.from(png, "base64") }); await theme(page, "dark"); await capture("03-publish-selected-files");
  await page.setViewportSize({ width: 390, height: 844 }); await theme(page, "light"); await capture("05-publish-mobile390"); await page.setViewportSize({ width: 1440, height: 900 }); await theme(page, "dark");
  await page.getByRole("button", { name: "发布作品", exact: true }).click(); await progress(page, 45); await expect(page.getByRole("progressbar")).toHaveAttribute("aria-valuenow", "45"); await capture("04-publish-uploading");
  await page.getByRole("button", { name: "取消上传", exact: true }).click();
});
