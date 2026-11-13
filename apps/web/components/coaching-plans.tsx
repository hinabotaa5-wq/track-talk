"use client";
import { L } from "@/lib/i18n";

import { CoachConsultSettings } from "@/components/coach-consult-settings";

import { Button } from "@/components/ui/button";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { coachPledgeItems, planIncludes, planSummary, sortTrialFirst } from "@/lib/coaching-plan";
import type { CoachApplication, CoachingPlan, PlanIncludes } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useEffect, useState } from "react";

// サポートプラン（1ヶ月プランのみ）。コーチ画面では誓約と受付設定をする。コーチ一覧では公開中のプランを見せる。
// 対面プラン・固定プランの新規作成と、担当サービスの選択は廃止した（2026-11-13）。

function CoachPledgeForm({ token, onSaved }: { token: string | null; onSaved: (c: CoachApplication) => void }) {
  const [agree, setAgree] = useState(false);
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();

  async function submit() {
    if (!acquire()) return;
    setError("");
    try {
      const d = await apiFetch<{ application: CoachApplication }>("/v1/coach/me/pledge", {
        method: "POST",
        token,
        body: JSON.stringify({ agree }),
      });
      onSaved(d.application);
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.fields?.agree || err.message : L("送信に失敗しました"));
    } finally {
      release();
    }
  }

  return (
    <div className="space-y-3">
      <p className="text-sm font-semibold text-white">{L("約束してください（利用者の多くは中高生です）")}</p>
      <ul className="list-disc space-y-1.5 pl-5 text-sm text-slate-100">
        {coachPledgeItems.map((item) => (
          <li key={item}>{L(item)}</li>
        ))}
      </ul>
      <label className="flex items-start gap-2 text-sm text-white">
        <input type="checkbox" className="mt-1 size-4" checked={agree} onChange={(e) => setAgree(e.target.checked)} />
        <span>{L("上のことを守ります")}</span>
      </label>
      <Button type="button" className="w-full" disabled={!agree || pending} onClick={() => void submit()}>{L("誓約する")}</Button>
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
    </div>
  );
}

// PlanIncludesList は、プランで「何が返ってくるか」。書いていない項目は出さない。
export function PlanIncludesList({ plan }: { plan: PlanIncludes }) {
  const rows = planIncludes(plan);
  if (rows.length === 0) return null;
  return (
    <dl className="mt-1 grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
      {rows.map((r) => (
        <div key={r.label} className="contents">
          <dt className="text-slate-300">{r.label}</dt>
          <dd className="text-slate-100">{r.value}</dd>
        </div>
      ))}
    </dl>
  );
}

// PublishedOldPlans は、廃止前に作って公開中のまま残っているプラン（従来のプラン・対面プランなど）。
// 新規作成・編集はできないが、公開中のものを止められるように「非公開にする」だけ残す。該当がなければ何も出さない。
function PublishedOldPlans({ token }: { token: string | null }) {
  const [plans, setPlans] = useState<CoachingPlan[]>([]);
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();

  useEffect(() => {
    let cancelled = false;
    apiFetch<{ plans: CoachingPlan[] }>("/v1/coach/me/plans", { token })
      .then((d) => {
        if (!cancelled) setPlans(d.plans);
      })
      .catch(() => {
        if (!cancelled) setError(L("プランを読み込めませんでした"));
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  async function unpublish(p: CoachingPlan) {
    if (!acquire()) return;
    setError("");
    try {
      const d = await apiFetch<{ plan: CoachingPlan }>(`/v1/coach/me/plans/${p.id}`, {
        method: "PATCH",
        token,
        body: JSON.stringify({
          kind: p.kind,
          service_type: p.service_type,
          additional_consultation_price_yen: p.additional_consultation_price_yen,
          duration_days: p.duration_days,
          title: p.title,
          description: p.description,
          price_yen: p.price_yen,
          capacity: p.capacity,
          published: false,
          competition_name: p.competition_name, competition_date: p.competition_date, support_start_date: p.support_start_date, call_count: p.call_count, call_minutes: p.call_minutes,
          venue_area: p.venue_area, venue_name: p.venue_name, session_count: p.session_count, session_minutes: p.session_minutes, travel_fee_yen: p.travel_fee_yen, max_participants: p.max_participants,
          video_reviews: p.video_reviews,
          menu_updates: p.menu_updates,
          reply_formats: p.reply_formats,
          start_on_first_video: p.start_on_first_video,
        }),
      });
      setPlans((list) => list.map((x) => (x.id === d.plan.id ? d.plan : x)));
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : L("保存できませんでした"));
    } finally {
      release();
    }
  }

  const published = plans.filter((p) => p.published && p.service_type !== "video_review");
  if (published.length === 0 && !error) return null;
  return (
    <section aria-label={L("公開中の以前のプラン")} className="space-y-2 rounded-xl border border-white/20 glass bg-white/10 p-3 text-sm">
      {published.length ? (
        <>
          <h2 className="font-bold">{L("公開中の以前のプラン")}</h2>
          <p className="text-xs text-slate-300">{L("新しいプランは作れません。非公開にできます。")}</p>
          <ul className="space-y-2">
            {published.map((p) => (
              <li key={p.id} className="flex items-center justify-between gap-2">
                <div className="min-w-0">
                  <p className="font-semibold text-white">{p.title}</p>
                  <p className="text-xs text-slate-300">{planSummary(p)}</p>
                </div>
                <button
                  type="button"
                  disabled={pending}
                  onClick={() => void unpublish(p)}
                  className="min-h-11 shrink-0 rounded-md border border-white/25 bg-white/10 px-4 text-sm font-semibold text-slate-100 transition active:scale-[0.98] disabled:opacity-50"
                >{L("非公開にする")}</button>
              </li>
            ))}
          </ul>
        </>
      ) : null}
      {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
    </section>
  );
}

// CoachPlansSection は、コーチ画面の「サポートプラン」（1ヶ月プランのみ）。誓約がまだなら誓約から。
export function CoachPlansSection({
  coach,
  token,
  onCoachChange,
}: {
  coach: CoachApplication;
  token: string | null;
  onCoachChange: (c: CoachApplication) => void;
}) {
  return (
    <section className="space-y-3">
      <p className="text-sm text-slate-200">{L("1ヶ月プランの受付設定です。")}</p>
      {!coach.pledged ? (
        <div className="rounded-xl border border-white/20 glass bg-white/10 p-3">
          <CoachPledgeForm token={token} onSaved={onCoachChange} />
        </div>
      ) : (
        <>
          {coach.pledged_current === false ? <div className="space-y-2 rounded-xl border border-amber-300/40 bg-amber-500/10 p-3"><p className="text-sm">{L("誓約の文面が新しくなりました。もう一度同意してください。")}</p><CoachPledgeForm token={token} onSaved={onCoachChange} /></div> : null}
          <CoachConsultSettings coach={coach} token={token} onCoachChange={onCoachChange} />
          <PublishedOldPlans token={token} />
        </>
      )}
    </section>
  );
}

// TrialBadge は、お試しプランにつける印。
export function TrialBadge() {
  return (
    <p className="mb-1 inline-flex rounded-full bg-blue-500/30 px-2 py-0.5 text-xs font-semibold text-blue-100">{L("はじめての人だけのお試し")}</p>
  );
}

// PublicPlanList は、コーチ詳細に出す公開中のサポートプラン。
export function PublicPlanList({ plans }: { plans: CoachingPlan[] }) {
  if (plans.length === 0) return null;
  return (
    <div className="space-y-1.5 border-t border-white/15 pt-2">
      <p className="text-xs text-slate-300">{L("公開中のプラン")}</p>
      <ul className="space-y-2">
        {sortTrialFirst(plans).map((p) => (
          <li key={p.id} className={`rounded-lg p-2 ${p.kind === "trial" ? "border border-blue-400/40 bg-blue-500/10" : "bg-white/5"}`}>
            {p.kind === "trial" ? <TrialBadge /> : null}
            <p className="font-semibold text-white">{p.title}</p>
            <p className="text-xs text-slate-300">{planSummary(p)}</p>
            <PlanIncludesList plan={p} />
            <p className="mt-1 whitespace-pre-wrap text-slate-200">{p.description}</p>
            {p.full ? (
              <p className="mt-2 text-xs font-semibold text-amber-300">{L("いまは満員です")}</p>
            ) : (
              <Link
                href={`/coaching/personal/plans/${p.id}`}
                className="mt-2 flex min-h-11 w-full items-center justify-center rounded-md bg-blue-600 px-4 text-sm font-semibold text-pure-white"
              >{L("このプランに申し込む")}</Link>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}
