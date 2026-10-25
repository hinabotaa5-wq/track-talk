import { expect, test } from "@playwright/test";

test("起動直後の通信が終わらなくても検索とホームの切替を操作できる", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const pending: (() => void)[] = [];
  await page.route("**/v1/**", async (route) => {
    await new Promise<void>((resolve) => pending.push(resolve));
    await route.abort().catch(() => {});
  });
  const prefetches: string[] = [];
  page.on("request", (request) => {
    if (request.headers()["next-router-prefetch"]) prefetches.push(request.url());
  });
  try {
    await page.goto("/?sort=new", { waitUntil: "domcontentloaded" });
    await page.getByRole("button", { name: "キーワード検索を開く" }).click();
    await expect(page.getByRole("searchbox", { name: "キーワード" })).toBeFocused();
    await page.getByRole("button", { name: "検索を閉じる" }).click();
    await page.getByRole("link", { name: "人気", exact: true }).click();
    await expect(page.getByRole("link", { name: "人気", exact: true })).toHaveAttribute("aria-current", "page");
    await expect(page).toHaveURL(/sort=popular/);
    await expect.poll(() => pending.length).toBeGreaterThan(0);
    expect(prefetches).toEqual([]);
  } finally {
    for (const resolve of pending) resolve();
    await page.unrouteAll({ behavior: "ignoreErrors" });
  }
});

test("下部バーは初回の画面取得待ちを表示し、待っている間も別のタブへ移れる", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  let release: (() => void) | undefined;
  await page.route("**/competitions?*", async (route) => {
    if (!route.request().headers().rsc) return route.continue();
    await new Promise<void>((resolve) => { release = resolve; });
    await route.abort().catch(() => {});
  });
  try {
    await page.goto("/?sort=new", { waitUntil: "domcontentloaded" });
    const nav = page.getByRole("navigation", { name: "メイン", exact: true }).filter({ visible: true });
    const competitions = nav.getByRole("link", { name: "大会", exact: true });
    await competitions.click();
    await expect(competitions.locator('[aria-busy="true"]')).toBeVisible();
    await nav.getByRole("link", { name: "コーチ", exact: true }).click();
    await expect(page).toHaveURL(/\/coaching$/);
    await expect(nav.getByRole("link", { name: "コーチ", exact: true })).toHaveAttribute("aria-current", "page");
  } finally {
    release?.();
    await page.unrouteAll({ behavior: "ignoreErrors" });
  }
});
