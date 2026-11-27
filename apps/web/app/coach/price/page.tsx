"use client";
import { L } from "@/lib/i18n";

import { useAuth } from "@/components/auth-provider";
import { CoachOfferSection } from "@/components/coach-offer-form";
import { CoachSubpageShell } from "@/components/coach-subpage";
import { PageLoading } from "@/components/page-loading";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { apiFetch, ApiRequestError } from "@/lib/api";
import {
  coachPriceMinYen,
  coachPricePlanLabels,
  parseYenInput,
  type CoachPricePlan,
  type CoachPricePlanState,
  type CoachPrices,
} from "@/lib/coach-price";
import { formatDateTime, formatYen } from "@/lib/format";
import { useActionLock } from "@/lib/use-action-lock";
import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";

// 価格の申請（本体 #1052）。プラン（スタンダード／プレミアム）ごとに、いまの価格と申請の状態を見て、価格を申請する。
// 承認待ちのあいだは金額の上書きと取り下げができる。承認後はコーチからは変えられず、
// 承認の時点で対応中だった依頼がすべて終わってから新しい価格に切り替わる。
export default function CoachPricePage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [data, setData] = useState<CoachPrices | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!loading && !session) router.replace(`/login?next=${encodeURIComponent("/coach/price")}`);
  }, [loading, session, router]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<CoachPrices>("/v1/coach/me/prices", { token })
      .then((d) => { if (!cancelled) setData(d); })
      .catch((err) => {
        if (!cancelled) setError(err instanceof ApiRequestError && err.status === 403 ? L("承認されたコーチだけが価格を申請できます") : L("価格を読み込めませんでした"));
      });
    return () => { cancelled = true; };
  }, [token]);

  if (loading || !session || (data === null && !error)) return <PageLoading screen label={L("読み込み中...")} />;

  return (
    <CoachSubpageShell title={L("価格")}>
      {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
      {data ? (
        <>
          <p className="text-sm leading-relaxed text-slate-200">
            {L("プランごとに1回あたりの価格を申請できます（{v0}以上）。運営が承認したあと、承認の時点で対応中だった依頼がすべて終わってから新しい価格に切り替わります。進行中の依頼は申し込まれたときの価格のままです。", { v0: formatYen(data.min_price_yen ?? coachPriceMinYen) })}
          </p>
          {data.plans.map((plan) => (
            <PlanPrice key={plan.plan} state={plan} token={token} onChange={setData} />
          ))}
          {/* 添削項目・見本動画の申請（本体 #1086）。価格とは別に出せる。 */}
          <CoachOfferSection token={token} />
        </>
      ) : null}
    </CoachSubpageShell>
  );
}

function PlanPrice({ state, token, onChange }: { state: CoachPricePlanState; token: string | null | undefined; onChange: (d: CoachPrices) => void }) {
  const [value, setValue] = useState(state.pending ? String(state.pending.price_yen) : "");
  const [fieldError, setFieldError] = useState("");
  const [message, setMessage] = useState("");
  const { pending, acquire, release } = useActionLock();
  const label = coachPricePlanLabels[state.plan];

  async function submit(event: FormEvent) {
    event.preventDefault();
    setFieldError("");
    setMessage("");
    const price = parseYenInput(value);
    if (price === null || Number.isNaN(price)) { setFieldError(L("金額を整数で入力してください")); return; }
    if (price < coachPriceMinYen) { setFieldError(L("価格は500円以上で入力してください")); return; }
    if (!acquire()) return;
    try {
      const d = await apiFetch<CoachPrices>(`/v1/coach/me/prices/${state.plan}`, { method: "PUT", token, body: JSON.stringify({ price_yen: price }) });
      onChange(d);
      setMessage(state.pending ? L("申請の金額を変更しました") : L("価格を申請しました。運営の承認をお待ちください。"));
    } catch (err) {
      setFieldError(err instanceof ApiRequestError ? err.fields?.price_yen || err.message : L("申請できませんでした"));
    } finally {
      release();
    }
  }

  async function withdraw(plan: CoachPricePlan) {
    if (!acquire()) return;
    setFieldError("");
    setMessage("");
    try {
      const d = await apiFetch<CoachPrices>(`/v1/coach/me/prices/${plan}/withdraw`, { method: "POST", token });
      onChange(d);
      setValue("");
      setMessage(L("申請を取り下げました"));
    } catch (err) {
      setFieldError(err instanceof ApiRequestError ? err.message : L("取り下げられませんでした"));
    } finally {
      release();
    }
  }

  return (
    <section aria-label={label} className="space-y-3 rounded-xl border border-white/20 glass bg-white/10 p-3">
      <h2 className="font-semibold text-white">{label}</h2>
      <dl className="space-y-1 text-sm">
        <div className="flex justify-between gap-3">
          <dt className="text-slate-300">{L("いまの価格")}</dt>
          <dd data-testid={`current-price-${state.plan}`}>
            {state.price_yen != null ? formatYen(state.price_yen) : L("未設定")}
            {state.is_default ? L("（共通の価格）") : null}
          </dd>
        </div>
        {state.approved ? (
          <div className="flex justify-between gap-3">
            <dt className="text-slate-300">{L("切り替え待ち")}</dt>
            <dd>{formatYen(state.approved.price_yen)}</dd>
          </div>
        ) : null}
        {state.pending ? (
          <div className="flex justify-between gap-3">
            <dt className="text-slate-300">{L("承認待ち")}</dt>
            <dd>{formatYen(state.pending.price_yen)}</dd>
          </div>
        ) : null}
      </dl>
      {state.approved?.approved_at ? (
        <p className="text-xs text-slate-300">
          {L("{v0}に承認されました。承認の時点で対応中だった依頼がすべて終わると切り替わります。", { v0: formatDateTime(state.approved.approved_at) })}
        </p>
      ) : null}
      <form onSubmit={(e) => void submit(e)} className="space-y-2">
        <label className="block space-y-1 text-sm">
          <span>{state.pending ? L("申請の金額を変更する（円）") : L("新しい価格を申請する（円）")}</span>
          <Input inputMode="numeric" value={value} onChange={(e) => setValue(e.target.value)} aria-invalid={fieldError ? true : undefined} />
        </label>
        {fieldError ? <p role="alert" className="text-sm text-red-400">{fieldError}</p> : null}
        {message ? <p role="status" className="text-sm text-green-300">{message}</p> : null}
        <Button type="submit" className="w-full" disabled={pending}>
          {state.pending ? L("金額を変更する") : L("申請する")}
        </Button>
        {state.pending ? (
          <Button type="button" className="w-full bg-white/15 hover:bg-white/25" disabled={pending} onClick={() => void withdraw(state.plan)}>
            {L("申請を取り下げる")}
          </Button>
        ) : null}
      </form>
    </section>
  );
}
