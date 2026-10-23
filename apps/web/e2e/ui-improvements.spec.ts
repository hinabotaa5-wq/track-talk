import { expect, test } from "@playwright/test";

const official = "https://example.com/meet/timetable";
const meet = {
  meet_key: "m:nihon-championships:2026", name: "テスト陸上大会", date_label: "10月23日",
  post_id: "test-meet", comment_count: 3, href: "/posts/test-meet", now_running: ["男子100m決勝"],
  major_slug: "nihon-championships", major_year: 2026, competition_id: "",
};

test.beforeEach(async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.addInitScript(() => {
    // 自動で出る大会の案内を閉じ、各アイコンの操作を確かめる。
    const today = new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Tokyo", year: "numeric", month: "2-digit", day: "2-digit" }).format(new Date());
    sessionStorage.setItem("today-meet-balloon:closed", today);
  });
  await page.route("**/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    let body: unknown = {};
    if (path.endsWith("/meet-threads/today")) body = { meets: [meet] };
    else if (path.endsWith("/domestic-majors")) body = { majors: [{
      slug: "nihon-championships", year: 2026, name: "テスト陸上大会", category: "open",
      starts_on: "2026-10-23", ends_on: "2026-10-23", schedule_label: "10月23日 9:00", venue: "テスト競技場", site_url: official,
    }] };
    else if (path.endsWith("/major-hosts")) body = { hosts: [] };
    else if (path.endsWith("/competitions")) body = { competitions: [] };
    else if (path.endsWith("/posts")) body = { posts: [], meta: { page: 1, per_page: 20, total: 0, total_pages: 0 } };
    await route.fulfill({ json: body });
  });
});

test("ホームの大会をヘッダーに集約し、マイページを再タップで閉じる", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("button", { name: "今日開催中の大会", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "今日開催中", exact: true })).toHaveCount(0);
  const account = page.getByRole("button", { name: "マイページ", exact: true });
  await account.click();
  await expect(page.getByRole("dialog", { name: "マイページ", exact: true })).toBeVisible();
  // 前面の閉じる領域も同じタップ位置で閉じられる。
  const box = await account.boundingBox();
  await page.mouse.click(box!.x + box!.width / 2, box!.y + box!.height / 2);
  await expect(account).toHaveAttribute("aria-expanded", "false");
  await expect(page).toHaveURL("/");
});

test("開催中でもタイムテーブルは外部、実況スレッドは内部へ移動する", async ({ page }) => {
  await page.goto("/competitions");
  const timetable = page.getByRole("link", { name: "日程・タイムテーブル", exact: true });
  await expect(timetable).toHaveAttribute("href", official);
  await expect(timetable).toHaveAttribute("target", "_blank");
  await expect(page.getByRole("link", { name: /実況スレッドへ/ })).toHaveAttribute("href", meet.href);
  await expect(page.locator("a a")).toHaveCount(0);
});

test("大会のサブ画面にはヘッダーの戻るだけを残す", async ({ page }) => {
  for (const path of ["/competitions/regional", "/competitions/major-hosts"]) {
    await page.goto(path);
    await expect(page.getByRole("button", { name: "戻る", exact: true })).toHaveCount(1);
    await expect(page.locator("main").getByRole("button", { name: /戻る/ })).toHaveCount(0);
  }
});

test("検索アイコンと4つのタブを1行に並べ、検索中はタブを隠す", async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 740 });
  await page.goto("/");
  const nav = page.getByRole("navigation", { name: "投稿の並び替えと検索" });
  const search = nav.getByRole("button", { name: "キーワード検索を開く" });
  await expect(nav.getByRole("link")).toHaveText(["新しい", "人気", "募集", "レビュー"]);
  // フォントやヘッダーの読み込みで測定の途中に位置が変わらないよう、同じフレームで比較する。
  const boxes = await nav.locator("button, a").evaluateAll((items) => items.map((item) => {
    const rect = item.getBoundingClientRect();
    return { x: rect.x, y: rect.y, width: rect.width, height: rect.height };
  }));
  const searchBox = boxes[0];
  for (const box of boxes.slice(1)) {
    expect(Math.abs((box.y + box.height / 2) - (searchBox.y + searchBox.height / 2))).toBeLessThan(2);
    expect(box.x + box.width).toBeLessThanOrEqual(320);
  }
  await nav.getByRole("link", { name: "人気", exact: true }).click();
  await expect(page).toHaveURL(/sort=popular/);
  await search.click();
  await expect(nav).toHaveCount(0);
  const keyword = page.getByRole("searchbox", { name: "キーワード" });
  await expect(keyword).toBeFocused();
  await keyword.fill("朝練");
  await keyword.press("Enter");
  await expect(page).toHaveURL(/sort=popular/);
  await expect(page).toHaveURL(/q=/);
  await expect(keyword).toHaveValue("朝練");
  await page.getByRole("button", { name: "検索を閉じる" }).click();
  await expect(nav).toBeVisible();
  await expect(page).not.toHaveURL(/q=/);
  await page.goBack();
  await expect(keyword).toBeVisible();
  await expect(keyword).toHaveValue("朝練");
  await keyword.press("Escape");
  await expect(nav).toBeVisible();
  await expect(search).toBeFocused();
});

test("リールの詳細は中央に小さく表示する", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.route("**/api/v1/posts/reel-preview**", async (route) => {
    const comments = new URL(route.request().url()).pathname.endsWith("/comments");
    await route.fulfill({ json: comments ? { comments: [], meta: { page: 1, total_pages: 0 } } : {
      id: "reel-preview", title: "リール表示テスト", body: "走りの動画", kind: "instagram", instagram_type: "reel", instagram_shortcode: "C1a2B3c4D5e",
      mine: false, like_count: 0, comment_count: 0, liked_by_me: false, saved_by_me: false,
      created_at: "2026-10-23T00:00:00Z", updated_at: "2026-10-23T00:00:00Z",
    } });
  });
  // 外部サービスに依存せず、iframe自体の寸法と枠内の配置を確認する。
  await page.route("https://www.instagram.com/**", (route) => route.fulfill({ contentType: "text/html", body: "<p>リール動画</p>" }));
  await page.goto("/posts/reel-preview");
  const frame = page.getByTitle("Instagram「リール表示テスト」");
  await expect(frame).toBeVisible();
  const bounds = await frame.boundingBox();
  expect(bounds!.width).toBeLessThanOrEqual(320);
  expect(bounds!.height).toBe(600);
  await page.setViewportSize({ width: 320, height: 740 });
  const mobile = await frame.boundingBox();
  expect(mobile!.x).toBeGreaterThanOrEqual(0);
  expect(mobile!.x + mobile!.width).toBeLessThanOrEqual(320);
});

test("投稿を左へ横送りでき、末尾から次のページを開ける", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  // CIのAPIに投稿がある場合も、古いSSRデータをブラウザの固定テストデータで更新させる。
  await page.clock.setFixedTime(Date.now() + 120_000);
  const portrait = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="320" height="1280"><rect width="320" height="1280" fill="skyblue"/></svg>')}`;
  await page.route("**/api/v1/posts?**", async (route) => {
    const current = Number(new URL(route.request().url()).searchParams.get("page") ?? 1);
    await route.fulfill({ json: {
      posts: Array.from({ length: 6 }, (_, index) => ({
        id: `horizontal-${(current - 1) * 6 + index + 1}`, title: `横送りの投稿 ${(current - 1) * 6 + index + 1}`, body: "練習の話", kind: "text",
        thumbnail_url: index === 0 ? portrait : undefined,
        mine: false, liked_by_me: false, saved_by_me: false, like_count: 0, comment_count: 0,
        created_at: "2026-10-23T00:00:00Z", updated_at: "2026-10-23T00:00:00Z",
      })), meta: { page: current, per_page: 6, total: 12 },
    } });
  });
  await page.goto("/");
  const track = page.getByRole("region", { name: "投稿一覧", exact: true });
  await expect(track).toBeVisible();
  await expect(track.getByRole("link")).toHaveCount(7);
  const first = track.getByRole("link").first();
  const second = track.getByRole("link").nth(1);
  const before = await first.boundingBox();
  const secondBox = await second.boundingBox();
  expect(Math.abs(before!.y - secondBox!.y)).toBeLessThan(2);
  expect(secondBox!.x).toBeGreaterThan(before!.x);
  expect(await track.evaluate((el) => el.scrollWidth > el.clientWidth)).toBe(true);
  const originalY = await page.evaluate(() => window.scrollY);
  await page.getByRole("button", { name: "次の投稿へ", exact: true }).click();
  await expect.poll(() => track.evaluate((el) => el.scrollLeft)).toBeGreaterThan(100);
  expect((await first.boundingBox())!.x).toBeLessThan(before!.x);
  expect(await page.evaluate(() => window.scrollY)).toBe(originalY);
  // 横長の一覧がページ全体にはみ出したり、縦長画像でカードが伸びたりしない。
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  const trackBox = await track.boundingBox();
  expect(trackBox!.height).toBeLessThan(600);
  await track.focus();
  const beforeKey = await track.evaluate((el) => el.scrollLeft);
  await page.keyboard.press("ArrowRight");
  await expect.poll(() => track.evaluate((el) => el.scrollLeft)).toBeGreaterThan(beforeKey);
  const next = track.getByRole("link", { name: "次の投稿を読み込む" });
  await next.scrollIntoViewIfNeeded();
  await expect(next).toBeVisible();
  await next.click();
  await expect(page).toHaveURL(/page=2/);
  await expect(track.getByRole("link").first()).toHaveAttribute("href", "/posts/horizontal-7");
  await expect(track.getByRole("link", { name: "次の投稿を読み込む" })).toHaveCount(0);
});

test("PCでは複数の商品が見え、縦ホイールで左右に進める", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.route("**/api/v1/products?**", (route) => route.fulfill({ json: {
    products: Array.from({ length: 6 }, (_, index) => ({ id: `product-${index}`, brand: "テスト", name: `シューズ${index}`, kind: "shoe", review_count: 0, avg_rating: 0, image_url: "" })),
    meta: { page: 1, per_page: 20, total: 6 },
  } }));
  await page.goto("/?sort=review");
  const track = page.getByRole("region", { name: "レビューの商品一覧" });
  await expect(track).toBeVisible();
  const cards = track.getByRole("link");
  const first = await cards.nth(0).boundingBox();
  const third = await cards.nth(2).boundingBox();
  expect(Math.abs(first!.y - third!.y)).toBeLessThan(2);
  expect(third!.x + third!.width).toBeLessThanOrEqual(1280);
  await track.hover();
  const originalY = await page.evaluate(() => window.scrollY);
  await page.mouse.wheel(0, -240);
  await expect.poll(() => track.evaluate((el) => el.scrollLeft)).toBeGreaterThan(100);
  expect(await page.evaluate(() => window.scrollY)).toBe(originalY);
  await page.mouse.wheel(0, 240);
  await expect.poll(() => track.evaluate((el) => el.scrollLeft)).toBeLessThan(2);
  // トラックパッドの横移動も、そのまま使える。
  await page.mouse.wheel(220, 0);
  await expect.poll(() => track.evaluate((el) => el.scrollLeft)).toBeGreaterThan(100);
  expect(await track.evaluate((el) => {
    const event = new WheelEvent("wheel", { deltaY: 120, ctrlKey: true, bubbles: true, cancelable: true });
    el.dispatchEvent(event);
    return event.defaultPrevented;
  })).toBe(false);
  await page.getByRole("button", { name: "次の商品へ" }).click();
  await expect.poll(() => track.evaluate((el) => el.scrollLeft)).toBeGreaterThan(300);
  // 横方向の終端では、ページの縦スクロールへ自然に戻れる。
  await page.addStyleTag({ content: "body { padding-bottom: 1200px; }" });
  await track.evaluate((el) => { el.scrollLeft = 0; });
  await expect(page.getByRole("button", { name: "前の商品へ" })).toBeDisabled();
  await track.hover();
  const edgeY = await page.evaluate(() => window.scrollY);
  await page.mouse.wheel(0, 180);
  await expect.poll(() => page.evaluate(() => window.scrollY)).toBeGreaterThan(edgeY);
});
