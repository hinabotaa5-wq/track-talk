import { devices, expect, test } from "@playwright/test";



test("フィードの見出しとモバイルナビを表示する", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 664 });
  await page.goto("/?sort=new");
  await expect(page.getByRole("link", { name: "TrackTalk", exact: true })).toBeVisible();
  // 本体 #629：ご意見の左の Instagram（別タブの外部リンク）。
  await expect(page.getByRole("link", { name: "Instagram（@tracktalk.office）" })).toBeVisible();
  // 下のメニューから主な画面へ行ける。
  const nav = page.getByRole("navigation", { name: "メイン" });
  await expect(nav.getByRole("link")).toHaveText(["コーチ", "大会", "フィード", "メモ", "ケガ"]);
  await expect(nav.getByRole("link", { name: "フィード", exact: true })).toHaveAttribute("href", "/");
  await expect(page.getByRole("heading", { name: "フィード", exact: true })).toBeVisible();
});

test.describe("英語のデスクトップナビ", () => {
  test.use({ locale: "en-US", viewport: { width: 1280, height: 720 } });

  test("Feed を3番目に表示する", async ({ page }) => {
    await page.goto("/?sort=new");
    const nav = page.locator("header nav[data-main-nav]");
    await expect(nav).toBeVisible();
    await expect(nav.getByRole("link")).toHaveText(["Coach", "Meets", "Feed", "Notes", "Injury"]);
    await expect(nav.getByRole("link", { name: "Feed", exact: true })).toHaveAttribute("href", "/");
    await expect(page.getByRole("heading", { name: "Feed", exact: true })).toBeVisible();
  });
});

test("legal pages are reachable", async ({ page }) => {
  await page.goto("/terms");
  await expect(page.getByRole("heading", { name: "利用規約" })).toBeVisible();
  await page.goto("/privacy");
  await expect(page.getByRole("heading", { name: "プライバシーポリシー" })).toBeVisible();
});

test.describe("iPhone 相当のフィード", () => {
  test.use({
    viewport: { width: 390, height: 664 },
    userAgent: devices["iPhone 13"].userAgent,
    isMobile: true,
    hasTouch: true,
  });

  test("投稿の左端に少し余白を取り、右端から横に送れる", async ({ page }) => {
    await page.clock.setFixedTime(Date.now() + 120_000);
    await page.route("**/api/v1/posts?*", (route) => route.fulfill({
      json: {
        posts: Array.from({ length: 12 }, (_, index) => ({
          id: `home-sample-${index}`,
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

    const tabs = page.getByRole("navigation", { name: "投稿の並び替え", exact: true });
    const post = tabs.getByRole("link", { name: "投稿する", exact: true });
    const banner = page.locator(".install-guide-banner");
    const nav = page.getByRole("navigation", { name: "メイン" });
    await expect(banner).toBeVisible();
    const feed = page.getByRole("region", { name: "投稿一覧", exact: true });
    await expect(feed.locator(".home-feed-grid-canvas")).toHaveAttribute("data-ready", "true");
    await post.tap({ trial: true });
    const navBounds = (await nav.boundingBox())!;
    await expect.poll(() => feed.evaluate((element, navTop) => {
      const viewportWidth = document.documentElement.clientWidth;
      const track = element.getBoundingClientRect();
      const cards = [...element.querySelectorAll('a[href^="/posts/"]')].map((card) => card.getBoundingClientRect());
      return {
        leftInset: Math.min(...cards.map((card) => card.left)),
        reachesRightEdge: track.right === viewportWidth,
        documentFits: document.documentElement.scrollWidth === viewportWidth,
        fillsHeight: Math.abs(navTop - Math.max(...cards.map((card) => card.bottom))) <= 8,
        hasMoreToRight: element.scrollWidth > element.clientWidth,
      };
    }, navBounds.y)).toEqual({ leftInset: 8, reachesRightEdge: true, documentFits: true, fillsHeight: true, hasMoreToRight: true });

    await page.getByRole("button", { name: "追加方法" }).tap();
    await expect(page.getByRole("dialog", { name: "ホーム画面に追加" })).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(banner).toBeHidden();
    await feed.evaluate((element) => { element.scrollLeft = element.scrollWidth; });
    const lastPost = feed.locator('a[href="/posts/home-sample-11"]');
    await expect(lastPost).toBeInViewport({ ratio: 0.98 });
    await lastPost.tap({ trial: true });

    await post.tap();
    await expect(page).toHaveURL(/\/login\?next=(?:\/|%2F)upload/, { timeout: 20_000 });
  });
});
