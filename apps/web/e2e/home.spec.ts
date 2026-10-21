import { expect, test } from "@playwright/test";

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
