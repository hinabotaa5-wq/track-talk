"use client";

import Link from "next/link";
import type { ComponentType, ReactNode } from "react";
import { Copy, Gift, GraduationCap, Keyboard, Ticket } from "lucide-react";
import { L } from "@/lib/i18n";

// 無料チケットの説明ページ（本体 #823）。ログイン不要の静的ページで、データ取得はしない。
type Step = { icon: ComponentType<{ className?: string; "aria-hidden"?: boolean }>; title: string; body: string };

function StepCard({ step, index, children }: { step: Step; index: number; children?: ReactNode }) {
  const Icon = step.icon;
  return (
    <li className="app-card rounded-xl p-4">
      <div className="flex items-start gap-3">
        <span className="flex size-11 shrink-0 items-center justify-center rounded-full bg-yellow-400 text-slate-900">
          <Icon className="size-6" aria-hidden />
        </span>
        <span className="min-w-0">
          <span className="block font-bold">
            <span className="mr-1">{index + 1}.</span>
            {L(step.title)}
          </span>
          <span className="block text-sm leading-relaxed">{L(step.body)}</span>
        </span>
      </div>
      {children}
    </li>
  );
}

const steps: Step[] = [
  { icon: Ticket, title: "購入すると、あなたの紹介IDが発行されます", body: "購入完了の画面とマイページに、あなたの紹介ID（6文字）が表示されます。" },
  { icon: Copy, title: "紹介IDを友達に伝える", body: "マイページで、紹介IDをコピーしたり共有ボタンで送ったりできます。" },
  { icon: GraduationCap, title: "友達がコーチを選んで、申し込みに進む", body: "友達は、1回プランの都度払いでも、チケット6枚パックの購入でも申し込めます。" },
  { icon: Keyboard, title: "友達が、購入の手続きで紹介IDを入力して購入を完了する", body: "「紹介ID」の欄に、受け取ったIDを入力します。入力しないと、紹介として数えられません。購入が完了すると、紹介してくれた方に「1人分」として数えられます。" },
  { icon: Gift, title: "友達2人が購入すると、無料チケットをプレゼント", body: "あと何人かは、マイページで確認できます。" },
];

const notes = [
  "紹介IDを使って購入すると、この購入ではあなたの紹介IDは発行されません。",
  "紹介IDの入力は任意です。空欄でも購入できます。",
  "使えない紹介IDを入力しても、購入は止まりません（紹介として数えられないだけです）。",
  "自分の紹介IDは、自分の購入には使えません。",
  "同じ友達は、同じ紹介者には1回だけ数えられます。",
  "対象は1回プランの都度払いとチケット6枚パックです。追加質問は対象外です。",
  "無料チケットは、もらった本人だけが使えます。払戻しはできません。",
];

export function ReferralGuideView() {
  return (
    <div className="mx-auto max-w-2xl space-y-8">
      <div className="space-y-2">
        <h1 className="text-2xl font-bold">{L("友達2人に紹介すると、1回分(1,000円)無料！")}</h1>
      </div>

      <section className="space-y-3">
        <h2 className="text-lg font-bold">{L("紹介の流れ")}</h2>
        <ol className="space-y-3">
          {steps.map((step, index) => <StepCard key={step.title} step={step} index={index} />)}
        </ol>
      </section>

      <section className="space-y-2">
        <h2 className="text-base font-bold">{L("注意点")}</h2>
        <ul className="list-disc space-y-1 pl-5 text-sm leading-relaxed">
          {notes.map((note) => (
            <li key={note}>{L(note)}</li>
          ))}
        </ul>
      </section>

      <div className="flex flex-col gap-3">
        <Link href="/coaching" className="app-card rounded-xl px-4 py-3 text-center font-bold">
          {L("コーチ一覧へ")}
        </Link>
      </div>
    </div>
  );
}
