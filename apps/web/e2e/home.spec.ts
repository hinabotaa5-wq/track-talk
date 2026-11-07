import { devices, expect, test } from "@playwright/test";

test("top page shows branding", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("link", { name: "TrackTalk" })).toBeVisible();
  // 下のメニューから主な画面へ行ける。
  const nav = page.getByRole("navigation", { name: "メイン" });
  for (const name of ["ホーム", "大会", "メモ", "ケガ"]) {
    await expect(nav.getByRole("link", { name })).toBeVisible();
  }
});

test("legal pages are reachable", async ({ page }) => {
  await page.goto("/terms");
  await expect(page.getByRole("heading", { name: "利用規約" })).toBeVisible();
  await page.goto("/privacy");
  await expect(page.getByRole("heading", { name: "プライバシーポリシー" })).toBeVisible();
});

test.describe("iPhone 相当のホーム", () => {
  test.use({
    viewport: { width: 390, height: 664 },
    userAgent: devices["iPhone 13"].userAgent,
    isMobile: true,
    hasTouch: true,
  });

  test("追加案内があっても投稿ボタンを押せて、一覧の末尾まで読める", async ({ page }) => {
    await page.clock.setFixedTime(Date.now() + 120_000);
    await page.route("**/api/v1/posts?*", (route) => route.fulfill({
      json: {
        posts: Array.from({ length: 12 }, (_, index) => ({
          id: `fab-sample-${index}`,
          kind: "video",
          title: `練習サンプル ${index + 1}`,
          excerpt: "テスト投稿",
          thumbnail_url: `/samples/${index % 2 ? "vertical" : "horizontal"}.jpg`,
          like_count: 0,
          comment_count: 0,
          created_at: "2026-10-24T00:00:00Z",
          updated_at: "2026-10-24T00:00:00Z",
        })),
        meta: { page: 1, per_page: 12, total: 12, has_more: false },
      },
    }));
    await page.route("**/api/v1/meet-threads/today*", (route) => route.fulfill({ json: { meets: [] } }));
    await page.goto("/?sort=new");

    const fab = page.getByRole("link", { name: "投稿する" });
    const banner = page.locator(".install-guide-banner");
    const nav = page.getByRole("navigation", { name: "メイン" });
    await expect(banner).toBeVisible();
    const feed = page.getByRole("region", { name: "投稿一覧", exact: true });
    await expect(feed.locator(".masonry-feed-canvas")).toHaveAttribute("data-ready", "true");
    await expect.poll(async () => {
      const [button, guide, menu] = await Promise.all([fab.boundingBox(), banner.boundingBox(), nav.boundingBox()]);
      if (!button || !guide || !menu) return 0;
      return Math.min(guide.y, menu.y) - button.y - button.height;
    }).toBeGreaterThanOrEqual(16);
    expect(await fab.boundingBox()).toMatchObject({ x: 16, width: 56, height: 56 });
    await fab.tap({ trial: true });
    expect(await fab.evaluate((button) => {
      const rect = button.getBoundingClientRect();
      return Boolean(document.elementFromPoint(rect.right + 4, rect.top + rect.height / 2)?.closest('a[href^="/posts/"]'));
    })).toBe(true);

    await feed.evaluate((element) => { element.scrollLeft = element.scrollWidth; });
    const lastPost = feed.locator('a[href="/posts/fab-sample-11"]');
    await expect(lastPost).toBeVisible();
    await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
    await expect.poll(async () => {
      const [post, button] = await Promise.all([lastPost.boundingBox(), fab.boundingBox()]);
      return post && button ? button.y - post.y - post.height : -1;
    }).toBeGreaterThanOrEqual(0);
    await lastPost.tap({ trial: true });

    await page.getByRole("button", { name: "追加方法" }).tap();
    await expect(page.getByRole("dialog", { name: "ホーム画面に追加" })).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(banner).toBeHidden();
    await expect.poll(async () => {
      const [button, menu] = await Promise.all([fab.boundingBox(), nav.boundingBox()]);
      return button && menu ? menu.y - button.y - button.height : 0;
    }).toBeGreaterThanOrEqual(16);
    await fab.tap();
    await expect(page).toHaveURL(/\/login\?next=(?:\/|%2F)upload/, { timeout: 20_000 });
  });
});
