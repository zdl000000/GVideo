import { expect, test, type Page } from "@playwright/test";
import { pageFixture, videoFixture } from "../src/shared/test/videoFixtures";
import type { CreatorProfile, User, Video } from "../src/types";

const viewports = [[1536, 960], [1440, 900], [1180, 820], [920, 900], [768, 1024], [430, 932], [390, 844], [320, 720]];
const creator: CreatorProfile = { id: 42, username: "频道创作者", avatar_url: "", bio: "真实功能，隔离验收数据。", created_at: "2026-01-01T00:00:00Z", videos_count: 13, followers_count: 3, following_count: 0, followed: false };
const viewer: User = { ...creator, id: 1, username: "验收账户", is_admin: false };

// Only API responses are isolated. The player streams an existing local media asset;
// these tests never register accounts, upload media or write to the real database.
async function fixture(page: Page, options: { self?: boolean; anonymous?: boolean; empty?: boolean; video?: Partial<Video>; profile?: Partial<CreatorProfile> } = {}) {
  const mediaVideoId = process.env.E2E_MEDIA_VIDEO_ID || '56';
  const mediaResponse = await page.request.get(`/api/v1/videos/${mediaVideoId}?count_view=false`);
  expect(mediaResponse.ok()).toBe(true);
  const media = (await mediaResponse.json()).data as Video;
  const profile = { ...creator, ...options.profile };
  const first = videoFixture(1, { ...media, id: 1, user_id: 42, username: profile.username, title: "播放验收作品一", description: "第一行简介。\n第二行简介。", comments_count: 0, ...options.video });
  const second = videoFixture(2, { ...first, id: 2, title: "播放验收作品二", cover_url: "/phase3-missing-cover.jpg" });
  let comments: { id: number; video_id: number; user_id: number; username: string; avatar_url: string; content: string; created_at: string }[] = [];
  await page.route('**/phase3-missing-cover.jpg', (route) => route.fulfill({ status: 404, body: 'not found' }));
  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    if (request.method() !== 'GET') expect(request.headers()['x-csrf-token']).toBe('test-csrf');
    let data: unknown;
    if (path === '/api/v1/auth/me') {
      if (options.anonymous) { await route.fulfill({ status: 401, json: { error: 'unauthorized' } }); return; }
      data = { user: options.self ? { ...profile, is_admin: false } : viewer, csrf_token: 'test-csrf' };
    } else if (path === '/api/v1/me/notifications') {
      data = { items: [], page: 1, page_size: 1, total: 0, has_next: false, unread_count: 0 };
    } else if (/\/videos\/\d+\/comments\/\d+$/.test(path)) {
      comments = comments.filter((comment) => !path.endsWith(`/${comment.id}`)); data = { deleted: true };
    } else if (/\/videos\/\d+\/comments$/.test(path)) {
      if (request.method() === 'POST') {
        const comment = { id: 101, video_id: 1, user_id: options.self ? 42 : 1, username: '验收账户', avatar_url: '', content: request.postDataJSON().content, created_at: '2026-01-01T00:00:00Z' };
        comments = [comment, ...comments]; data = comment;
      } else data = comments;
    } else if (/\/videos\/\d+\/(like|favorite)$/.test(path)) data = { active: true };
    else if (/\/users\/\d+\/follow$/.test(path)) data = { active: true };
    else if (/\/videos\/\d+\/reports$/.test(path)) data = { id: 1, ...request.postDataJSON() };
    else if (/\/videos\/\d+$/.test(path)) data = path.endsWith('/2') ? second : first;
    else if (/\/users\/\d+\/videos$/.test(path)) {
      const isNext = url.searchParams.get('page') === '2';
      data = pageFixture(options.empty ? [] : isNext ? [second] : Array.from({ length: 12 }, (_, index) => ({ ...second, id: index + 2, title: `频道作品${index + 1}` })), { page: isNext ? 2 : 1, page_size: 12, total: options.empty ? 0 : 13, has_next: !options.empty && !isNext });
    } else if (/\/users\/\d+$/.test(path)) data = profile;
    else if (path === '/api/v1/videos') data = pageFixture(options.empty ? [] : [first, second]);
    else { await route.fulfill({ status: 404, json: { error: 'unexpected isolated API request' } }); return; }
    await route.fulfill({ json: { data } });
  });
  return { first, second };
}

async function noOverflow(page: Page) {
  const size = await page.evaluate(() => ({ viewport: innerWidth, width: document.documentElement.scrollWidth, overflowing: [...document.querySelectorAll('main *')].filter((element) => element.getBoundingClientRect().right > innerWidth + 1).slice(0, 6).map((element) => ({ tag: element.tagName, className: element.className, right: element.getBoundingClientRect().right })) }));
  expect(size.width, JSON.stringify(size)).toBeLessThanOrEqual(size.viewport + 1);
}

async function switchTheme(page: Page, theme: 'light' | 'dark') {
  if (await page.locator('.mobile-menu-button').isVisible()) {
    await page.locator('.mobile-menu-button').click();
    await page.locator('.mobile-account .theme-row').click();
    await page.getByRole('dialog', { name: '导航菜单' }).press('Escape');
  } else await page.getByRole('button', { name: `切换到${theme === 'light' ? '浅' : '深'}色主题`, exact: true }).click();
  await expect(page.locator('html')).toHaveAttribute('data-theme', theme);
}

test('Phase 3.1 playback light/dark uses WATCH canvas and surfaces with a dark media stage', async ({ page }) => {
  await fixture(page);
  await page.goto('/video/1');
  // A fresh browser context has no saved preference: the product default stays light.
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
  for (const theme of ['light', 'dark', 'light'] as const) {
    if (await page.locator('html').getAttribute('data-theme') !== theme) await switchTheme(page, theme);
    await page.reload();
    await expect(page.locator('html')).toHaveAttribute('data-theme', theme);
    await expect(page.locator('.gv-watch-page')).toBeVisible();
    const canvas = theme === 'light' ? 'rgb(238, 237, 233)' : 'rgb(8, 10, 13)';
    const surface = theme === 'light' ? 'rgb(250, 249, 246)' : 'rgb(17, 22, 29)';
    const canvasToken = theme === 'light' ? '#eeede9' : '#080a0d';
    const surfaceToken = theme === 'light' ? '#faf9f6' : '#11161d';
    if (theme === 'light') await expect(page.locator('.gv-watch-shell')).toHaveCSS('background-color', canvas);
    await expect(page.locator('.comment-form textarea')).toHaveCSS('background-color', surface);
    await expect(page.locator('.player-wrap')).toHaveCSS('background-color', 'rgb(12, 13, 16)');
    await expect(page.locator('.watch-title-row h1')).toHaveCSS('color', theme === 'light' ? 'rgb(17, 19, 24)' : 'rgb(247, 248, 250)');
    // Transparent editorial sections must actually reveal the warm shell, not an opaque dark ancestor.
    for (const selector of ['.watch-title-row', '.watch-meta', '.creator-strip', '.gv-watch-description', '.comment-section', '.related-panel']) {
      const colors = await page.locator(selector).evaluate((element) => {
        const css = getComputedStyle(element);
        let ancestor: Element | null = element;
        while (ancestor && getComputedStyle(ancestor).backgroundColor === 'rgba(0, 0, 0, 0)') ancestor = ancestor.parentElement;
        return { canvas: css.getPropertyValue('--gv-bg').trim(), surface: css.getPropertyValue('--gv-surface').trim(), visibleBackground: ancestor ? getComputedStyle(ancestor).backgroundColor : '' };
      });
      expect(colors, `${theme}: ${selector}`).toEqual({ canvas: canvasToken, surface: surfaceToken, visibleBackground: canvas });
    }
    await noOverflow(page);
  }
});

test('Phase 3.1 related broken and empty covers retain a compact brand mark without placeholder copy', async ({ page }) => {
  const { first, second } = await fixture(page);
  await page.route('**/api/v1/videos?*', (route) => route.fulfill({ json: { data: pageFixture([first, second, { ...second, id: 3, cover_url: '' }]) } }));
  await page.goto('/video/1');
  for (const theme of ['light', 'dark'] as const) {
    if (await page.locator('html').getAttribute('data-theme') !== theme) await switchTheme(page, theme);
    for (const width of [1440, 390]) {
      await page.setViewportSize({ width, height: 900 });
      const covers = page.locator('.related-video .gv-video-cover--failed');
      await expect(covers).toHaveCount(2);
      for (const cover of await covers.all()) {
        const mark = cover.locator('.gv-media-fallback > img');
        await expect(mark).toBeVisible();
        await expect.poll(() => mark.evaluate((element: HTMLImageElement) => element.naturalWidth)).toBeGreaterThan(0);
        await expect(cover.locator('.gv-cover-image')).toHaveCount(0);
        await expect(cover.locator('.gv-media-caption')).toBeHidden();
        await expect(cover.locator('.gv-media-label')).toBeHidden();
        const bounds = await cover.boundingBox();
        const markBounds = await mark.boundingBox();
        expect(markBounds!.width).toBe(26); expect(markBounds!.height).toBe(26);
        expect(markBounds!.x).toBeGreaterThan(bounds!.x);
        expect(markBounds!.x + markBounds!.width).toBeLessThan(bounds!.x + bounds!.width);
      }
      await noOverflow(page);
    }
  }
});

test('Phase 3.1 channel geometry shares the identity stage on desktop and mobile without a banner surface', async ({ page }) => {
  await fixture(page, { self: true });
  await page.goto('/users/42');
  for (const [width, height] of viewports) {
    await page.setViewportSize({ width, height });
    const backdrop = page.locator('.gv-channel-backdrop');
    await expect(backdrop).toHaveCSS('position', 'absolute');
    await expect(backdrop).toHaveCSS('background-color', 'rgba(0, 0, 0, 0)');
    await expect(backdrop).toHaveCSS('border-top-width', '0px');
    await expect(backdrop.locator('img')).toHaveCount(0);
    const bounds = await backdrop.boundingBox();
    const hero = await page.locator('.creator-hero').boundingBox();
    const avatar = await page.locator('.creator-hero-avatar').boundingBox();
    const name = await page.getByRole('heading', { level: 1 }).boundingBox();
    expect(bounds!.x).toBeGreaterThan(hero!.x + hero!.width * .2);
    expect(avatar!.y).toBeLessThan(bounds!.y + bounds!.height);
    expect(name!.y).toBeLessThan(bounds!.y + bounds!.height);
    await expect(page.locator('.creator-stats dd')).toHaveText(['13', '3', '0']);
    await expect(page.getByRole('button', { name: '编辑资料', exact: true })).toBeVisible();
    await expect(page.getByRole('link', { name: '管理投稿', exact: true })).toHaveAttribute('href', '/me/videos');
    await expect(page.getByRole('link', { name: '创作者中心', exact: true })).toHaveAttribute('href', '/creator');
    await noOverflow(page);
  }
});

test('playback media stays mounted while playing, resizing and toggling theater', async ({ page }) => {
  await fixture(page);
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('/video/1');
  await expect(page.locator('.watch-main > :first-child')).toHaveClass(/player-wrap/);
  const media = page.locator('video');
  await expect.poll(() => media.evaluate((element: HTMLVideoElement) => element.readyState)).toBeGreaterThanOrEqual(2);
  // Slow an existing short fixture so resize assertions occur before natural completion.
  await media.evaluate((element: HTMLVideoElement) => { element.playbackRate = 0.5; element.currentTime = 2; });
  await expect.poll(() => media.evaluate((element: HTMLVideoElement) => element.seeking)).toBe(false);
  await expect.poll(() => media.evaluate((element: HTMLVideoElement) => element.readyState)).toBeGreaterThanOrEqual(2);
  const handle = await media.elementHandle();
  const source = await media.evaluate((element: HTMLVideoElement) => element.currentSrc);
  await media.evaluate((element: HTMLVideoElement) => { element.dataset.reloads = '0'; element.addEventListener('loadstart', () => { element.dataset.reloads = String(Number(element.dataset.reloads) + 1); }); });
  await page.getByRole('button', { name: '播放', exact: true }).click();
  await expect.poll(() => media.evaluate((element: HTMLVideoElement) => element.currentTime)).toBeGreaterThan(2);
  await page.getByRole('button', { name: '宽屏模式', exact: true }).click();
  for (const width of [920, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await noOverflow(page);
    expect(await handle!.evaluate((element) => element === document.querySelector('video'))).toBe(true);
    expect(await media.evaluate((element: HTMLVideoElement) => element.currentSrc)).toBe(source);
    expect(await media.evaluate((element: HTMLVideoElement) => element.currentTime)).toBeGreaterThanOrEqual(2);
    expect(await media.getAttribute('data-reloads')).toBe('0');
    expect(await media.evaluate((element: HTMLVideoElement) => element.paused)).toBe(false);
  }
  await page.getByRole('button', { name: '暂停', exact: true }).click();
  const pausedTime = await media.evaluate((element: HTMLVideoElement) => element.currentTime);
  await page.getByRole('button', { name: '宽屏模式', exact: true }).click();
  await expect(page.locator('.watch-layout')).not.toHaveClass(/theater-mode/);
  expect(await media.evaluate((element: HTMLVideoElement) => element.currentTime)).toBeCloseTo(pausedTime, 1);
  expect(await handle!.evaluate((element) => element === document.querySelector('video'))).toBe(true);
  expect(await media.getAttribute('data-reloads')).toBe('0');
});

test('playback quality, speed and subtitle menus support keyboard focus and selection', async ({ page }) => {
  await fixture(page, { video: { subtitle_tracks: [{ id: 7, video_id: 1, language: 'zh', label: '中文', url: '/phase3-test.vtt', is_default: true, created_at: '2026-01-01T00:00:00Z' }] } });
  await page.route('**/phase3-test.vtt', (route) => route.fulfill({ contentType: 'text/vtt', body: 'WEBVTT\n\n00:00:00.000 --> 00:00:09.000\n字幕验收\n' }));
  await page.goto('/video/1');
  const quality = page.getByRole('button', { name: /选择视频清晰度，当前/ });
  await expect(quality).toBeVisible();
  for (const [trigger, group] of [[quality, '选择视频清晰度'], [page.getByRole('button', { name: '播放速度', exact: true }), '选择播放速度'], [page.getByRole('button', { name: '选择字幕', exact: true }), '字幕轨道']] as const) {
    await trigger.click();
    const first = page.getByRole('group', { name: group, exact: true }).getByRole('button').first();
    await expect(first).toBeFocused();
    await first.press('Tab'); await page.keyboard.press('Shift+Tab');
    await expect(first).toBeFocused();
    await expect(first).toHaveCSS('outline-width', '2px');
    await expect(first).toHaveCSS('outline-color', 'rgb(255, 255, 255)');
    await first.press('Escape');
    await expect(trigger).toBeFocused();
    await expect(trigger).toHaveAttribute('aria-expanded', 'false');
  }
  await page.getByRole('button', { name: '播放速度', exact: true }).click();
  await page.getByRole('button', { name: '1.5x', exact: true }).press('Enter');
  expect(await page.locator('video').evaluate((element: HTMLVideoElement) => element.playbackRate)).toBe(1.5);
  await page.getByRole('button', { name: '选择字幕', exact: true }).click();
  await page.getByRole('group', { name: '字幕轨道', exact: true }).getByRole('button', { name: '关闭', exact: true }).press('Enter');
  await expect(page.getByRole('button', { name: '选择字幕', exact: true })).toHaveAttribute('aria-pressed', 'false');
});

test('playback related navigation replaces id and preserves real interaction contracts', async ({ page }) => {
  await fixture(page);
  await page.goto('/video/1');
  await page.getByRole('button', { name: '点赞', exact: true }).click();
  await expect(page.getByRole('button', { name: '点赞', exact: true })).toHaveAttribute('aria-pressed', 'true');
  await page.getByRole('button', { name: '收藏', exact: true }).click();
  await expect(page.getByRole('button', { name: '收藏', exact: true })).toHaveAttribute('aria-pressed', 'true');
  await page.getByRole('button', { name: '关注', exact: true }).click();
  await expect(page.getByRole('button', { name: '已关注', exact: true })).toBeVisible();
  await page.getByRole('textbox', { name: '评论内容' }).fill('隔离评论验收');
  await page.getByRole('button', { name: '发布', exact: true }).click();
  await expect(page.locator('.comment-item')).toContainText('隔离评论验收');
  page.once('dialog', (dialog) => dialog.accept());
  await page.getByRole('button', { name: '删除', exact: true }).click();
  await expect(page.locator('.comment-item')).toHaveCount(0);
  await page.getByRole('button', { name: '举报', exact: true }).click();
  await page.getByLabel('举报原因').selectOption('copyright');
  await page.getByLabel('补充说明').fill('隔离举报验收');
  await page.getByRole('button', { name: '提交举报', exact: true }).click();
  await expect(page.getByText('举报已提交', { exact: true })).toBeVisible();
  await expect(page.locator('.related-video .gv-media-fallback')).toBeVisible();
  await page.locator('.related-title').click();
  await expect(page).toHaveURL(/\/video\/2$/);
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('播放验收作品二');
  await expect(page.locator('.watch-main')).not.toContainText('播放验收作品一');
});

test('playback long titles, controls and channel dialog fit all eight viewports', async ({ page }) => {
  const title = '极长中文标题'.repeat(25) + 'ASCII_0123456789_'.repeat(20);
  await fixture(page, { self: true, video: { title }, profile: { username: '长用户名_ASCII_'.repeat(8), bio: '长简介\n'.repeat(35) } });
  await page.goto('/video/1');
  await expect(page.getByRole('heading', { level: 1 })).toHaveText(title);
  for (const [width, height] of viewports) {
    await page.setViewportSize({ width, height });
    await noOverflow(page);
    const bounds = await page.locator('.player-wrap').boundingBox();
    expect(bounds!.x).toBeGreaterThanOrEqual(0);
    expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(width + 1);
    expect(await page.locator('.watch-title-row h1').evaluate((element) => element.clientHeight <= Number.parseFloat(getComputedStyle(element).lineHeight) * 3 + 1)).toBe(true);
    const buttons = await page.locator('.player-control-row button').evaluateAll((elements) => elements.filter((element) => element.getBoundingClientRect().width > 0).map((element) => { const rect = element.getBoundingClientRect(); return { x: rect.x, y: rect.y, w: rect.width, h: rect.height }; }));
    for (let i = 0; i < buttons.length; i += 1) {
      expect(buttons[i].w).toBeGreaterThanOrEqual(44); expect(buttons[i].h).toBeGreaterThanOrEqual(44);
      expect(buttons[i].x).toBeGreaterThanOrEqual(0); expect(buttons[i].x + buttons[i].w).toBeLessThanOrEqual(width + 1);
      for (let j = i + 1; j < buttons.length; j += 1) expect(Math.min(buttons[i].x + buttons[i].w, buttons[j].x + buttons[j].w) - Math.max(buttons[i].x, buttons[j].x) > 1 && Math.min(buttons[i].y + buttons[i].h, buttons[j].y + buttons[j].h) - Math.max(buttons[i].y, buttons[j].y) > 1).toBe(false);
    }
  }
  await page.goto('/users/42');
  await expect(page.getByRole('button', { name: '编辑资料', exact: true })).toBeVisible();
  for (const [width, height] of viewports) {
    await page.setViewportSize({ width, height });
    await noOverflow(page);
    await page.getByRole('button', { name: '编辑资料', exact: true }).click();
    const dialog = page.getByRole('dialog', { name: '编辑作者信息' });
    await expect(dialog).toBeVisible();
    const bounds = await dialog.boundingBox();
    expect(bounds!.x).toBeGreaterThanOrEqual(0); expect(bounds!.y).toBeGreaterThanOrEqual(0);
    expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(width + 1); expect(bounds!.y + bounds!.height).toBeLessThanOrEqual(height + 1);
    await dialog.press('Escape');
    await expect(page.getByRole('button', { name: '编辑资料', exact: true })).toBeFocused();
  }
});

test('channel standard cards, pagination, zero stats and anonymous empty follow guard', async ({ page }) => {
  await fixture(page);
  await page.goto('/users/42');
  await expect(page.locator('.gv-video-card--standard')).toHaveCount(12);
  await page.getByRole('button', { name: '下一页', exact: true }).click();
  await expect(page).toHaveURL(/page=2/);
  await expect(page.locator('.gv-video-card--standard')).toHaveCount(1);
  await page.unroute('**/api/v1/**');
  await fixture(page, { anonymous: true, empty: true, profile: { bio: '', videos_count: 0, followers_count: 0, following_count: 0 } });
  await page.goto('/users/42');
  await expect(page.getByRole('heading', { name: '还没有公开投稿', exact: true })).toBeVisible();
  await expect(page.locator('.creator-stats dd')).toHaveText(['0', '0', '0']);
  await page.getByRole('button', { name: '关注', exact: true }).click();
  await expect(page).toHaveURL(/\/auth\?next=%2Fusers%2F42$/);
});

test('playback anonymous actions retain next redirect and empty related semantics', async ({ page }) => {
  await fixture(page, { anonymous: true, empty: true });
  for (const name of ['点赞', '收藏', '关注', '举报']) {
    await page.goto('/video/1');
    await expect(page.getByText('暂无更多视频', { exact: true })).toBeVisible();
    await expect(page.getByRole('textbox', { name: '评论内容' })).toBeDisabled();
    await page.getByRole('button', { name, exact: true }).click();
    await expect(page).toHaveURL(/\/auth\?next=%2Fvideo%2F1$/);
  }
});

test('playback processing polling updates to ready without replacing media', async ({ page }) => {
  const { first } = await fixture(page, { video: { processing_status: 'processing', processing_progress: 45, processing_stage: 'transcoding', hls_url: '' } });
  await page.route('**/api/v1/videos/1?count_view=false', (route) => route.fulfill({ json: { data: { ...first, processing_status: 'ready', processing_progress: 100, processing_stage: 'done' } } }));
  await page.goto('/video/1');
  await expect(page.locator('.processing-notice')).toBeVisible();
  const media = await page.locator('video').elementHandle();
  await expect(page.locator('.processing-notice')).toHaveCount(0);
  expect(await media!.evaluate((element) => element === document.querySelector('video'))).toBe(true);
});

test('playback fullscreen and picture-in-picture retain the same media element', async ({ page }) => {
  await fixture(page);
  await page.goto('/video/1');
  const media = page.locator('video');
  await expect.poll(() => media.evaluate((element: HTMLVideoElement) => element.readyState)).toBeGreaterThanOrEqual(2);
  const handle = await media.elementHandle();
  const fullscreen = page.getByRole('button', { name: '全屏', exact: true });
  const capabilities = await page.evaluate(() => ({ fullscreen: document.fullscreenEnabled, pictureInPicture: document.pictureInPictureEnabled }));
  console.info(`${test.info().project.name} native media capabilities: ${JSON.stringify(capabilities)}`);
  if (capabilities.fullscreen) {
    await fullscreen.click();
    await expect.poll(() => page.evaluate(() => document.fullscreenElement === document.querySelector('.player-wrap'))).toBe(true);
    const exitFullscreen = page.getByRole('button', { name: '退出全屏', exact: true });
    await exitFullscreen.press('Tab'); await page.keyboard.press('Shift+Tab');
    await expect(exitFullscreen).toBeFocused();
    await expect(exitFullscreen).toHaveCSS('outline-width', '2px');
    await expect(exitFullscreen).toHaveCSS('outline-color', 'rgb(255, 255, 255)');
    await exitFullscreen.click();
    await expect.poll(() => page.evaluate(() => document.fullscreenElement === null)).toBe(true);
  } else await expect(fullscreen).toBeDisabled();
  const pip = page.getByRole('button', { name: '画中画', exact: true });
  if (capabilities.pictureInPicture) {
    await pip.click();
    await expect.poll(() => page.evaluate(() => document.pictureInPictureElement === document.querySelector('video'))).toBe(true);
    await pip.click();
    await expect.poll(() => page.evaluate(() => document.pictureInPictureElement === null)).toBe(true);
  } else await expect(pip).toBeDisabled();
  expect(await handle!.evaluate((element) => element === document.querySelector('video'))).toBe(true);
});

test('Phase 6 direct playback restores and saves progress with real volume and mute controls', async ({ page }) => {
  await fixture(page, { video: { hls_url: '' } });
  await page.addInitScript(() => {
    if (!sessionStorage.getItem('phase6-resume-seeded')) {
      localStorage.setItem('gvideo-progress-1', '8');
      sessionStorage.setItem('phase6-resume-seeded', 'true');
    }
  });
  await page.goto('/video/1');
  const media = page.locator('video');
  await expect.poll(() => media.evaluate((element: HTMLVideoElement) => element.readyState)).toBeGreaterThanOrEqual(2);
  await expect.poll(() => media.evaluate((element: HTMLVideoElement) => element.currentTime)).toBeCloseTo(8, 0);
  await expect(page.locator('.resume-notice')).toContainText('继续播放');
  const player = page.getByRole('region', { name: /视频播放器$/ });
  await player.focus();
  await player.press('ArrowDown');
  expect(await media.evaluate((element: HTMLVideoElement) => element.volume)).toBeCloseTo(0.95, 2);
  await player.press('m');
  await expect(page.getByRole('button', { name: '取消静音', exact: true })).toBeVisible();
  expect(await media.evaluate((element: HTMLVideoElement) => element.muted)).toBe(true);
  await page.getByRole('button', { name: '取消静音', exact: true }).click();
  await media.evaluate((element: HTMLVideoElement) => { element.currentTime = 9; });
  await page.getByRole('button', { name: '播放', exact: true }).click();
  await expect.poll(() => media.evaluate((element: HTMLVideoElement) => element.currentTime)).toBeGreaterThan(9);
  await page.getByRole('button', { name: '暂停', exact: true }).click();
  await expect.poll(() => page.evaluate(() => Number(localStorage.getItem('gvideo-progress-1')))).toBeGreaterThanOrEqual(9);
  await page.reload();
  await expect.poll(() => media.evaluate((element: HTMLVideoElement) => element.readyState)).toBeGreaterThanOrEqual(2);
  await expect.poll(() => media.evaluate((element: HTMLVideoElement) => element.currentTime)).toBeGreaterThanOrEqual(9);
  await page.getByRole('button', { name: '从头播放', exact: true }).click();
  await expect.poll(() => media.evaluate((element: HTMLVideoElement) => element.currentTime)).toBe(0);
});

test('Phase 6 real HLS failure falls back to the existing MP4 and failed fallback offers retry', async ({ page }) => {
  await fixture(page, { video: { hls_url: '/phase6-invalid-stream.m3u8' } });
  await page.route('**/phase6-invalid-stream.m3u8', (route) => route.fulfill({ contentType: 'application/vnd.apple.mpegurl', body: 'invalid manifest' }));
  await page.goto('/video/1');
  await expect(page.locator('.stream-status')).toContainText('已切换原始视频', { timeout: 30_000 });
  const media = page.locator('video');
  await expect.poll(() => media.evaluate((element: HTMLVideoElement) => element.readyState)).toBeGreaterThanOrEqual(2);
  expect(await media.evaluate((element: HTMLVideoElement) => element.currentSrc)).toContain('.mp4');
  await media.evaluate((element) => element.dispatchEvent(new Event('error')));
  await expect(page.getByRole('button', { name: '重试播放', exact: true })).toBeVisible();
  await page.getByRole('button', { name: '重试播放', exact: true }).click();
  await expect(page.locator('.stream-status')).toContainText('已切换原始视频', { timeout: 30_000 });
  await expect.poll(() => media.evaluate((element: HTMLVideoElement) => element.readyState)).toBeGreaterThanOrEqual(2);
});

test('Phase 6 failed direct media has visible recovery instead of an empty player', async ({ page }) => {
  await fixture(page, { video: { hls_url: '', video_url: '/phase6-missing-direct.mp4' } });
  await page.route('**/phase6-missing-direct.mp4', (route) => route.fulfill({ status: 404, body: '' }));
  await page.goto('/video/1');
  await expect.poll(() => page.locator('video').evaluate((element: HTMLVideoElement) => element.error?.code)).toBeGreaterThan(0);
  await expect(page.locator('.stream-status')).toContainText('视频加载失败，请重试');
  await expect(page.getByRole('button', { name: '重试播放', exact: true })).toBeVisible();
});

test('Phase 6 HLS quality selects an actual level and sharing preserves copy/cancel semantics', async ({ page }) => {
  await fixture(page);
  await page.addInitScript(() => {
    Object.defineProperty(navigator, 'share', { configurable: true, value: undefined });
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: async (value: string) => { sessionStorage.setItem('phase6-shared-url', value); } } });
  });
  await page.goto('/video/1');
  const trigger = page.getByRole('button', { name: /选择视频清晰度，当前/ });
  await trigger.click();
  const group = page.getByRole('group', { name: '选择视频清晰度', exact: true });
  const resolution = group.getByRole('button', { name: /^\d+p$/ }).first();
  await expect(resolution).toBeVisible();
  const label = await resolution.textContent();
  await resolution.press('Enter');
  await expect(trigger).toHaveAttribute('aria-label', `选择视频清晰度，当前${label}`);
  await expect(trigger).toBeFocused();
  await page.getByRole('button', { name: '分享', exact: true }).click();
  await expect(page.locator('.share-feedback')).toHaveText('链接已复制');
  expect(await page.evaluate(() => sessionStorage.getItem('phase6-shared-url'))).toBe(page.url());
  await page.evaluate(() => Object.defineProperty(navigator, 'share', { configurable: true, value: () => Promise.reject(new DOMException('cancel', 'AbortError')) }));
  await page.getByRole('button', { name: '分享', exact: true }).click();
  await expect(page.locator('.inline-error')).toHaveCount(0);
});
