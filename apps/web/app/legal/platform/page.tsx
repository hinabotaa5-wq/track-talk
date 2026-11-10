import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { Rule } from "@/components/legal-prose";
import { coachingEnabled, coachingMessagingEnabled } from "@/lib/features";
import { JapaneseOfficialNote } from "@/components/japanese-official-note";

// 取引デジタルプラットフォーム消費者保護法 第3条の措置の概要（サポートプラン向け）。文面は運営者が承認したもの。
// 措置1の「メッセージ機能で連絡を取り合える」は、契約のやりとり欄の送信が開いていないと事実にならない。
// そのため、サポートプランの提供（coachingEnabled）とメッセージの送信（coachingMessagingEnabled）の両方が
// 開くまでは 404 にして、一般からは見えないようにする。文面を変えるときは運営者の承認を経ること。
export const metadata: Metadata = {
  title: "取引デジタルプラットフォーム消費者保護法に基づく措置について",
  description: "TrackTalk のサポートプランについて、運営者が取引デジタルプラットフォーム消費者保護法第3条に基づいて講じている措置。",
  robots: { index: false },
};

export default function PlatformMeasuresPage() {
  if (!coachingEnabled || !coachingMessagingEnabled) notFound();
  return (
    <div className="mx-auto max-w-2xl space-y-6 [overflow-wrap:anywhere]">
      <div className="space-y-2">
        <h1 className="text-2xl font-bold">取引デジタルプラットフォーム消費者保護法に基づく措置について</h1>
        <JapaneseOfficialNote />
        <p className="text-sm leading-7 text-slate-300">
          TrackTalkのサポートプランでは、出品者であるコーチ（販売者）と購入者が取引を行い、運営者は、取引の場の提供と、代金の代理受領を行っています。運営者は、取引デジタルプラットフォーム消費者保護法第3条に基づき、次の措置を講じています。
        </p>
      </div>
      <Rule title="1. 円滑な連絡手段の確保">
        <p>
          購入者とコーチは、サービス内のメッセージ機能を使って連絡を取り合うことができます。また、運営者へのお問い合わせ窓口を設けています。
        </p>
      </Rule>
      <Rule title="2. 苦情への対応">
        <p>購入者またはコーチから苦情や相談があった場合は、運営者が事実関係の確認など必要な調査を行い、適切に対応します。</p>
      </Rule>
      <Rule title="3. 販売者の特定に必要な情報の確保と開示への対応">
        <p>
          コーチには、プランの提供を始める前に、氏名・住所・電話番号の登録を求め、運営者が保管しています。購入者から、法令に基づいて販売者に関する情報の開示を求められた場合は、お問い合わせ窓口を通じて受け付け、法令に従って対応します。
        </p>
      </Rule>
      <nav className="flex flex-wrap gap-4 text-sm text-blue-300 underline">
        <Link href="/terms">利用規約</Link>
        <Link href="/privacy">プライバシーポリシー</Link>
        <Link href="/contact">運営者・お問い合わせ</Link>
      </nav>
    </div>
  );
}
