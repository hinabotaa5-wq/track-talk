import { expect, test } from "@playwright/test";

test("top page shows branding", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("link", { name: "TrackTalk" })).toBeVisible();
  await expect(page.getByRole("link", { name: "投稿を見る" })).toBeVisible();
});

test("legal pages are reachable", async ({ page }) => {
  await page.goto("/terms");
  await expect(page.getByRole("heading", { name: "利用規約" })).toBeVisible();
  await page.goto("/privacy");
  await expect(page.getByRole("heading", { name: "プライバシーポリシー" })).toBeVisible();
});
