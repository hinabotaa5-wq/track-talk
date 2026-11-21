"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { apiFetch } from "@/lib/api";
import type { Referral } from "@/lib/coaching-tickets";
import { formatDateTime } from "@/lib/format";
import { L } from "@/lib/i18n";

// ReferralWarning は、紹介ID欄の警告。counted は、その紹介者にすでに数えられている購入者（本体 #686）。
export type ReferralWarning = false | "invalid" | "counted";

export async function checkReferralInput(code: string, token: string | null): Promise<"ok" | "invalid" | "counted"> {
  if (!code.trim()) return "ok";
  const result = await apiFetch<{ valid: boolean; already_counted?: boolean }>("/v1/coaching/referrals/check", {
    method: "POST", token, body: JSON.stringify({ code }),
  });
  if (result.valid) return "ok";
  return result.already_counted ? "counted" : "invalid";
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
  warning: ReferralWarning;
  disabled?: boolean;
  token?: string | null;
}) {
  const mine = useMyReferral(token);
  // 有効な紹介IDを持たない人が紹介IDを入れたときだけ、その購入では自分のIDが出ないことを添える（本体 #686）。
  const noOwnCode = mine.loaded && !mine.referral?.active && code.trim() !== "" && !warning;
  return <div className="space-y-1">
    <label className="block text-sm">
      {L("紹介ID")}
      <input type="text" className="mt-1 block w-full rounded-lg border border-slate-500/40 bg-transparent p-2"
        autoComplete="off" autoCapitalize="characters" maxLength={32} value={code} disabled={disabled}
        onChange={(event) => onChange(event.target.value)} />
    </label>
    {warning === "counted" ? <p role="status" className="text-xs text-amber-300">{L("この紹介IDの紹介者には、あなたの購入がすでに数えられています。空欄のまま購入できます。")}</p>
      : warning ? <p role="status" className="text-xs text-amber-300">{L("この紹介IDは使えません。空欄のまま購入できます。")}</p> : null}
    {noOwnCode ? <p className="text-xs">{L("紹介IDを使って購入すると、この購入ではあなたの紹介IDは発行されません。")}</p> : null}
  </div>;
}

const referralHeadline = () => L("友達2人に紹介すると、1回分(500円)無料！");
const referralBody = () => L("購入すると、紹介IDが発行されます。そのIDで友達2人が購入すると、紹介してくれた方に次回の動画添削1回分が無料になるクーポンをプレゼントします。");

export function LoadedReferralCard({ token, variant = "full" }: { token: string | null; variant?: "full" | "compact" }) {
  const { referral } = useMyReferral(token);
  if (referral) return <ReferralCard referral={referral} variant={variant} />;
  return null;
}

function ReferralCard({ referral, variant = "full" }: { referral: Referral; variant?: "full" | "compact" }) {
  const [copyStatus, setCopyStatus] = useState("");
  const { code, active, friend_count, granted_at, expires_at } = referral;
  const compact = variant === "compact";
  return <section className={compact ? "app-card space-y-1 rounded-xl p-3" : "app-card space-y-2 rounded-xl p-4"}>
    <h2 className={compact ? "text-sm font-semibold" : "font-semibold"}>{referralHeadline()}</h2>
    {compact ? null : <p className="text-sm">{referralBody()}</p>}
    <div className="flex items-center gap-3">
      <p className="text-sm">{L("あなたの紹介ID")} <strong className={active ? "font-mono tracking-wider" : "font-mono tracking-wider text-slate-500 line-through decoration-red-500 decoration-2"}>{code}</strong></p>
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
    {granted_at ? <p className="text-sm font-semibold">{L("無料クーポンをプレゼントしました")}</p>
      : active ? <p className="text-sm font-semibold">{friend_count === 1 ? L("あと1人で無料クーポン") : L("あと2人で無料クーポン")}</p> : null}
    {!compact && granted_at && expires_at ? <div className="space-y-1 text-xs">
      <p>{L("付与日 {v0}", { v0: formatDateTime(granted_at) })}</p>
      <p>{L("有効期限 {v0}", { v0: formatDateTime(expires_at) })}</p>
      <p>{L("特典クーポン：付与日から6か月有効／付与された本人だけが次回の購入で使えます／払戻しはできません")}</p>
    </div> : null}
    {compact || active ? null : <p className="text-sm">{L("このIDは終了しました。紹介IDを使わずに購入すると、新しい紹介IDが発行されます。")}</p>}
  </section>;
}
