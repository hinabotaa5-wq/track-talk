import { expect, test, type Page } from "@playwright/test";
import path from "node:path";

// コーチ画面のランチャーが、ヘッダーと下部ナビの間の高さいっぱいに広がることを確かめる（本体 #563）。
// NEXT_PUBLIC_COACHING_ENABLED=true のプレビューで実行する（起動手順は docs/coach-availability.md と同じ）。データと認証はテスト用。
test.skip(process.env.PLAYWRIGHT_COACHING_ENABLED !== "1", "コーチングを有効にしたローカルプレビューで実行します");
const coachID = "22222222-2222-4222-8222-222222222222";
const userID = "11111111-1111-4111-8111-111111111111";
const screenshotDir = path.resolve(process.cwd(), "../../docs/screenshots/coach-launcher");
const application = {
  id: coachID, name: "サンプルコーチ", tier: "coach", events: ["100m"], service_types: ["video_review"], status: "approved",
  applied_at: "2026-01-01T00:00:00Z", reviewed_at: null, on_duty: true, daily_cap: 5, photo_url: "", pledged: true, payouts_ready: false,
};
const reviewRequest = (id: string) => ({
  id, event: "100m", tier: "coach", question: "フォームを見てください", focus: [], status: "in_review",
  due_at: "2026-12-01T00:00:00Z", requested_coach_id: coachID, coach_reward_yen: 1050,
});

async function setup(page: Page, withOffers: boolean) {
  const context = page.context();
  const baseURL = test.info().project.use.baseURL ?? "http://localhost:3000";
  const user = { id: userID, aud: "authenticated", role: "authenticated", email: "preview@example.com", app_metadata: {}, user_metadata: {}, identities: [], created_at: new Date().toISOString() };
  const payload = { sub: userID, aud: "authenticated", role: "authenticated", aal: "aal1", exp: Math.floor(Date.now() / 1000) + 3600, iat: Math.floor(Date.now() / 1000) };
  const accessToken = `${Buffer.from('{"alg":"HS256","typ":"JWT"}').toString("base64url")}.${Buffer.from(JSON.stringify(payload)).toString("base64url")}.preview`;
  const session = { access_token: accessToken, token_type: "bearer", refresh_token: "preview", expires_in: 3600, expires_at: payload.exp, user };
  await context.addCookies([{ name: "sb-127-auth-token", value: `base64-${Buffer.from(JSON.stringify(session)).toString("base64url")}`, url: baseURL }]);
  await page.route("**/auth/v1/**", (route) => route.fulfill({ json: user }));
  await page.route("**/api/v1/**", async (route) => {
    const pathname = new URL(route.request().url()).pathname;
    let body: unknown = {};
    if (pathname.endsWith("/coach/me")) body = { application, rating: { count: 3, average: 4.5, paused: false } };
    else if (pathname.endsWith("/me")) body = { id: userID, nickname: "サンプル選手", event: "100m", mfa_required: false };
    else if (pathname.endsWith("/coach/requests")) {
      body = {
        offers: withOffers
          ? [{ id: "o1", expires_at: new Date(Date.now() + 50 * 60000).toISOString(), request: reviewRequest("r0") }]
          : [],
        reviewing: [reviewRequest("r1")],
      };
    } else if (pathname.endsWith("/coach/consultations")) body = { consultations: [{ id: "c1", status: "open" }] };
    else if (pathname.endsWith("/coach/contracts")) body = { contracts: [{ id: "k1", status: "applied" }] };
    else if (pathname.endsWith("/me/earnings")) body = { unpaid_yen: 1050, next_payout_date: "2026-12-15", next_payout_yen: 1050 };
    else if (pathname.endsWith("/me/identity-status")) body = { registered: false, applies: true };
    else if (pathname.endsWith("/notifications")) body = { notifications: [], unread: 0 };
    else if (pathname.endsWith("/meet-threads/today")) body = { meets: [] };
    else if (pathname.endsWith("/domestic-majors")) body = { majors: [] };
    else if (pathname.endsWith("/competitions")) body = { competitions: [] };
    await route.fulfill({ json: body });
  });
  await page.goto("/coach");
  await expect(page.locator("#startup-splash")).toHaveCount(0);
  await expect(page.locator("nav.grid a")).toHaveCount(11);
  await expect(page.locator("footer").getByText("コーチをやめる")).toBeVisible();
  if (withOffers) await expect(page.getByRole("link", { name: /新しい依頼/ })).toBeVisible();
  await expect(page.locator(".screen-loader")).toBeHidden();
}

async function measure(page: Page) {
  return page.evaluate(() => {
    const rect = (el: Element | null) => el!.getBoundingClientRect();
    const grid = rect(document.querySelector(".coach-tiles"));
    const footer = rect(document.querySelector(".coach-launcher > footer"));
    const bottomNav = [...document.querySelectorAll("nav")].find((n) => getComputedStyle(n).position === "fixed");
    const tiles = [...document.querySelectorAll("nav.grid a")].map((a) => a.getBoundingClientRect());
    return {
      gridTop: grid.top, gridBottom: grid.bottom, footerTop: footer.top, footerBottom: footer.bottom,
      navTop: bottomNav ? bottomNav.getBoundingClientRect().top : window.innerHeight,
      tileHeights: tiles.map((t) => Math.round(t.height)),
      squareDiffs: tiles.map((t) => Math.abs(t.width - t.height)),
      rowTops: [0, 3, 6, 9].map((i) => tiles[i].top), lastRowBottom: tiles[9].bottom, columnsInFirstRow: tiles.filter((t) => t.top === tiles[0].top).length,
      scrollHeight: document.documentElement.scrollHeight, innerHeight: window.innerHeight,
    };
  });
}

for (const withOffers of [false, true]) {
  const name = withOffers ? "alert" : "no-alert";
  test(`390×844 でタイルが下部ナビの手前まで広がり、スクロールせずに全部見える（${withOffers ? "期限アラートあり" : "アラートなし"}）`, async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await setup(page, withOffers);
    const m = await measure(page);
    // スクロールなし。フッターは下部ナビに重ならず、その手前（main の下余白 24px 以内）に来る。
    expect(m.scrollHeight).toBeLessThanOrEqual(m.innerHeight);
    expect(m.footerBottom).toBeLessThanOrEqual(m.navTop);
    expect(m.navTop - m.footerBottom).toBeLessThanOrEqual(26);
    // タイルの枠はフッターの直前まで届き（間は gap の 12px）、タイルは3列の正方形で間を詰め（8px）、
    // 枠の中央に置かれる（上詰め＋下の大きな余白にならない）。従来（約70px）より大きい。
    expect(m.footerTop - m.gridBottom).toBeLessThanOrEqual(26);
    expect(m.columnsInFirstRow).toBe(3);
    expect(Math.max(...m.squareDiffs)).toBeLessThanOrEqual(1);
    expect(m.tileHeights[0]).toBeGreaterThan(90);
    for (let i = 1; i < 4; i++) expect(Math.abs(m.rowTops[i] - m.rowTops[i - 1] - m.tileHeights[0] - 8)).toBeLessThanOrEqual(1);
    const above = m.rowTops[0] - m.gridTop;
    const below = m.gridBottom - m.lastRowBottom;
    expect(Math.abs(above - below)).toBeLessThanOrEqual(2);
    await expect(page.locator("body")).toHaveJSProperty("scrollWidth", 390);
    if (process.env.PLAYWRIGHT_SCREENSHOTS === "1") await page.screenshot({ path: path.join(screenshotDir, `mobile-${name}.png`) });
  });
}

test("低い画面（390×560）ではタイルが正方形のまま潰れず、収まらない分はページが縦にスクロールする", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 560 });
  await setup(page, true);
  const m = await measure(page);
  expect(m.columnsInFirstRow).toBe(3);
  expect(Math.max(...m.squareDiffs)).toBeLessThanOrEqual(1);
  expect(Math.min(...m.tileHeights)).toBeGreaterThanOrEqual(70);
  await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
  const end = await measure(page);
  expect(end.footerBottom).toBeLessThanOrEqual(end.navTop);
  if (process.env.PLAYWRIGHT_SCREENSHOTS === "1") await page.screenshot({ path: path.join(screenshotDir, "mobile-short.png") });
});

test("PC 幅では広げず、3列の正方形のタイルを上から並べる", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await setup(page, false);
  const m = await measure(page);
  expect(m.columnsInFirstRow).toBe(3);
  expect(Math.max(...m.squareDiffs)).toBeLessThanOrEqual(1);
  expect(m.rowTops[0] - m.gridTop).toBeLessThanOrEqual(1);
});
