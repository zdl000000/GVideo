import { expect, test, type Locator, type Page } from "@playwright/test";
import { installFinalQaFixture } from "./final-qa.fixture";

// Category B read-only seed lookup; all browser API reads and writes are isolated.
// Delayed mutations exercise actual disabled controls without touching live data.
async function tabTo(page: Page, target: Locator) {
  for (let count = 0; count < 70; count++) {
    if (await target.evaluate((element) => element === document.activeElement)) return;
    await page.keyboard.press("Tab");
  }
  throw new Error("Keyboard traversal did not reach the expected control");
}

async function selectVideo(page: Page) {
  await page.locator('input[aria-label="视频文件"]').setInputFiles({ name: "isolated-busy.mp4", mimeType: "video/mp4", buffer: Buffer.from("isolated busy fixture") });
  await page.getByLabel(/^标题/).fill("隔离 Busy 验收作品");
  await page.getByLabel("分区", { exact: true }).selectOption("音乐");
}

async function failureGate(page: Page, path: string, message: string) {
  let release!: () => void;
  const held = new Promise<void>((done) => { release = done; });
  await page.route(`**${path}`, async (route) => {
    if (route.request().method() === "GET") { await route.fallback(); return; }
    await held;
    if (!route.request().failure()) await route.fulfill({ status: 500, json: { error: message } });
  });
  return release;
}

test("Publish Tab / Enter keeps focus on cancel while busy and returns to submit after cancel", async ({ page }) => {
  const fixture = await installFinalQaFixture(page, { role: "authenticated" });
  fixture.gateMutation("/api/v1/videos");
  try {
    await page.goto("/upload"); await selectVideo(page);
    const submit = page.getByRole("button", { name: "发布作品", exact: true });
    await tabTo(page, submit); await page.keyboard.press("Enter");
    await expect(page.getByRole("button", { name: "正在上传...", exact: true })).toBeDisabled();
    const cancel = page.getByRole("button", { name: "取消上传", exact: true });
    await expect(cancel).toBeFocused(); await page.keyboard.press("Enter");
    await expect(page.getByRole("alert")).toHaveText("上传已取消，文件和表单内容已保留");
    await expect(submit).toBeFocused(); await expect(submit).toBeEnabled();
    await expect(page.getByLabel(/^标题/)).toHaveValue("隔离 Busy 验收作品");
    expect(fixture.unknownRequests).toEqual([]);
  } finally { fixture.releaseMutation("/api/v1/videos"); }
});

test("Publish Enter from its title field retains the same busy / cancel keyboard context", async ({ page }) => {
  const fixture = await installFinalQaFixture(page, { role: "authenticated" });
  fixture.gateMutation("/api/v1/videos");
  try {
    await page.goto("/upload"); await selectVideo(page);
    await tabTo(page, page.getByLabel(/^标题/)); await page.keyboard.press("Enter");
    const cancel = page.getByRole("button", { name: "取消上传", exact: true });
    await expect(cancel).toBeFocused();
    await expect(page.getByLabel(/^标题/)).toBeDisabled();
    await page.keyboard.press("Enter");
    await expect(page.getByRole("alert")).toHaveText("上传已取消，文件和表单内容已保留");
    await expect(page.getByRole("button", { name: "发布作品", exact: true })).toBeFocused();
    expect(fixture.unknownRequests).toEqual([]);
  } finally { fixture.releaseMutation("/api/v1/videos"); }
});

for (const moved of [false, true]) {
  test(`Publish failure ${moved ? "preserves the user's new focus" : "restores submit focus"}`, async ({ page }) => {
    const fixture = await installFinalQaFixture(page, { role: "authenticated" });
    const release = await failureGate(page, "/api/v1/videos", "隔离上传失败");
    try {
      await page.goto("/upload"); await selectVideo(page);
      const submit = page.getByRole("button", { name: "发布作品", exact: true });
      await tabTo(page, submit); await page.keyboard.press("Enter");
      await expect(page.getByRole("button", { name: "取消上传", exact: true })).toBeFocused();
      const destination = moved ? page.getByRole("link", { name: "跳到主要内容", exact: true }) : submit;
      if (moved) await tabTo(page, destination);
      release(); await expect(page.getByRole("alert")).toHaveText("隔离上传失败");
      await expect(destination).toBeFocused(); await expect(submit).toBeEnabled();
      expect(fixture.unknownRequests).toEqual([]);
    } finally { release(); }
  });
}

test("Publish success navigates to its returned video after the user moves focus", async ({ page }) => {
  const fixture = await installFinalQaFixture(page, { role: "authenticated" });
  fixture.gateMutation("/api/v1/videos");
  try {
    await page.goto("/upload"); await selectVideo(page);
    await tabTo(page, page.getByRole("button", { name: "发布作品", exact: true })); await page.keyboard.press("Enter");
    await expect(page.getByRole("button", { name: "取消上传", exact: true })).toBeFocused();
    await tabTo(page, page.getByRole("link", { name: "跳到主要内容", exact: true }));
    fixture.releaseMutation("/api/v1/videos");
    await expect(page).toHaveURL(/\/video\/100$/);
    await expect(page.locator(".gv-watch-page")).toBeVisible({ timeout: 20_000 });
    await expect(page.locator(".gvideo-publish-page")).toHaveCount(0);
    expect(fixture.unknownRequests).toEqual([]);
  } finally { fixture.releaseMutation("/api/v1/videos"); }
});

for (const outcome of ["success", "error"] as const) for (const moved of [false, true]) {
  test(`Governance ${outcome} ${moved ? "preserves the user's new focus" : "retains the same row focus"}`, async ({ page }) => {
    const fixture = await installFinalQaFixture(page, { role: "admin" });
    const path = "/api/v1/admin/reports/1";
    let release: () => void;
    if (outcome === "error") release = await failureGate(page, path, "隔离审核失败");
    else { fixture.gateMutation(path); release = () => fixture.releaseMutation(path); }
    try {
      await page.goto("/admin/reports");
      const row = page.locator(".gv-report-row").first();
      const group = page.getByRole("group", { name: "处理举报 #1", exact: true });
      const complete = group.getByRole("button", { name: "处理完成", exact: true });
      await expect(complete).toBeEnabled({ timeout: 20_000 });
      await tabTo(page, complete); await page.keyboard.press("Enter");
      const title = row.locator("h2 a");
      await expect(title).toBeFocused(); await expect(complete).toBeDisabled();
      await expect(group.getByRole("status")).toHaveText("正在更新举报状态…");
      const destination = moved ? page.getByRole("group", { name: "举报状态筛选", exact: true }).getByRole("button", { name: "全部", exact: true }) : title;
      if (moved) await tabTo(page, destination);
      release(); await expect(group.getByRole("status")).toHaveCount(0);
      await expect(destination).toBeFocused();
      await expect(row).toHaveAttribute("data-status", outcome === "success" ? "resolved" : "pending");
      if (outcome === "error") await expect(page.getByRole("alert")).toHaveText("隔离审核失败");
      expect(fixture.unknownRequests).toEqual([]);
    } finally { release(); }
  });
}
