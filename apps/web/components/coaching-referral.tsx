"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { apiFetch } from "@/lib/api";
import type { Referral } from "@/lib/coaching-tickets";
import { formatDateTime } from "@/lib/format";
import { L } from "@/lib/i18n";

export async function validReferralInput(code: string, token: string | null): Promise<boolean> {
  if (!code.trim()) return true;
  const result = await apiFetch<{ valid: boolean }>("/v1/coaching/referrals/check", {
    method: "POST", token, body: JSON.stringify({ code }),
  });
  return result.valid;
}

export function ReferralInput({ code, onChange, warning, disabled }: {
  code: string;
  onChange: (value: string) => void;
  warning: boolean;
  disabled?: boolean;
}) {
  return <div className="space-y-1">
    <label className="block text-sm">
      {L("紹介ID（友達からもらった場合）")}
      <input type="text" className="mt-1 block w-full rounded-lg border border-slate-500/40 bg-transparent p-2"
        autoComplete="off" autoCapitalize="characters" maxLength={32} value={code} disabled={disabled}
        onChange={(event) => onChange(event.target.value)} />
    </label>
    {warning ? <p role="status" className="text-xs text-amber-300">{L("この紹介IDは使えません。空欄のまま購入できます。")}</p> : null}
  </div>;
}

export function LoadedReferralCard({ token }: { token: string | null }) {
  const [referral, setReferral] = useState<Referral | null>(null);
  useEffect(() => {
    if (!token) return;
    let active = true;
    apiFetch<{ referral: Referral | null }>("/v1/me/coaching/referral", { token })
      .then((data) => { if (active) setReferral(data.referral ?? null); })
      .catch(() => { if (active) setReferral(null); });
    return () => { active = false; };
  }, [token]);

  return referral ? <ReferralCard referral={referral} /> : null;
}

function ReferralCard({ referral }: { referral: Referral }) {
  const [copyStatus, setCopyStatus] = useState("");
  const { code, friend_count, granted_at, expires_at } = referral;
  return <section className="app-card space-y-2 rounded-xl p-4">
    <h2 className="font-semibold">{L("友達2人を紹介すると、1回無料！")}</h2>
    <p className="text-sm">{L("あなたが購入すると紹介IDをお渡しします。そのIDで友達2人が購入すると、あなたに無料チケットをプレゼントします。")}</p>
    <div className="flex items-center gap-3">
      <p className="text-sm">{L("あなたの紹介ID")} <strong className="font-mono tracking-wider">{code}</strong></p>
      <Button type="button" variant="secondary" onClick={async () => {
        try {
          await navigator.clipboard.writeText(code);
          setCopyStatus(L("コピーしました"));
        } catch {
          setCopyStatus(L("紹介IDを選択してコピーしてください"));
        }
      }}>{L("コピー")}</Button>
    </div>
    {copyStatus ? <p role="status" className="text-xs">{copyStatus}</p> : null}
    <p className="text-sm font-semibold">{granted_at ? L("無料チケットをプレゼントしました") : friend_count === 1 ? L("あと1人で無料チケット") : L("あと2人で無料チケット")}</p>
    {granted_at && expires_at ? <div className="space-y-1 text-xs">
      <p>{L("付与日 {v0}", { v0: formatDateTime(granted_at) })}</p>
      <p>{L("有効期限 {v0}", { v0: formatDateTime(expires_at) })}</p>
      <p>{L("特典チケット：付与日から6か月有効／付与された本人だけが使えます／払戻しはできません")}</p>
    </div> : null}
  </section>;
}
