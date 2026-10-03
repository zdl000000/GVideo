import { expect, test } from "@playwright/test";

// Category C: creates a user, likes/favorites/follows, a report and a comment.
// Opt in only after verifying a disposable database/media environment.
test('Phase 6 disposable real API social interactions and own comment deletion', async ({ page, baseURL }) => {
  test.skip(process.env.E2E_DISPOSABLE !== 'true', 'Persistent journey requires explicit disposable environment opt-in.');
  expect(new URL(baseURL!).port, 'Phase 6 dedicated environment, never the development database').toBe('18088');
  const username = `rc_${Date.now()}_${Math.random().toString(36).slice(2, 6)}`;
  const videoID = process.env.E2E_MEDIA_VIDEO_ID || '1';
  const media = await page.request.get(`/api/v1/videos/${videoID}?count_view=false`);
  expect(media.ok()).toBe(true);
  const video = (await media.json()).data;
  await page.goto(`/auth?next=${encodeURIComponent(`/video/${videoID}`)}`);
  await page.locator('.gv-auth-mode').getByRole('button', { name: '注册', exact: true }).click();
  await page.getByLabel('用户名', { exact: true }).fill(username);
  await page.getByLabel('密码', { exact: true }).fill('phase6-test-password');
  await page.locator('.gv-auth-form').getByRole('button', { name: '创建账号', exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`/video/${videoID}$`));
  await page.getByRole('button', { name: '点赞', exact: true }).click();
  await expect(page.getByRole('button', { name: '点赞', exact: true })).toHaveAttribute('aria-pressed', 'true');
  await page.getByRole('button', { name: '收藏', exact: true }).click();
  await expect(page.getByRole('button', { name: '收藏', exact: true })).toHaveAttribute('aria-pressed', 'true');
  await page.getByRole('button', { name: '关注', exact: true }).click();
  await expect(page.getByRole('button', { name: '已关注', exact: true })).toBeVisible();
  const marker = `RC真实评论_${username}`;
  await page.getByRole('textbox', { name: '评论内容' }).fill(marker);
  await page.locator('.comment-form').getByRole('button', { name: '发布', exact: true }).click();
  const comment = page.locator('.comment-item').filter({ hasText: marker });
  await expect(comment).toBeVisible();
  page.once('dialog', (dialog) => dialog.accept());
  await comment.getByRole('button', { name: '删除', exact: true }).click();
  await expect(comment).toHaveCount(0);
  await page.getByRole('button', { name: '举报', exact: true }).click();
  await page.getByLabel('举报原因').selectOption('other');
  await page.getByLabel('补充说明').fill(`独立环境RC举报_${username}`);
  await page.getByRole('button', { name: '提交举报', exact: true }).click();
  await expect(page.getByText('举报已提交', { exact: true })).toBeVisible();
  await page.goto('/favorites');
  await expect(page.locator('[data-video-id]').filter({ has: page.getByRole('link', { name: video.title, exact: true }) }).first()).toBeVisible();
  await page.goto('/following');
  await expect(page.locator('[data-video-id]').filter({ has: page.getByRole('link', { name: video.title, exact: true }) }).first()).toBeVisible();
  await page.goto(`/users/${video.user_id}`);
  await expect(page.getByRole('button', { name: '已关注', exact: true })).toBeVisible();
  await expect(page.locator('.creator-identity h1')).toHaveText(video.username);
});
