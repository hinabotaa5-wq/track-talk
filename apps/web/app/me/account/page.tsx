import { redirect } from "next/navigation";

// メールアドレスとパスワードの変更は別々のページに分けた。前の確認メールのリンクから来た人をメールアドレスの方へ送る。
export default function AccountRedirect() {
  redirect("/me/email");
}
