"use client";
import { L } from "@/lib/i18n";

import { PublicAvailability } from "@/components/coach-availability";
import { hasAvailability } from "@/lib/coach-availability";
import { PageLoading } from "@/components/page-loading";

import { CoachAvatar } from "@/components/coach-photo";
import { CoachSnsEntry } from "@/components/coach-sns-entry";
import { CoachServicePlans } from "@/components/coach-service-plans";
import { apiFetch } from "@/lib/api";
import { coachingServiceLabels } from "@/lib/coaching-plan";
import { coachEventLabels } from "@/lib/coach-recruit";
import type { PublicCoach } from "@/lib/types";
import { useParams } from "next/navigation";
import { useEffect, useState } from "react";

export default function CoachDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [data, setData] = useState<{ id: string; coach: PublicCoach | null } | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    apiFetch<{ coaches: PublicCoach[] }>("/v1/coaches")
      .then(({ coaches }) => {
        if (!cancelled) {
          setError("");
          setData({ id, coach: coaches.find((coach) => coach.id === id) ?? null });
        }
      })
      .catch(() => {
        if (!cancelled) setError(L("コーチの詳細を読み込めませんでした"));
      });
    return () => { cancelled = true; };
  }, [id]);

  const coach = data?.id === id ? data.coach : undefined;

  return (
    <div className="mx-auto max-w-md space-y-6">
      {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : coach === undefined ? <PageLoading label={L("読み込み中...")} /> : coach === null ? <p className="text-sm text-slate-200">{L("このコーチは現在公開されていません。")}</p> : <>
        <section className="glass space-y-4 rounded-2xl border border-white/20 bg-white/10 p-4 text-sm" aria-label={L("コーチのプロフィール")}>
          <div className="flex items-center gap-4">
            <CoachAvatar url={coach.photo_url} name={coach.nickname} size={80} />
            <div className="min-w-0 space-y-1 break-words">
              <h1 className="text-xl font-bold text-white">{coach.nickname ?? L("名前なし")}</h1>
              <p className="text-xs text-slate-200">{coach.rating_count > 0 ? L("★{v0}（{rating_count}件）", { v0: coach.rating_average.toFixed(1), rating_count: coach.rating_count }) : L("評価はまだありません")}</p>
            </div>
          </div>
          <dl className="space-y-3">
            {[
              [L("担当種目"), coach.events.map((event) => coachEventLabels[event]).join(L("、"))],
              [L("記録"), coach.best_record],
              [L("成績や指導経験"), coach.achievements],
              [L("担当サービス"), (coach.service_types ?? []).map((service) => coachingServiceLabels[service]).join(L("・"))],
            ].filter(([, value]) => value).map(([label, value]) => <div key={label}>
              <dt className="text-xs text-slate-300">{label}</dt>
              <dd className="whitespace-pre-wrap break-words text-slate-100">{value}</dd>
            </div>)}
            {(coach.sns_urls ?? []).length ? <div>
              <dt className="text-xs text-slate-300">{L("SNS・ホームページ")}</dt>
              <dd className="space-y-1">
                {coach.sns_urls.map((entry) => <CoachSnsEntry key={entry} value={entry} />)}
              </dd>
            </div> : null}
          </dl>
        </section>
        {(coach.plans ?? []).some((plan) => hasAvailability(plan.service_type)) ? <PublicAvailability key={coach.id} coachID={coach.id} /> : null}
        <section className="space-y-3 text-sm" aria-labelledby="coach-plans">
          <h2 id="coach-plans" className="text-lg font-bold">{L("サポートプラン")}</h2>
          <CoachServicePlans key={coach.id} services={coach.service_types ?? []} plans={coach.plans ?? []} coachID={coach.id} consult={coach.support_consult} />
        </section>
      </>}
    </div>
  );
}
