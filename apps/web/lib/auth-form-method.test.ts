import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

// JavaScript が動く前にネイティブ送信されても、入力値が URL のクエリに載らないようにする。
const webRoot = join(__dirname, "..");
const credentialInput = /type="password"|autoComplete="one-time-code"/;
// パスワード欄は無いが、再設定の申請でメールアドレスを入力するフォーム。
const emailOnlyAuthForms = ["app/reset-password/page.tsx"];

function sourceFiles(dir: string): string[] {
  return readdirSync(join(webRoot, dir), { recursive: true, encoding: "utf8" })
    .filter((name) => name.endsWith(".tsx") && !name.endsWith(".test.tsx"))
    .map((name) => join(dir, name));
}

const authFormFiles = [
  ...["app", "components"].flatMap(sourceFiles).filter((file) => credentialInput.test(readFileSync(join(webRoot, file), "utf8"))),
  ...emailOnlyAuthForms,
];

describe("認証まわりのフォーム", () => {
  it("対象のファイルを見つけている", () => {
    expect(authFormFiles).toEqual(expect.arrayContaining(["app/login/page.tsx", "app/register/page.tsx", "components/account-forms.tsx"]));
  });

  it.each(authFormFiles)("%s の form はすべて method=\"post\" を持つ", (file) => {
    const forms = readFileSync(join(webRoot, file), "utf8").match(/<form\b[^>]*/g) ?? [];
    expect(forms.length).toBeGreaterThan(0);
    for (const form of forms) expect(form).toContain('method="post"');
  });
});
