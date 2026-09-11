import { expect, test, type Page } from "@playwright/test";

type VideoListEnvelope = {
  data?: { items?: Array<{ id: number; processing_status: string }> };
};

async function expectNoHorizontalOverflow(page: Page) {
  const widths = await page.evaluate(() => ({
    viewport: window.innerWidth,
    document: document.documentElement.scrollWidth,
    body: document.body.scrollWidth
  }));
  expect(widths.document).toBeLessThanOrEqual(widths.viewport + 1);
  expect(widths.body).toBeLessThanOrEqual(widths.viewport + 1);
}

async function firstPublicVideoID(page: Page) {
  const response = await page.request.get("/api/v1/videos?page=1&page_size=24");
  expect(response.ok()).toBeTruthy();
  const payload = await response.json() as VideoListEnvelope;
  return payload.data?.items?.find((video) => video.processing_status === "ready")?.id;
}

test("home renders without overflow and protected routes preserve the destination", async ({ page }) => {
  await page.goto("/");
  await expect(page).toHaveTitle("GVideo");
  await expect(page.locator(".app-shell")).toBeVisible();
  await expectNoHorizontalOverflow(page);

  await page.goto("/upload");
  await expect(page).toHaveURL(/\/auth\?next=%2Fupload$/);
  await expect(page.locator(".auth-form-wrap form")).toBeVisible();

  await page.goto("/me/videos");
  await expect(page).toHaveURL(/\/auth\?next=%2Fme%2Fvideos$/);
  await expect(page.locator(".auth-form-wrap form")).toBeVisible();
});

test("theme selection persists after navigation and reload", async ({ page, isMobile }) => {
  await page.goto("/");
  await page.evaluate(() => localStorage.removeItem("gvideo-theme"));
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");

  if (isMobile) {
    await page.locator(".mobile-menu-button").click();
    await expect(page.locator(".sidebar.open")).toBeVisible();
  }
  const themeButton = isMobile
    ? page.locator(".mobile-account .theme-row")
    : page.locator(".desktop-actions .theme-button");
  await themeButton.click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");

  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expectNoHorizontalOverflow(page);
});

test("video page keeps subtitle management out of playback and comments", async ({ page }) => {
  const videoID = await firstPublicVideoID(page);
  test.skip(!videoID, "No public video is available in the acceptance environment.");

  await page.goto(`/video/${videoID}`, { waitUntil: "domcontentloaded" });
  await expect(page.locator(".watch-page")).toBeVisible({ timeout: 20_000 });
  await expect(page.locator(".player-wrap video")).toBeVisible({ timeout: 20_000 });
  await expect(page.locator(".comment-section")).toBeVisible({ timeout: 20_000 });
  await expect(page.locator(".watch-page .subtitle-manager")).toHaveCount(0);
  await expect(page.locator(".comment-section .subtitle-manager")).toHaveCount(0);
  await expectNoHorizontalOverflow(page);
});

test("video player keyboard scope and controls are accessible", async ({ page, isMobile }) => {
  test.skip(isMobile, "Desktop keyboard model is covered by the desktop project.");
  const videoID = await firstPublicVideoID(page);
  test.skip(!videoID, "No public video is available in the acceptance environment.");

  await page.goto(`/video/${videoID}`, { waitUntil: "domcontentloaded" });
  const player = page.getByRole("region", { name: /视频播放器$/ });
  const media = player.locator("video");
  await expect(player).toBeVisible({ timeout: 20_000 });
  await player.focus();
  await expect(player).toBeFocused();
  await expect(player).toHaveAttribute("aria-keyshortcuts", "Space K ArrowLeft ArrowRight J L Home End ArrowUp ArrowDown M C T F");

  await media.evaluate((element) => {
    const video = element as HTMLVideoElement;
    video.pause();
    let mockedTime = 30;
    Object.defineProperties(video, {
      duration: { configurable: true, get: () => 120 },
      currentTime: {
        configurable: true,
        get: () => mockedTime,
        set: (value: number) => { mockedTime = value; }
      }
    });
  });
  await page.keyboard.press("ArrowRight");
  await expect.poll(() => media.evaluate((element) => (element as HTMLVideoElement).currentTime)).toBe(35);

  const searchInput = page.getByRole("textbox", { name: "搜索" });
  await searchInput.focus();
  await expect(searchInput).toBeFocused();
  await page.keyboard.press("ArrowRight");
  await expect.poll(() => media.evaluate((element) => (element as HTMLVideoElement).currentTime)).toBe(35);

  await player.focus();
  await page.keyboard.press("t");
  const layout = page.locator(".watch-layout");
  await expect(layout).toHaveClass(/theater-mode/);
  await expect(player).toHaveClass(/theater-mode/);
  await expect(player.getByRole("button", { name: "宽屏模式" })).toHaveAttribute("aria-pressed", "true");
  const [mainBox, relatedBox] = await Promise.all([
    page.locator(".watch-main").boundingBox(),
    page.locator(".related-panel").boundingBox()
  ]);
  expect(mainBox).not.toBeNull();
  expect(relatedBox).not.toBeNull();
  expect(relatedBox!.y).toBeGreaterThanOrEqual(mainBox!.y + mainBox!.height);
  await expectNoHorizontalOverflow(page);

  await player.getByRole("button", { name: "宽屏模式" }).click();
  await expect(layout).not.toHaveClass(/theater-mode/);
  await expect(player.getByRole("button", { name: "宽屏模式" })).toHaveAttribute("aria-pressed", "false");
});

test("mobile navigation opens as a bounded, keyboard-dismissible panel", async ({ page, isMobile }) => {
  test.skip(!isMobile, "Mobile navigation behavior is only relevant to the mobile project.");
  await page.goto("/");

  const menuButton = page.locator(".mobile-menu-button");
  await menuButton.click();
  await expect(menuButton).toHaveAttribute("aria-expanded", "true");
  await expect(page.locator(".sidebar.open")).toBeVisible();
  const sidebar = await page.locator(".sidebar.open").boundingBox();
  expect(sidebar?.width || 0).toBeLessThanOrEqual(280);
  await expectNoHorizontalOverflow(page);

  await page.keyboard.press("Escape");
  await expect(menuButton).toHaveAttribute("aria-expanded", "false");
  await expect(menuButton).toBeFocused();
});
