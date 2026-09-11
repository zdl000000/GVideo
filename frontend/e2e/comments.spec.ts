import { expect, test, type Page } from "@playwright/test";

// 旅程一：登录用户在视频页发表评论；空评论被表单拒绝。
// 每个用例使用全局唯一的注册用户，避免并行 worker 之间的注册冲突。

test.setTimeout(90_000);

const PASSWORD = "password123";

function uniqueSuffix() {
  return Math.random().toString(36).slice(2, 6).padEnd(4, "0");
}

// 用户名规则：3-24 位中文、字母、数字或下划线（e2e_c_ 前缀 + 时间戳 + 4 位随机后缀，恰好 24 位）
function uniqueUsername(prefix: string) {
  return `${prefix}${Date.now()}_${uniqueSuffix()}`;
}

async function register(page: Page, username: string) {
  await page.goto("/auth");
  await expect(page.locator(".auth-form-wrap form")).toBeVisible({ timeout: 20_000 });
  await page.locator(".segmented-control").getByRole("button", { name: "注册" }).click();
  await page.getByLabel("用户名").fill(username);
  await page.getByLabel("密码").fill(PASSWORD);
  await page.locator(".stack-form").getByRole("button", { name: "创建账号" }).click();
  await page.waitForURL((url) => !url.pathname.startsWith("/auth"), { timeout: 20_000 });
  // 登录态成立：顶栏出现退出登录按钮（桌面顶栏或移动侧栏，任一即可）
  await expect(page.locator('button[title="退出登录"]').first()).toBeAttached({ timeout: 15_000 });
  await expect.poll(() => page.locator(".user-chip").first().textContent(), { timeout: 15_000 }).toContain(username);
}

test("registered user publishes a comment and empty comments are rejected", async ({ page }) => {
  const username = uniqueUsername("e2e_c_");
  const marker = `E2E评论_${Date.now()}_${uniqueSuffix()}`;
  await register(page, username);

  // 从首页打开第一个视频卡片。前置条件：本用例依赖环境中已有至少一个
  // 公开视频（开发/验收库由验收脚本播种；纯净环境请先上传或改为自建视频）。
  await page.goto("/");
  const firstCard = page.locator(".cover-link").first();
  await expect(firstCard).toBeVisible({ timeout: 20_000 });
  await firstCard.click();
  await expect(page).toHaveURL(/\/video\/\d+$/);
  await expect(page.locator(".comment-section")).toBeVisible({ timeout: 20_000 });

  const commentInput = page.getByPlaceholder("说说你的想法");
  await expect(commentInput).toBeEnabled();
  const submitButton = page.locator(".comment-form").getByRole("button", { name: "发布" });

  // 空评论（未填写或纯空白）提交被拒绝：发布按钮不可用
  await expect(submitButton).toBeDisabled();
  await commentInput.fill("   ");
  await expect(submitButton).toBeDisabled();

  // 发表评论
  await commentInput.fill(marker);
  await expect(submitButton).toBeEnabled();
  await submitButton.click();

  const newComment = page.locator(".comment-item").filter({ hasText: marker });
  await expect(newComment.first()).toBeVisible({ timeout: 15_000 });
  await expect(newComment.first()).toContainText(username);

  // 提交成功后输入框被清空，再次回到"空评论不可提交"的状态
  await expect(commentInput).toHaveValue("");
  await expect(submitButton).toBeDisabled();
});
