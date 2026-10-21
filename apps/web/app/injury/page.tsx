import type { Metadata } from "next";
import { Suspense } from "react";
import { InjuryHome } from "./injury-home";

export const metadata: Metadata = {
  title: "ケガ部｜同じケガをした陸上選手が集まる場所",
  description:
    "肉離れ・捻挫・シンスプリント・ジャンパー膝など、同じケガをした陸上選手が集まるグループ。復帰までの目安や、みんなが意識的にやったこと、次の人への一言を見られます。",
  alternates: { canonical: "/injury" },
};

export default function InjuryPage() {
  // 読み込み中に中身が1画面に収まると、ページがスクロールできなくなり、
  // iPhone の Safari が下のツールバーを出し直して、下部バーが上下に動く。
  // 他のタブと同じく、読み込み中も1画面より高くしておく。
  return (
    <div className="min-h-svh md:min-h-0">
      <Suspense fallback={<p className="text-sm text-slate-200">読み込み中...</p>}>
        <InjuryHome initialView="share" />
      </Suspense>
    </div>
  );
}
