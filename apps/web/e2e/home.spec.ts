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

  test("投稿を画面の端まで表示し、右端から横に送れる", async ({ page }) => {
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
    await expect(feed.locator(".home-feed-grid-canvas")).toHaveAttribute("data-ready", "true");
    await expect.poll(async () => {
      const [button, guide, menu] = await Promise.all([fab.boundingBox(), banner.boundingBox(), nav.boundingBox()]);
      if (!button || !guide || !menu) return 0;
      return Math.min(guide.y, menu.y) - button.y - button.height;
    }).toBeGreaterThanOrEqual(16);
    expect(await fab.boundingBox()).toMatchObject({ x: 16, width: 56, height: 56 });
    await fab.tap({ trial: true });
    const navBounds = (await nav.boundingBox())!;
    await expect.poll(() => feed.evaluate((element, navTop) => {
      const viewportWidth = document.documentElement.clientWidth;
      const track = element.getBoundingClientRect();
      const cards = [...element.querySelectorAll('a[href^="/posts/"]')].map((card) => card.getBoundingClientRect());
      const button = document.querySelector(".home-post-fab")!.getBoundingClientRect();
      return {
        fillsWidth: track.left === 0 && track.right === viewportWidth,
        documentFits: document.documentElement.scrollWidth === viewportWidth,
        fillsHeight: Math.abs(navTop - Math.max(...cards.map((card) => card.bottom))) <= 8,
        clippedOnRight: cards.some((card) => card.left < track.right && card.right > track.right),
        fabOverPost: cards.some((card) => card.left < button.right && card.right > button.left
          && card.top < button.bottom && card.bottom > button.top),
      };
    }, navBounds.y)).toEqual({ fillsWidth: true, documentFits: true, fillsHeight: true, clippedOnRight: true, fabOverPost: true });

    await page.getByRole("button", { name: "追加方法" }).tap();
    await expect(page.getByRole("dialog", { name: "ホーム画面に追加" })).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(banner).toBeHidden();
    await expect.poll(async () => {
      const [button, menu] = await Promise.all([fab.boundingBox(), nav.boundingBox()]);
      return button && menu ? menu.y - button.y - button.height : 0;
    }).toBeGreaterThanOrEqual(16);
    await feed.evaluate((element) => { element.scrollLeft = element.scrollWidth; });
    const lastPost = feed.locator('a[href="/posts/fab-sample-11"]');
    await expect(lastPost).toBeInViewport({ ratio: 0.98 });
    await lastPost.tap({ trial: true });

    await fab.tap();
    await expect(page).toHaveURL(/\/login\?next=(?:\/|%2F)upload/, { timeout: 20_000 });
  });
});
