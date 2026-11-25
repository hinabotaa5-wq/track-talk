"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@/components/auth-provider";
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
// 本体 #1009：読んだ値は「どの利用者の分か」と組で持ち、表示中の利用者と一致するときだけ返す（切替直後は未読込扱い）。
// キーは利用者 ID。session が取れない（Provider の外など）ときは token にフォールバックする。
// 履歴キャッシュ（bfcache）から戻ったとき（pageshow の persisted）にも取り直す。
const notLoaded = { loaded: false, referral: null };
function useMyReferral(token: string | null | undefined) {
  const { session } = useAuth();
  const userKey = token ? session?.user?.id ?? token : null;
  const [state, setState] = useState<{ key: string | null; loaded: boolean; referral: Referral | null }>({ key: null, ...notLoaded });
  useEffect(() => {
    if (!token || !userKey) return;
    let active = true;
    const load = () => {
      apiFetch<{ referral: Referral | null }>("/v1/me/coaching/referral", { token })
        .then((data) => { if (active) setState({ key: userKey, loaded: true, referral: data.referral ?? null }); })
        .catch(() => { if (active) setState({ key: userKey, ...notLoaded }); });
    };
    const restored = (event: PageTransitionEvent) => {
      if (event.persisted) load();
    };
    load();
    window.addEventListener("pageshow", restored);
    return () => {
      active = false;
      window.removeEventListener("pageshow", restored);
    };
  }, [token, userKey]);
  if (!userKey || state.key !== userKey) return notLoaded;
  return { loaded: state.loaded, referral: state.referral };
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

// 共有ボタンで送る、紹介の流れの公開ページ（本体 #996）。
const REFERRAL_SHARE_URL = "https://track-talk.netlify.app/referral";

// 紹介の訴求文面（本体 #1031）。特典は ¥500 クーポン。
const referralHeadline = () => L("2人に紹介すると500円分無料！");
const referralBody = () => L("購入すると、紹介IDが発行されます。そのIDで友達2人が購入すると、紹介してくれた方に¥500クーポンがプレゼントされます。");
// クーポンの条件。API の referralRewardTerms と同じ。
const couponTerms = () => L("¥500クーポン：付与日から6か月有効／付与された本人だけが使えます／現金への払戻しはできません／1回の申込に1枚まで");

export function LoadedReferralCard({ token, variant = "full" }: { token: string | null; variant?: "full" | "compact" }) {
  const { referral } = useMyReferral(token);
  if (referral) return <ReferralCard referral={referral} variant={variant} />;
  return null;
}

function ReferralCard({ referral, variant = "full" }: { referral: Referral; variant?: "full" | "compact" }) {
  const [copyStatus, setCopyStatus] = useState("");
  const { code, active, friend_count, granted_at, expires_at, reward_kind } = referral;
  // 発行済みの特典チケット（本体 #1031 より前の付与）は、発行時の条件のまま表示する。
  const legacyTicket = reward_kind === "ticket";
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
      {active ? <Button type="button" variant="secondary" onClick={async () => {
        const text = L("TrackTalkでコーチにフォーム添削してもらえるよ。購入のときに紹介ID「{v0}」を入れてね。", { v0: code });
        if (typeof navigator.share === "function") {
          try {
            await navigator.share({ text, url: REFERRAL_SHARE_URL });
            return;
          } catch (error) {
            // 利用者が共有シートを閉じたときは何もしない（本体 #996）。
            if ((error as { name?: unknown } | null)?.name === "AbortError") return;
          }
        }
        try {
          await navigator.clipboard.writeText(`${text}\n${REFERRAL_SHARE_URL}`);
          setCopyStatus(L("共有文をコピーしました"));
        } catch {
          setCopyStatus(L("共有できませんでした"));
        }
      }}>{L("共有")}</Button> : null}
    </div>
    {copyStatus ? <p role="status" className="text-xs">{copyStatus}</p> : null}
    {granted_at ? <p className="text-sm font-semibold">{legacyTicket ? L("紹介特典をプレゼントしました") : L("¥500クーポンをプレゼントしました")}</p>
      : active ? <p className="text-sm font-semibold">{friend_count === 1 ? L("あと1人で¥500クーポン") : L("あと2人で¥500クーポン")}</p> : null}
    {!compact && granted_at && expires_at ? <div className="space-y-1 text-xs">
      <p>{L("付与日 {v0}", { v0: formatDateTime(granted_at) })}</p>
      <p>{L("有効期限 {v0}", { v0: formatDateTime(expires_at) })}</p>
      <p>{legacyTicket ? L("特典チケット：付与日から6か月有効／付与された本人だけが使えます／払戻しはできません") : couponTerms()}</p>
    </div> : null}
    {compact || active ? null : <p className="text-sm">{L("このIDは終了しました。紹介IDを使わずに購入すると、新しい紹介IDが発行されます。")}</p>}
  </section>;
}
