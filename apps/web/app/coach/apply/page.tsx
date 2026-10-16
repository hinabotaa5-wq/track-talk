"use client";

import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { coachEventLabels, coachEvents, coachStatusLabels, coachTierLabels } from "@/lib/coach-recruit";
import { formatDateTime } from "@/lib/format";
import type { CoachApplication, CoachEvent, CoachTier } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";

// コーチ（先輩を含む）の応募。集めるのは担当種目・記録・成績・競技年数だけ。
// 運営が審査し、結果はこの画面と通知で伝える。

type Draft = {
  tier: CoachTier;
  events: CoachEvent[];
  best_record: string;
  achievements: string;
  years: string;
};

const emptyDraft: Draft = { tier: "coach", events: [], best_record: "", achievements: "", years: "" };

const chip = (on: boolean) =>
  `rounded-full border px-3 py-1.5 text-sm transition active:scale-95 ${
    on ? "border-blue-400 bg-blue-500/30 text-white" : "border-white/25 bg-white/10 text-slate-200"
  }`;

const statusNotes: Record<CoachApplication["status"], string> = {
  pending: "運営が内容を確認しています。審査中は内容を書き直せます。",
  approved: "応募が承認されました。コーチ画面で受付を始められます。",
  rejected: "今回は見送りになりました。内容を見直して、もう一度応募できます。",
  suspended: "コーチの活動を停止しています。詳しくはお問い合わせください。",
};

function draftFrom(a: CoachApplication): Draft {
  return {
    tier: a.tier,
    events: a.events,
    best_record: a.best_record,
    achievements: a.achievements,
    years: String(a.years),
  };
}

export default function CoachApplyPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [application, setApplication] = useState<CoachApplication | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [draft, setDraft] = useState<Draft>(emptyDraft);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();

  useEffect(() => {
    if (!loading && !session) {
      router.replace(`/login?next=${encodeURIComponent(window.location.pathname)}`);
    }
  }, [loading, session, router]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<{ application: CoachApplication | null }>("/v1/coach/me", { token })
      .then((d) => {
        if (cancelled) return;
        setApplication(d.application);
        if (d.application) setDraft(draftFrom(d.application));
        setLoaded(true);
      })
      .catch(() => {
        if (!cancelled) setError("読み込みに失敗しました");
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  function toggleEvent(group: CoachEvent) {
    setDraft((d) => ({
      ...d,
      events: d.events.includes(group) ? d.events.filter((g) => g !== group) : [...d.events, group],
    }));
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (!acquire()) return;
    setFieldErrors({});
    setMessage("");
    setError("");
    try {
      const years = draft.years.trim() === "" ? null : Number(draft.years);
      const d = await apiFetch<{ application: CoachApplication }>("/v1/coach/applications", {
        method: "POST",
        token,
        body: JSON.stringify({ ...draft, years }),
      });
      setApplication(d.application);
      setDraft(draftFrom(d.application));
      setMessage(application?.status === "pending" ? "応募内容を更新しました" : "応募しました。審査の結果をお待ちください。");
    } catch (err) {
      if (err instanceof ApiRequestError) {
        if (err.fields) setFieldErrors(err.fields);
        else setError(err.message);
      } else {
        setError("送信に失敗しました");
      }
    } finally {
      release();
    }
  }

  if (loading || !session || (!loaded && !error)) {
    return <p className="text-sm text-slate-200">読み込み中...</p>;
  }

  const editable = !application || application.status === "pending" || application.status === "rejected";
  const fieldError = (key: string) =>
    fieldErrors[key] ? <p className="text-sm text-red-400">{fieldErrors[key]}</p> : null;

  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-2">
        <h1 className="text-2xl font-bold">コーチに応募する</h1>
        <p className="text-sm text-slate-200">運営が内容を確認して、結果をこの画面でお知らせします。</p>
      </div>

      {application ? (
        <section className="space-y-1 rounded-xl border border-blue-400/40 bg-blue-500/15 p-3">
          <p className="text-sm font-semibold text-white">
            {coachStatusLabels[application.status]}
            <span className="ml-2 text-xs font-normal text-slate-300">応募日 {formatDateTime(application.applied_at)}</span>
          </p>
          <p className="text-sm text-slate-100">{statusNotes[application.status]}</p>
          {application.status === "approved" ? (
            <Link href="/coach" className="text-sm text-blue-300 underline">
              コーチ画面へ
            </Link>
          ) : null}
          {application.status === "pending" ? (
            <p className="text-xs text-slate-300">
              結果を通知で受け取るには、
              <Link href="/me/coaching" className="text-blue-300 underline">
                通知の設定
              </Link>
              をしてください。
            </p>
          ) : null}
        </section>
      ) : null}

      {error ? <p className="text-sm text-red-400">{error}</p> : null}

      {editable ? (
        <form onSubmit={onSubmit} className="space-y-5">
          <fieldset className="space-y-2">
            <legend className="text-sm font-semibold">応募する枠</legend>
            <div className="flex flex-wrap gap-2">
              {(Object.keys(coachTierLabels) as CoachTier[]).map((tier) => (
                <button
                  key={tier}
                  type="button"
                  aria-pressed={draft.tier === tier}
                  onClick={() => setDraft((d) => ({ ...d, tier }))}
                  className={chip(draft.tier === tier)}
                >
                  {coachTierLabels[tier]}
                </button>
              ))}
            </div>
            {fieldError("tier")}
          </fieldset>

          <fieldset className="space-y-2">
            <legend className="text-sm font-semibold">担当種目(複数可)</legend>
            <div className="flex flex-wrap gap-2">
              {coachEvents.map((group) => (
                <button
                  key={group}
                  type="button"
                  aria-pressed={draft.events.includes(group)}
                  onClick={() => toggleEvent(group)}
                  className={chip(draft.events.includes(group))}
                >
                  {coachEventLabels[group]}
                </button>
              ))}
            </div>
            {fieldError("events")}
          </fieldset>

          <label className="block space-y-1 text-sm">
            <span className="font-semibold">記録</span>
            <Input
              value={draft.best_record}
              maxLength={200}
              placeholder="例: 100m 10秒95(2025年)"
              onChange={(e) => setDraft((d) => ({ ...d, best_record: e.target.value }))}
            />
            {fieldError("best_record")}
          </label>

          <label className="block space-y-1 text-sm">
            <span className="font-semibold">成績</span>
            <Textarea
              rows={3}
              value={draft.achievements}
              maxLength={500}
              placeholder="例: インターハイ 100m 準決勝"
              onChange={(e) => setDraft((d) => ({ ...d, achievements: e.target.value }))}
            />
            {fieldError("achievements")}
          </label>

          <label className="block space-y-1 text-sm">
            <span className="font-semibold">競技年数</span>
            <div className="flex items-center gap-2">
              <Input
                type="number"
                inputMode="numeric"
                min={0}
                max={60}
                className="w-24"
                value={draft.years}
                onChange={(e) => setDraft((d) => ({ ...d, years: e.target.value }))}
              />
              <span className="text-slate-200">年</span>
            </div>
            {fieldError("years")}
          </label>

          {message ? <p className="text-sm text-blue-400">{message}</p> : null}
          <Button type="submit" className="w-full" disabled={pending}>
            {application?.status === "pending" ? "応募内容を更新する" : "応募する"}
          </Button>
        </form>
      ) : application ? (
        <dl className="space-y-2 rounded-xl border border-white/20 glass bg-white/10 p-3 text-sm">
          <div>
            <dt className="text-xs text-slate-300">枠</dt>
            <dd>{coachTierLabels[application.tier]}</dd>
          </div>
          <div>
            <dt className="text-xs text-slate-300">担当種目</dt>
            <dd>{application.events.map((g) => coachEventLabels[g]).join("、")}</dd>
          </div>
          <div>
            <dt className="text-xs text-slate-300">記録</dt>
            <dd className="whitespace-pre-wrap">{application.best_record}</dd>
          </div>
          <div>
            <dt className="text-xs text-slate-300">成績</dt>
            <dd className="whitespace-pre-wrap">{application.achievements}</dd>
          </div>
          <div>
            <dt className="text-xs text-slate-300">競技年数</dt>
            <dd>{application.years}年</dd>
          </div>
        </dl>
      ) : null}
    </div>
  );
}
