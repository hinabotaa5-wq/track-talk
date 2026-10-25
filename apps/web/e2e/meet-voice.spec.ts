import { expect, test } from "@playwright/test";
const postId = "bf48de68-d3c1-4492-9d91-0ac334b589fd";
test.beforeEach(async ({ page }) => {
  page.on("console", (message) => { if (message.type() === "error" && message.text().includes("same key")) throw new Error(message.text()); });
  await page.route("**/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    let json: unknown = {};
    if (path.endsWith(`/posts/${postId}`)) json = { id: postId, kind: "meet", title: "秋季陸上大会の実況", body: "会場: 国立競技場", comment_count: 0, like_count: 0, created_at: "2026-10-25T03:00:00Z" };
    else if (path.endsWith("/meet-resources")) json = { official_url: "https://example.com/", youtube_video_id: "abcdefghijk" };
    else if (path.endsWith("/comments")) json = { comments: [], meta: { total: 0, total_pages: 1 } };
    else if (path.endsWith("/voice")) json = { enabled: true, participants: 4, capacity: 4, full: true };
    else if (path.endsWith("/me")) json = { id: "user", nickname: "検証者", event: "100m", nickname_public: true, is_admin: false };
    else if (path.endsWith("/today")) json = { meets: [] };
    else if (path.endsWith("/competitions")) json = { competitions: [] };
    else if (path.endsWith("/major-hosts")) json = { hosts: [] };
    else if (path.endsWith("/domestic-majors")) json = { majors: [] };
    await route.fulfill({ json });
  });
  await page.route("https://www.youtube-nocookie.com/**", (route) => route.abort());
});
test("スマホ幅で未ログインの入口と動画音量を表示し、自動接続しない", async ({ page }) => {
  const joins: string[] = []; page.on("request", (request) => { if (request.url().endsWith("/voice/join")) joins.push(request.url()); });
  await page.goto(`/posts/${postId}`);
  const panel = page.getByRole("region", { name: "実況ルームの音声通話" });
  await expect(panel.getByRole("link", { name: "ログインして参加" })).toBeVisible();
  await expect(page.getByRole("slider", { name: "動画の音量" })).toBeVisible();
  expect(joins).toEqual([]);
  expect(await panel.evaluate((node) => node.scrollWidth <= node.clientWidth + 1)).toBe(true);
  await page.screenshot({ path: test.info().outputPath("voice-mobile.png"), fullPage: true });
});
test("ログイン済みの満員表示は参加を送信せず、PC幅でも収まる", async ({ page, context }) => {
  const expires = Math.floor(Date.now() / 1000) + 3600;
  const encode = (value: object) => Buffer.from(JSON.stringify(value)).toString("base64url");
  const token = `${encode({ alg: "HS256", typ: "JWT" })}.${encode({ sub: "user", exp: expires, aud: "authenticated" })}.e2e-signature`;
  const session = { access_token: token, refresh_token: "refresh", token_type: "bearer", expires_in: 3600, expires_at: expires, user: { id: "user", aud: "authenticated", email: "voice@example.test", app_metadata: {}, user_metadata: {}, created_at: new Date().toISOString() } };
  await context.addCookies([{ name: "sb-127-auth-token", value: `base64-${Buffer.from(JSON.stringify(session)).toString("base64url")}`, url: "http://127.0.0.1:3127" }]);
  await page.setViewportSize({ width: 1100, height: 900 });
  await page.goto(`/posts/${postId}`);
  await expect(page.getByRole("button", { name: "満員です", exact: true })).toBeDisabled();
  expect(await page.getByRole("region", { name: "実況ルームの音声通話" }).evaluate((node) => node.scrollWidth <= node.clientWidth + 1)).toBe(true);
  await page.screenshot({ path: test.info().outputPath("voice-desktop-full.png"), fullPage: true });
});
