import { expect, test } from "@playwright/test";
import path from "node:path";

// NEXT_PUBLIC_COACHING_ENABLED=true のプレビューで実行する。データと認証はテスト用。
test.skip(process.env.PLAYWRIGHT_COACHING_ENABLED !== "1", "コーチングを有効にしたローカルプレビューで実行します");
const coachID = "22222222-2222-4222-8222-222222222222";
const planID = "33333333-3333-4333-8333-333333333333";
const userID = "11111111-1111-4111-8111-111111111111";
const today = new Intl.DateTimeFormat("sv-SE", { timeZone: "Asia/Tokyo" }).format(new Date());
const days = Array.from({ length: 21 }, (_, i) => {
  const day = new Date(`${today}T00:00:00Z`); day.setUTCDate(day.getUTCDate() + i);
  return { date: day.toISOString().slice(0, 10), status: ["available", "consult", "unavailable", "", "available", "consult", ""][i % 7], note: i === 0 ? "午後のみ" : "" };
});
const plan = {
  id: planID, coach_id: coachID, kind: "period", title: "スタートを磨く対面レッスン", description: "フォームを確認し、スタートから加速までを一緒に練習します。",
  duration_days: 30, price_yen: 8000, capacity: 3, published: true, full: false, service_type: "in_person",
  venue_area: "東京都", venue_name: "陸上競技場", session_count: 1, session_minutes: 60, travel_fee_yen: 0, max_participants: 1,
  video_reviews: null, menu_updates: null, reply_formats: [], start_on_first_video: false,
};
const coach = { id: coachID, name: "サンプルコーチ", nickname: "サンプルコーチ", tier: "coach", events: ["100m"], best_record: "100m 10秒台", achievements: "短距離の指導経験があります。", years: 5, sns_urls: [], photo_url: "", status: "approved", pledged: true, payouts_ready: true, on_duty: false, daily_cap: 5, service_types: ["in_person", "competition_support"], plans: [plan], rating_count: 0, rating_average: 0 };
const screenshotStyle = "header, nav, nextjs-portal { visibility: hidden !important; }";
const screenshotDir = path.resolve(process.cwd(), "../../docs/screenshots/coach-availability");

test.beforeEach(async ({ page, context, baseURL }) => {
  await page.setViewportSize({ width: 390, height: 950 });
  const user = { id: userID, aud: "authenticated", role: "authenticated", email: "preview@example.com", app_metadata: {}, user_metadata: {}, identities: [], created_at: new Date().toISOString() };
  const payload = { sub: userID, aud: "authenticated", role: "authenticated", aal: "aal1", exp: Math.floor(Date.now() / 1000) + 3600, iat: Math.floor(Date.now() / 1000) };
  const accessToken = `${Buffer.from('{"alg":"HS256","typ":"JWT"}').toString("base64url")}.${Buffer.from(JSON.stringify(payload)).toString("base64url")}.preview`;
  const session = { access_token: accessToken, token_type: "bearer", refresh_token: "preview", expires_in: 3600, expires_at: payload.exp, user };
  await context.addCookies([{ name: "sb-127-auth-token", value: `base64-${Buffer.from(JSON.stringify(session)).toString("base64url")}`, url: baseURL! }]);
  await page.route("**/auth/v1/**", (route) => route.fulfill({ json: user }));
  await page.route("**/api/v1/**", async (route) => {
    const url = new URL(route.request().url()); const pathname = url.pathname;
    if (pathname.endsWith("/coaching/contracts") && route.request().method() === "POST") {
      await route.fulfill({ status: 422, json: { error: { code: "validation_error", message: "入力を確認してください", fields: { preferred_dates: "空き状況が変わりました。選び直してください" } } } });
      return;
    }
    let body: unknown = {};
    if (pathname.endsWith("/availability")) body = { today, days };
    else if (pathname.endsWith("/me")) body = pathname.includes("/coach/") ? { application: coach } : { id: userID, nickname: "サンプル選手", event: "100m", mfa_required: false };
    else if (pathname.endsWith("/me/coaching")) body = { demo: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
    else if (pathname.endsWith(`/coaching/plans/${planID}`)) body = { payments: false, plan, coach };
    else if (pathname.endsWith("/coaches")) body = { coaches: [coach] };
    else if (pathname.endsWith("/coach/me/plans")) body = { plans: [plan] };
    else if (pathname.endsWith("/coach/requests")) body = { offers: [], reviewing: [] };
    else if (pathname.endsWith("/contracts")) body = { contracts: [] };
    else if (pathname.endsWith("/consultations")) body = { consultations: [] };
    else if (pathname.endsWith("/earnings")) body = { payouts_ready: true, stripe_configured: false, totals: {}, months: [] };
    else if (pathname.endsWith("/notifications")) body = { notifications: [], unread: 0 };
    else if (pathname.endsWith("/meet-threads/today")) body = { meets: [] };
    else if (pathname.endsWith("/domestic-majors")) body = { majors: [] };
    else if (pathname.endsWith("/competitions")) body = { competitions: [] };
    await route.fulfill({ json: body });
  });
});

test("コーチが21日分を入力して保存でき、スマホで横にはみ出さない", async ({ page }) => {
  await page.goto("/coach");
  await expect(page.locator("#startup-splash")).toHaveCount(0);
  const editor = page.getByRole("region", { name: "空き状況の入力" });
  await expect(editor.getByRole("button", { name: "○ 空きあり", exact: true })).toBeVisible();
  await editor.scrollIntoViewIfNeeded();
  await expect(editor.locator('[aria-label="3週間の空き状況"] > button')).toHaveCount(21);
  await expect(editor.locator('[aria-label="3週間の空き状況"]')).toHaveCSS("display", "grid");
  await expect(page.locator("body")).toHaveJSProperty("scrollWidth", 390);
  await expect(page.locator(".screen-loader")).toBeHidden();
  if (process.env.PLAYWRIGHT_SCREENSHOTS === "1") await editor.screenshot({ style: screenshotStyle, path: path.join(screenshotDir, "coach-editor.png") });
  await editor.getByRole("button", { name: "△ 要相談", exact: true }).click();
  const saved = page.waitForRequest((request) => request.method() === "PUT" && request.url().endsWith("/availability"));
  await editor.getByRole("button", { name: "空き状況を保存する" }).click();
  expect((await saved).postDataJSON().days[0].status).toBe("consult");
  await expect(editor.getByRole("status")).toHaveText("保存しました");
});

test("プランのカレンダーから希望日を3日選び申し込みに送る", async ({ page }) => {
  await page.goto(`/coaching/personal/plans/${planID}`);
  await expect(page.locator("#startup-splash")).toHaveCount(0);
  const picker = page.getByRole("region", { name: "希望日の選択" });
  await expect(picker.getByText("午後のみ")).toBeVisible();
  await expect(page.locator("body")).toHaveJSProperty("scrollWidth", 390);
  await expect(page.locator(".screen-loader")).toBeHidden();
  await picker.scrollIntoViewIfNeeded();
  if (process.env.PLAYWRIGHT_SCREENSHOTS === "1") await page.locator("main").screenshot({ style: screenshotStyle, path: path.join(screenshotDir, "plan-calendar.png") });
  const buttons = picker.locator('[aria-label="3週間の空き状況"] > button');
  for (const index of [0, 1, 4]) await buttons.nth(index).click();
  await expect(buttons.nth(5)).toBeDisabled();
  await expect(buttons.nth(2)).toBeDisabled();
  await expect(buttons.nth(3)).toBeDisabled();
  if (process.env.PLAYWRIGHT_SCREENSHOTS === "1") await picker.screenshot({ style: screenshotStyle, path: path.join(screenshotDir, "preferred-dates.png") });
  await page.getByRole("button", { name: "100m", exact: true }).click();
  for (const checkbox of await page.locator('form input[type="checkbox"]').all()) await checkbox.check();
  const submitted = page.waitForRequest((request) => request.method() === "POST" && request.url().endsWith("/coaching/contracts"));
  await page.getByRole("button", { name: "申し込む", exact: true }).click();
  expect((await submitted).postDataJSON().preferred_dates).toEqual([days[0].date, days[1].date, days[4].date]);
  await expect(picker.getByText("○か△の日を選べます。0/3日選択中です。")).toBeVisible();
  await expect(page.getByText("空き状況が変わりました。選び直してください")).toBeVisible();
});

test("公開プロフィールのカレンダーは動画添削だけのコーチに出さない", async ({ page }) => {
  await page.goto(`/coaches/${coachID}`);
  await expect(page.getByRole("region", { name: "コーチの空き状況" })).toBeVisible();
  await page.route("**/api/v1/coaches", (route) => route.fulfill({ json: { coaches: [{ ...coach, service_types: ["video_review"], plans: [{ ...plan, service_type: "video_review" }] }] } }));
  await page.reload();
  await expect(page.getByRole("heading", { name: "サンプルコーチ" })).toBeVisible();
  await expect(page.getByRole("region", { name: "コーチの空き状況" })).toHaveCount(0);
});
