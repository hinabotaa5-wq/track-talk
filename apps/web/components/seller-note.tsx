import Link from "next/link";
import { operatorName } from "@/lib/operator";

// 誰が販売者か(契約の相手か)を購入の前に示す。フォーム添削・チケットは運営者、サポートプランはコーチ。
// 弁護士Q&Aの回答(立場をサイト上に明示する)をもとにしたたたき台。
export function SellerNote({ coachName }: { coachName?: string | null }) {
  if (coachName === undefined) {
    return (
      <p className="text-xs leading-5 text-slate-300">
        販売者: TrackTalk の運営者（{operatorName}）。コーチは運営者から依頼を受けて添削します。
        <Link href="/legal/commerce" className="ml-1 text-blue-300 underline">特定商取引法に基づく表記</Link>
      </p>
    );
  }
  return (
    <p className="text-xs leading-5 text-slate-300">
      販売者: このプランを出しているコーチ（{coachName ?? "名前なし"}）。TrackTalk の運営者は申し込みの場を提供します。
    </p>
  );
}
