"use client";

import { useAuth } from "@/components/auth-provider";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { coachApplyPath, coachEventLabels, coachTierLabels } from "@/lib/coach-recruit";
import type { CoachApplication, CoachingMe } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

// コーチ画面。承認されたコーチが「受付中」と1日の上限本数を決める。
// 依頼が回ってくる仕組み（打診と返答）はこの後の PR で足す。

const capOptions = [1, 2, 3, 5, 8, 10, 15, 20];

export default function CoachPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [coach, setCoach] = useState<CoachApplication | null | undefined>(undefined);
  const [devices, setDevices] = useState(0);
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
    Promise.all([
      apiFetch<{ application: CoachApplication | null }>("/v1/coach/me", { token }),
      apiFetch<CoachingMe>("/v1/me/coaching", { token }),
    ])
      .then(([c, me]) => {
        if (cancelled) return;
        setCoach(c.application);
        setDevices(me.push_devices);
      })
      .catch(() => {
        if (!cancelled) setError("読み込みに失敗しました");
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  async function save(next: { on_duty?: boolean; daily_cap?: number }) {
    if (!acquire()) return;
    setError("");
    try {
      const d = await apiFetch<{ application: CoachApplication }>("/v1/coach/me", {
        method: "PATCH",
        token,
        body: JSON.stringify(next),
      });
      setCoach(d.application);
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.fields?.daily_cap || err.message : "保存できませんでした");
    } finally {
      release();
    }
  }

  if (error && coach === undefined) return <p className="text-sm text-red-400">{error}</p>;
  if (loading || !session || coach === undefined) return <p className="text-sm text-slate-200">読み込み中...</p>;

  if (!coach || coach.status !== "approved") {
    return (
      <div className="mx-auto max-w-md space-y-4">
        <h1 className="text-2xl font-bold">コーチ画面</h1>
        <p className="text-sm text-slate-200">承認されたコーチだけが使えます。</p>
        <Link href={coachApplyPath} className="text-sm text-blue-300 underline">
          応募の画面へ
        </Link>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-1">
        <h1 className="text-2xl font-bold">コーチ画面</h1>
        <p className="text-sm text-slate-300">
          {coachTierLabels[coach.tier]} · {coach.events.map((e) => coachEventLabels[e]).join("、")}
        </p>
      </div>

      <section className="space-y-3 rounded-2xl border border-white/20 glass bg-white/10 p-4">
        <div className="flex items-center justify-between gap-3">
          <div>
            <p className="text-lg font-bold">{coach.on_duty ? "受付中" : "受付停止中"}</p>
            <p className="text-xs text-slate-300">受付中の間だけ、添削の依頼が通知で届きます。</p>
          </div>
          <button
            type="button"
            role="switch"
            aria-checked={coach.on_duty}
            aria-label="受付中"
            disabled={pending}
            onClick={() => void save({ on_duty: !coach.on_duty })}
            className={`relative h-8 w-14 shrink-0 rounded-full transition disabled:opacity-50 ${coach.on_duty ? "bg-blue-600" : "bg-white/25"}`}
          >
            <span
              className={`absolute top-1 size-6 rounded-full bg-white transition-all ${coach.on_duty ? "left-7" : "left-1"}`}
            />
          </button>
        </div>
        <label className="flex items-center justify-between gap-3 text-sm">
          <span>1日に受ける上限</span>
          <select
            className="rounded-md border border-white/25 bg-white/10 px-3 py-2 text-base text-slate-100"
            value={coach.daily_cap}
            disabled={pending}
            onChange={(e) => void save({ daily_cap: Number(e.target.value) })}
          >
            {capOptions.map((n) => (
              <option key={n} value={n}>
                {n}本
              </option>
            ))}
          </select>
        </label>
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
      </section>

      <section className="space-y-2">
        <h2 className="font-bold">通知</h2>
        <p className="text-sm text-slate-300">
          依頼は通知で届き、30分以内に引き受けないと次のコーチに回ります。通知を受け取れる端末: {devices}台
        </p>
        <Link href="/me/coaching" className="text-sm text-blue-300 underline">
          通知の設定
        </Link>
      </section>
    </div>
  );
}
