import { expect, test } from "@playwright/test";

test.skip(process.env.PLAYWRIGHT_COACHING_ENABLED !== "1", "合成セッションのローカルプレビューで実行");
const userID = "11111111-1111-4111-8111-111111111111";
for (const age of ["unregistered", "adult", "minor"]) {
 test(`${age}: 生年月による準備画面を経ずに購入前チェックを表示する`, async ({ page, context, baseURL }) => {
  const user = { id: userID, aud: "authenticated", role: "authenticated", email: "preview@example.com", app_metadata: {}, user_metadata: {}, identities: [], created_at: new Date().toISOString() };
  const payload = { sub: userID, aud: "authenticated", role: "authenticated", aal: "aal1", exp: Math.floor(Date.now()/1000)+3600, iat: Math.floor(Date.now()/1000) };
  const access_token = `${Buffer.from('{"alg":"HS256","typ":"JWT"}').toString("base64url")}.${Buffer.from(JSON.stringify(payload)).toString("base64url")}.preview`;
  const session = { access_token, token_type: "bearer", refresh_token: "preview", expires_in: 3600, expires_at: payload.exp, user };
  await context.addCookies([{ name: "sb-127-auth-token", value: `base64-${Buffer.from(JSON.stringify(session)).toString("base64url")}`, url: baseURL! }]);
  await page.route("**/auth/v1/**", route => route.fulfill({ json: user }));
  await page.route("**/api/v1/**", async route => {
   const p = new URL(route.request().url()).pathname;
   let body: unknown = {};
   if (p.endsWith("/me")) body = { id:userID,nickname:"テスト選手",event:"100m",mfa_required:false,overseas_consent:true };
   else if (p.endsWith("/me/coaching")) body = { demo:true,review_open:true,birth_month:age==="unregistered"?null:age==="adult"?"1995-01":"2012-01",is_minor:age==="unregistered"?null:age==="minor",guardian:null,push_devices:0 };
   else if (p.endsWith("/tickets")) body = { balance:{available:0,reserved:0},products:[{id:"six",quantity:6,amount_yen:2500}],purchases:[],ledger:[],can_purchase:true,sales_open:true,single_price_yen:500,review_units:1,validity_months:6 };
   else if (p.endsWith("/estimate")) body = { due_at:new Date(Date.now()+86400000).toISOString(),coaches_on_duty:true,payments:true,price_yen:500 };
   else if (p.endsWith("/nominees")) body = { price_yen:500,coaches:[] };
   else if (p.endsWith("/notifications")) body = { notifications:[],unread:0 };
   await route.fulfill({json:body});
  });
  await page.goto("/coaching/new");
  await expect(page.getByRole("heading",{name:"動画添削を頼む"})).toBeVisible();
  const checkbox = page.getByRole("checkbox",{name:"未成年の方は保護者に許可を取ってください"});
  await expect(checkbox).toBeVisible();
  await expect(checkbox).not.toBeChecked();
  await expect(page.getByText("生まれた年月を登録する",{exact:true})).toHaveCount(0);
  await checkbox.check();
  await expect(checkbox).toBeChecked();
  await page.reload();
  await expect(checkbox).not.toBeChecked();
  await page.goto("/me/coaching/tickets");
  await page.getByRole("button",{name:"購入内容を確認する",exact:true}).click();
  await expect(checkbox).toBeVisible();
  await expect(page.getByRole("button",{name:/確定してお支払いへ/})).toBeDisabled();
  await checkbox.check();
  await expect(page.getByRole("button",{name:/確定してお支払いへ/})).toBeEnabled();
 });
}
