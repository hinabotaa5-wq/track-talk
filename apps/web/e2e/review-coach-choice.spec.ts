import { expect, test } from "@playwright/test";

test.skip(process.env.PLAYWRIGHT_COACHING_ENABLED !== "1", "コーチングを有効にしたローカルプレビューで実行します");
const userID="11111111-1111-4111-8111-111111111111";
test("スマホの申込み画面でコーチを選び、料金は1,500円のまま",async({page,context,baseURL})=>{
 await page.setViewportSize({width:390,height:844});
 const user={id:userID,aud:"authenticated",role:"authenticated",email:"preview@example.com",app_metadata:{},user_metadata:{},identities:[],created_at:new Date().toISOString()};
 const payload={sub:userID,aud:"authenticated",role:"authenticated",aal:"aal1",exp:Math.floor(Date.now()/1000)+3600,iat:Math.floor(Date.now()/1000)};
 const access_token=`${Buffer.from('{"alg":"HS256","typ":"JWT"}').toString("base64url")}.${Buffer.from(JSON.stringify(payload)).toString("base64url")}.preview`;
 const session={access_token,token_type:"bearer",refresh_token:"preview",expires_in:3600,expires_at:payload.exp,user};
 await context.addCookies([{name:"sb-127-auth-token",value:`base64-${Buffer.from(JSON.stringify(session)).toString("base64url")}`,url:baseURL!}]);
 await page.route("**/auth/v1/**",route=>route.fulfill({json:user}));
 await page.route("**/api/v1/**",async route=>{
  const p=new URL(route.request().url()).pathname;
  let body:unknown={};
  if(p.endsWith("/me"))body={id:userID,nickname:"テスト選手",event:"100m",mfa_required:false,overseas_consent:true};
  else if(p.endsWith("/me/coaching"))body={demo:true,birth_month:"1995-01",is_minor:false,guardian:null,push_devices:0};
  else if(p.endsWith("/tickets"))body={balance:{available:0,reserved:0},products:[],purchases:[]};
  else if(p.endsWith("/estimate"))body={due_at:new Date(Date.now()+86400000).toISOString(),coaches_on_duty:true,payments:true,price_yen:1500};
  else if(p.endsWith("/nominees"))body={price_yen:1500,coaches:[{id:"coach-1",nickname:"サンプルコーチ",photo_url:"",events:["100m"],years:10,best_record:"100m 10秒台",achievements:"短距離のフォームと練習方法をアドバイスします。"},{id:"coach-2",nickname:"別のコーチ",photo_url:"",events:["100m"],years:5,best_record:"",achievements:""}]};
  else if(p.endsWith("/notifications"))body={notifications:[],unread:0};
  await route.fulfill({json:body});
 });
 await page.goto("/coaching/new");
 await expect(page.getByRole("heading",{name:"動画添削を頼む"})).toBeVisible();
 await expect(page.getByText("おまかせ")).toHaveCount(0);
 await expect(page.getByText("1,500円／回",{exact:true})).toBeVisible();
 await page.getByLabel("サンプルコーチ").check();
 await expect(page.getByText("1,500円／回",{exact:true})).toBeVisible();
 await expect(page.locator("body")).toHaveJSProperty("scrollWidth",390);
 await expect(page.locator("#startup-splash")).toHaveCount(0);
 await expect(page.locator(".screen-loader")).toBeHidden();
 await page.screenshot({path:"../../docs/screenshots/review-coach-choice-mobile.png",fullPage:true});
 expect(new URL(page.url()).pathname).toBe("/coaching/new");
});
