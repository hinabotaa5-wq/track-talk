import { L } from "@/lib/i18n";
import Link from "next/link";
import { operatorName } from "@/lib/operator";

// 誰が契約の相手かを購入の前に示す。フォーム添削・チケットは運営者が提供し、サポートプランはコーチが販売する。
// 両者は支払いの仕組みが違うので、文言を混ぜない（利用規約の第6条とフォーム添削、第8条とサポートプランを合わせる）。
export function SellerNote({ coachName }: { coachName?: string | null }) {
  if (coachName === undefined) {
    return (
      <div className="space-y-1 text-xs leading-5 text-slate-300">
        <p>{L("販売者: TrackTalk の運営者（{operatorName}）。フォーム添削の提供者は運営者で、添削は運営者がコーチに委託して行います。", { operatorName })}</p>
        <p>{L("代金のお支払い先は運営者です。料金・キャンセル・返金は運営者が責任を持って対応します。")}</p>
        <p>
          <Link href="/terms" className="text-blue-300 underline">{L("利用規約")}</Link>
          <Link href="/legal/commerce" className="ml-3 text-blue-300 underline">{L("特定商取引法に基づく表記")}</Link>
        </p>
      </div>
    );
  }
  return (
    <div className="space-y-1 text-xs leading-5 text-slate-300">
      <p>{L("販売者: このプランを出しているコーチ（{v0}）。契約の相手はコーチで、プランの内容・料金・期間もコーチが決めます。", { v0: coachName ?? L("名前なし") })}</p>
      <p>{L("代金を請求する権利はコーチにあり、TrackTalk の運営者はコーチから代理受領の権限を受けて代金を受け取ります。運営者へ支払った時点でコーチへの支払いは済み、コーチから同じ代金を重ねて請求されることはありません。")}</p>
      <p>
        <Link href="/terms" className="text-blue-300 underline">{L("利用規約（サポートプランの契約と代金の受け取り）")}</Link>
      </p>
    </div>
  );
}
