import { expect, test } from "@playwright/test";

test("login then publish a text post", async ({ page }) => {
  test.skip(!process.env.CI, "CI の API と一緒に実行する");
  test.setTimeout(90_000);

  await page.goto("/login?next=/posts/new");
  // 未ログイン時の言語選択プロンプトは米国からのアクセス時だけ出る（本体 #565）。国の判定がない CI では出ない。
  await page.getByPlaceholder("メール").fill("runner@example.com");
  await page.getByPlaceholder("パスワード").fill("password1");
  await page.getByRole("button", { name: "ログイン", exact: true }).click();
  // 登録画面を通らずにログインした人も、規約などと海外への提供に1回で同意する。
  await expect(page.getByRole("heading", { name: "利用にあたっての同意" })).toBeVisible({ timeout: 20_000 });
  await expect(page.getByRole("checkbox")).toHaveCount(1);
  await expect(page.getByRole("checkbox")).toHaveAccessibleName("利用規約・コミュニティガイドライン・プライバシーポリシー（海外への個人情報提供を含む）に同意する");
  await expect(page.getByRole("button", { name: "同意して続ける" })).toBeDisabled();
  await page.getByRole("checkbox").check();
  await page.getByRole("button", { name: "同意して続ける" }).click();
  // はじめてのログインは、ニックネーム・種目・生まれた年を決めるまで先に進めない。
  await expect(page.getByRole("heading", { name: "ニックネームと種目を決めよう" })).toBeVisible({ timeout: 20_000 });
  await page.getByPlaceholder("ニックネーム").fill("あされん");
  await page.getByLabel("種目").selectOption("100m");
  // 生まれた年も登録するまで先に進めない（本体 #1045）。欄は種目の下に出る。
  await page.getByLabel("生まれた年").fill("2000");
  await page.getByRole("button", { name: "保存" }).click();
  await expect(page).toHaveURL("/coaching", { timeout: 20_000 });

  await page.goto("/posts/new");
  await expect(page.getByRole("heading", { name: "文章・写真" })).toBeVisible({ timeout: 20_000 });

  const body = `朝練の話 ${Date.now()}\n今日のスタートを共有します`;
  await page.getByLabel("詳細").fill(body);
  await page.getByRole("button", { name: "投稿する" }).click();
  // フィードを当面閉じたため、投稿後は /?sort=new からリダイレクトで /coaching?sort=new に着地する（#1008）。
  // フィード復帰時に投稿の見え方確認（投稿本文のリンクが見えること）へ戻す。
  await expect.poll(() => {
    const url = new URL(page.url());
    return `${url.pathname}?${url.searchParams.toString()}`;
  }, { timeout: 20_000 }).toBe("/coaching?sort=new");
});
