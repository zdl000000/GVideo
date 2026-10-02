import { expect, test, type Page } from "@playwright/test";
import { pageFixture, videoFixture } from "../src/shared/test/videoFixtures";
import type { Video, VideoPage as VideoPageData } from "../src/types";

async function noOverflow(page: Page) {
  const size = await page.evaluate(() => ({ viewport: innerWidth, width: document.documentElement.scrollWidth }));
  expect(size.width).toBeLessThanOrEqual(size.viewport + 1);
}
async function loaded(page: Page) {
  await expect(page.locator('.gv-discovery-page')).toBeVisible();
  await expect(page.locator('[aria-label="正在加载视频"]')).toHaveCount(0);
  await expect(page.locator('.error-state')).toHaveCount(0);
}
async function publicPage(page: Page): Promise<VideoPageData> {
  const response = await page.request.get('/api/v1/videos?sort=latest&page=1&page_size=36');
  expect(response.ok()).toBeTruthy();
  return (await response.json()).data;
}

test('discovery home uses real data once per video and pagination preserves the feed', async ({ page }) => {
  const data = await publicPage(page);
  await page.goto('/');
  await loaded(page);
  const ids = await page.locator('[data-video-id]').evaluateAll((nodes) => nodes.map((node) => node.getAttribute('data-video-id')));
  expect(new Set(ids).size).toBe(ids.length);
  for (const video of data.items) expect(ids).toContain(String(video.id));
  await noOverflow(page);
  if (data.has_next) {
    await page.getByRole('button', { name: '下一页' }).click();
    await expect(page).toHaveURL(/page=2/);
    await loaded(page);
    await expect(page.locator('.gv-video-hero')).toHaveCount(0);
    await noOverflow(page);
  }
});

test('discovery global search enters results mode and category changes keep q', async ({ page }) => {
  const data = await publicPage(page);
  const title = data.items[0]?.title;
  test.skip(!title, 'No public videos exist in this environment.');
  await page.goto('/');
  await loaded(page);
  const input = page.getByRole('textbox', { name: '搜索', exact: true });
  await input.fill(title!);
  await input.press('Enter');
  await expect(page).toHaveURL(/\?q=/);
  await loaded(page);
  await expect(page.getByRole('heading', { name: `“${title}”的搜索结果` })).toBeVisible();
  await expect(page.locator('.gv-video-hero')).toHaveCount(0);
  await expect(page.locator('.gv-popular-preview')).toHaveCount(0);
  const category = data.items[0].category;
  await page.getByRole('group', { name: '视频分类' }).getByRole('button', { name: category, exact: true }).click();
  await loaded(page);
  const params = new URL(page.url()).searchParams;
  expect(params.get('q')).toBe(title);
  expect(params.get('category')).toBe(category);
  await expect(page.getByRole('button', { name: category, exact: true })).toHaveAttribute('aria-pressed', 'true');
  await noOverflow(page);
});

test('discovery latest category and popular switch retain real filters', async ({ page }) => {
  const response = await page.request.get('/api/v1/categories');
  expect(response.ok()).toBeTruthy();
  const categories: string[] = (await response.json()).data;
  test.skip(!categories.length, 'No categories exist in this environment.');
  await page.goto('/latest');
  await loaded(page);
  await page.getByRole('group', { name: '视频分类' }).getByRole('button', { name: categories[0], exact: true }).click();
  await expect(page).toHaveURL(/category=/);
  await loaded(page);
  await page.getByRole('navigation', { name: '视频排序' }).getByRole('link', { name: '热门', exact: true }).click();
  await expect(page).toHaveURL(/\/popular\?category=/);
  await loaded(page);
  expect(new URL(page.url()).searchParams.get('category')).toBe(categories[0]);
  await noOverflow(page);
});

test('discovery popular respects API ordering and ranks continue on page two', async ({ page }) => {
  const response = await page.request.get('/api/v1/videos?sort=popular&page=1&page_size=36');
  expect(response.ok()).toBeTruthy();
  const data: VideoPageData = (await response.json()).data;
  await page.goto('/popular');
  await loaded(page);
  const ids = await page.locator('[data-video-id]').evaluateAll((nodes) => nodes.map((node) => Number(node.getAttribute('data-video-id'))));
  expect(ids).toEqual(data.items.map((video) => video.id));
  for (let i = 0; i < Math.min(3, data.items.length); i++) await expect(page.getByLabel(`第 ${i + 1} 名`, { exact: true })).toBeVisible();
  await noOverflow(page);
  if (data.has_next) {
    await page.getByRole('button', { name: '下一页' }).click();
    await expect(page).toHaveURL(/page=2/);
    await loaded(page);
    await expect(page.getByLabel(`第 ${data.page_size + 1} 名`, { exact: true })).toBeVisible();
    await expect(page.locator('.gv-popular-leaders')).toHaveCount(0);
    await noOverflow(page);
  }
});

test('discovery following and favorites keep login guards', async ({ page }) => {
  for (const path of ['/following', '/favorites']) {
    await page.goto(path);
    await expect(page).toHaveURL(new RegExp(`/auth\\?next=${encodeURIComponent(path)}$`));
    await expect(page.getByLabel('用户名')).toBeVisible();
  }
});

// Isolated browser fixtures exercise personal-library UI without touching persistent users/data.
async function personalFixture(page: Page, items: Video[]) {
  await page.route('**/api/v1/auth/me', (route) => route.fulfill({ json: { data: { user: { id: 1, username: '测试账户', avatar_url: '', bio: '', is_admin: false, created_at: '2026-01-01T00:00:00Z' }, csrf_token: 'test-csrf' } } }));
  await page.route('**/api/v1/me/notifications?*', (route) => route.fulfill({ json: { data: { items: [], page: 1, page_size: 1, total: 0, has_next: false, unread_count: 0 } } }));
  await page.route('**/api/v1/me/following/videos?*', (route) => route.fulfill({ json: { data: pageFixture(items, { page_size: 24 }) } }));
  await page.route('**/api/v1/me/favorites?*', (route) => route.fulfill({ json: { data: pageFixture(items, { page_size: 24 }) } }));
}

test('discovery personal fixtures: following identities and favorites removal stay usable', async ({ page }) => {
  const items = [videoFixture(901, { title: '测试收藏作品', username: '中文创作者'.repeat(8), views_count: 0 }), videoFixture(902)];
  await personalFixture(page, items);
  let calls = 0;
  await page.route('**/api/v1/videos/901/favorite', async (route) => {
    calls += 1;
    expect(route.request().headers()['x-csrf-token']).toBe('test-csrf');
    await route.fulfill({ json: { data: { active: false } } });
  });
  await page.route('**/api/v1/me/favorites?*', (route) => route.fulfill({ json: { data: pageFixture(calls ? items.slice(1) : items, { page_size: 24 }) } }));
  await page.goto('/following');
  await loaded(page);
  await expect(page.locator('.gv-video-card--creator-first')).toHaveCount(2);
  await expect(page.getByRole('link', { name: items[0].username, exact: true })).toBeVisible();
  await noOverflow(page);
  await page.goto('/favorites');
  await loaded(page);
  const entry = page.locator('.gv-saved-entry').first();
  const next = page.locator('.gv-saved-entry').nth(1);
  const before = await next.boundingBox();
  await page.getByRole('button', { name: '取消收藏 测试收藏作品', exact: true }).click();
  await expect(entry.getByText('已取消收藏', { exact: true })).toBeVisible();
  await expect(page.getByText('1 条收藏', { exact: true })).toBeVisible();
  expect(calls).toBe(1);
  const after = await next.boundingBox();
  expect(after!.x).toBeCloseTo(before!.x, 0);
  expect(after!.y).toBeCloseTo(before!.y, 0);
  await expect(entry.locator('.gv-saved-content')).toHaveAttribute('inert', '');
  await noOverflow(page);
  await entry.getByRole('button', { name: '更新列表', exact: true }).click();
  await loaded(page);
  await expect(page.locator('.gv-saved-entry')).toHaveCount(1);
  await expect(page.getByRole('link', { name: '作品902', exact: true })).toBeVisible();
  await expect(page.getByText('已取消收藏', { exact: true })).toHaveCount(0);
});

test('discovery fixture covers zero/one/many records, long text, 404 and eight viewports', async ({ page }) => {
  test.setTimeout(120_000);
  let items = Array.from({ length: 40 }, (_, i) => videoFixture(i + 1, { title: '长中文标题'.repeat(16), username: 'long_creator_name_'.repeat(10), cover_url: '/phase2-missing-cover.jpg', views_count: 0, comments_count: 0, description: '描述文本'.repeat(40) }));
  await page.route('**/phase2-missing-cover.jpg', (route) => route.fulfill({ status: 404, body: 'not found' }));
  await page.route('**/api/v1/videos?*', (route) => {
    const params = new URL(route.request().url()).searchParams;
    const pageSize = Number(params.get('page_size') || 36);
    return route.fulfill({ json: { data: pageFixture(items.slice(0, pageSize), { page_size: pageSize, total: items.length, has_next: items.length > pageSize }) } });
  });
  await page.route('**/api/v1/categories', (route) => route.fulfill({ json: { data: Array.from({ length: 24 }, (_, i) => `测试分类${i}`) } }));
  for (const [width, height] of [[1536, 960], [1440, 900], [1180, 820], [920, 900], [768, 1024], [430, 932], [390, 844], [320, 720]]) {
    await page.setViewportSize({ width, height });
    await page.goto('/');
    await loaded(page);
    await expect(page.locator('.gv-video-hero .gv-cover-image')).toHaveCount(0);
    await expect(page.locator('.gv-video-hero .gv-media-fallback')).toBeVisible();
    await noOverflow(page);
    const hero = await page.locator('.gv-video-hero').boundingBox();
    if (width <= 520) expect(hero!.height).toBeLessThanOrEqual(410);
    const cta = await page.getByRole('link', { name: '立即观看', exact: true }).boundingBox();
    expect(cta!.y + cta!.height).toBeLessThanOrEqual(hero!.y + hero!.height + 1);
    if (width <= 520) expect(await page.locator('.gv-latest-mosaic').evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(' ').length)).toBe(1);
    await page.goto('/popular');
    await loaded(page);
    await noOverflow(page);
  }
  items = [items[0]];
  for (const [width, height] of [[1536, 960], [1440, 900], [1180, 820], [920, 900], [768, 1024], [430, 932], [390, 844], [320, 720]]) {
    await page.setViewportSize({ width, height });
    await page.goto('/');
    await loaded(page);
    await expect(page.locator('[data-video-id]')).toHaveCount(1);
    await expect(page.locator('.gv-secondary-stories')).toHaveCount(0);
    await noOverflow(page);
    const hero = await page.locator('.gv-video-hero').boundingBox();
    const cta = await page.getByRole('link', { name: '立即观看', exact: true }).boundingBox();
    expect(cta!.y + cta!.height).toBeLessThanOrEqual(hero!.y + hero!.height + 1);
    if (width <= 520) expect(hero!.height).toBeLessThanOrEqual(410);
  }
  items = [];
  await page.reload();
  await loaded(page);
  await expect(page.getByText('这里还没有视频', { exact: true })).toBeVisible();
});
