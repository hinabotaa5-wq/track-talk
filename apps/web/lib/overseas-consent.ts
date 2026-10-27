// 登録画面で「海外の事業者に預けること」にも同意してから登録を始めた印。
// 登録後に初めて /v1/me を読んだとき、この印があれば同意の画面を出さずにそのまま記録する。
// タブを閉じれば消える保存領域に、1時間だけ置く(確認メールを別のタブで開いた人は、同意の画面で1回同意する)。
const key = "tracktalk-overseas-consent";
const maxAgeMs = 60 * 60 * 1000;

export function rememberOverseasConsent(now = Date.now()) {
  try {
    sessionStorage.setItem(key, String(now + maxAgeMs));
  } catch {
    // 保存できないときは、同意の画面で同意してもらう。
  }
}

export function takeOverseasConsent(now = Date.now()) {
  try {
    const value = sessionStorage.getItem(key);
    sessionStorage.removeItem(key);
    return value !== null && Number(value) > now;
  } catch {
    return false;
  }
}
