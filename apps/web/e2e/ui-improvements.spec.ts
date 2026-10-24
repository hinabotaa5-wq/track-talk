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

test("ホームの大会をヘッダーに集約し、未ログインのマイページはログインへ移動する", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("button", { name: "今日開催中の大会", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "今日開催中", exact: true })).toHaveCount(0);
  const account = page.getByRole("link", { name: "マイページ", exact: true });
  await expect(account).toHaveAttribute("href", "/login");
  await account.click();
  await expect(page).toHaveURL(/\/login$/);
  await expect(page.getByRole("dialog", { name: "マイページ", exact: true })).toHaveCount(0);
});

test("大会のタイムテーブル導線を消し、実況スレッドは内部へ移動する", async ({ page }) => {
  await page.goto("/competitions");
  const timetable = page.getByRole("link", { name: "日程・タイムテーブル", exact: true });
  await expect(timetable).toHaveCount(0);
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

test("コーチ画面は3つの小さな入口にまとめ、未ログインなら応募も案内する", async ({ page }) => {
  await page.goto("/coaching");
  const choices = page.getByRole("navigation", { name: "コーチングのプランを選ぶ" });
  await expect(choices.locator("a")).toHaveCount(3);
  expect(await choices.locator("a").evaluateAll((links) => links.map((link) => link.getAttribute("href")))).toEqual(["/coaching/new", "/coaches", "/coach"]);
  await choices.locator('a[href="/coach"]').click();
  await expect(page).toHaveURL(/\/login\?next=%2Fcoach$/);
  await expect(page.getByRole("link", { name: "コーチに応募する", exact: true })).toHaveAttribute("href", "/coach/apply");
});

test("投稿を左へ横送りすると続きが追加され、先の投稿も残る", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
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
  await expect(track.getByRole("link")).toHaveCount(6);
  await expect.poll(() => track.locator("img").evaluateAll((images) => images.every((image) => (image as HTMLImageElement).naturalWidth > 0))).toBe(true);
  await expect.poll(() => track.evaluate((el) => el.scrollWidth > el.clientWidth)).toBe(true);
  const first = track.getByRole("link").first();
  const second = track.getByRole("link").nth(1);
  const before = await first.boundingBox();
  await expect(second).toBeAttached();
  expect(await track.evaluate((el) => el.scrollWidth > el.clientWidth)).toBe(true);
  const originalY = await page.evaluate(() => window.scrollY);
  await expect(page.getByRole("button", { name: "次の投稿へ", exact: true })).toHaveCount(0);
  await track.hover();
  await page.mouse.wheel(0, 240);
  await expect.poll(() => track.evaluate((el) => el.scrollLeft)).toBeGreaterThan(0);
  await expect(track.getByRole("link")).toHaveCount(12);
  expect((await first.boundingBox())!.x).toBeLessThan(before!.x);
  expect(await page.evaluate(() => window.scrollY)).toBe(originalY);
  // 横長の一覧がページ全体にはみ出したり、縦長画像でカードが伸びたりしない。
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await track.focus();
  const beforeKey = await track.evaluate((el) => el.scrollLeft);
  await page.keyboard.press("ArrowRight");
  await expect.poll(() => track.evaluate((el) => el.scrollLeft)).toBeGreaterThan(beforeKey);
  await track.evaluate((el) => { el.scrollLeft = el.scrollWidth; el.dispatchEvent(new Event("scroll")); });
  await expect(page).not.toHaveURL(/page=2/);
  await expect(track.getByRole("link").first()).toHaveAttribute("href", "/posts/horizontal-1");
  await expect(track.getByRole("button", { name: "続きを読み込む" })).toHaveCount(0);
  await page.route("**/api/v1/posts/horizontal-12**", (route) => route.fulfill({ json: {
    id: "horizontal-12", title: "戻り位置の確認", body: "本文", kind: "text",
    mine: false, liked_by_me: false, saved_by_me: false, like_count: 0, comment_count: 0,
    created_at: "2026-10-23T00:00:00Z", updated_at: "2026-10-23T00:00:00Z",
  } }));
  const opened = track.locator('a[href="/posts/horizontal-12"]');
  await opened.scrollIntoViewIfNeeded();
  const openedX = (await opened.boundingBox())!.x;
  await opened.click();
  await expect(page).toHaveURL(/posts\/horizontal-12$/);
  await page.getByRole("button", { name: "戻る", exact: true }).click();
  await expect(track.getByRole("link")).toHaveCount(12);
  await expect.poll(async () => Math.abs((await opened.boundingBox())!.x - openedX)).toBeLessThan(3);
});

test("レビューは商品を縦スクロールで見られる", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.route("**/api/v1/products?**", (route) => route.fulfill({ json: {
    products: Array.from({ length: 12 }, (_, index) => ({ id: `product-${index}`, brand: "テスト", name: `シューズ${index}`, kind: "shoe", review_count: 0, avg_rating: 0, image_url: "" })),
    meta: { page: 1, per_page: 20, total: 12 },
  } }));
  await page.goto("/?sort=review");
  const track = page.getByRole("region", { name: "レビューの商品一覧" });
  await expect(track.getByRole("link")).toHaveCount(12);
  const first = await track.getByRole("link").first().boundingBox();
  const third = await track.getByRole("link").nth(2).boundingBox();
  expect(third!.y).toBeGreaterThan(first!.y + first!.height);
  expect(await track.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(true);
  await track.hover();
  await page.mouse.wheel(0, 240);
  await expect.poll(() => page.evaluate(() => window.scrollY)).toBeGreaterThan(100);
  expect(await track.evaluate((el) => el.scrollLeft)).toBe(0);
});

test("サムネイルの縦横比と異なる高さを保ち、右側の投稿を左へ送れる", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.clock.setFixedTime(Date.now() + 120_000);
  await page.route("**/api/v1/posts?**", (route) => route.fulfill({ json: {
    posts: Array.from({ length: 8 }, (_, index) => index % 2 ? "vertical" : "horizontal").map((name, index) => ({
      id: `ratio-${index}`, title: `${name}動画${index}`, body: "サムネイルの表示確認", kind: "video",
      playback_url: `/samples/${name}.mp4`, thumbnail_url: `/samples/${name}.jpg`,
      mine: false, liked_by_me: false, saved_by_me: false, like_count: 0, comment_count: 0,
      created_at: "2026-10-23T00:00:00Z", updated_at: "2026-10-23T00:00:00Z",
    })), meta: { page: 1, per_page: 20, total: 8 },
  } }));
  await page.goto("/");
  const track = page.getByRole("region", { name: "投稿一覧", exact: true });
  await expect(track.locator("img")).toHaveCount(8);
  await expect(track.locator("video, iframe")).toHaveCount(0);
  await expect.poll(() => track.locator("img").evaluateAll((images) => images.every((image) => (image as HTMLImageElement).naturalWidth > 0))).toBe(true);
  const sizes = await track.locator("img").evaluateAll((images) => images.map((image) => {
    const img = image as HTMLImageElement;
    const bounds = img.getBoundingClientRect();
    return { width: bounds.width, height: bounds.height, ratio: img.naturalWidth / img.naturalHeight };
  }));
  for (const image of sizes) expect(image.width / image.height).toBeCloseTo(image.ratio, 2);
  expect(sizes[0].width).toBeGreaterThan(sizes[0].height);
  expect(sizes[1].height).toBeGreaterThan(sizes[1].width);
  expect(Math.abs(sizes[0].height - sizes[1].height)).toBeGreaterThan(50);
  const first = track.locator("img").first();
  const before = (await first.boundingBox())!.x;
  await track.focus();
  await page.keyboard.press("ArrowRight");
  await expect.poll(() => track.evaluate((el) => el.scrollLeft)).toBeGreaterThan(0);
  expect((await first.boundingBox())!.x).toBeLessThan(before);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});

test("フィードを更新しても投稿の大きさと配置を保つ", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.clock.setFixedTime(Date.now() + 120_000);
  await page.route("**/api/v1/posts?**", (route) => route.fulfill({ json: {
    posts: Array.from({ length: 6 }, (_, index) => ({
      id: `fixed-${index}`, title: `固定投稿${index}`, body: "更新後も形を保つ", kind: "video",
      playback_url: `/samples/${index % 2 ? "vertical" : "horizontal"}.mp4`, thumbnail_url: `/samples/${index % 2 ? "vertical" : "horizontal"}.jpg`,
      mine: false, liked_by_me: false, saved_by_me: false, like_count: 0, comment_count: 0,
      created_at: "2026-10-23T00:00:00Z", updated_at: "2026-10-23T00:00:00Z",
    })), meta: { page: 1, per_page: 20, total: 6 },
  } }));
  const layout = () => page.getByRole("region", { name: "投稿一覧", exact: true }).locator(".masonry-feed-tile").evaluateAll((tiles) => tiles.map((tile) => {
    const rect = tile.getBoundingClientRect();
    return [Math.round(rect.x), Math.round(rect.y), Math.round(rect.width), Math.round(rect.height)];
  }));
  await page.goto("/");
  const track = page.getByRole("region", { name: "投稿一覧", exact: true });
  await expect(track.locator("img")).toHaveCount(6);
  await expect.poll(() => track.locator("img").evaluateAll((images) => images.every((image) => (image as HTMLImageElement).naturalWidth > 0))).toBe(true);
  await expect(track.locator(".masonry-feed-canvas[data-ready=\"true\"]")).toBeVisible();
  const before = await layout();
  await page.reload();
  await expect(track.locator("img")).toHaveCount(6);
  await expect.poll(() => track.locator("img").evaluateAll((images) => images.every((image) => (image as HTMLImageElement).naturalWidth > 0))).toBe(true);
  await expect(track.locator(".masonry-feed-canvas[data-ready=\"true\"]")).toBeVisible();
  expect(await layout()).toEqual(before);
});

test("空きに組み合わせても新しい順・人気順と固定タブを保つ", async ({ page }) => {
  await page.clock.setFixedTime(Date.now() + 120_000);
  const newOrder = ["newest", "middle", "oldest", "fourth", "fifth", "sixth", "seventh", "eighth"];
  const popularOrder = ["oldest", "fourth", "newest", "middle", "fifth", "sixth", "seventh", "eighth"];
  await page.route("**/api/v1/posts?**", (route) => {
    const order = new URL(route.request().url()).searchParams.get("sort") === "popular" ? popularOrder : newOrder;
    return route.fulfill({ json: { posts: order.map((id, index) => ({
      id, title: id, body: id, kind: "video", playback_url: "/samples/vertical.mp4",
      thumbnail_url: `/samples/${index % 3 === 0 ? "vertical" : "horizontal"}.jpg`,
      mine: false, liked_by_me: false, saved_by_me: false, like_count: 0, comment_count: 0,
      created_at: "2026-10-23T00:00:00Z", updated_at: "2026-10-23T00:00:00Z",
    })), meta: { page: 1, per_page: 20, total: order.length } } });
  });
  await page.goto("/");
  const track = page.getByRole("region", { name: "投稿一覧", exact: true });
  const nav = page.getByRole("navigation", { name: "投稿の並び替えと検索" });
  await expect(track.locator('.masonry-feed-canvas[data-ready="true"]')).toBeVisible();
  await expect(track.getByRole("link")).toHaveText(newOrder);
  const before = await nav.boundingBox();
  const originalY = await page.evaluate(() => window.scrollY);
  await track.hover();
  await page.mouse.wheel(0, 200);
  await expect.poll(() => track.evaluate((el) => el.scrollLeft)).toBeGreaterThan(0);
  expect((await nav.boundingBox())!.x).toBe(before!.x);
  expect((await nav.boundingBox())!.y).toBe(before!.y);
  expect(await page.evaluate(() => window.scrollY)).toBe(originalY);
  await nav.getByRole("link", { name: "人気", exact: true }).click();
  await expect(track.getByRole("link")).toHaveText(popularOrder);
  await expect(nav.getByRole("link", { name: "人気", exact: true })).toHaveAttribute("aria-current", "page");
  await nav.getByRole("link", { name: "新しい", exact: true }).click();
  await expect(track.getByRole("link")).toHaveText(newOrder);
});
