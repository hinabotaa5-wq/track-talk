import type { Metadata } from "next";
import { CoachList } from "./coach-list";

export const metadata: Metadata = {
  title: "指名コーチ一覧",
  description: "TrackTalk のオンラインコーチングで見てくれるコーチの一覧です。記録・成績や指導経験を、依頼する前に確かめられます。",
};

// 依頼する前に、どんなコーチがいるかを見られる。添削でもパーソナル指導でも同じ一覧を使う。
export default function CoachesPage() {
  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-2">
        <h1 className="text-2xl font-bold">指名コーチ一覧</h1>
        <p className="text-sm text-slate-200">コーチを選ぶと、詳しいプロフィールと指名プランを確認できます。</p>
      </div>
      <CoachList />
    </div>
  );
}
