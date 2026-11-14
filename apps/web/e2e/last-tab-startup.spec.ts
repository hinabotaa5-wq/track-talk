import { expect, test } from "@playwright/test";

test("前回のタブがあっても JavaScript の読み込みを待たずにコーチを開く", async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem("tracktalk:last-tab", "/memo"));
  await page.route(/\/_next\/.*\.js(?:\?|$)/, (route) => route.abort());
  await page.goto("/", { waitUntil: "commit" });
  await expect(page).toHaveURL(/\/coaching$/);
});

test("保存がない初回はコーチを開く", async ({ page }) => {
  await page.route(/\/_next\/.*\.js(?:\?|$)/, (route) => route.abort());
  await page.goto("/", { waitUntil: "commit" });
  await expect(page).toHaveURL(/\/coaching$/);
});

test("前回のタブがフィードのときはコーチを開く", async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem("tracktalk:last-tab", "/"));
  await page.route(/\/_next\/.*\.js(?:\?|$)/, (route) => route.abort());
  await page.goto("/", { waitUntil: "commit" });
  await expect(page).toHaveURL(/\/coaching$/);
});
