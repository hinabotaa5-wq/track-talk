import { JapaneseOfficialNote } from "./japanese-official-note";
// 1ヶ月プランの報酬と精算（コーチ向け条件）。コーチの応募画面と「コーチ機能について」に出し、登録時に明示する。
// 契約ごとの取引条件の明示（support_contract_statements）とは別に、登録の時点で示すための文。
export const supportPlanCoachTerms: { title: string; text: string }[] = [
  {
    title: "報酬の確定時期と仮払い",
    text: "1ヶ月プランの報酬は、契約が終わった時に、終わった日までの経過分（契約価格から選手への日割りの返金額を差し引いた額）の75%（1円未満切り捨て）で確定します。日割りは「契約価格 ÷ 期間の日数 × 残りの日数」（1円未満切り捨て）で、期間の日数は契約に記録された開始日から終了日まで（両端を含む30日）、終わることが決まった日は経過した日に数えます。期間中は、毎月末日までの経過日数ぶんを確定前の仮払いとして、翌月15日に送金します（合計3,000円未満は翌月へ繰り越し）。確定額との差額は、契約終了後の送金で精算します。解約料・違約金は選手からいただかないため、途中で終わった場合も、あなたの報酬は経過分だけです。",
  },
  {
    title: "コーチ都合の精算",
    text: "あなたの責めに帰すべき事由で契約が途中で終わった場合、選手へは運営が全額を返金します。これは債務不履行に基づく返還・精算で、あなたは、選手への返金額の75%を上限として、受け取った仮払いを返還します（以後の報酬から差し引く方法を含みます）。それを超える賠償は求めません。返金が終わるまで、その契約の報酬の送金を留めます。",
  },
  {
    title: "やむを得ない事情と判定の手続",
    text: "入院・死亡、災害による業務不能、法令による業務停止など、あなたの責めに帰すことができない事情で続けられない場合は、通常の精算（終了日までの経過分を支払い、残りは選手へ返金）とします。運営は、コーチ都合かどうかの判定とその理由を、書面（電磁的記録）でお知らせします。判定に不服がある場合は、1回に限り、期限なく異議を申し立てて再審査を求められます。再審査の結果も、理由とともにお知らせします。精算は、異議申立てがない場合または再審査が終わった後に、運営の手続きで確定します。再審査でやむを得ない事情に変わった場合は、選手への全額返金はそのままにし、あなたには終了日までの経過分を支払います。",
  },
  {
    title: "大会の中止・延期",
    text: "選手の目標の大会が中止・延期になっても、契約は期間どおり続きます。選手が途中で終える場合は、1 の通常の精算です。",
  },
  {
    title: "契約ごとの取引条件",
    text: "契約が成立するたびに、その契約の業務の内容・期間・価格・報酬の算定方法・仮払いと支払期日・途中で終わった場合の扱いを、電磁的方法でお知らせします（フリーランス・事業者間取引適正化等法第3条に対応）。",
  },
];

export function SupportPlanCoachTerms() {
  return (
    <section aria-labelledby="support-plan-coach-terms" className="space-y-2 rounded-xl border border-white/20 bg-white/5 p-3">
      <h3 id="support-plan-coach-terms" className="text-sm font-bold text-white">1ヶ月プランの報酬と精算</h3>
      <JapaneseOfficialNote subject="section" />
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
