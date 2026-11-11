"use client";
import { L } from "@/lib/i18n";

import Link from "next/link";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { formatYen } from "@/lib/format";
import { payoutAccountStatus, payoutAccountStatusLabels } from "@/lib/payout-account-status";
import type { AdminCoachPayout, AdminPayoutCoach, CoachEarnings, CoachPayoutStatus } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import { useCallback, useEffect, useState } from "react";

// コーチへの報酬。コーチの画面の「報酬」と、運営の「送金」。
// 支払いは TrackTalk が受け取り、月に1回、前の月までの報酬をまとめて Stripe でコーチのアカウントへ送金する。
// 合計が最低送金額に届かない分は、累積が届くまで翌月以降に繰り越す（取引が終わったコーチには届かなくても送る）。

const payoutStatusLabels: Record<CoachPayoutStatus, string> = {
  get pending() { return L("送金待ち"); },
  get paid() { return L("送金済み"); },
  get failed() { return L("送金できませんでした"); },
};

function monthLabel(month: string): string {
  const [y, m] = month.split("-");
  return L("{y}年{v1}月分", { y, v1: Number(m) });
}

// CoachEarningsSection は、コーチの画面の「報酬」。口座の登録と、報酬・送金の記録。
export function CoachEarningsSection({ token, onReady }: { token: string | null; onReady: () => void }) {
  const [data, setData] = useState<CoachEarnings | null>(null);
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<CoachEarnings>("/v1/coach/me/earnings", { token })
      .then((d) => {
        if (cancelled) return;
        setData(d);
        if (d.payouts_enabled) onReady();
      })
      .catch(() => {
        if (!cancelled) setError(L("報酬を読み込めませんでした"));
      });
    return () => {
      cancelled = true;
    };
  }, [token, onReady]);

  async function onboard() {
    if (!acquire()) return;
    setError("");
    try {
      const d = await apiFetch<{ url: string }>("/v1/coach/stripe/onboarding", { method: "POST", token });
      window.location.assign(d.url);
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : L("登録の画面を開けませんでした"));
      release();
    }
  }

  if (!data) return error ? <p className="text-sm text-red-400">{error}</p> : null;
  if (!data.payments) return null;

  const [year, month, day] = data.next_payout_date?.split("-") ?? [];

  return (
    <section className="space-y-3 rounded-2xl border border-white/20 glass bg-white/10 p-4">
      <h2 className="font-bold">{L("報酬")}</h2>
      {payoutAccountStatus(data) === "registered" ? (
        <div className="space-y-2 rounded-xl border border-emerald-400/40 bg-emerald-500/15 p-3">
          <p>
            <span className="inline-block rounded-full border border-emerald-300/50 bg-emerald-500/25 px-3 py-1 text-sm font-bold text-white">{L("● 受け取り口座: {registered}", { registered: payoutAccountStatusLabels.registered })}</span>
          </p>
          <p className="text-sm text-white">{L("報酬は登録済みの口座に振り込まれます。")}</p>
        </div>
      ) : (
        <div className="space-y-2 rounded-xl border border-amber-400/40 bg-amber-500/15 p-3">
          <p>
            <span className="inline-block rounded-full border border-amber-300/50 bg-amber-500/25 px-3 py-1 text-sm font-bold text-white">{L("● 受け取り口座: {v0}", { v0: payoutAccountStatusLabels[payoutAccountStatus(data)] })}</span>
          </p>
          <p className="text-sm text-white">{L("報酬は Stripe を通して銀行口座に振り込みます。受付を始めたり、パーソナル指導の申し込みを承諾したりする前に、本人確認と口座を登録してください。")}</p>
          {data.stripe_linked ? (
            <p className="text-xs text-slate-200">{L("登録の途中か、Stripe で確認中です。済んでいなければ続きから登録できます。")}</p>
          ) : null}
          <button
            type="button"
            disabled={pending}
            onClick={() => void onboard()}
            className="min-h-11 w-full rounded-md bg-blue-600 px-4 text-sm font-semibold text-pure-white transition active:scale-[0.98] disabled:opacity-50"
          >
            {data.stripe_linked ? L("登録を続ける") : L("受け取り口座を登録する")}
          </button>
          <Link href="/coach/payout-guide" className="block text-center text-sm text-blue-300 underline">{L("登録の手順を見る（模式図付き・10分ほど）")}</Link>
        </div>
      )}
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
      <div className="grid grid-cols-2 gap-2 text-center">
        <div className="rounded-xl border border-white/15 bg-white/5 p-2">
          <p className="text-xs text-slate-300">{L("今月の報酬")}</p>
          <p className="text-lg font-bold">{formatYen(data.this_month_yen)}</p>
        </div>
        <div className="rounded-xl border border-white/15 bg-white/5 p-2">
          <p className="text-xs text-slate-300">{L("まだ送金していない分")}</p>
          <p className="text-lg font-bold">{formatYen(data.unpaid_yen)}</p>
        </div>
      </div>
      {data.next_payout_date && data.next_payout_yen != null ? (
        <div className="space-y-1 rounded-xl border border-white/15 bg-white/5 p-3 text-sm">
          <p>{L("次回のお支払い予定日：{year}年{v1}月{v2}日", { year, v1: Number(month), v2: Number(day) })}</p>
          <p>{L("お支払い予定額：{v0}", { v0: formatYen(data.next_payout_yen) })}</p>
          <p className="text-xs text-slate-300">{L("金額と日付は予定です。返金の精算や口座の状態によって変わることがあります。")}</p>
        </div>
      ) : null}
      <div className="space-y-2 text-xs text-slate-300">
        <p>{L("動画添削の報酬は1本1,050円、添削後の追加相談は1件420円です（いずれも税込）。動画添削は返却時、追加相談は回答時に報酬が確定します。")}</p>
        <p>{L("サポートプランの受取額は、代金から手数料{personal_fee}%を差し引いた額です。プランの期間が終了した時に確定します。途中で返金して終了した場合は、その時点で確定します。", { personal_fee: data.personal_fee })}</p>
        <p>{L("確定した報酬は月末で締め、翌月15日にStripeを通じて登録済みの銀行口座へ振り込みます。各締切時点の支払額が{v0}未満の場合は翌月以降に繰り越し、累積した支払額が{v0}以上となった締切月の翌月15日に支払います。コーチとの取引が終了した場合は、金額にかかわらず、終了月の翌月15日に未払額の全額を支払います。", { v0: formatYen(data.min_payout_yen) })}</p>
      </div>
      {data.payouts.length > 0 ? (
        <ul className="space-y-1 text-sm">
          {data.payouts.map((p) => (
            <li key={p.id} className="flex justify-between gap-2 border-t border-white/10 pt-1">
              <span>{L("{v0}まで", { v0: monthLabel(p.month) })}</span>
              <span>
                {formatYen(p.amount_yen)} · {payoutStatusLabels[p.status]}
              </span>
            </li>
          ))}
        </ul>
      ) : null}
    </section>
  );
}

// PayoutsAdmin は、運営の「送金」。月に1回押して前の月までの報酬を送る。
export function PayoutsAdmin({ token }: { token: string | null }) {
  const [payments, setPayments] = useState(true);
  const [coaches, setCoaches] = useState<AdminPayoutCoach[]>([]);
  const [payouts, setPayouts] = useState<AdminCoachPayout[]>([]);
  const [message, setMessage] = useState("");
  const { pending, acquire, release } = useActionLock();

  const load = useCallback(async () => {
    try {
      const d = await apiFetch<{ payments: boolean; coaches: AdminPayoutCoach[]; payouts: AdminCoachPayout[] }>(
        "/v1/admin/coaching/payouts",
        { token },
      );
      setPayments(d.payments);
      setCoaches(d.coaches);
      setPayouts(d.payouts);
    } catch {
      setMessage(L("取得に失敗しました"));
    }
  }, [token]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  async function run() {
    if (!confirm(L("前の月までの報酬を、口座を登録したコーチに送金しますか？前に失敗した送金もやり直します。"))) return;
    if (!acquire()) return;
    setMessage("");
    try {
      const d = await apiFetch<{ created: number; paid: number; failed: number }>("/v1/admin/coaching/payouts/run", {
        method: "POST",
        token,
      });
      setMessage(L("送金しました: {paid}件{v1}", { paid: d.paid, v1: d.failed > 0 ? L("、できなかったもの {failed}件（Stripe の残高を確かめて、もう一度押してください）", { failed: d.failed }) : "" }));
      await load();
    } catch (err) {
      setMessage(err instanceof ApiRequestError ? err.message : L("送金できませんでした"));
    } finally {
      release();
    }
  }

  return (
    <div className="space-y-4">
      {!payments ? <p className="text-sm text-slate-200">{L("いまは支払いのないデモ版なので、送金はありません。")}</p> : null}
      <button
        type="button"
        disabled={pending || !payments}
        onClick={() => void run()}
        className="min-h-11 w-full rounded-md bg-blue-600 px-4 text-sm font-semibold text-pure-white transition active:scale-[0.98] disabled:opacity-50"
      >{L("前の月までの報酬を送金する")}</button>
      <p className="text-xs text-slate-300">{L("毎月1日から14日のあいだに押してください。コーチの銀行口座への入金は毎月15日です。前の月までの分（繰り越した分を含む累積）が3,000円に届かないコーチは、届くまで翌月以降に繰り越します（承認済みでなくなったコーチには、届かなくても送ります）。")}</p>
      {message ? <p className="text-sm text-slate-100">{message}</p> : null}
      <section className="space-y-2">
        <h2 className="text-sm font-semibold">{L("コーチ")}</h2>
        <ul className="space-y-2">
          {coaches.map((c) => (
            <li key={c.profile_id} className="space-y-1 rounded-xl border border-white/20 bg-white/10 p-3 text-sm">
              <p className="font-semibold text-white">{c.nickname ?? L("名前なし")}</p>
              <p className="text-xs text-slate-300">{L("口座: {v0} · まだ送金していない分: {v1}", { v0: payoutAccountStatusLabels[payoutAccountStatus(c)], v1: formatYen(c.unpaid_yen) })}</p>
            </li>
          ))}
        </ul>
      </section>
      <section className="space-y-2">
        <h2 className="text-sm font-semibold">{L("送金の記録")}</h2>
        {payouts.length === 0 ? <p className="text-sm text-slate-300">{L("まだありません")}</p> : null}
        <ul className="space-y-1 text-sm">
          {payouts.map((p) => (
            <li key={p.id} className="border-t border-white/10 pt-1">
              <p>{L("{v0} · {v1}まで · {v2} · {v3}", { v0: p.nickname ?? L("退会したコーチ"), v1: monthLabel(p.month), v2: formatYen(p.amount_yen), v3: payoutStatusLabels[p.status] })}</p>
              {p.failure ? <p className="text-xs text-red-300">{p.failure}</p> : null}
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}
