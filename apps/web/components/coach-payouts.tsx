"use client";

import { apiFetch, ApiRequestError } from "@/lib/api";
import { formatDateTime, formatYen } from "@/lib/format";
import type { AdminCoachPayout, AdminPayoutCoach, CoachEarnings, CoachPayoutStatus } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import { useCallback, useEffect, useState } from "react";

// コーチへの報酬。コーチの画面の「報酬」と、運営の「送金」。
// 支払いは TrackTalk が受け取り、月に1回、前の月までの報酬をまとめて Stripe でコーチのアカウントへ送金する。

const payoutStatusLabels: Record<CoachPayoutStatus, string> = {
  pending: "送金待ち",
  paid: "送金済み",
  failed: "送金できませんでした",
};

function monthLabel(month: string): string {
  const [y, m] = month.split("-");
  return `${y}年${Number(m)}月分`;
}

// CoachEarningsSection は、コーチの画面の「報酬」。口座の登録と、報酬・送金の記録。
export function CoachEarningsSection({ token, onReady }: { token: string | null; onReady: () => void }) {
  const [data, setData] = useState<CoachEarnings | null>(null);
  const [error, setError] = useState("");
  const [now] = useState(() => Date.now());
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
        if (!cancelled) setError("報酬を読み込めませんでした");
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
      setError(err instanceof ApiRequestError ? err.message : "登録の画面を開けませんでした");
      release();
    }
  }

  if (!data) return error ? <p className="text-sm text-red-400">{error}</p> : null;
  if (!data.payments) return null;

  const reduced = data.reduced_fee_until && new Date(data.reduced_fee_until).getTime() > now;
  return (
    <section className="space-y-3 rounded-2xl border border-white/20 glass bg-white/10 p-4">
      <h2 className="font-bold">報酬</h2>
      {data.payouts_enabled ? (
        <p className="text-sm text-slate-200">受け取り口座: 登録済み</p>
      ) : (
        <div className="space-y-2 rounded-xl border border-amber-400/40 bg-amber-500/15 p-3">
          <p className="text-sm text-white">
            報酬は Stripe を通して銀行口座に振り込みます。受付を始めたり、パーソナル指導の申し込みを承諾したりする前に、本人確認と口座を登録してください。
          </p>
          {data.stripe_linked ? (
            <p className="text-xs text-slate-200">登録の途中か、Stripe で確認中です。済んでいなければ続きから登録できます。</p>
          ) : null}
          <button
            type="button"
            disabled={pending}
            onClick={() => void onboard()}
            className="min-h-11 w-full rounded-md bg-blue-600 px-4 text-sm font-semibold text-white transition active:scale-95 disabled:opacity-50"
          >
            {data.stripe_linked ? "登録を続ける" : "受け取り口座を登録する"}
          </button>
        </div>
      )}
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
      <div className="grid grid-cols-2 gap-2 text-center">
        <div className="rounded-xl border border-white/15 bg-white/5 p-2">
          <p className="text-xs text-slate-300">今月の報酬</p>
          <p className="text-lg font-bold">{formatYen(data.this_month_yen)}</p>
        </div>
        <div className="rounded-xl border border-white/15 bg-white/5 p-2">
          <p className="text-xs text-slate-300">まだ送金していない分</p>
          <p className="text-lg font-bold">{formatYen(data.unpaid_yen)}</p>
        </div>
      </div>
      <p className="text-xs text-slate-300">
        取り分は、フォーム添削が代金の{data.review_share}%、パーソナル指導が手数料{reduced ? 10 : data.personal_fee}%を引いた分
        {reduced ? `（初年10%、${formatDateTime(data.reduced_fee_until!)}までに始まった指導）` : ""}
        です。添削は返却した時、パーソナル指導は期間が終わった時に記録し、前の月までの分を毎月まとめて送金します。
      </p>
      {data.payouts.length > 0 ? (
        <ul className="space-y-1 text-sm">
          {data.payouts.map((p) => (
            <li key={p.id} className="flex justify-between gap-2 border-t border-white/10 pt-1">
              <span>{monthLabel(p.month)}まで</span>
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

// PayoutsAdmin は、運営の「送金」。月に1回押して前の月までの報酬を送り、コーチに「初年10%」を付ける。
export function PayoutsAdmin({ token }: { token: string | null }) {
  const [payments, setPayments] = useState(true);
  const [coaches, setCoaches] = useState<AdminPayoutCoach[]>([]);
  const [payouts, setPayouts] = useState<AdminCoachPayout[]>([]);
  const [message, setMessage] = useState("");
  const [now] = useState(() => Date.now());
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
      setMessage("取得に失敗しました");
    }
  }, [token]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  async function run() {
    if (!window.confirm("前の月までの報酬を、口座を登録したコーチに送金しますか？前に失敗した送金もやり直します。")) return;
    if (!acquire()) return;
    setMessage("");
    try {
      const d = await apiFetch<{ created: number; paid: number; failed: number }>("/v1/admin/coaching/payouts/run", {
        method: "POST",
        token,
      });
      setMessage(`送金しました: ${d.paid}件${d.failed > 0 ? `、できなかったもの ${d.failed}件（Stripe の残高を確かめて、もう一度押してください）` : ""}`);
      await load();
    } catch (err) {
      setMessage(err instanceof ApiRequestError ? err.message : "送金できませんでした");
    } finally {
      release();
    }
  }

  async function toggleFee(c: AdminPayoutCoach, enabled: boolean) {
    const ask = enabled
      ? `${c.nickname ?? "このコーチ"}さんに「初年10%」を付けますか？今から1年のあいだに始まったパーソナル指導の手数料が10%になります。`
      : `${c.nickname ?? "このコーチ"}さんの「初年10%」を外しますか？`;
    if (!window.confirm(ask)) return;
    if (!acquire()) return;
    try {
      await apiFetch(`/v1/admin/coaching/coaches/${c.profile_id}/reduced-fee`, {
        method: "POST",
        token,
        body: JSON.stringify({ enabled }),
      });
      await load();
    } catch (err) {
      setMessage(err instanceof ApiRequestError ? err.message : "変えられませんでした");
    } finally {
      release();
    }
  }

  return (
    <div className="space-y-4">
      {!payments ? <p className="text-sm text-slate-200">いまは支払いのないデモ版なので、送金はありません。</p> : null}
      <button
        type="button"
        disabled={pending || !payments}
        onClick={() => void run()}
        className="min-h-11 w-full rounded-md bg-blue-600 px-4 text-sm font-semibold text-white transition active:scale-95 disabled:opacity-50"
      >
        前の月までの報酬を送金する
      </button>
      {message ? <p className="text-sm text-slate-100">{message}</p> : null}
      <section className="space-y-2">
        <h2 className="text-sm font-semibold">コーチ</h2>
        <ul className="space-y-2">
          {coaches.map((c) => {
            const reduced = c.reduced_fee_until && new Date(c.reduced_fee_until).getTime() > now;
            return (
              <li key={c.profile_id} className="space-y-1 rounded-xl border border-white/20 bg-white/10 p-3 text-sm">
                <p className="font-semibold text-white">{c.nickname ?? "名前なし"}</p>
                <p className="text-xs text-slate-300">
                  口座: {c.payouts_enabled ? "登録済み" : c.stripe_linked ? "登録の途中" : "未登録"} · まだ送金していない分: {formatYen(c.unpaid_yen)}
                </p>
                <label className="flex items-center gap-2 text-xs text-slate-200">
                  <input type="checkbox" checked={Boolean(reduced)} disabled={pending} onChange={(e) => void toggleFee(c, e.target.checked)} />
                  初年10%{reduced ? `（${formatDateTime(c.reduced_fee_until!)}まで）` : ""}
                </label>
              </li>
            );
          })}
        </ul>
      </section>
      <section className="space-y-2">
        <h2 className="text-sm font-semibold">送金の記録</h2>
        {payouts.length === 0 ? <p className="text-sm text-slate-300">まだありません</p> : null}
        <ul className="space-y-1 text-sm">
          {payouts.map((p) => (
            <li key={p.id} className="border-t border-white/10 pt-1">
              <p>
                {p.nickname ?? "退会したコーチ"} · {monthLabel(p.month)}まで · {formatYen(p.amount_yen)} · {payoutStatusLabels[p.status]}
              </p>
              {p.failure ? <p className="text-xs text-red-300">{p.failure}</p> : null}
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}
