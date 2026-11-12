import { expect, test, type Page } from "@playwright/test";
import path from "node:path";

// コーチ向けメニューの11タイルを大きくし、画面にちょうど収める（本体 #580）。
// NEXT_PUBLIC_COACHING_ENABLED=true のプレビューで実行する（起動手順は docs/coach-availability.md と同じ）。データと認証はテスト用。
// #563 の高さ合わせ（JS で正方形の一辺を実測）は廃止し、タイルは横幅駆動（3列 1fr＋正方形）にする。
// 縦の余りは行ギャップ拡大（最大 +8px）→上下パディング拡大（最大 28px）→中央寄せの順で吸収し、
// 高さ不足の画面ではタイルを縮めずページ全体でスクロールする。
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

// Issue #580 の目標レイアウトから求まるタイル一辺。式の結果であり、個別の固定値で書かない。
function expectedTile(viewportWidth: number) {
  const pad = Math.min(24, Math.max(16, viewportWidth * 0.045));
  const gap = Math.min(16, Math.max(12, viewportWidth * 0.032));
  return { pad, gap, tile: (viewportWidth - pad * 2 - gap * 2) / 3 };
}

async function measure(page: Page) {
  return page.evaluate(() => {
    const rect = (el: Element | null) => el!.getBoundingClientRect();
    const wrapper = document.querySelector(".coach-tiles") as HTMLElement;
    const nav = document.querySelector(".coach-tiles > nav") as HTMLElement;
    const grid = rect(wrapper);
    const navBox = rect(nav);
    const footer = rect(document.querySelector(".coach-launcher > footer"));
    const bottomNav = [...document.querySelectorAll("nav")].find((n) => getComputedStyle(n).position === "fixed");
    const tiles = [...document.querySelectorAll("nav.grid a")];
    const tileRects = tiles.map((a) => a.getBoundingClientRect());
    const icons = tiles.map((a) => {
      const svg = a.querySelector("svg");
      if (!svg) return null;
      const b = svg.getBoundingClientRect();
      return { w: b.width, h: b.height };
    });
    const labels = tiles.map((a) => {
      const s = a.querySelector(":scope > span:not([aria-hidden])") as HTMLElement | null;
      if (!s) return null;
      const cs = getComputedStyle(s);
      return {
        whiteSpace: cs.whiteSpace, fontSize: cs.fontSize, textOverflow: cs.textOverflow,
        scrollW: s.scrollWidth, clientW: s.clientWidth, scrollH: s.scrollHeight, clientH: s.clientHeight,
      };
    });
    const badges = tiles.map((a) => {
      const b = a.querySelector(":scope > span[aria-hidden]") as HTMLElement | null;
      if (!b) return null;
      const r = b.getBoundingClientRect();
      const cs = getComputedStyle(b);
      const shown = cs.display !== "none" && cs.visibility !== "hidden" && r.width > 0 && r.height > 0;
      let clipped = false;
      let p = b.parentElement;
      while (p && p !== document.body) {
        if (getComputedStyle(p).overflow !== "visible") {
          const pr = p.getBoundingClientRect();
          if (r.top < pr.top - 0.5 || r.bottom > pr.bottom + 0.5 || r.left < pr.left - 0.5 || r.right > pr.right + 0.5) clipped = true;
        }
        p = p.parentElement;
      }
      return { shown, clipped, w: r.width, h: r.height, top: r.top, tileTop: a.getBoundingClientRect().top };
    });
    return {
      gridTop: grid.top, gridBottom: grid.bottom, gridLeft: grid.left, gridWidth: grid.width,
      navTop: navBox.top, navBottom: navBox.bottom,
      footerTop: footer.top, footerBottom: footer.bottom,
      bottomNavTop: bottomNav ? bottomNav.getBoundingClientRect().top : window.innerHeight,
      tileWidths: tileRects.map((t) => t.width),
      tileHeights: tileRects.map((t) => t.height),
      tileLefts: tileRects.map((t) => t.left),
      tileTops: tileRects.map((t) => t.top),
      rowTops: [0, 3, 6, 9].map((i) => tileRects[i].top), lastRowBottom: tileRects[9].bottom,
      labels, badges, icons,
      scrollHeight: document.documentElement.scrollHeight, innerHeight: window.innerHeight,
      bodyScrollWidth: document.body.scrollWidth, innerWidth: window.innerWidth,
    };
  });
}

function expectTiles(page: { width: number }, m: Awaited<ReturnType<typeof measure>>, withBadgeOnOffers: boolean, strictLabels = true) {
  const exp = expectedTile(page.width);
  // タイルは正方形で、想定値と ±2px 以内で一致する。
  for (const w of m.tileWidths) expect(Math.abs(w - exp.tile)).toBeLessThanOrEqual(2);
  for (let i = 0; i < m.tileWidths.length; i++) expect(Math.abs(m.tileWidths[i] - m.tileHeights[i])).toBeLessThanOrEqual(1);
  // 3列のまま。最終行の2つは左寄せ（1行目と同じ左端、中央寄せや引き伸ばしをしない）。
  expect(m.tileTops[1]).toBeCloseTo(m.tileTops[0], 0);
  expect(m.tileTops[2]).toBeCloseTo(m.tileTops[0], 0);
  expect(Math.abs(m.tileLefts[9] - m.tileLefts[0])).toBeLessThanOrEqual(1);
  expect(Math.abs(m.tileLefts[10] - m.tileLefts[1])).toBeLessThanOrEqual(1);
  // 横ギャップは基準値どおり（縦の吸収で横は広げない）。
  expect(Math.abs(m.tileLefts[1] - m.tileLefts[0] - m.tileWidths[0] - exp.gap)).toBeLessThanOrEqual(2);
  // 行ギャップは基準〜最大 +8px の範囲。
  for (let i = 1; i < 4; i++) {
    const rowGap = m.rowTops[i] - m.rowTops[i - 1] - m.tileHeights[0];
    expect(rowGap).toBeGreaterThanOrEqual(exp.gap - 1);
    expect(rowGap).toBeLessThanOrEqual(exp.gap + 8 + 1);
  }
  // 全ラベルが1行に収まり、折り返し・はみ出しがない。
  // 英語の長いラベルは ellipsis で見た目にはみ出さない設計（折り返しはしない）。
  expect(m.labels).toHaveLength(11);
  for (const label of m.labels) {
    expect(label).not.toBeNull();
    expect(label!.whiteSpace).toBe("nowrap");
    if (strictLabels) {
      expect(label!.scrollW).toBeLessThanOrEqual(label!.clientW + 1);
      expect(label!.scrollH).toBeLessThanOrEqual(label!.clientH + 2);
    } else {
      const fits = label!.scrollW <= label!.clientW + 1;
      const ellipsized = label!.textOverflow === "ellipsis" && label!.scrollH <= label!.clientH + 2;
      expect(fits || ellipsized).toBe(true);
    }
  }
  // バッジは切れていない。固定の5つ（添削中・追加相談・契約申込みの件数、特商法・報酬口座の点）に、
  // 期限アラートありの時は届いた依頼の件数が加わる（モックの件数どおり）。
  const badged = m.badges.filter((b) => b !== null);
  if (withBadgeOnOffers) expect(m.badges[0]).not.toBeNull();
  else expect(m.badges[0]).toBeNull();
  expect(m.badges[1]).not.toBeNull();
  expect(badged.length).toBe(withBadgeOnOffers ? 6 : 5);
  for (let i = 0; i < m.badges.length; i++) {
    const badge = m.badges[i];
    if (badge === null) continue;
    expect(badge.shown).toBe(true);
    expect(badge.clipped).toBe(false);
    // 件数バッジ（届いた依頼・添削中・追加相談・契約申込み）は h-5、注意の点（特商法・報酬口座）は size-3。
    const expectedH = i === 7 || i === 8 ? 12 : 20;
    expect(Math.abs(badge.h - expectedH)).toBeLessThanOrEqual(1);
    expect(badge.top).toBeGreaterThanOrEqual(m.navTop - 1);
  }
  // 縦の余りは下に固まらない（上下パディング均等＋中央寄せ）。
  const above = m.rowTops[0] - m.navTop;
  const below = m.navBottom - m.lastRowBottom;
  expect(above).toBeGreaterThanOrEqual(7);
  expect(Math.abs(above - below)).toBeLessThanOrEqual(3);
  // 横はみ出しはない。
  expect(m.bodyScrollWidth).toBeLessThanOrEqual(page.width + 1);
}

async function expectFooterReachable(page: Page) {
  await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
  const end = await measure(page);
  expect(end.footerBottom).toBeLessThanOrEqual(end.bottomNavTop + 1);
  return end;
}

for (const viewport of [{ width: 360, height: 740 }, { width: 390, height: 844 }, { width: 430, height: 932 }]) {
  test(`${viewport.width}px 幅で11タイルが大きく正方形に並び、ラベル・バッジ・縦バランスが保たれる（#580）`, async ({ page }) => {
    await page.setViewportSize(viewport);
    await setup(page, false);
    const m = await measure(page);
    expectTiles(viewport, m, false);
    await expectFooterReachable(page);
    if (process.env.PLAYWRIGHT_SCREENSHOTS === "1") {
      await page.setViewportSize(viewport);
      await page.evaluate(() => window.scrollTo(0, 0));
      await page.screenshot({ path: path.join(screenshotDir, `tile-${viewport.width}.png`) });
    }
  });
}

test("390×844 で期限アラートがあってもタイルの大きさは変わらず、バッジが切れない（#580）", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await setup(page, true);
  const m = await measure(page);
  expectTiles({ width: 390 }, m, true);
  // アラートなしの時と同じ大きさ（横幅駆動なので高さの影響を受けない）。
  const plainPage = await page.context().newPage();
  await plainPage.setViewportSize({ width: 390, height: 844 });
  await setup(plainPage, false);
  const plain = await measure(plainPage);
  expect(Math.abs(m.tileWidths[0] - plain.tileWidths[0])).toBeLessThanOrEqual(1);
  await expectFooterReachable(page);
  if (process.env.PLAYWRIGHT_SCREENSHOTS === "1") {
    await page.evaluate(() => window.scrollTo(0, 0));
    await page.screenshot({ path: path.join(screenshotDir, "tile-390-alert.png") });
  }
});

test("低い画面（390×560）ではタイルを縮めず、収まらない分はページが縦にスクロールする（#580）", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 560 });
  await setup(page, true);
  const m = await measure(page);
  const exp = expectedTile(390);
  expect(Math.abs(m.tileWidths[0] - exp.tile)).toBeLessThanOrEqual(2);
  for (let i = 0; i < m.tileWidths.length; i++) expect(Math.abs(m.tileWidths[i] - m.tileHeights[i])).toBeLessThanOrEqual(1);
  expect(m.scrollHeight).toBeGreaterThan(m.innerHeight);
  await expectFooterReachable(page);
  if (process.env.PLAYWRIGHT_SCREENSHOTS === "1") {
    await page.evaluate(() => window.scrollTo(0, 0));
    await page.screenshot({ path: path.join(screenshotDir, "tile-390-short.png") });
  }
});

test("PC 幅では広げず、3列の正方形のタイルを上から並べる", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await setup(page, false);
  const m = await measure(page);
  const firstRowTops = m.tileTops.slice(0, 3);
  for (const top of firstRowTops) expect(Math.abs(top - firstRowTops[0])).toBeLessThanOrEqual(1);
  for (let i = 0; i < m.tileWidths.length; i++) expect(Math.abs(m.tileWidths[i] - m.tileHeights[i])).toBeLessThanOrEqual(1);
  expect(m.rowTops[0] - m.navTop).toBeLessThanOrEqual(1);
});

// 受付トグルの保存エラー（赤文字）が出てもタイルの大きさは変わらない（#580 追補）。
// 再現：プッシュ購読なしで受付を ON にし、API が 422 を返してヘッドカード内のエラー欄に実物と同じ文言が出て、
// トグルが受付停止中に戻った状態を作る。テスト専用の別表示は作らない。
// API の実物と一字一句同じにする（apps/api/internal/httpx/review.go の 422 文言）。違うと英語辞書に当たらない。
const pushErrorJa = "依頼は通知で届くので、先にこの端末で通知を受け取れるようにしてください";
const pushErrorEn = "Requests arrive as notifications, so turn on notifications on this device first";

async function setupSaveToggle(page: Page, saveOk: boolean, locale: "ja" | "en") {
  const context = page.context();
  const baseURL = test.info().project.use.baseURL ?? "http://localhost:3000";
  const user = { id: userID, aud: "authenticated", role: "authenticated", email: "preview@example.com", app_metadata: {}, user_metadata: {}, identities: [], created_at: new Date().toISOString() };
  const payload = { sub: userID, aud: "authenticated", role: "authenticated", aal: "aal1", exp: Math.floor(Date.now() / 1000) + 3600, iat: Math.floor(Date.now() / 1000) };
  const accessToken = `${Buffer.from('{"alg":"HS256","typ":"JWT"}').toString("base64url")}.${Buffer.from(JSON.stringify(payload)).toString("base64url")}.preview`;
  const session = { access_token: accessToken, token_type: "bearer", refresh_token: "preview", expires_in: 3600, expires_at: payload.exp, user };
  await context.addCookies([{ name: "sb-127-auth-token", value: `base64-${Buffer.from(JSON.stringify(session)).toString("base64url")}`, url: baseURL }]);
  // 英語表示はアカウントの言語設定（profiles.locale）の実物経路で再現する。
  const stopped = { ...application, on_duty: false };
  await page.route("**/auth/v1/**", (route) => route.fulfill({ json: user }));
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    if (request.method() === "PATCH" && pathname.endsWith("/coach/me")) {
      if (saveOk) {
        await route.fulfill({ json: { application: { ...stopped, on_duty: true } } });
      } else {
        // API の実物と同じ 422・同じ文言（apps/api/internal/httpx/review.go）。
        await route.fulfill({ status: 422, json: { error: { code: "validation_error", message: pushErrorJa } } });
      }
      return;
    }
    let body: unknown = {};
    if (pathname.endsWith("/coach/me")) body = { application: stopped, rating: { count: 3, average: 4.5, paused: false } };
    else if (pathname.endsWith("/me")) body = { id: userID, nickname: "サンプル選手", event: "100m", mfa_required: false, ...(locale === "en" ? { locale: "en" } : null) };
    else if (pathname.endsWith("/coach/requests")) {
      body = { offers: [], reviewing: [reviewRequest("r1")] };
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
  await expect(page.locator(".screen-loader")).toBeHidden();
  const switchName = locale === "en" ? "Accepting" : "受付中";
  await page.getByRole("switch", { name: switchName }).click();
  if (saveOk) {
    await expect(page.getByRole("switch", { name: switchName })).toHaveAttribute("aria-checked", "true");
  } else {
    // 赤文字がヘッドカード内のエラー欄に実物と同じ文言で出て、トグルは受付停止中に戻る。
    await expect(page.getByText(locale === "en" ? pushErrorEn : pushErrorJa)).toBeVisible();
    await expect(page.getByRole("switch", { name: switchName })).toHaveAttribute("aria-checked", "false");
  }
  await measureStable(page);
}

// トグル操作後のレイアウトが落ち着くまで待つ（赤文字の出現・再描画の非同期反映を拾うため）。
async function measureStable(page: Page) {
  let prev = await measure(page);
  await expect.poll(async () => {
    const m = await measure(page);
    const settled =
      m.tileWidths.every((w, i) => Math.abs(w - prev.tileWidths[i]) < 0.05) &&
      m.tileHeights.every((h, i) => Math.abs(h - prev.tileHeights[i]) < 0.05);
    prev = m;
    return settled;
  }, { timeout: 5000 }).toBe(true);
  return prev;
}

function expectSameTiles(base: Awaited<ReturnType<typeof measure>>, err: Awaited<ReturnType<typeof measure>>) {
  // タイル・アイコン・ラベルの大きさはエラーあり／なしで変わらない。
  for (let i = 0; i < 11; i++) {
    expect(Math.abs(err.tileWidths[i] - base.tileWidths[i])).toBeLessThanOrEqual(1);
    expect(Math.abs(err.tileHeights[i] - base.tileHeights[i])).toBeLessThanOrEqual(1);
    expect(err.labels[i]!.fontSize).toBe(base.labels[i]!.fontSize);
    expect(Math.abs(err.icons[i]!.w - base.icons[i]!.w)).toBeLessThanOrEqual(0.5);
    expect(Math.abs(err.icons[i]!.h - base.icons[i]!.h)).toBeLessThanOrEqual(0.5);
  }
}

for (const viewport of [{ width: 360, height: 740 }, { width: 390, height: 844 }, { width: 430, height: 932 }]) {
  test(`保存エラー（赤文字）が出てもタイルの大きさは変わらない（${viewport.width}px・日本語・#580追補）`, async ({ page }) => {
    await page.setViewportSize(viewport);
    await setupSaveToggle(page, true, "ja");
    const base = await measure(page);
    const errPage = await page.context().newPage();
    await errPage.setViewportSize(viewport);
    await setupSaveToggle(errPage, false, "ja");
    const err = await measure(errPage);
    expectSameTiles(base, err);
    expectTiles(viewport, err, false);
    await expectFooterReachable(errPage);
    if (process.env.PLAYWRIGHT_SCREENSHOTS === "1") {
      await errPage.evaluate(() => window.scrollTo(0, 0));
      await errPage.screenshot({ path: path.join(screenshotDir, `tile-${viewport.width}-error.png`) });
    }
  });

  test(`保存エラー時も英語表示でタイルの大きさは変わらない（${viewport.width}px・#580追補）`, async ({ page }) => {
    await page.setViewportSize(viewport);
    await setupSaveToggle(page, true, "en");
    const base = await measure(page);
    const errPage = await page.context().newPage();
    await errPage.setViewportSize(viewport);
    await setupSaveToggle(errPage, false, "en");
    const err = await measure(errPage);
    expectSameTiles(base, err);
    // 英語の長いラベルは ellipsis で見た目にはみ出さない設計。折り返しはしない。
    expectTiles(viewport, err, false, false);
    await expectFooterReachable(errPage);
    if (process.env.PLAYWRIGHT_SCREENSHOTS === "1") {
      await errPage.evaluate(() => window.scrollTo(0, 0));
      await errPage.screenshot({ path: path.join(screenshotDir, `tile-${viewport.width}-error-en.png`) });
    }
  });
}
