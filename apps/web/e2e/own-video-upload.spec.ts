import { readFileSync } from "node:fs";
import { join } from "node:path";
import { expect, test, type BrowserContext, type Page } from "@playwright/test";

// 「自分の動画」: 選んだらすぐ送り始め、送っている間も詳細を書け、投稿ボタンは保存だけになっていること。
// 署名 API・Cloudinary・投稿 API はこのテストの中で差し替える（送信には uploadDelayMs かかる想定）。
const userID = "11111111-1111-4111-8111-111111111111";
const uploadDelayMs = 3000;
const video = readFileSync(join(process.cwd(), "public/samples/vertical.mp4"));

async function signIn(context: BrowserContext, page: Page, baseURL: string) {
  const user = { id: userID, aud: "authenticated", role: "authenticated", email: "preview@example.com", app_metadata: {}, user_metadata: {}, identities: [], created_at: new Date().toISOString() };
  const exp = Math.floor(Date.now() / 1000) + 3600;
  const payload = { sub: userID, aud: "authenticated", role: "authenticated", aal: "aal1", exp, iat: exp - 3600 };
  const access_token = `${Buffer.from('{"alg":"HS256","typ":"JWT"}').toString("base64url")}.${Buffer.from(JSON.stringify(payload)).toString("base64url")}.preview`;
  const session = { access_token, token_type: "bearer", refresh_token: "preview", expires_in: 3600, expires_at: exp, user };
  await context.addCookies([{ name: "sb-127-auth-token", value: `base64-${Buffer.from(JSON.stringify(session)).toString("base64url")}`, url: baseURL }]);
  await page.route("**/auth/v1/**", (route) => route.fulfill({ json: user }));
}

test("動画を選んだらすぐ送り始め、送信中も詳細を書け、選び直すと前の送信を止める", async ({ page, context, baseURL }) => {
  test.skip(!process.env.CI && process.env.PLAYWRIGHT_OWN_VIDEO !== "1", "auth-stub と一緒に実行する");
  await page.setViewportSize({ width: 390, height: 844 });
  await signIn(context, page, baseURL!);

  let signs = 0;
  let uploadsStarted = 0;
  const posted: unknown[] = [];
  await page.route(/\/v1\/(uploads\/video|posts|me|notifications)(\?|$)/, async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith("/uploads/video")) {
      signs++;
      return route.fulfill({ json: { cloud_name: "demo", api_key: "k", timestamp: Math.floor(Date.now() / 1000), public_id: `tracktalk/videos/v${signs}`, signature: "s", upload_preset: "p", allowed_formats: "mp4,mov,webm", resource_type: "video" } });
    }
    if (path.endsWith("/posts")) {
      posted.push(route.request().postDataJSON());
      return route.fulfill({ json: { id: "p1" } });
    }
    if (path.endsWith("/me")) return route.fulfill({ json: { id: userID, nickname: "テスト選手", event: "100m", mfa_required: false, overseas_consent: true } });
    return route.fulfill({ json: { notifications: [], unread: 0 } });
  });
  await page.route("https://api.cloudinary.com/**", async (route) => {
    const n = ++uploadsStarted;
    await new Promise((resolve) => setTimeout(resolve, uploadDelayMs));
    // 選び直しで止められた送信は、ブラウザ側で既に打ち切られている。
    await route.fulfill({ json: { public_id: `tracktalk/videos/v${n}` } }).catch(() => {});
  });

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
  expect(uploadsStarted).toBe(2);
  expect(signs).toBe(2);

  const started = Date.now();
  await button.click();
  await expect(page).toHaveURL(/\/\?sort=new$/);
  const saveMs = Date.now() - started;
  test.info().annotations.push({ type: "保存ボタンを押してからホームまで", description: `${saveMs}ms` });
  expect(posted).toEqual([{ body: "スタートの確認", kind: "video", video_public_id: "tracktalk/videos/v2" }]);
});
