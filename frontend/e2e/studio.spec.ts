import { expect, test, type Page } from "@playwright/test";
import { pageFixture, videoFixture } from "../src/shared/test/videoFixtures";
import type { CreatorStats } from "../src/types";

const viewports = [[1536, 960], [1440, 900], [1180, 820], [920, 900], [768, 1024], [430, 932], [390, 844], [320, 720]];
const longTitle = "超长标题验收_ABCDEFGHIJKLMNOPQRSTUVWXYZ_0123456789_".repeat(8);
const tracks = [{ id: 1, language: "zh-CN", label: "中文", url: "/one.vtt", is_default: true }, { id: 2, language: "en", label: "English", url: "/two.vtt", is_default: false }];
// All mutations are intercepted. These tests never change real accounts, videos or tracks.
async function fixture(page: Page, options: { zero?: boolean; poll?: boolean; lastPage?: boolean; slowEdit?: boolean; error?: boolean; loading?: boolean } = {}) {
  let items = Array.from({ length: 12 }, (_, index) => videoFixture(index + 1, { user_id: 42, username: "验收创作者", title: index === 0 ? longTitle : `隔离验收作品 ${index + 1}`, description: "长简介与下划线_ASCII_".repeat(90), cover_url: "/studio-missing-cover.jpg", views_count: 125430, subtitle_tracks: index === 0 ? [] : tracks, processing_status: index === 1 ? "processing" : index === 2 ? "failed" : "ready", processing_stage: index === 1 ? "transcoding" : index === 2 ? "failed" : "completed", processing_progress: index === 1 ? 20 : 100, processing_error: index === 2 ? "隔离验收：媒体编码失败，可通过操作菜单重新处理。" : undefined }));
  if (options.zero) items = [];
  let reads = 0; let retried = false; let initialReads = 0;
  let allowPoll = false;
  const stats: CreatorStats = { videos_count: options.zero ? 0 : 12543, views_count: options.zero ? 0 : 1234567890, followers_count: options.zero ? 0 : 9876543, likes_count: options.zero ? 0 : 125430, favorites_count: options.zero ? 0 : 6421, comments_count: options.zero ? 0 : 1280, public_count: options.zero ? 0 : 10000, unlisted_count: options.zero ? 0 : 2000, private_count: options.zero ? 0 : 543, processing_count: options.zero ? 0 : 2, recent_videos: items.slice(0, 5) };
  await page.route('**/studio-missing-cover.jpg', (route) => route.fulfill({ status: 404, body: "missing" }));
  await page.route('**/api/v1/**', async (route) => {
    const request = route.request(); const url = new URL(request.url()); const path = url.pathname; let data: unknown;
    if (request.method() !== "GET") expect(request.headers()["x-csrf-token"]).toBe("studio-test-csrf");
    if (path === '/api/v1/auth/me') data = { user: { id: 42, username: "验收创作者", avatar_url: "", bio: "", is_admin: false, created_at: "2026-01-01T00:00:00Z" }, csrf_token: "studio-test-csrf" };
    else if (path === '/api/v1/me/notifications') data = { items: [], page: 1, page_size: 1, total: 0, has_next: false, unread_count: 0 };
    else if (path === '/api/v1/categories') data = ["音乐", "游戏"];
    else if (path === '/api/v1/me/creator/stats') data = stats;
    else if (path === '/api/v1/me/videos') {
      expect(url.searchParams.get('page_size')).toBe('12'); reads++;
      if (options.loading) await new Promise((resolve) => setTimeout(resolve, 1500));
      if (options.error) { await route.fulfill({ status: 500, json: { error: "隔离验收列表错误" } }); return; }
      if (options.poll && allowPoll) items = items.map((item) => item.id === 2 ? { ...item, processing_progress: reads - initialReads === 1 ? 60 : 100, processing_status: reads - initialReads === 1 ? "processing" : "ready", processing_stage: reads - initialReads === 1 ? "transcoding" : "completed" } : item);
      if (retried) items = items.map((item) => item.id === 3 ? { ...item, processing_status: "pending", processing_stage: "queued", processing_progress: 0, processing_error: undefined } : item);
      const pageNumber = Number(url.searchParams.get('page') ?? 1);
      data = pageFixture(options.lastPage && pageNumber === 2 ? items.slice(0, 1) : items, { page: pageNumber, page_size: 12, total: options.lastPage ? 13 : items.length, has_next: options.lastPage && pageNumber === 1 });
    } else if (/\/videos\/\d+\/subtitles/.test(path)) {
      const id = Number(path.split('/')[4]); const item = items.find((video) => video.id === id)!;
      if (request.method() === 'POST') { await new Promise((resolve) => setTimeout(resolve, 800)); data = { ...tracks[0], id: 101 }; item.subtitle_tracks = [...item.subtitle_tracks, data as typeof tracks[number]]; }
      else if (path.endsWith('/default')) { const trackID = Number(path.split('/')[6]); item.subtitle_tracks = item.subtitle_tracks.map((track) => ({ ...track, is_default: track.id === trackID })); data = item.subtitle_tracks; }
      else { const trackID = Number(path.split('/')[6]); item.subtitle_tracks = item.subtitle_tracks.filter((track) => track.id !== trackID); data = item.subtitle_tracks; }
    } else if (/\/videos\/\d+\/retry$/.test(path)) { retried = true; data = { processing_status: "pending" }; }
    else if (/\/videos\/\d+$/.test(path) && request.method() === 'DELETE') { items = items.filter((item) => !path.endsWith(`/${item.id}`)); data = { deleted: true }; }
    else if (/\/videos\/\d+$/.test(path) && request.method() === 'PATCH') {
      if (options.slowEdit) await new Promise((resolve) => setTimeout(resolve, 1200));
      data = items.find((item) => path.endsWith(`/${item.id}`));
    } else { await route.fulfill({ status: 404, json: { error: "unexpected isolated request" } }); return; }
    await route.fulfill({ json: { data } });
  });
  return { stats, reads: () => reads, startPoll: () => { initialReads = reads; allowPoll = true; } };
}
async function noOverflow(page: Page) { expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual((await page.evaluate(() => innerWidth)) + 1); }
async function bounded(page: Page, selector: string) {
  const box = await page.locator(selector).boundingBox(); const viewport = page.viewportSize()!;
  expect(box).not.toBeNull(); expect(box!.x).toBeGreaterThanOrEqual(0); expect(box!.y).toBeGreaterThanOrEqual(0);
  expect(box!.x + box!.width).toBeLessThanOrEqual(viewport.width + 1); expect(box!.y + box!.height).toBeLessThanOrEqual(viewport.height + 1);
}
async function theme(page: Page, value: "light" | "dark") {
  if (await page.locator('html').getAttribute('data-theme') === value) return;
  if (await page.locator('.mobile-menu-button').isVisible()) { await page.locator('.mobile-menu-button').click(); await page.locator('.mobile-account .theme-row').click(); await page.getByRole('dialog', { name: '导航菜单' }).press('Escape'); }
  else await page.getByRole('button', { name: `切换到${value === 'light' ? '浅' : '深'}色主题`, exact: true }).click();
  await expect(page.locator('html')).toHaveAttribute('data-theme', value);
}

test('Studio exact aggregates, independent processing count, default light and saved themes', async ({ page }) => {
  await fixture(page); await page.goto('/creator');
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
  await expect(page.locator('.gv-studio-total strong')).toHaveText('12,543');
  await expect(page.locator('.gv-studio-reach dd')).toHaveText(['1,234,567,890', '9,876,543']);
  await expect(page.locator('.gv-studio-status dd')).toHaveText(['10,000', '2,000', '543']);
  await expect(page.locator('.gv-studio-processing')).toContainText('2 条投稿正在处理');
  await expect(page.locator('.gv-studio-recent-row')).toHaveCount(5);
  for (const value of ['dark', 'light'] as const) {
    await theme(page, value); await page.reload(); await expect(page.locator('html')).toHaveAttribute('data-theme', value);
    await expect(page.locator('.gv-studio-shell')).toHaveCSS('background-color', value === 'light' ? 'rgb(240, 241, 238)' : 'rgb(8, 10, 13)');
  }
});

test('zero Studio is truthful and publishes through the existing route', async ({ page }) => {
  await fixture(page, { zero: true }); await page.goto('/creator');
  await expect(page.getByRole('heading', { name: '还没有投稿' })).toBeVisible();
  await expect(page.getByRole('link', { name: '发布第一条作品' })).toHaveAttribute('href', '/upload');
  await expect(page.locator('.gv-studio-total strong')).toHaveText('0');
  await page.goto('/me/videos'); await expect(page.getByRole('heading', { name: '还没有投稿' })).toBeVisible();
});

test('menu keyboard, dialog focus return, trap, backdrop and busy cancellation', async ({ page }) => {
  await fixture(page, { slowEdit: true }); await page.goto('/me/videos');
  const trigger = page.locator('.gv-content-menu-trigger').first(); await trigger.focus(); await trigger.press('ArrowDown');
  await expect(page.getByRole('menuitem', { name: '编辑视频' })).toBeFocused();
  await page.keyboard.press('End'); await expect(page.getByRole('menuitem', { name: '删除', exact: true })).toBeFocused();
  await page.keyboard.press('Home'); await expect(page.getByRole('menuitem', { name: '编辑视频' })).toBeFocused();
  await page.keyboard.press('Escape'); await expect(trigger).toBeFocused();
  for (const label of ['编辑视频', '字幕管理', '删除']) {
    await trigger.click(); await page.getByRole('menuitem', { name: label, exact: true }).click();
    const dialog = page.getByRole(label === '删除' ? 'alertdialog' : 'dialog'); await expect(dialog).toBeVisible();
    if (label === '删除') await expect(dialog).toHaveAttribute('aria-describedby', 'delete-video-description');
    const buttons = dialog.getByRole('button'); await buttons.last().focus(); await page.keyboard.press('Tab');
    expect(await dialog.evaluate((element) => element.contains(document.activeElement))).toBe(true);
    if (label === '编辑视频') {
      // Existing maxLength is retained; fixture deliberately exceeds it until user edits.
      await dialog.getByLabel(/^标题/).fill('隔离验收修改'); await dialog.getByRole('button', { name: '保存修改' }).click();
      await expect(dialog.getByRole('button', { name: '保存中...' })).toBeDisabled();
      await page.keyboard.press('Escape'); await expect(dialog).toBeVisible();
      await page.locator('.dialog-backdrop').click({ position: { x: 2, y: 2 } }); await expect(dialog).toBeVisible();
      await expect(dialog).toBeHidden(); await expect(page.getByRole('status')).toContainText('投稿信息已保存');
    } else {
      await page.keyboard.press('Escape'); await expect(dialog).toBeHidden();
    }
    await expect(trigger).toBeFocused();
  }
  await trigger.click(); await page.getByRole('heading', { name: '我的投稿' }).click(); await expect(page.getByRole('menu')).toBeHidden();
});

test('polling updates progress 20 -> 60 -> ready and retains row, menu and dialog DOM', async ({ page }) => {
  const state = await fixture(page, { poll: true }); await page.goto('/me/videos');
  const row = page.locator('.gv-content-row').nth(1); await expect(row.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '20');
  const handle = await row.elementHandle(); await row.locator('.gv-content-menu-trigger').click();
  await expect(page.getByRole('menuitem', { name: '删除', exact: true })).toBeDisabled(); state.startPoll();
  await expect(row.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '60');
  await expect(page.getByRole('menu')).toBeVisible();
  await page.getByRole('menuitem', { name: '字幕管理' }).click(); await expect(page.getByRole('dialog')).toBeVisible();
  await expect(row).toHaveClass(/--ready/); await expect(row.getByRole('progressbar')).toHaveCount(0);
  expect(await handle!.evaluate((element) => element === document.querySelectorAll('.gv-content-row')[1])).toBe(true);
  await expect(page.getByRole('dialog')).toBeVisible(); await page.keyboard.press('Escape');
  expect(state.reads()).toBeGreaterThanOrEqual(3);
});

test('failed retry queues media and resumes polling without real writes', async ({ page }) => {
  const state = await fixture(page); await page.goto('/me/videos'); const row = page.locator('.gv-content-row').nth(2);
  await expect(row).toContainText('媒体编码失败'); await row.locator('.gv-content-menu-trigger').click();
  await page.getByRole('menuitem', { name: '重新处理' }).click();
  await expect(row).toHaveClass(/--pending/); await expect(page.getByRole('status')).toContainText('已重新加入转码队列');
  await expect.poll(state.reads).toBeGreaterThanOrEqual(2); await expect(row).toContainText('排队中');
  await expect(row).not.toHaveClass(/--ready/);
});

test('deleting the isolated last row on page 2 returns to page 1', async ({ page }) => {
  await fixture(page, { lastPage: true }); await page.goto('/me/videos?page=2');
  await page.locator('.gv-content-menu-trigger').first().click(); await page.getByRole('menuitem', { name: '删除', exact: true }).click();
  await page.getByRole('button', { name: '确认删除' }).click();
  await expect(page).toHaveURL(/\/me\/videos$/); await expect(page.locator('.gv-content-row')).toHaveCount(11);
});

test('loading and API errors have visible feedback', async ({ page }) => {
  await fixture(page, { loading: true, error: true }); await page.goto('/me/videos');
  await expect(page.getByRole('status', { name: '正在加载投稿' })).toBeVisible();
  await expect(page.locator('.management-error')).toContainText('隔离验收列表错误');
});

test('eight viewports bound large numbers, twelve long rows, menus and all dialogs in both themes', async ({ page }) => {
  test.setTimeout(180_000); await fixture(page);
  for (const value of ['light', 'dark'] as const) {
    for (const [width, height] of viewports) {
      await page.setViewportSize({ width, height }); await page.goto('/creator'); await theme(page, value);
      await expect(page.locator('.gv-studio-total')).toBeVisible(); await noOverflow(page);
      const reach = await page.locator('.gv-studio-reach').boundingBox(); const engagement = await page.locator('.gv-studio-engagement').boundingBox();
      expect(reach!.x + reach!.width).toBeLessThanOrEqual(width); expect(engagement!.x + engagement!.width).toBeLessThanOrEqual(width);
      await page.goto('/me/videos'); await expect(page.locator('.gv-content-row')).toHaveCount(12); await noOverflow(page);
      await expect(page.locator('.gv-video-cover--failed')).toHaveCount(12);
      const trigger = page.locator('.gv-content-menu-trigger').first(); const target = await trigger.boundingBox(); expect(target!.width).toBeGreaterThanOrEqual(44); expect(target!.height).toBeGreaterThanOrEqual(44);
      if (width <= 700) await expect(page.locator('.gv-content-columns')).toBeHidden();
      for (const label of ['编辑视频', '字幕管理', '删除']) {
        await trigger.click(); await bounded(page, '.gv-content-menu'); await page.getByRole('menuitem', { name: label, exact: true }).click();
        await bounded(page, '.gv-studio-dialog'); await noOverflow(page);
        await page.keyboard.press('Escape'); await expect(trigger).toBeFocused();
      }
      const last = page.locator('.gv-content-menu-trigger').last(); await last.scrollIntoViewIfNeeded(); await last.click(); await bounded(page, '.gv-content-menu'); await page.keyboard.press('Escape');
    }
  }
});


test('subtitle upload busy guard and real track actions use isolated API responses', async ({ page }) => {
  await fixture(page); await page.goto('/me/videos'); await page.locator('.gv-content-menu-trigger').first().click(); await page.getByRole('menuitem', { name: '字幕管理' }).click();
  const dialog = page.getByRole('dialog'); await expect(dialog.locator('.subtitle-track-empty')).toBeVisible();
  await dialog.getByLabel('字幕文件').setInputFiles({ name: 'isolated.vtt', mimeType: 'text/vtt', buffer: Buffer.from('WEBVTT\n\n00:00:00.000 --> 00:00:01.000\n验收') });
  await dialog.getByRole('button', { name: '上传轨道' }).click(); await expect(dialog.getByRole('button', { name: '关闭字幕管理' })).toBeDisabled();
  await page.keyboard.press('Escape'); await expect(dialog).toBeVisible(); await expect(dialog.getByRole('status')).toContainText('字幕轨道已上传'); await expect(dialog.locator('.subtitle-track-row')).toHaveCount(1);
  await page.keyboard.press('Escape'); await page.locator('.gv-content-menu-trigger').nth(1).click(); await page.getByRole('menuitem', { name: '字幕管理' }).click();
  await dialog.getByRole('button', { name: '设为默认' }).click(); await expect(dialog.getByRole('status')).toContainText('English');
  page.on('dialog', (native) => native.accept()); await dialog.locator('.subtitle-track-row').first().getByRole('button', { name: '删除', exact: true }).click();
  await expect(dialog.locator('.subtitle-track-row')).toHaveCount(1); await expect(dialog.getByRole('status')).toContainText('已删除');
});


test('same-page deletion preserves inline status and moves focus to the heading', async ({ page }) => {
  await fixture(page); await page.goto('/me/videos'); await page.locator('.gv-content-menu-trigger').first().click();
  await page.getByRole('menuitem', { name: '删除', exact: true }).click(); await page.getByRole('button', { name: '确认删除' }).click();
  await expect(page.locator('.gv-content-row')).toHaveCount(11); await expect(page.getByRole('status')).toContainText('投稿及其媒体文件已删除');
  await expect(page.getByRole('heading', { name: '我的投稿' })).toBeFocused();
});
