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

// useMyReferral は、自分の紹介IDの周（有効な周、なければ直近に終了した周）を読む。
// 読み込み前・失敗時は loaded=false のまま。
function useMyReferral(token: string | null | undefined) {
  const [state, setState] = useState<{ loaded: boolean; referral: Referral | null }>({ loaded: false, referral: null });
  useEffect(() => {
    if (!token) return;
    let active = true;
    apiFetch<{ referral: Referral | null }>("/v1/me/coaching/referral", { token })
      .then((data) => { if (active) setState({ loaded: true, referral: data.referral ?? null }); })
      .catch(() => { if (active) setState({ loaded: false, referral: null }); });
    return () => { active = false; };
  }, [token]);
  return state;
}

export function ReferralInput({ code, onChange, warning, disabled, token }: {
  code: string;
  onChange: (value: string) => void;
  warning: boolean;
  disabled?: boolean;
  token?: string | null;
}) {
  const mine = useMyReferral(token);
  // 有効な紹介IDを持たない人が紹介IDを入れたときだけ、その購入では自分のIDが出ないことを添える（本体 #686）。
  const noOwnCode = mine.loaded && !mine.referral?.active && code.trim() !== "" && !warning;
  return <div className="space-y-1">
    <label className="block text-sm">
      {L("紹介ID（友達からもらった場合）")}
      <input type="text" className="mt-1 block w-full rounded-lg border border-slate-500/40 bg-transparent p-2"
        autoComplete="off" autoCapitalize="characters" maxLength={32} value={code} disabled={disabled}
        onChange={(event) => onChange(event.target.value)} />
    </label>
    {warning ? <p role="status" className="text-xs text-amber-300">{L("この紹介IDは使えません。空欄のまま購入できます。")}</p> : null}
    {noOwnCode ? <p className="text-xs">{L("紹介IDを使って購入すると、この購入ではあなたの紹介IDは発行されません。")}</p> : null}
  </div>;
}

const referralHeadline = () => L("友達2人を紹介すると、1回無料！");
const referralBody = () => L("購入すると、紹介IDが発行されます。そのIDで友達2人が購入すると、紹介者に無料チケットがプレゼントされます。");

// showUnissued を付けた画面（/me）では、IDをまだ持たない人にも発行条件を案内する。
export function LoadedReferralCard({ token, showUnissued }: { token: string | null; showUnissued?: boolean }) {
  const { loaded, referral } = useMyReferral(token);
  if (referral) return <ReferralCard referral={referral} />;
  if (!showUnissued || !loaded) return null;
  return <section className="app-card space-y-2 rounded-xl p-4">
    <h2 className="font-semibold">{referralHeadline()}</h2>
    <p className="text-sm">{referralBody()}</p>
    <p className="text-sm">{L("紹介IDを使わずに購入すると、あなたの紹介IDが発行されます。")}</p>
  </section>;
}

function ReferralCard({ referral }: { referral: Referral }) {
  const [copyStatus, setCopyStatus] = useState("");
  const { code, active, friend_count, granted_at, expires_at } = referral;
  return <section className="app-card space-y-2 rounded-xl p-4">
    <h2 className="font-semibold">{referralHeadline()}</h2>
    <p className="text-sm">{referralBody()}</p>
    <div className="flex items-center gap-3">
      <p className="text-sm">{L("あなたの紹介ID")} <strong className="font-mono tracking-wider">{code}</strong></p>
      {active ? <Button type="button" variant="secondary" onClick={async () => {
        try {
          await navigator.clipboard.writeText(code);
          setCopyStatus(L("コピーしました"));
        } catch {
          setCopyStatus(L("紹介IDを選択してコピーしてください"));
        }
      }}>{L("コピー")}</Button> : null}
    </div>
    {copyStatus ? <p role="status" className="text-xs">{copyStatus}</p> : null}
    {granted_at ? <p className="text-sm font-semibold">{L("無料チケットをプレゼントしました")}</p>
      : active ? <p className="text-sm font-semibold">{friend_count === 1 ? L("あと1人で無料チケット") : L("あと2人で無料チケット")}</p> : null}
    {granted_at && expires_at ? <div className="space-y-1 text-xs">
      <p>{L("付与日 {v0}", { v0: formatDateTime(granted_at) })}</p>
      <p>{L("有効期限 {v0}", { v0: formatDateTime(expires_at) })}</p>
      <p>{L("特典チケット：付与日から6か月有効／付与された本人だけが使えます／払戻しはできません")}</p>
    </div> : null}
    {active ? null : <p className="text-sm">{L("このIDは終了しました。紹介IDを使わずに購入すると、新しい紹介IDが発行されます。")}</p>}
  </section>;
}
