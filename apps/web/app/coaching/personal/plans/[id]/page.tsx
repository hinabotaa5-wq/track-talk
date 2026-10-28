"use client";

import { PublicAvailability } from "@/components/coach-availability";
import { hasAvailability } from "@/lib/coach-availability";
import { PageLoading } from "@/components/page-loading";

import { useAuth } from "@/components/auth-provider";
import { CoachAvatar } from "@/components/coach-photo";
import { ContractConsentChecks, noConsents, type ContractConsents } from "@/components/coaching-contracts";
import { DemoCodeForm } from "@/components/coaching-demo";
import { PlanIncludesList, TrialBadge } from "@/components/coaching-plans";
import { SellerNote } from "@/components/seller-note";
import { CoachCommerceDisclosure } from "@/components/coach-commerce";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { coachEventLabels, coachTierLabels } from "@/lib/coach-recruit";
import { planSummary } from "@/lib/coaching-plan";
import type { CoachEvent, CoachingContract, CoachingMe, PublicCoachingPlan } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState, type FormEvent } from "react";

// パーソナル指導に申し込む。いまは招待コードを入れた人だけが申し込める。
// コーチが48時間以内に承諾し、72時間以内に支払う（18歳未満は保護者が同意して支払う）と期間が始まる。
// Stripe の鍵がない間は支払いなしのデモ版で、承諾したら始まる。

const chip = (on: boolean) =>
  `rounded-full border px-3 py-1.5 text-sm transition active:scale-[0.98] ${
    on ? "border-blue-400 bg-blue-500/30 text-white" : "border-white/25 bg-white/10 text-slate-200"
  }`;

export default function ApplyCoachingPlanPage() {
  const { id } = useParams<{ id: string }>();
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [data, setData] = useState<PublicCoachingPlan | null>(null);
  const [me, setMe] = useState<CoachingMe | null>(null);
  const [loadError, setLoadError] = useState("");
  const [event, setEvent] = useState<CoachEvent | null>(null);
  const [goal, setGoal] = useState("");
  const [record, setRecord] = useState("");
  const [message, setMessage] = useState("");
  const [preferredDates, setPreferredDates] = useState<string[]>([]);
  const [availabilityRevision, setAvailabilityRevision] = useState(0);
  const [consents, setConsents] = useState<ContractConsents>(noConsents);
  const [fields, setFields] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();

  useEffect(() => {
    if (!loading && !session) {
      router.replace(`/login?next=${encodeURIComponent(window.location.pathname)}`);
    }
  }, [loading, session, router]);

  useEffect(() => {
    let cancelled = false;
    apiFetch<PublicCoachingPlan>(`/v1/coaching/plans/${id}`)
      .then((d) => {
        if (!cancelled) setData(d);
      })
      .catch((err) => {
        if (!cancelled) setLoadError(err instanceof ApiRequestError ? err.message : "読み込みに失敗しました");
      });
    return () => {
      cancelled = true;
    };
  }, [id]);

  const loadMe = useCallback(() => {
    if (!token) return;
    apiFetch<CoachingMe>("/v1/me/coaching", { token })
      .then(setMe)
      .catch(() => setLoadError("読み込みに失敗しました"));
  }, [token]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<CoachingMe>("/v1/me/coaching", { token })
      .then((d) => {
        if (!cancelled) setMe(d);
      })
      .catch(() => {
        if (!cancelled) setLoadError("読み込みに失敗しました");
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!acquire()) return;
    setError("");
    setFields({});
    try {
      const d = await apiFetch<{ contract: CoachingContract }>("/v1/coaching/contracts", {
        method: "POST",
        token,
        body: JSON.stringify({ plan_id: id, event, goal, current_record: record, message, preferred_dates: hasAvailability(data?.plan.service_type) ? preferredDates : [], ...consents }),
      });
      router.push(`/coaching/personal/${d.contract.id}`);
    } catch (err) {
      if (err instanceof ApiRequestError && err.fields) {
        setFields(err.fields);
        if (err.fields.preferred_dates) {
          setPreferredDates([]);
          setAvailabilityRevision((value) => value + 1);
        }
      }
      else setError(err instanceof ApiRequestError ? err.message : "送信に失敗しました");
      release();
    }
  }

  if (loadError) return <p className="text-sm text-red-400">{loadError}</p>;
  if (loading || !session || !data || !me) return <PageLoading label="読み込み中..." />;

  const { plan, coach } = data;
  const fieldError = (key: string) => (fields[key] ? <p className="text-sm text-red-400">{fields[key]}</p> : null);

  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-2">
        {data.payments ? null : (
          <p className="inline-flex rounded-full border border-blue-400/40 bg-blue-500/15 px-2.5 py-0.5 text-xs font-semibold text-blue-300">
            デモ版
          </p>
        )}
        <h1 className="text-2xl font-bold">パーソナル指導に申し込む</h1>
        <SellerNote coachName={coach.nickname} />
        {data.commerce ? <CoachCommerceDisclosure commerce={data.commerce} /> : null}
      </div>

      <section className="space-y-3 rounded-2xl border border-white/20 glass bg-white/10 p-4 text-sm">
        <div className="flex items-center gap-3">
          <CoachAvatar url={coach.photo_url} name={coach.nickname} />
          <div>
            <p className="font-bold text-white">{coach.nickname ?? "名前なし"}</p>
            <p className="text-xs text-slate-300">{coachTierLabels[coach.tier]}</p>
          </div>
        </div>
        <div className="space-y-1 border-t border-white/15 pt-3">
          {plan.kind === "trial" ? <TrialBadge /> : null}
          <p className="font-semibold text-white">{plan.title}</p>
          <p className="text-xs text-slate-300">{planSummary(plan)}</p>
          <PlanIncludesList plan={plan} />
          <p className="whitespace-pre-wrap text-slate-200">{plan.description}</p>
          {plan.kind === "trial" ? (
            <p className="text-xs text-slate-300">このコーチの指導を前に受けたことがある人は、お試しには申し込めません。</p>
          ) : null}
        </div>
      </section>

      {hasAvailability(plan.service_type) && (plan.full || !me.demo || !me.birth_month) ? <PublicAvailability key={coach.id} coachID={coach.id} /> : null}

      {plan.full ? (
        <p className="rounded-xl border border-amber-400/40 bg-amber-500/15 p-3 text-sm text-white">
          このプランはいま満員です。空いたらまた申し込めます。
        </p>
      ) : !me.demo ? (
        <DemoCodeForm
          intro="パーソナル指導は準備中です。いまはデモ版として、運営から招待コードを受け取った人だけが申し込めます。"
          token={token}
          onJoined={loadMe}
        />
      ) : !me.birth_month ? (
        <div className="space-y-3">
          <p className="text-sm text-slate-200">
            先に生まれた年月を登録してください。18歳未満の人は、コーチが引き受けたあとに保護者の同意が必要です。
          </p>
          <Link
            href="/me/coaching"
            className="flex min-h-11 w-full items-center justify-center rounded-md bg-blue-600 px-4 text-sm font-semibold text-white"
          >
            生まれた年月を登録する
          </Link>
        </div>
      ) : (
        <form onSubmit={onSubmit} className="space-y-5">
          {hasAvailability(plan.service_type) ? <div className="space-y-2"><PublicAvailability key={`${coach.id}-${availabilityRevision}`} coachID={coach.id} selected={preferredDates} onChange={setPreferredDates} />{fieldError("preferred_dates")}</div> : null}
          <fieldset className="space-y-2">
            <legend className="text-sm font-semibold">種目</legend>
            <div className="flex flex-wrap gap-2">
              {coach.events.map((ev) => (
                <button key={ev} type="button" aria-pressed={event === ev} onClick={() => setEvent(ev)} className={chip(event === ev)}>
                  {coachEventLabels[ev]}
                </button>
              ))}
            </div>
            {fieldError("event")}
          </fieldset>
          <label className="block space-y-1.5 text-sm">
            <span className="font-semibold">目標（200文字まで）</span>
            <Input value={goal} maxLength={200} onChange={(e) => setGoal(e.target.value)} placeholder="例: 秋の大会で12秒台を出したい" />
            {fieldError("goal")}
          </label>
          <label className="block space-y-1.5 text-sm">
            <span className="font-semibold">いまの記録（100文字まで）</span>
            <Input value={record} maxLength={100} onChange={(e) => setRecord(e.target.value)} placeholder="例: 100m 13秒20" />
            {fieldError("current_record")}
          </label>
          <label className="block space-y-1.5 text-sm">
            <span className="font-semibold">相談したいこと（1000文字まで）</span>
            <Textarea
              rows={5}
              value={message}
              maxLength={1000}
              onChange={(e) => setMessage(e.target.value)}
              placeholder="例: スタートの出遅れが気になっています。練習は週4回です。"
            />
            <span className="block text-xs text-slate-300">連絡先（URL・SNS の ID・電話番号など）は書けません。</span>
            {fieldError("message")}
          </label>
          <ContractConsentChecks value={consents} onChange={setConsents} errors={fields} />
          <p className="text-xs text-slate-300">
            {!data.payments
              ? "コーチが48時間以内に引き受けると、その時から期間が始まります。デモ版なので支払いはありません。"
              : me.is_minor
                ? "コーチが48時間以内に引き受けたら、そのあとの画面で保護者のメールアドレスを入れてください。保護者が72時間以内に同意してお支払いをすると、その時から期間が始まります。始まってから72時間コーチの書き込みがなければ、全額を返金します。"
                : "コーチが48時間以内に引き受けたら、72時間以内にお支払いください（カード・Apple Pay・Google Pay・PayPay）。支払った時から期間が始まります。始まってから72時間コーチの書き込みがなければ、全額を返金します。"}
          </p>
          {error ? <p className="text-sm text-red-400">{error}</p> : null}
          <Button
            type="submit"
            className="w-full"
            disabled={pending || !event || !consents.agree_report || !consents.agree_detection}
          >
            申し込む
          </Button>
        </form>
      )}
    </div>
  );
}
