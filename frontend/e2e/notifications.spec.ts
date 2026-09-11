import { expect, test, type Page } from "@playwright/test";

// 旅程二：跨用户通知。B 上传视频 -> A 评论 -> B 在通知中心收到通知并全部已读。
// 上传为异步转码，无需等待处理完成；断言均为"至少一个匹配"，容忍列表中的其他通知。

test.setTimeout(120_000);

const PASSWORD = "password123";

// 与后端上传测试夹具一致的最小 MP4 签名（标准库 MIME 嗅探可识别）
const MINIMAL_MP4 = Buffer.from("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom<\x06t\xbfmdat", "latin1");

function uniqueSuffix() {
  return Math.random().toString(36).slice(2, 6).padEnd(4, "0");
}

// 用户名规则：3-24 位中文、字母、数字或下划线（e2e_n_/e2e_a_ 前缀 + 时间戳 + 4 位随机后缀，恰好 24 位）
function uniqueUsername(prefix: string) {
  return `${prefix}${Date.now()}_${uniqueSuffix()}`;
}

async function expectLoggedIn(page: Page, username: string) {
  await expect(page.locator('button[title="退出登录"]').first()).toBeAttached({ timeout: 15_000 });
  await expect.poll(() => page.locator(".user-chip").first().textContent(), { timeout: 15_000 }).toContain(username);
}

async function fillCredentialsAndSubmit(page: Page, username: string, submitLabel: string) {
  await page.getByLabel("用户名").fill(username);
  await page.getByLabel("密码").fill(PASSWORD);
  await page.locator(".stack-form").getByRole("button", { name: submitLabel }).click();
  await page.waitForURL((url) => !url.pathname.startsWith("/auth"), { timeout: 30_000 });
  await expectLoggedIn(page, username);
}

async function register(page: Page, username: string) {
  await page.goto("/auth");
  await expect(page.locator(".auth-form-wrap form")).toBeVisible({ timeout: 20_000 });
  await page.locator(".segmented-control").getByRole("button", { name: "注册" }).click();
  await fillCredentialsAndSubmit(page, username, "创建账号");
}

async function login(page: Page, username: string) {
  await page.goto("/auth");
  await expect(page.locator(".auth-form-wrap form")).toBeVisible({ timeout: 20_000 });
  await fillCredentialsAndSubmit(page, username, "登录");
}

async function logout(page: Page) {
  const desktopLogout = page.locator('.desktop-actions button[title="退出登录"]');
  if (await desktopLogout.isVisible().catch(() => false)) {
    await desktopLogout.click();
  } else {
    // 移动端：退出入口在侧边栏账户区
    await page.locator(".mobile-menu-button").click();
    await page.locator(".mobile-account").getByRole("button", { name: "退出登录" }).click();
  }
  await page.waitForURL((url) => url.pathname === "/", { timeout: 20_000 });
  await expect(page.locator('button[title="退出登录"]')).toHaveCount(0, { timeout: 15_000 });
}

async function openNotifications(page: Page) {
  const bell = page.locator(".notification-button");
  if (await bell.isVisible().catch(() => false)) {
    await bell.click();
  } else {
    await page.locator(".mobile-menu-button").click();
    await page.locator(".sidebar").getByRole("link", { name: "通知中心" }).click();
  }
  await expect(page).toHaveURL(/\/notifications$/, { timeout: 20_000 });
  await expect(page.locator(".notifications-page")).toBeVisible({ timeout: 20_000 });
}

test("uploader receives the commenter's notification and can mark all read", async ({ page }) => {
  const stamp = Date.now();
  const userB = uniqueUsername("e2e_n_");
  const userA = uniqueUsername("e2e_a_");
  const videoTitle = `E2E通知旅程视频_${stamp}_${uniqueSuffix()}`;
  const commentMarker = `E2E跨用户评论_${stamp}_${uniqueSuffix()}`;

  // B 注册后进入投稿上传页（分类取首个可用分区；上传为异步转码，跳转即视为提交成功）
  await register(page, userB);
  await page.goto("/upload");
  const titleInput = page.getByLabel("标题");
  await expect(titleInput).toBeVisible({ timeout: 20_000 });
  await titleInput.fill(videoTitle);
  const categorySelect = page.getByLabel("分区");
  await expect(categorySelect).not.toHaveValue("", { timeout: 20_000 });
  await categorySelect.selectOption({ index: 0 });
  await page.locator('input[type="file"][accept^="video/"]').setInputFiles({
    name: "e2e-minimal.mp4",
    mimeType: "video/mp4",
    buffer: MINIMAL_MP4
  });
  await page.getByRole("button", { name: "发布作品" }).click();
  await page.waitForURL(/\/video\/\d+$/, { timeout: 90_000 });
  const videoID = page.url().match(/\/video\/(\d+)$/)?.[1];
  expect(videoID, "上传成功后应跳转到视频页").toBeTruthy();
  await logout(page);

  // A 注册，从「最新」页找到 B 的视频并发表带唯一标记的评论
  await register(page, userA);
  await page.goto("/latest");
  const titleLink = page.getByRole("link", { name: videoTitle }).first();
  await expect(titleLink).toBeVisible({ timeout: 30_000 });
  await titleLink.click();
  await expect(page).toHaveURL(new RegExp(`/video/${videoID}$`), { timeout: 20_000 });
  await expect(page.locator(".comment-section")).toBeVisible({ timeout: 20_000 });
  await page.getByPlaceholder("说说你的想法").fill(commentMarker);
  await page.locator(".comment-form").getByRole("button", { name: "发布" }).click();
  await expect(page.locator(".comment-item").filter({ hasText: commentMarker }).first()).toBeVisible({ timeout: 15_000 });
  await logout(page);

  // B 登录并打开通知中心，至少能看到一条 A 的评论通知
  await login(page, userB);
  await openNotifications(page);
  const commentNotice = page.locator(".notification-row").filter({ hasText: commentMarker });
  await expect(commentNotice.first()).toBeVisible({ timeout: 30_000 });
  await expect(commentNotice.first()).toContainText(userA);
  await expect(commentNotice.first()).toContainText(videoTitle);
  await expect(commentNotice.first()).toHaveClass(/unread/);

  // 全部已读：未读态清除
  const markAllButton = page.getByRole("button", { name: "全部标记已读" });
  await expect(markAllButton).toBeEnabled({ timeout: 15_000 });
  await markAllButton.click();
  await expect(page.getByText("所有通知都已读")).toBeVisible({ timeout: 15_000 });
  await expect(page.locator(".notification-row.unread")).toHaveCount(0, { timeout: 15_000 });
  await expect(commentNotice.first()).not.toHaveClass(/unread/);
  await expect(markAllButton).toBeDisabled();

  // 顶栏铃铛未读徽章清除（aria-label 恢复为 0 条未读，且不再渲染数字角标）
  await expect(page.locator(".notification-button")).toHaveAttribute("aria-label", "通知中心，0 条未读", { timeout: 15_000 });
  await expect(page.locator(".notification-button span")).toHaveCount(0, { timeout: 15_000 });
});
