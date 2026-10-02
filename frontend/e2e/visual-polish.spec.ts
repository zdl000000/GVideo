import { expect, test } from "@playwright/test";
import { pageFixture, videoFixture } from "../src/shared/test/videoFixtures";

for (const [kind, title] of [["Chinese", "超长中文视频标题".repeat(40)], ["ASCII", "GVIDEO_LONG_TITLE_1234567890_".repeat(30)]]) {
  test(`discovery visual polish: ${kind} hero keeps title, metadata and CTA within bounds`, async ({ page }) => {
    const video = videoFixture(1, { title, username: "long_creator_".repeat(12), description: "用于验收的描述文本".repeat(40) });
    await page.route('**/api/v1/videos?*', (route) => route.fulfill({ json: { data: pageFixture([video]) } }));
    await page.route('**/api/v1/categories', (route) => route.fulfill({ json: { data: [] } }));
    for (const width of [1440, 920, 768, 430, 390, 320]) {
      await page.setViewportSize({ width, height: 900 });
      await page.goto('/');
      const hero = page.locator('.gv-video-hero');
      await expect(hero).toBeVisible();
      await expect(hero.locator('.gv-media-caption')).toHaveText('封面占位');
      const geometry = await hero.evaluate((element) => {
        const box = element.getBoundingClientRect();
        const titleLink = element.querySelector('h2 a')!;
        const titleBox = titleLink.getBoundingClientRect();
        const titleCSS = getComputedStyle(titleLink);
        const metaBox = element.querySelector('.gv-hero-meta')!.getBoundingClientRect();
        const ctaBox = element.querySelector('.gv-button')!.getBoundingClientRect();
        return { width: innerWidth, documentWidth: document.documentElement.scrollWidth, heroTop: box.top, heroBottom: box.bottom, height: box.height, titleTop: titleBox.top, titleBottom: titleBox.bottom, titleLines: titleBox.height / parseFloat(titleCSS.lineHeight), metaTop: metaBox.top, metaBottom: metaBox.bottom, ctaTop: ctaBox.top, ctaBottom: ctaBox.bottom };
      });
      expect(geometry.documentWidth).toBeLessThanOrEqual(width + 1);
      expect(geometry.titleTop).toBeGreaterThanOrEqual(geometry.heroTop);
      expect(geometry.titleLines).toBeLessThanOrEqual(width <= 700 ? 2.05 : 3.05);
      expect(geometry.titleBottom).toBeLessThanOrEqual(geometry.metaTop);
      expect(geometry.metaBottom).toBeLessThanOrEqual(geometry.ctaTop);
      expect(geometry.ctaBottom).toBeLessThanOrEqual(geometry.heroBottom);
      expect(geometry.height).toBeLessThanOrEqual(width <= 520 ? 410 : 520);
    }
  });
}

test('discovery visual polish: personal empty states retain semantics and CTA', async ({ page }) => {
  await page.route('**/api/v1/auth/me', (route) => route.fulfill({ json: { data: { user: { id: 1, username: '测试账户', avatar_url: '', bio: '', is_admin: false, created_at: '2026-01-01T00:00:00Z' }, csrf_token: 'test-csrf' } } }));
  await page.route('**/api/v1/me/notifications?*', (route) => route.fulfill({ json: { data: { ...pageFixture([]), unread_count: 0 } } }));
  await page.route('**/api/v1/me/following/videos?*', (route) => route.fulfill({ json: { data: pageFixture([]) } }));
  await page.route('**/api/v1/me/favorites?*', (route) => route.fulfill({ json: { data: pageFixture([]) } }));
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    for (const [path, title, cta] of [['/following', '关注动态还是空的', '发现创作者'], ['/favorites', '还没有收藏', '去发现作品']]) {
      await page.goto(path);
      const empty = page.locator('.gv-watch-empty');
      await expect(empty.getByRole('heading', { name: title, exact: true })).toBeVisible();
      await expect(empty.getByRole('link', { name: cta, exact: true })).toHaveAttribute('href', '/popular');
      expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width + 1);
    }
  }
});
