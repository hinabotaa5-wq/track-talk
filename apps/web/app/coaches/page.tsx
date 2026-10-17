import type { Metadata } from "next";
import { CoachList } from "./coach-list";

export const metadata: Metadata = {
  title: "コーチ一覧",
  description: "TrackTalk のオンラインコーチングで見てくれるコーチと先輩の一覧です。記録・成績・競技年数を、依頼する前に確かめられます。",
};

// 依頼する前に、どんなコーチがいるかを見られる。添削でもパーソナル指導でも同じ一覧を使う。
export default function CoachesPage() {
  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-2">
        <h1 className="text-2xl font-bold">コーチ一覧</h1>
        <p className="text-sm text-slate-200">フォーム添削とパーソナル指導で見てくれるコーチと先輩です。</p>
        <p className="text-xs text-slate-300">フォーム添削では、枠と種目が合う受付中のコーチに順番に回ります。返答と一緒に、見てくれたコーチのプロフィールが届きます。</p>
      </div>
      <CoachList />
    </div>
  );
}
