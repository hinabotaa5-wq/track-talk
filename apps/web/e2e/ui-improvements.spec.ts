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

test("ホームの大会をヘッダーに集約し、マイページを再タップで閉じる", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("button", { name: "今日開催中の大会", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "今日開催中", exact: true })).toHaveCount(0);
  const account = page.getByRole("button", { name: "マイページ", exact: true });
  await account.click();
  await expect(page.getByRole("dialog", { name: "マイページ", exact: true })).toBeVisible();
  // 前面の閉じる領域も同じタップ位置で閉じられる。
  const box = await account.boundingBox();
  await page.mouse.click(box!.x + box!.width / 2, box!.y + box!.height / 2);
  await expect(account).toHaveAttribute("aria-expanded", "false");
  await expect(page).toHaveURL("/");
});

test("開催中でもタイムテーブルは外部、実況スレッドは内部へ移動する", async ({ page }) => {
  await page.goto("/competitions");
  const timetable = page.getByRole("link", { name: "日程・タイムテーブル", exact: true });
  await expect(timetable).toHaveAttribute("href", official);
  await expect(timetable).toHaveAttribute("target", "_blank");
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
