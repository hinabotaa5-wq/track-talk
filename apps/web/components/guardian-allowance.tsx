import { formatYen } from "@/lib/format";
import type { CoachingGuardian } from "@/lib/types";

// GuardianAllowance は、保護者が同意済みの18歳未満に見せる、今月あといくら自分で払えるか。依頼画面とマイページで同じ文面を出す。
export function GuardianAllowance({ guardian, className }: { guardian: CoachingGuardian; className?: string }) {
  const limit = guardian.self_pay_limit_yen;
  const spent = guardian.month_spent_yen ?? 0;
  return (
    <div className={className ?? "space-y-1 text-sm text-slate-300"}>
      <p>
        今月の利用可能額：あと{formatYen(Math.max(limit - spent, 0))}（月の上限{formatYen(limit)}、今月の利用合計{formatYen(spent)}）
      </p>
      <p>上限を超える購入はできません。上限の変更は保護者の方にお願いしてください。</p>
    </div>
  );
}
