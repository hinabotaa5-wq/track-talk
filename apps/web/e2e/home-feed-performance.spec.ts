import { expect, test } from "@playwright/test";

test("720件を読んでも描画を絞り、過去ページの再取得と戻る位置を保つ", async ({ page }) => {
  test.setTimeout(120_000);
  await page.setViewportSize({ width: 390, height: 844 });
  // サーバーの初期一覧を古いものとして扱い、固定したブラウザ用データを取得する。
  await page.clock.setFixedTime(Date.now() + 120_000);
  await page.addInitScript(() => {
    const today = new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Tokyo", year: "numeric", month: "2-digit", day: "2-digit" }).format(new Date());
    sessionStorage.setItem("today-meet-balloon:closed", today);
  });
  const requests: number[] = [];
  const post = (i: number) => ({
    id: `perf-${i}`, title: `投稿${i}`, excerpt: `走りの確認${i}`, kind: i % 3 ? "video" : "text",
    thumbnail_url: i % 3 ? `/samples/${i % 2 ? "vertical" : "horizontal"}.jpg` : undefined,
    youtube_shorts: Boolean(i % 2), mine: false, liked_by_me: false, saved_by_me: false,
    like_count: 0, comment_count: 0, created_at: "2026-10-24T00:00:00Z", updated_at: "2026-10-24T00:00:00Z",
  });
  await page.route("**/api/v1/**", async (route) => {
    const url = new URL(route.request().url());
    let json: unknown = {};
    if (url.pathname.endsWith("/posts")) {
      const number = Number(url.searchParams.get("cursor")?.replace("page-", "") ?? "1");
      requests.push(number);
      json = { posts: Array.from({ length: 12 }, (_, j) => post((number - 1) * 12 + j)), meta: {
        page: number, per_page: 12, total: 0, has_more: number < 60, next_cursor: number < 60 ? `page-${number+1}` : "",
      } };
    } else if (url.pathname.endsWith("/comments")) json = { comments: [], meta: { total: 0 } };
    else if (url.pathname.includes("/posts/perf-")) json = { ...post(Number(url.pathname.split("perf-")[1])), body: "投稿の本文" };
    else if (url.pathname.endsWith("/meet-threads/today")) json = { meets: [] };
    else if (url.pathname.endsWith("/domestic-majors")) json = { majors: [] };
    else if (url.pathname.endsWith("/major-hosts")) json = { hosts: [] };
    else if (url.pathname.endsWith("/competitions")) json = { competitions: [] };
    await route.fulfill({ json });
  });
  await page.goto("/?sort=new");
  const track = page.getByRole("region", { name: "投稿一覧", exact: true });
  const canvas = track.locator(".masonry-feed-canvas");
  await expect(canvas).toHaveAttribute("data-ready", "true");
  for (let number = 2; number <= 60; number++) {
    await track.evaluate((el) => { el.scrollLeft = el.scrollWidth; el.dispatchEvent(new Event("scroll")); });
    await expect.poll(async () => Number(await canvas.getAttribute("data-count"))).toBeGreaterThanOrEqual(number * 12);
    expect(await track.locator(".masonry-feed-tile").count()).toBeLessThan(50);
  }
  const firstPageReads = requests.filter((n) => n === 1).length;
  await track.evaluate((el) => { el.scrollLeft = 0; el.dispatchEvent(new Event("scroll")); });
  await expect(track.locator('a[href="/posts/perf-0"]')).toBeVisible();
  expect(requests.filter((n) => n === 1).length).toBeGreaterThan(firstPageReads);
  await track.evaluate((el) => { el.scrollLeft = el.scrollWidth / 2; el.dispatchEvent(new Event("scroll")); });
  const findTarget = () => track.evaluate((el) => {
    const bounds = el.getBoundingClientRect();
    return [...el.querySelectorAll("a")].find((a) => { const r = a.getBoundingClientRect(); return r.left >= bounds.left && r.right <= bounds.right; })?.getAttribute("href");
  });
  await expect.poll(findTarget).toBeTruthy();
  const target = await findTarget();
  const link = track.locator(`a[href="${target}"]`);
  await expect(link).toBeVisible();
  const before = await track.evaluate((el) => el.scrollLeft);
  const href = await link.getAttribute("href");
  // 再取得と画像の測定中でも、フォーカス保持したカードから確実に詳細へ進む。
  await link.focus();
  await page.keyboard.press("Enter");
  // CIの開発サーバーで初めて詳細ルートをコンパイルする時間も待つ。
  await expect(page).toHaveURL(new RegExp(`${href}$`), { timeout: 20_000 });
  await page.goBack();
  await expect(canvas).toHaveAttribute("data-count", "720");
  await expect.poll(() => track.evaluate((el) => el.scrollLeft)).toBeCloseTo(before, 0);
  expect(await track.locator(".masonry-feed-tile").count()).toBeLessThan(50);
});
