import { coachingEnabled } from "@/lib/features";
import { CoachApplyLink } from "./coach-apply-link";

// 提供前は直接URLを開いても申込み画面を出さず、募集だけを案内する。
// 閉じる画面の layout から使う。API 側の COACHING_ENABLED と揃えること。
export function CoachingGate({ children }: { children: React.ReactNode }) {
  if (!coachingEnabled) return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-3">
        <p className="text-sm font-semibold text-blue-300">TrackTalkのコーチング</p>
        <h1 className="text-2xl font-bold">ただいま準備中です</h1>
        <p className="text-sm leading-relaxed text-slate-200">動画添削や指名プランの受付・チケットの購入は、公開までお待ちください。現在は、選手を支えるコーチを募集しています。</p>
      </div>
      <CoachApplyLink />
    </div>
  );
  return children;
}
