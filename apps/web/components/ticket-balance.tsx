"use client";
import { L } from "@/lib/i18n";

import type { TicketSummary } from "@/lib/coaching-tickets";
import { formatDateTime } from "@/lib/format";

// TicketBalance は、チケットの残りと、いちばん早く失効する日時を常に出す（本体 #569）。
export function TicketBalance({ summary }: { summary: Pick<TicketSummary, "balance" | "next_expiry"> }) {
  const { available, reserved } = summary.balance;
  return (
    <div className="space-y-1 rounded-xl border border-white/20 bg-white/5 p-3 text-sm" aria-label={L("チケットの残り")}>
      <p className="font-semibold">{L("チケットの残り {available}枚", { available })}{reserved > 0 ? L("（このほかに依頼で確保中 {reserved}枚）", { reserved }) : ""}</p>
      {summary.next_expiry ? (
        <p className="text-xs text-slate-200">{L("いちばん早い有効期限：{v0}（{available}枚）。過ぎると使えなくなります。", { v0: formatDateTime(summary.next_expiry.expires_at), available: summary.next_expiry.available })}</p>
      ) : available > 0 ? (
        <p className="text-xs text-slate-300">{L("有効期限のないチケットです（以前に購入した分）。")}</p>
      ) : null}
    </div>
  );
}
