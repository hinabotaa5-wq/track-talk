import { L } from "@/lib/i18n";
import type { ClarifyReason, CoachEvent, HelpfulAnswer, ReviewFocus, ReviewInfoReason, ReviewPayMethod, ReviewRequest, ReviewStatus } from "./types";

// 添削を受け付ける種目。最初は100mだけにし、コーチが増えたら足す（API の reviewEvents と合わせる）。
export const reviewEvents: CoachEvent[] = ["100m"];

// 以前の依頼で選ばれた項目の表示名。
const reviewFocusLabels: Record<ReviewFocus, string> = {
  get start() { return L("スタート"); },
  get acceleration() { return L("加速"); },
  get "top-speed"() { return L("トップスピード"); },
  get takeoff() { return L("踏切"); },
  get arm() { return L("腕振り"); },
  get contact() { return L("接地"); },
  get rhythm() { return L("リズム"); },
  get other() { return L("その他"); },
};

export function reviewQuestionText(request: Pick<ReviewRequest, "question" | "focus">) {
  return request.question || request.focus.map((focus) => reviewFocusLabels[focus]).join(L("、"));
}

export const reviewStatusLabels: Record<ReviewStatus, string> = {
  get awaiting_payment() { return L("お支払い待ち"); },
  get queued() { return L("コーチを探しています"); },
  get offered() { return L("コーチを探しています"); },
  get in_review() { return L("添削中"); },
  get needs_info() { return L("コーチが内容の確認を求めています"); },
  get returned() { return L("返却済み"); },
  get clarifying() { return L("補足を待っています"); },
  get expired() { return L("期限切れ"); },
};

export const reviewPayMethodLabels: Record<ReviewPayMethod, string> = {
  get card() { return L("カード・Apple Pay・Google Pay"); },
  paypay: "PayPay",
};

// 動画添削とその追加相談で今使える支払い方法。API の reviewPaymentMethods（apps/api/internal/httpx/coaching_payment.go）と揃える。
// 当面はカード（Apple Pay・Google Pay を含む）のみ。PayPay を使えるようにするときは、ここに "paypay" を足すと
// 購入画面の案内（reviewPayNotice）が消える。特商法の支払方法（app/legal/commerce）も合わせて直す。
// reviewPayMethodLabels は、過去の支払いの表示に使うので PayPay も残す。
export const availableReviewPayMethods: ReviewPayMethod[] = ["card"];

// PayPay が使えるようになるまで、動画添削・追加相談の購入画面に出す案内。
export const reviewPayNotice: string | null = availableReviewPayMethods.includes("paypay")
  ? null
  : "現在、お支払いはクレジットカードのみとなります。PayPayでのお支払いにも対応予定です（時期は未定です）。対応が整い次第、この画面でご案内します。";

// 「ここがわからなかった」の選択肢。自由記述は置かない（やりとりは一方通行のまま）。
export const clarifyReasonLabels: Record<ClarifyReason, string> = {
  get term() { return L("用語がわからない"); },
  get drill() { return L("ドリルのやり方がわからない"); },
  get fix() { return L("直す点の意味がわからない"); },
  get other() { return L("その他"); },
};

// 返却前の「内容確認のための差し戻し」の種類（任意）。並びは API と同じ。
export const reviewInfoReasonLabels: Record<ReviewInfoReason, string> = {
  get focus() { return L("見てほしいポイントが分からない"); },
  get scene() { return L("動画のどの場面か分からない"); },
  get goal() { return L("目標・現状が分からない"); },
  get mismatch() { return L("動画と説明が食い違っている"); },
  get other() { return L("その他"); },
};

// 差し戻しの質問の最短の長さ（サーバーと同じ。空白と記号は数えない）。
export const minInfoRequestChars = 15;

// 引受後の辞退でコーチが添えられる理由の長さ（サーバーと同じ）。
export const maxWithdrawReasonChars = 200;

export const helpfulLabels: Record<HelpfulAnswer, string> = {
  get yes() { return L("役に立った"); },
  get some() { return L("少し"); },
  get no() { return L("役に立たなかった"); },
};

// 返答の1項目の上限（サーバーと同じ）。
export const maxResponseChars = 300;

// 添削の動画は30秒まで。少しの誤差はサーバーと同じく許す。
export const maxReviewSeconds = 30;
const secondsSlop = 1;
const videoDurationTimeoutMs = 10_000;

// videoDuration は、選んだ動画の長さ（秒）を読む。読めなければ null。
export function videoDuration(file: File): Promise<number | null> {
  return new Promise((resolve) => {
    const url = URL.createObjectURL(file);
    const video = document.createElement("video");
    video.preload = "metadata";
    // 読めない形式でメタデータも失敗も来ないと待ち続けて画面が固まるので、一定時間で諦める。
    const timer = window.setTimeout(() => done(null), videoDurationTimeoutMs);
    const done = (value: number | null) => {
      window.clearTimeout(timer);
      video.onloadedmetadata = null;
      video.onerror = null;
      video.removeAttribute("src");
      video.load();
      URL.revokeObjectURL(url);
      resolve(value);
    };
    video.onloadedmetadata = () => done(Number.isFinite(video.duration) ? video.duration : null);
    video.onerror = () => done(null);
    video.src = url;
  });
}

export function tooLong(seconds: number | null) {
  return seconds !== null && seconds > maxReviewSeconds + secondsSlop;
}

// 動画添削の動画ファイルの保存期間（運営者決定 2026-11-10、本体 #506）。スレッドの終わり（返却と「わからなかった」の
// 確認の往復が終わった時。確認を頼まなかったときは、頼める返却から7日が過ぎた時）から7日で、依頼の動画とコーチの見本動画を消す。
export const reviewVideoRetentionDays = 7;
export const reviewVideosPurgedNote = `動画は保存期間（添削のやりとりが終わってから${reviewVideoRetentionDays}日）を過ぎたため削除しました。添削の文章は引き続き見られます。`;
