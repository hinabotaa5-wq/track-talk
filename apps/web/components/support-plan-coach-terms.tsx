import { JapaneseOfficialNote } from "./japanese-official-note";
// 1ヶ月プランの報酬と精算（コーチ向け条件）。コーチの応募画面と「コーチ機能について」に出し、登録時に明示する。

export function SupportPlanCoachTerms() {
  return (
    <section aria-labelledby="support-plan-coach-terms" className="space-y-2 rounded-xl border border-white/20 bg-white/5 p-3">
      <h3 id="support-plan-coach-terms" className="text-sm font-bold text-white">1ヶ月プランの報酬と精算</h3>
      <JapaneseOfficialNote subject="section" />
      <div className="space-y-2 text-sm leading-relaxed text-slate-200">
        <p>報酬は経過分の75%です。月末までの分を翌月15日に仮払いし（3,000円未満は繰越し）、契約終了時に精算します。解約料はありません。</p>
        <p>コーチ都合で終了した場合は選手へ全額返金し、受け取った仮払いはその75%まで返還いただきます。やむを得ない事情の場合は通常の精算です。</p>
        <p>日割りの計算方法、異議申立て、契約ごとの条件などの詳細は、契約成立時の通知と利用規約をご確認ください。</p>
      </div>
    </section>
  );
}
