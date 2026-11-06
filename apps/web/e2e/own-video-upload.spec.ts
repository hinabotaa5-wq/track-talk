import { readFileSync } from "node:fs";
import { join } from "node:path";
import { expect, test, type BrowserContext, type Page } from "@playwright/test";

// 「自分の動画」: 選んだらすぐ送り始め、送っている間も詳細を書け、投稿ボタンは保存だけになっていること。
// 署名 API・Cloudinary・投稿 API はこのテストの中で差し替える（送信には uploadDelayMs かかる想定）。
const userID = "11111111-1111-4111-8111-111111111111";
const uploadDelayMs = 3000;
const video = readFileSync(join(process.cwd(), "public/samples/vertical.mp4"));
// 投稿の request_key（#382）。フォームごとの UUID なので形だけ確かめる。
const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

async function signIn(context: BrowserContext, page: Page, baseURL: string) {
  const user = { id: userID, aud: "authenticated", role: "authenticated", email: "preview@example.com", app_metadata: {}, user_metadata: {}, identities: [], created_at: new Date().toISOString() };
  const exp = Math.floor(Date.now() / 1000) + 3600;
  const payload = { sub: userID, aud: "authenticated", role: "authenticated", aal: "aal1", exp, iat: exp - 3600 };
  const access_token = `${Buffer.from('{"alg":"HS256","typ":"JWT"}').toString("base64url")}.${Buffer.from(JSON.stringify(payload)).toString("base64url")}.preview`;
  const session = { access_token, token_type: "bearer", refresh_token: "preview", expires_in: 3600, expires_at: exp, user };
  await context.addCookies([{ name: "sb-127-auth-token", value: `base64-${Buffer.from(JSON.stringify(session)).toString("base64url")}`, url: baseURL }]);
  await page.route("**/auth/v1/**", (route) => route.fulfill({ json: user }));
}

type Mocks = { signs: () => number; uploads: string[]; posted: unknown[] };

// 署名 API・投稿 API・Cloudinary を差し替える。Cloudinary は送られてきた public_id をそのまま返す。
async function mockApis(page: Page, { signDelayMs = 0 } = {}): Promise<Mocks> {
  let signs = 0;
  const uploads: string[] = [];
  const posted: unknown[] = [];
  await page.route(/\/v1\/(uploads\/video|posts|me|notifications)(\?|$)/, async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith("/uploads/video")) {
      const n = ++signs;
      if (signDelayMs) await new Promise((resolve) => setTimeout(resolve, signDelayMs));
      return route.fulfill({ json: { cloud_name: "demo", api_key: "k", timestamp: Math.floor(Date.now() / 1000), public_id: `tracktalk/videos/v${n}`, signature: "s", upload_preset: "p", allowed_formats: "mp4,mov,webm", resource_type: "video" } });
    }
    if (path.endsWith("/posts")) {
      // 投稿後に開くホームの一覧取得（GET）は数えない。
      if (route.request().method() === "POST") posted.push(route.request().postDataJSON());
      return route.fulfill({ json: { id: "p1" } });
    }
    if (path.endsWith("/me")) return route.fulfill({ json: { id: userID, nickname: "テスト選手", event: "100m", mfa_required: false, overseas_consent: true } });
    return route.fulfill({ json: { notifications: [], unread: 0 } });
  });
  await page.route("https://api.cloudinary.com/**", async (route) => {
    const publicID = /name="public_id"\r\n\r\n([^\r]+)/.exec(route.request().postDataBuffer()?.toString("latin1") ?? "")?.[1] ?? "";
    uploads.push(publicID);
    await new Promise((resolve) => setTimeout(resolve, uploadDelayMs));
    // 選び直しで止められた送信は、ブラウザ側で既に打ち切られている。
    await route.fulfill({ json: { public_id: publicID } }).catch(() => {});
  });
  return { signs: () => signs, uploads, posted };
}

test("動画を選んだらすぐ送り始め、送信中も詳細を書け、選び直すと前の送信を止める", async ({ page, context, baseURL }) => {
  test.skip(!process.env.CI && process.env.PLAYWRIGHT_OWN_VIDEO !== "1", "auth-stub と一緒に実行する");
  await page.setViewportSize({ width: 390, height: 844 });
  await signIn(context, page, baseURL!);

  const mocks = await mockApis(page);

  await page.goto("/posts/video");
  const button = page.getByRole("button", { name: /動画を選んでください|動画を送っています|投稿する/ });
  await expect(button).toHaveText("動画を選んでください");
  await expect(button).toBeDisabled();
  await expect(page.getByText("iPhoneの準備で少し時間がかかることがあります")).toBeVisible();

  const input = page.locator('input[type="file"]');
  await input.setInputFiles({ name: "first.mp4", mimeType: "video/mp4", buffer: video });
  await expect(button).toHaveText(/動画を送っています \d+%/);
  await expect(button).toBeDisabled();

  // 選び直すと、前の送信を止めて新しい動画で送り直す。
  await input.setInputFiles({ name: "second.mp4", mimeType: "video/mp4", buffer: video });
  await expect(page.getByText("second.mp4")).toBeVisible();

  // 送っている間も詳細は書ける。
  const details = page.getByLabel("詳細");
  await details.fill("スタートの確認");
  await expect(details).toBeEditable();
  await expect(button).toHaveText(/動画を送っています/);

  await expect(button).toHaveText("投稿する", { timeout: uploadDelayMs + 5000 });
  await expect(page.getByText("送信済み")).toBeVisible();
  // 1本ずつ選んだときは、選ぶたびに署名を取る（同時に取るときだけ1回にまとまる）。
  expect(mocks.uploads).toEqual(["tracktalk/videos/v1", "tracktalk/videos/v2"]);
  expect(mocks.signs()).toBe(2);

  const started = Date.now();
  await button.click();
  await expect(page).toHaveURL(/\/\?sort=new$/);
  const saveMs = Date.now() - started;
  test.info().annotations.push({ type: "保存ボタンを押してからホームまで", description: `${saveMs}ms` });
  expect(mocks.posted).toEqual([{ body: "スタートの確認", kind: "video", video_public_id: "tracktalk/videos/v2", request_key: expect.stringMatching(uuidPattern) }]);
});

test("署名の取得中にもう一度求めても、取得は1回にまとめ、同じ署名を2本で使わない", async ({ page, context, baseURL }) => {
  test.skip(!process.env.CI && process.env.PLAYWRIGHT_OWN_VIDEO !== "1", "auth-stub と一緒に実行する");
  await page.setViewportSize({ width: 390, height: 844 });
  await signIn(context, page, baseURL!);
  const mocks = await mockApis(page, { signDelayMs: 1500 });
  await page.goto("/posts/video");
  const button = page.getByRole("button", { name: /動画を選んでください|動画を送っています|投稿する/ });
  await expect(button).toHaveText("動画を選んでください");
  const input = page.locator('input[type="file"]');

  // 1. タップで先読みを始め、その取得中に動画を選ぶ → 先読みを引き取って送る（取得は1回）。
  const chooser = page.waitForEvent("filechooser");
  await input.click();
  await (await chooser).setFiles({ name: "first.mp4", mimeType: "video/mp4", buffer: video });
  await expect.poll(() => mocks.uploads.length).toBe(1);
  expect(mocks.signs()).toBe(1);
  expect(mocks.uploads).toEqual(["tracktalk/videos/v1"]);

  // 2. 2本目の署名の取得中に3本目を選び直す → 止まった2本目の取得を引き継ぐ（取得は1回）。
  await input.setInputFiles({ name: "second.mp4", mimeType: "video/mp4", buffer: video });
  await expect.poll(() => mocks.signs()).toBe(2);
  await input.setInputFiles({ name: "third.mp4", mimeType: "video/mp4", buffer: video });
  await expect.poll(() => mocks.uploads.length).toBe(2);
  expect(mocks.signs()).toBe(2);
  expect(mocks.uploads).toEqual(["tracktalk/videos/v1", "tracktalk/videos/v2"]);

  await expect(button).toHaveText("投稿する", { timeout: uploadDelayMs + 5000 });
  await button.click();
  await expect(page).toHaveURL(/\/\?sort=new$/);
  expect(mocks.posted).toEqual([{ body: "", kind: "video", video_public_id: "tracktalk/videos/v2", request_key: expect.stringMatching(uuidPattern) }]);
  expect(mocks.signs()).toBe(2);
});
