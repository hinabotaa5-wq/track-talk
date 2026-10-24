"use client";

import { useAuth } from "@/components/auth-provider";
import { CoachPhotoForm, uploadCoachPhoto } from "@/components/coach-photo";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { coachEventLabels, coachEvents, coachStatusLabels, coachTierLabels } from "@/lib/coach-recruit";
import { coachingEnabled } from "@/lib/features";
import { formatDateTime } from "@/lib/format";
import type { CoachApplication, CoachEvent, CoachTier } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState, type FormEvent } from "react";

// コーチの応募。SNSまたはホームページは最低1件必要。
// 運営が審査し、結果はこの画面と通知で伝える。

type Draft = {
  name: string;
  tier: CoachTier;
  events: CoachEvent[];
  best_record: string;
  achievements: string;
  sns_urls: string;
};

const emptyDraft: Draft = { name: "", tier: "coach", events: [], best_record: "", achievements: "", sns_urls: "" };

const chip = (on: boolean) =>
  `rounded-full border px-3 py-1.5 text-sm transition active:scale-[0.98] ${
    on ? "border-blue-400 bg-blue-500/30 text-white" : "border-white/25 bg-white/10 text-slate-200"
  }`;

const statusNotes: Record<CoachApplication["status"], string> = {
  pending: "運営が内容を確認しています。審査中は内容を書き直せます。",
  approved: coachingEnabled ? "応募が承認されました。コーチ画面で受付を始められます。" : "応募が承認されました。コーチングは準備中です。活動の開始は公開後にご案内します。",
  rejected: "今回は見送りになりました。内容を見直して、もう一度応募できます。",
  suspended: "コーチの活動を停止しています。詳しくはお問い合わせください。",
};

function draftFrom(a: CoachApplication): Draft {
  return {
    name: a.name ?? "",
    tier: "coach",
    events: a.events,
    best_record: a.best_record,
    achievements: a.achievements,
    sns_urls: (a.sns_urls ?? []).join("\n"),
  };
}

export default function CoachApplyPage() {
  const { session, profile, token, loading } = useAuth();
  const router = useRouter();
  const [application, setApplication] = useState<CoachApplication | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [draft, setDraft] = useState<Draft>(emptyDraft);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();
  // 入力中の内容を、トークンの更新による読み直しで消さないよう、フォームを埋めるのは初回の読み込みだけにする。
  const draftFilled = useRef(false);

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
        if (d.application && !draftFilled.current) setDraft(draftFrom(d.application));
        draftFilled.current = true;
        setLoaded(true);
      })
      .catch(() => {
        if (!cancelled) setError("読み込みに失敗しました");
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  const [photo, setPhoto] = useState<File | null>(null);

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
      const sns_urls = draft.sns_urls.split("\n").map((v) => v.trim()).filter(Boolean);
      const missing: Record<string, string> = {};
      if (!draft.name.trim()) missing.name = "名前を入力してください";
      if (draft.events.length === 0) missing.events = "担当種目を1つ以上選んでください";
      if (!draft.best_record.trim()) missing.best_record = "記録を入力してください";
      if (!draft.achievements.trim()) missing.achievements = "成績や指導経験を入力してください";
      if (sns_urls.length === 0) missing.sns_urls = "SNSまたはホームページのURLを1件以上入力してください";
      if (!photo && !application?.photo_url) missing.photo = "顔写真を選んでください";
      if (Object.keys(missing).length > 0) {
        setFieldErrors(missing);
        return;
      }
      const d = await apiFetch<{ application: CoachApplication }>("/v1/coach/applications", {
        method: "POST",
        token,
        body: JSON.stringify({ ...draft, sns_urls }),
      });
      setApplication(d.application);
      setDraft(draftFrom(d.application));
      const done = application?.status === "pending" ? "応募内容を更新しました" : "応募しました。審査の結果をお待ちください。";
      if (photo) {
        try {
          setApplication(await uploadCoachPhoto(photo, token));
          setPhoto(null);
          setMessage(done);
        } catch (err) {
          setMessage(`応募は受け付けましたが、顔写真を登録できませんでした（${err instanceof Error ? err.message : "エラー"}）。下の「顔写真」からもう一度選んでください。`);
        }
      } else {
        setMessage(done);
      }
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
          {application.status === "pending" && coachingEnabled ? (
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

      {application && (application.status === "pending" || application.status === "approved") ? (
        <CoachPhotoForm application={application} nickname={application.name || profile?.nickname || null} token={token} onSaved={setApplication} />
      ) : null}

      {error ? <p className="text-sm text-red-400">{error}</p> : null}

      {editable ? (
        <form onSubmit={onSubmit} className="space-y-5">
          <p className="text-sm text-slate-200">
            コーチの仕事内容や報酬など、コーチ機能の詳細は
            <Link href="/coach/about" replace className="text-blue-300 underline">
              コーチ機能について
            </Link>
            で確認できます。
          </p>
          <label className="block space-y-1 text-sm">
            <span className="font-semibold">名前</span>
            <Input value={draft.name} maxLength={80} autoComplete="name" required
              onChange={(e) => setDraft((d) => ({ ...d, name: e.target.value }))} />
            {fieldError("name")}
          </label>
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
              required
              maxLength={200}
              placeholder="例: 100m 10秒95(2025年)"
              onChange={(e) => setDraft((d) => ({ ...d, best_record: e.target.value }))}
            />
            {fieldError("best_record")}
          </label>

          <label className="block space-y-1 text-sm">
            <span className="font-semibold">成績や指導経験</span>
            <Textarea
              rows={3}
              value={draft.achievements}
              required
              maxLength={500}
              placeholder="例: インターハイ 100m 準決勝"
              onChange={(e) => setDraft((d) => ({ ...d, achievements: e.target.value }))}
            />
            {fieldError("achievements")}
          </label>

          <label className="block space-y-1 text-sm">
            <span className="font-semibold">SNS・ホームページ</span>
            <Textarea
              rows={3}
              required
              value={draft.sns_urls}
              placeholder={"例:\nhttps://www.instagram.com/example/\nhttps://example-club.jp/"}
              onChange={(e) => setDraft((d) => ({ ...d, sns_urls: e.target.value }))}
            />
            <span className="block text-xs text-slate-300">InstagramなどのSNS、または所属するクラブチームのホームページを1件以上入力してください。1行に1つずつ、最大3件まで。公開後はコーチの紹介に表示されます。</span>
            {fieldError("sns_urls")}
          </label>

          {!application?.photo_url ? (
            <label className="block space-y-1 text-sm">
              <span className="font-semibold">顔写真</span>
              <input
                className="block w-full text-sm text-slate-300 file:mr-3 file:rounded-full file:border-0 file:bg-white/15 file:px-3 file:py-2 file:text-sm file:font-semibold file:text-slate-100"
                type="file"
                required
                accept="image/jpeg,image/png,image/webp,image/heic,image/heif"
                onChange={(e) => setPhoto(e.target.files?.[0] ?? null)}
                disabled={pending}
              />
              <span className="block text-xs text-slate-300">顔がはっきり写った写真にしてください。コーチ一覧に載ります。</span>
              {fieldError("photo")}
            </label>
          ) : null}
          {message ? (
            <div className="flex items-center gap-3 rounded-xl border border-green-400/40 bg-green-500/15 p-4">
              <svg className="size-6 shrink-0 text-green-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.5} strokeLinecap="round" strokeLinejoin="round">
                <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
                <polyline points="22 4 12 14.01 9 11.01" />
              </svg>
              <p className="text-sm font-semibold text-green-300">{message}</p>
            </div>
          ) : null}
          <Button type="submit" className="w-full" disabled={pending || (!photo && !application?.photo_url)}>
            {application?.status === "pending" ? "応募内容を更新する" : "応募する"}
          </Button>
        </form>
      ) : application ? (
        <dl className="space-y-2 rounded-xl border border-white/20 glass bg-white/10 p-3 text-sm">
          <div>
            <dt className="text-xs text-slate-300">名前</dt>
            <dd>{application.name || profile?.nickname || "未登録"}</dd>
          </div>
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
            <dt className="text-xs text-slate-300">成績や指導経験</dt>
            <dd className="whitespace-pre-wrap">{application.achievements}</dd>
          </div>
          {(application.sns_urls ?? []).length ? (
            <div>
              <dt className="text-xs text-slate-300">SNS・ホームページ</dt>
              <dd className="space-y-1">
                {application.sns_urls.map((url) => (
                  <a key={url} href={url} target="_blank" rel="noopener noreferrer" className="block break-all text-blue-300 underline">
                    {url}
                  </a>
                ))}
              </dd>
            </div>
          ) : null}
        </dl>
      ) : null}
    </div>
  );
}
