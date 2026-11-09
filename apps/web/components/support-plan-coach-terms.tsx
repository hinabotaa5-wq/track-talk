// 大会までプランの報酬と精算（コーチ向け条件）。コーチの応募画面と「コーチ機能について」に出し、登録時に明示する。
// 契約ごとの取引条件の明示（support_contract_statements）とは別に、登録の時点で示すための文。
export const supportPlanCoachTerms: { title: string; text: string }[] = [
  {
    title: "報酬の確定時期と仮払い",
    text: "大会までプランの報酬は、契約が終わった時に、終わった日までの経過日数の日割り（契約価格 ÷ 期間の日数 × 経過日数、1円未満切り捨て）の80%で確定します。期間中は、毎月末日までの経過日数ぶんを確定前の仮払いとして、翌月15日に送金します（合計3,000円未満は翌月へ繰り越し）。確定額との差額は、契約終了後の送金で精算します。",
  },
  {
    title: "コーチ都合の精算",
    text: "あなたの責めに帰すべき事由で契約が途中で終わった場合、選手へは運営が全額を返金します。あなたは、選手への返金額の80%を上限として、受け取った仮払いを返還します（以後の報酬から差し引く方法を含みます）。それを超える賠償は求めません。返金が終わるまで、その契約の報酬の送金を留めます。",
  },
  {
    title: "やむを得ない事情と判定の手続",
    text: "入院・死亡、災害による業務不能、法令による業務停止など、あなたの責めに帰すことができない事情で続けられない場合は、通常の精算（終了日までの経過分を支払い、残りは選手へ返金）とします。運営は、コーチ都合かどうかの判定とその理由を、書面（電磁的記録）でお知らせします。判定に不服がある場合は、1回に限り異議を申し立てて再審査を求められます。再審査の結果も、理由とともにお知らせします。精算は、異議申立てがない場合または再審査が終わった後に、運営の手続きで確定します。",
  },
  {
    title: "大会の中止・延期",
    text: "目標大会が中止・延期になった場合は、主催者が正式に発表した日で契約を終え、その日までの経過分で報酬を確定します。振り替え・延長はしません。",
  },
];

export function SupportPlanCoachTerms() {
  return (
    <section aria-labelledby="support-plan-coach-terms" className="space-y-2 rounded-xl border border-white/20 bg-white/5 p-3">
      <h3 id="support-plan-coach-terms" className="text-sm font-bold text-white">大会までプランの報酬と精算</h3>
      <ol className="list-decimal space-y-2 pl-5 text-sm leading-relaxed text-slate-200">
        {supportPlanCoachTerms.map((t) => (
          <li key={t.title}>
            <span className="font-semibold text-slate-100">{t.title}：</span>
            {t.text}
          </li>
        ))}
      </ol>
    </section>
  );
}
