import { JapaneseOfficialNote } from "./japanese-official-note";
// 1ヶ月プランの報酬と精算（コーチ向け条件）。コーチの応募画面と「コーチ機能について」に出し、登録時に明示する。

export function SupportPlanCoachTerms() {
  return (
    <section aria-labelledby="support-plan-coach-terms" className="space-y-2 rounded-xl border border-white/20 bg-white/5 p-3">
      <h3 id="support-plan-coach-terms" className="text-sm font-bold text-white">1ヶ月プランの報酬と精算</h3>
      <JapaneseOfficialNote subject="section" />
      <div className="space-y-2 text-sm leading-relaxed text-slate-200">
        <p>報酬は経過分の75%です。月末までの分を翌月15日に仮払いし（3,000円未満は繰越し）、契約終了時に精算します。解約料はありません。</p>
        <p>コーチの責めに帰すべき事由により契約期間中に継続できなくなった場合は、購入者に過ぎた日数分を含めて全額を返金します。すでに受け取った仮払い報酬は、購入者への返金額の75%に相当する額を上限として、返還を求められる場合があります。コーチの責めに帰すことができない事情による場合は、通常どおり精算します。</p>
        <p>日割りの計算方法、異議申立て、契約ごとの条件などの詳細は、契約成立時の通知と利用規約をご確認ください。</p>
      </div>
    </section>
  );
}
