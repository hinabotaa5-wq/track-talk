import type { Metadata } from "next";
import Link from "next/link";
import { coachApplyPath } from "@/lib/coach-recruit";

export const metadata: Metadata = {
  title: "オンラインコーチング",
  description: "TrackTalk のオンラインコーチングは準備中です。コーチを募集しています。",
};

// オンラインコーチングを始める前に、コーチだけ先に募集する。機能の中身はまだ置かない。
export default function CoachingPage() {
  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-2">
        <p className="inline-flex rounded-full border border-blue-400/40 bg-blue-500/15 px-2.5 py-0.5 text-xs font-semibold text-blue-300">
          準備中
        </p>
        <h1 className="text-2xl font-bold">オンラインコーチング</h1>
        <p className="text-sm leading-7 text-slate-200">
          陸上のコーチに、オンラインで走りを見てもらえる場所を準備しています。始まったら、このタブからお知らせします。
        </p>
      </div>

      <section
        aria-labelledby="coach-recruit-title"
        className="space-y-3 rounded-2xl border border-blue-400/40 glass bg-blue-500/15 p-4"
      >
        <h2 id="coach-recruit-title" className="text-lg font-bold text-white">
          コーチを募集しています
        </h2>
        <p className="text-sm leading-7 text-slate-200">
          動画の添削やオンライン指導をしてくれる方を探しています。YouTube や SNS で教えている方も歓迎です。
        </p>
        <Link
          href={coachApplyPath}
          className="flex min-h-11 w-full items-center justify-center rounded-md bg-blue-600 px-4 text-sm font-semibold text-white hover:bg-blue-500"
        >
          応募する
        </Link>
        <p className="text-xs text-slate-300">お問い合わせフォームから応募できます(ログインが必要です)。</p>
      </section>
    </div>
  );
}
