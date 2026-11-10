import { L } from "@/lib/i18n";
import { reviewPayNotice } from "@/lib/review";

// 動画添削・追加相談の購入画面に出す、今使える支払い方法の案内（PayPay が使えるようになれば出さない）。
export function ReviewPayNotice() {
  return reviewPayNotice ? <p className="text-xs text-slate-300">{L(reviewPayNotice)}</p> : null;
}
