import { expect, test } from "@playwright/test";

test("login then publish a text post", async ({ page }) => {
  test.skip(!process.env.CI, "CI の API と一緒に実行する");
  test.setTimeout(90_000);

  await page.goto("/login?next=/posts/new");
  await page.getByPlaceholder("メール").fill("runner@example.com");
  await page.getByPlaceholder("パスワード").fill("password1");
  await page.getByRole("button", { name: "ログイン", exact: true }).click();
  // 登録画面を通らずにログインした人は、海外の事業者に預けることへの同意から始まる。
  await expect(page.getByRole("heading", { name: "海外の事業者に預けることへの同意" })).toBeVisible({ timeout: 20_000 });
  await page.getByRole("checkbox").check();
  await page.getByRole("button", { name: "同意して続ける" }).click();
  // はじめてのログインは、ニックネームと種目を決めるまで先に進めない。
  await expect(page.getByRole("heading", { name: "ニックネームと種目を決めよう" })).toBeVisible({ timeout: 20_000 });
  await page.getByPlaceholder("ニックネーム").fill("あされん");
  await page.getByLabel("種目").selectOption("100m");
  await page.getByRole("button", { name: "保存" }).click();
  await expect(page).toHaveURL("/", { timeout: 20_000 });

  await page.goto("/posts/new");
  await expect(page.getByRole("heading", { name: "文章・写真" })).toBeVisible({ timeout: 20_000 });

  const body = `朝練の話 ${Date.now()}\n今日のスタートを共有します`;
  await page.getByLabel("詳細").fill(body);
  await page.getByRole("button", { name: "投稿する" }).click();
  await expect(page.getByRole("link", { name: new RegExp(body.split("\n")[0]) })).toBeVisible({ timeout: 20_000 });
});
