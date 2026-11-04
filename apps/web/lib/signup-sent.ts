// 登録の確認メールを送ったアドレスを、送信済みの画面に渡す。
// アドレスを URL に載せないよう、そのタブの中だけで読める保存領域に置く(タブを閉じれば消える)。
const key = "tracktalk-signup-sent-email";

export function rememberSignupSentEmail(email: string) {
  try {
    sessionStorage.setItem(key, email);
  } catch {
    // 保存できなくても、送信済みの画面はアドレス無しで出す。
  }
}

export function signupSentEmail(): string {
  try {
    return sessionStorage.getItem(key) ?? "";
  } catch {
    return "";
  }
}
