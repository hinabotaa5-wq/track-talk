import { expect, test } from "@playwright/test";

test("アプリの JavaScript の読み込みを待たずに前回のタブを開く", async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem("tracktalk:last-tab", "/memo"));
  await page.route(/\/_next\/.*\.js(?:\?|$)/, (route) => route.abort());
  await page.goto("/", { waitUntil: "commit" });
  await expect(page).toHaveURL(/\/memo$/);
});

test("保存がない初回はコーチを開く", async ({ page }) => {
  await page.route(/\/_next\/.*\.js(?:\?|$)/, (route) => route.abort());
  await page.goto("/", { waitUntil: "commit" });
  await expect(page).toHaveURL(/\/coaching$/);
});

test("前回のタブがホームのときはコーチを開く", async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem("tracktalk:last-tab", "/"));
  await page.route(/\/_next\/.*\.js(?:\?|$)/, (route) => route.abort());
  await page.goto("/", { waitUntil: "commit" });
  await expect(page).toHaveURL(/\/coaching$/);
});
