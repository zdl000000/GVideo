import { expect, test, type Locator, type Page } from "@playwright/test";
import { installFinalQaFixture } from "./final-qa.fixture";

// API requests are fail-closed fixtures; existing disposable media is read only.
// These tests exercise production CSS, not a preview stylesheet or injected fix.
async function theme(page: Page, value: "light" | "dark") {
  if (await page.locator("html").getAttribute("data-theme") === value) return;
  if (await page.locator(".mobile-menu-button").isVisible()) {
    await page.locator(".mobile-menu-button").click();
    await page.locator(".mobile-account .theme-row").click();
    await page.getByRole("dialog", { name: "导航菜单" }).press("Escape");
  } else await page.getByRole("button", { name: `切换到${value === "light" ? "浅" : "深"}色主题`, exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", value);
  await page.mouse.move(0, 0);
}

async function stationaryCover(trigger: Locator, image: Locator) {
  await trigger.scrollIntoViewIfNeeded();
  await expect(image).toBeVisible({ timeout: 20_000 });
  await expect.poll(() => image.evaluate((element: HTMLImageElement) => element.naturalWidth)).toBeGreaterThan(0);
  await trigger.hover();
  await expect(trigger).toHaveCSS("transform", "none");
  await expect(image).toHaveCSS("transform", "none");
}

async function finiteAnimation(locator: Locator) {
  await expect(locator).toBeVisible();
  const animation = await locator.evaluate((element) => {
    const style = getComputedStyle(element);
    return { iterations: style.animationIterationCount.split(","), durations: style.animationDuration.split(",").map(parseFloat), transitions: style.transitionDuration.split(",").map(parseFloat), opacity: Number(style.opacity) };
  });
  expect(animation.opacity, JSON.stringify(animation)).toBeGreaterThan(0);
  expect(animation.iterations.every((value) => value.trim() !== "infinite" && Number(value) <= 1), JSON.stringify(animation)).toBe(true);
  expect(animation.durations.every((duration) => duration <= .001), JSON.stringify(animation)).toBe(true);
  expect(animation.transitions.every((duration) => duration <= .001), JSON.stringify(animation)).toBe(true);
}

async function touchTarget(locator: Locator, width: number) {
  await expect(locator).toBeVisible();
  await locator.scrollIntoViewIfNeeded();
  const box = await locator.boundingBox();
  expect(box).not.toBeNull();
  expect(box!.width).toBeGreaterThanOrEqual(44);
  expect(box!.height).toBeGreaterThanOrEqual(44);
  expect(box!.x).toBeGreaterThanOrEqual(-1);
  expect(box!.x + box!.width).toBeLessThanOrEqual(width + 1);
}

test("reduced motion suppresses real hero, card and related cover transforms in Light / Dark", async ({ page }) => {
  const fixture = await installFinalQaFixture(page, { role: "authenticated" });
  await page.emulateMedia({ reducedMotion: "reduce" });
  for (const value of ["light", "dark"] as const) {
    await page.goto("/");
    await expect(page.locator(".gv-video-hero")).toBeVisible({ timeout: 20_000 });
    await theme(page, value);
    expect(await page.evaluate(() => matchMedia("(prefers-reduced-motion: reduce)").matches)).toBe(true);
    await stationaryCover(page.locator(".gv-video-hero"), page.locator(".gv-video-hero .gv-cover-image"));
    const card = page.locator(".gv-video-card:has(.gv-cover-image)").first();
    await stationaryCover(card.locator(".cover-link"), card.locator(".gv-cover-image"));
    for (const locator of [page.locator("html"), page.locator("body"), page.locator(".category-scroller")]) {
      await expect(locator).toHaveCSS("scroll-behavior", "auto");
    }
    await page.goto("/video/1");
    await expect(page.locator(".gv-watch-page")).toBeVisible({ timeout: 20_000 });
    const related = page.locator(".related-cover:has(.gv-cover-image)").first();
    await stationaryCover(related, related.locator(".gv-cover-image"));
  }
  expect(fixture.unknownRequests).toEqual([]);
});

test("reduced motion leaves skeleton, loading and notification busy states visible with text", async ({ page }) => {
  const fixture = await installFinalQaFixture(page, { role: "authenticated" });
  await page.emulateMedia({ reducedMotion: "reduce" });
  fixture.gateRead("/api/v1/videos");
  try {
    await page.goto("/");
    await expect(page.getByRole("status", { name: "正在加载视频", exact: true })).toBeVisible({ timeout: 20_000 });
    await expect(page.getByText("正在加载内容", { exact: true })).toBeVisible();
    await finiteAnimation(page.locator(".gv-skeleton").first());
    fixture.releaseRead("/api/v1/videos");
    await expect(page.locator(".gv-video-hero")).toBeVisible({ timeout: 20_000 });
  } finally { fixture.releaseRead("/api/v1/videos"); }

  fixture.gateRead("/api/v1/me/notifications");
  try {
    await page.goto("/notifications");
    await expect(page.getByText("正在读取通知", { exact: true })).toBeVisible({ timeout: 20_000 });
    await finiteAnimation(page.locator(".spinner"));
    fixture.releaseRead("/api/v1/me/notifications");
    await expect(page.locator(".gv-notification-row").first()).toBeVisible({ timeout: 20_000 });
  } finally { fixture.releaseRead("/api/v1/me/notifications"); }

  fixture.gateMutation("/api/v1/me/notifications/1/read");
  try {
    await page.getByRole("button", { name: "标记已读", exact: true }).first().click();
    const busy = page.locator('.gv-notification-row[aria-busy="true"]');
    await expect(busy.getByRole("status")).toHaveText("正在标记已读");
    await expect(busy.getByRole("button", { name: "标记已读", exact: true })).toBeDisabled();
    await finiteAnimation(busy.locator(".gv-notification-busy-icon"));
    await expect(busy.locator(".gv-notification-busy-icon")).toHaveCSS("animation-name", "none");
    fixture.releaseMutation("/api/v1/me/notifications/1/read");
    await expect(busy).toHaveCount(0);
  } finally { fixture.releaseMutation("/api/v1/me/notifications/1/read"); }
  expect(fixture.unknownRequests).toEqual([]);
});

test("reduced motion preserves processing status, numeric progress and its stable fill", async ({ page }) => {
  const fixture = await installFinalQaFixture(page, { role: "authenticated", videoStatus: "processing" });
  fixture.setVideoStatus("processing");
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/video/1");
  const progress = page.locator(".processing-notice .processing-progress");
  await expect(progress).toBeVisible({ timeout: 20_000 });
  const value = Number(await progress.getAttribute("aria-valuenow"));
  expect(value).toBeGreaterThanOrEqual(0);
  expect(value).toBeLessThanOrEqual(100);
  await expect(progress.locator("strong")).toHaveText(`${value}%`);
  await expect(progress.locator(".processing-progress-copy > span")).not.toHaveText("");
  const fill = progress.locator(".processing-progress-track > span");
  expect(await fill.evaluate((element) => parseFloat(getComputedStyle(element).transitionDuration))).toBeLessThanOrEqual(.001);
  expect(await fill.evaluate((element) => parseFloat((element as HTMLElement).style.width))).toBe(value);
  await expect(page.locator(".processing-notice")).toContainText("后台正在生成自适应清晰度");
  expect(fixture.unknownRequests).toEqual([]);
});

test("fine pointer retains optional cover hover enhancement without reduced motion", async ({ page, isMobile }) => {
  test.skip(isMobile, "Fine-pointer enhancement is verified in desktop-chromium");
  const fixture = await installFinalQaFixture(page, { role: "authenticated" });
  await page.emulateMedia({ reducedMotion: "no-preference" });
  await page.goto("/");
  await expect(page.locator(".gv-video-hero")).toBeVisible({ timeout: 20_000 });
  expect(await page.evaluate(() => matchMedia("(hover: hover) and (pointer: fine)").matches)).toBe(true);
  await page.locator(".gv-video-hero").hover();
  await expect.poll(() => page.locator(".gv-video-hero .gv-cover-image").evaluate((element) => new DOMMatrixReadOnly(getComputedStyle(element).transform).a)).toBeGreaterThan(1.005);
  const card = page.locator(".gv-video-card:has(.gv-cover-image)").first().locator(".cover-link");
  await card.hover();
  await expect.poll(() => card.evaluate((element) => new DOMMatrixReadOnly(getComputedStyle(element).transform).f)).toBeLessThan(-1);
  await page.goto("/video/1");
  const related = page.locator(".related-cover:has(.gv-cover-image)").first();
  await expect(related).toBeVisible({ timeout: 20_000 });
  await related.hover();
  await expect.poll(() => related.locator(".gv-cover-image").evaluate((element) => new DOMMatrixReadOnly(getComputedStyle(element).transform).a)).toBeGreaterThan(1.005);
  expect(fixture.unknownRequests).toEqual([]);
});

test("touch at 390 / 320 has discoverable actions and no sticky fine-pointer hover styles", async ({ page, isMobile }) => {
  test.skip(!isMobile, "Coarse-pointer behavior is verified in mobile-chromium");
  const fixture = await installFinalQaFixture(page, { role: "authenticated" });
  await page.emulateMedia({ reducedMotion: "no-preference" });
  for (const width of [390, 320]) {
    await page.setViewportSize({ width, height: 844 });
    await page.goto("/");
    await expect(page.locator(".gv-video-hero")).toBeVisible({ timeout: 20_000 });
    expect(await page.evaluate(() => matchMedia("(hover: hover) and (pointer: fine)").matches)).toBe(false);
    expect(await page.evaluate(() => matchMedia("(pointer: coarse)").matches)).toBe(true);
    await stationaryCover(page.locator(".gv-video-hero"), page.locator(".gv-video-hero .gv-cover-image"));
    const card = page.locator(".gv-video-card:has(.gv-cover-image)").first();
    await expect(card.locator(".video-title")).toBeVisible();
    await stationaryCover(card.locator(".cover-link"), card.locator(".gv-cover-image"));

    await page.goto("/video/1");
    await expect(page.locator(".gv-watch-page")).toBeVisible({ timeout: 20_000 });
    for (const name of ["点赞", "收藏", "分享", "举报"]) await touchTarget(page.getByRole("button", { name, exact: true }), width);
    await touchTarget(page.getByRole("button", { name: "播放", exact: true }), width);
    await touchTarget(page.locator(".creator-strip-action"), width);

    await page.goto("/creator");
    await touchTarget(page.locator(".gv-studio-recent-copy .gv-content-title").first(), width);

    await page.goto("/me/videos");
    await touchTarget(page.locator(".gv-content-copy .gv-content-title").first(), width);
    for (const link of await page.locator(".gv-studio-text-link").all()) await touchTarget(link, width);
    const trigger = page.locator(".gv-content-menu-trigger").first();
    await touchTarget(trigger, width);
    const initialMenuBackground = await trigger.evaluate((element) => getComputedStyle(element).backgroundColor);
    await trigger.hover();
    await expect(trigger).toHaveCSS("background-color", initialMenuBackground);
    await trigger.tap();
    await expect(trigger).toHaveAttribute("aria-expanded", "true");
    for (const name of ["编辑视频", "字幕管理", "删除"]) await touchTarget(page.getByRole("menuitem", { name, exact: true }), width);
    await page.keyboard.press("Escape");
    await expect(trigger).toHaveAttribute("aria-expanded", "false");

    await page.goto("/upload");
    for (const selector of [".gv-publish-file", ".gv-publish-cover", ".gv-publish-subtitle"]) await touchTarget(page.locator(selector), width);
    const file = page.locator(".gv-publish-file");
    const initialBorder = await file.evaluate((element) => getComputedStyle(element).borderTopColor);
    await file.hover();
    await expect(file).toHaveCSS("border-top-color", initialBorder);

    await page.goto("/notifications");
    const read = page.getByRole("button", { name: "标记已读", exact: true }).first();
    await touchTarget(read, width);
    const initialReadBackground = await read.evaluate((element) => getComputedStyle(element).backgroundColor);
    await read.hover();
    await expect(read).toHaveCSS("background-color", initialReadBackground);
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width + 1);
  }
  expect(fixture.unknownRequests).toEqual([]);
});
