import { expect, test } from "@playwright/test";
const post = {
  id: "ui-description", title: "走りの確認", body: "スタートで上体が起きてしまいます。改善点を教えてください。", excerpt: "スタートで上体が起きてしまいます。改善点を教えてください。",
  kind: "video", playback_url: "/samples/vertical.mp4", thumbnail_url: "/samples/vertical.jpg",
  mine: false, like_count: 0, comment_count: 1, liked_by_me: false, saved_by_me: false,
  created_at: "2026-10-24T00:00:00Z", updated_at: "2026-10-24T00:00:00Z",
};
const longBody = "折り返しのない英数字" + "A".repeat(1000);
test("実況ルームは大会名、会場、配信、公式サイト、タイムテーブルの順に表示し、運営名を出さない", async ({ page }) => {
  const meet = { ...post, kind: "meet", title: "秋季陸上大会", body: "会場: 国立競技場", author_name: "TrackTalk", author_post_master: true, playback_url: null, thumbnail_url: null };
  await page.route(`**/api/v1/posts/${post.id}`, (route) => route.fulfill({ json: meet }));
  await page.route(`**/api/v1/posts/${post.id}/meet-resources`, (route) => route.fulfill({ json: {
    official_url: "https://example.com/", timetable_url: "https://example.com/time.pdf", youtube_video_id: "abcdefghijk",
  } }));
  await page.route("https://www.youtube-nocookie.com/**", (route) => route.abort());
  await page.goto(`/posts/${post.id}`);
  const main = page.locator("main");
  await expect(main.getByRole("heading", { name: meet.title, exact: true })).toHaveCount(1);
  await expect(main.getByText("TrackTalk", { exact: true })).toHaveCount(0);
  await expect(main.getByText("投稿マスター", { exact: true })).toHaveCount(0);
  await expect(main.getByRole("link", { name: "タイムテーブル", exact: true })).toBeVisible();
  const order = await main.evaluate((element) => {
    const nodes = [
      element.querySelector("h1")!,
      [...element.querySelectorAll("div")].find((node) => node.textContent === "会場: 国立競技場")!,
      element.querySelector('section[aria-label="YouTube生配信"]')!,
      element.querySelector('a[href="https://example.com/"]')!,
      element.querySelector('a[href="https://example.com/time.pdf"]')!,
    ];
    return nodes.slice(1).every((node, index) => !!(nodes[index].compareDocumentPosition(node) & Node.DOCUMENT_POSITION_FOLLOWING));
  });
  expect(order).toBe(true);
  await page.screenshot({ path: test.info().outputPath("meet-room.png") });
});
test.beforeEach(async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 740 });
  await page.route("**/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    let json: unknown = {};
    if (path.endsWith("/comments")) json = { comments: [{ id: "long-comment", body: longBody, parent_id: null, reply_to_id: null, author_name: "匿名", mine: false, created_at: post.created_at }], meta: { total: 1 } };
    else if (path.endsWith(`/posts/${post.id}`)) json = post;
    else if (path.endsWith("/posts")) json = { posts: [post], meta: { page: 1, total: 1, total_pages: 1 } };
    else if (path.endsWith("/meet-threads/today")) json = { meets: [] };
    else if (path.endsWith("/domestic-majors")) json = { majors: [] };
    else if (path.endsWith("/major-hosts")) json = { hosts: [] };
    else if (path.endsWith("/competitions")) json = { competitions: [] };
    await route.fulfill({ json });
  });
});
test("詳細文を動画より上に大きく太字で表示し、枠を付けない", async ({ page }) => {
  await page.goto(`/posts/${post.id}`);
  const description = page.locator(".post-description");
  await expect(description).toHaveText(post.body, { timeout: 15000 });
  const bounds = await description.boundingBox();
  const video = await page.locator("main video").boundingBox();
  expect(bounds!.y + bounds!.height).toBeLessThanOrEqual(video!.y);
  const style = await description.evaluate((element) => ({ font: getComputedStyle(element).fontSize, weight: getComputedStyle(element).fontWeight, border: getComputedStyle(element).borderTopWidth }));
  expect(style).toEqual({ font: "18px", weight: "700", border: "0px" });
  await page.screenshot({ path: test.info().outputPath("post-detail.png") });
});
test("320pxのチャットで長文を小さく折り返し、横にはみ出さない", async ({ page }) => {
  await page.goto(`/posts/${post.id}?room=long-comment`);
  const bubble = page.locator("#comment-long-comment p");
  await expect(bubble).toHaveText(longBody);
  const threadTitle = page.locator("main h2").first();
  expect(await threadTitle.evaluate((element) => getComputedStyle(element).textAlign)).toBe("center");
  const dimensions = await bubble.evaluate((element) => ({ width: element.clientWidth, scroll: element.scrollWidth, font: getComputedStyle(element).fontSize, right: element.getBoundingClientRect().right }));
  expect(dimensions.scroll).toBeLessThanOrEqual(dimensions.width + 1);
  expect(dimensions.right).toBeLessThanOrEqual(320);
  expect(Number.parseFloat(dimensions.font)).toBeLessThan(16);
  await page.screenshot({ path: test.info().outputPath("chat.png") });
});
for (const kind of ["text", "video"] as const) {
test(`${kind}：画像の下部に詳細文を重ね、タイトル用の別枠を作らない`, async ({ page }) => {
  // 一覧APIと同じく本文を含めず、写真付き文章もexcerptから描く。
  await page.route("**/api/v1/posts?*", (route) => route.fulfill({ json: {
    posts: [{ ...post, kind, body: undefined, excerpt: post.body }],
    meta: { page: 1, total: 1, total_pages: 1 },
  } }));
  await page.goto("/");
  await page.getByRole("link", { name: "人気", exact: true }).click();
  const card = page.locator(`a[href="/posts/${post.id}"]`);
  await expect(card).toBeVisible();
  await expect.poll(() => card.locator("img").evaluate((image: HTMLImageElement) => image.naturalWidth)).toBeGreaterThan(0);
  await expect(card.locator(".feed-card-caption")).toHaveText(post.body);
  await expect(card.locator(".feed-card-title")).toHaveCount(0);
  const placement = await card.evaluate((element) => {
    const image = element.querySelector(".feed-card-thumbnail")!.getBoundingClientRect();
    const caption = element.querySelector(".feed-card-caption")!.getBoundingClientRect();
    return { imageBottom: image.bottom, captionBottom: caption.bottom, blur: getComputedStyle(element.querySelector(".feed-card-caption-background")!).backdropFilter };
  });
  expect(Math.abs(placement.imageBottom - placement.captionBottom)).toBeLessThan(2);
  expect(placement.blur).toBe("blur(8px)");
  await page.screenshot({ path: test.info().outputPath("feed.png") });
});

}
