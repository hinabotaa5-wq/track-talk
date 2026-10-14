import { expect, test } from "@playwright/test";

test("login then publish a text post", async ({ page }) => {
  test.skip(!process.env.CI, "CI の API と一緒に実行する");
  test.setTimeout(90_000);

  await page.goto("/login?next=/posts/new");
  await page.getByPlaceholder("メール").fill("runner@example.com");
  await page.getByPlaceholder("パスワード").fill("password1");
  await page.getByRole("button", { name: "ログイン", exact: true }).click();
  await expect(page.getByRole("heading", { name: "文章" })).toBeVisible({ timeout: 20_000 });

  const title = `朝練の話 ${Date.now()}`;
  await page.getByLabel("タイトル").fill(title);
  await page.getByLabel("本文").fill("今日のスタートを共有します");
  await page.getByRole("button", { name: "投稿する" }).click();
  await expect(page.getByRole("link", { name: title })).toBeVisible({ timeout: 20_000 });
});
