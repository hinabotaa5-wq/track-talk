"use client";
import { L } from "@/lib/i18n";

import { useAuth } from "@/components/auth-provider";
import { CoachPhotoForm } from "@/components/coach-photo";
import { CoachSubpageShell } from "@/components/coach-subpage";
import { PageLoading } from "@/components/page-loading";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { coachApplyPath, coachEventLabels, coachEvents } from "@/lib/coach-recruit";
import type { CoachApplication, CoachEvent } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState, type FormEvent } from "react";

// 顔写真と応募内容の確認。承認済みのコーチはここでプロフィールを自分で変えられる（本体 #649）。
// 変更はすぐに公開へ反映され、運営へ通知が送られる（再審査はしない）。
// ランチャーのタイルから開く。
type ProfileDraft = {
  name: string;
  events: CoachEvent[];
  best_record: string;
  achievements: string;
  sns_urls: string;
  hitokoto: string;
};

const emptyProfileDraft: ProfileDraft = { name: "", events: [], best_record: "", achievements: "", sns_urls: "", hitokoto: "" };

const chip = (on: boolean) =>
  `rounded-full border px-3 py-1.5 text-sm transition active:scale-[0.98] ${
    on ? "border-blue-400 bg-blue-500/30 text-pure-white" : "border-white/25 bg-white/10 text-slate-200"
  }`;

function draftFrom(a: CoachApplication): ProfileDraft {
  return {
    name: a.name ?? "",
    events: a.events,
    best_record: a.best_record,
    achievements: a.achievements,
    sns_urls: (a.sns_urls ?? []).join("\n"),
    hitokoto: a.hitokoto ?? "",
  };
}

export default function CoachProfilePage() {
  const { session, profile, token, loading } = useAuth();
  const router = useRouter();
  const [coach, setCoach] = useState<CoachApplication | null | undefined>(undefined);
  const [draft, setDraft] = useState<ProfileDraft>(emptyProfileDraft);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();
  // 入力中の内容を、トークンの更新による読み直しで消さないよう、フォームを埋めるのは初回の読み込みだけにする。
  const draftFilled = useRef(false);

  useEffect(() => {
    if (!loading && !session) {
      router.replace(`/login?next=${encodeURIComponent("/coach/profile")}`);
    }
  }, [loading, session, router]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<{ application: CoachApplication | null }>("/v1/coach/me", { token })
      .then((d) => {
        if (cancelled) return;
        setCoach(d.application);
        if (d.application && d.application.status === "approved" && !draftFilled.current) {
          setDraft(draftFrom(d.application));
        }
        draftFilled.current = true;
      })
      .catch(() => {
        if (!cancelled) setCoach(null);
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
      const sns_urls = draft.sns_urls.split("\n").map((v) => v.trim()).filter(Boolean);
      const missing: Record<string, string> = {};
      if (!draft.name.trim()) missing.name = L("名前を入力してください");
      if (draft.events.length === 0) missing.events = L("担当種目を1つ以上選んでください");
      if (!draft.best_record.trim()) missing.best_record = L("記録を入力してください");
      if (!draft.achievements.trim()) missing.achievements = L("成績や指導経験を入力してください");
      if (sns_urls.length === 0) missing.sns_urls = L("SNS・ホームページを1件以上入力してください");
      if (Object.keys(missing).length > 0) {
        setFieldErrors(missing);
        return;
      }
      const d = await apiFetch<{ application: CoachApplication }>("/v1/coach/me/profile", {
        method: "PUT",
        token,
        body: JSON.stringify({ ...draft, sns_urls }),
      });
      setCoach(d.application);
      setDraft(draftFrom(d.application));
      setMessage(L("プロフィールを更新しました"));
    } catch (err) {
      if (err instanceof ApiRequestError) {
        if (err.fields) setFieldErrors(err.fields);
        else setError(err.message);
      } else {
        setError(L("送信に失敗しました"));
      }
    } finally {
      release();
    }
  }

  if (loading || !session || coach === undefined) return <PageLoading screen label={L("読み込み中...")} />;

  const fieldError = (key: string) =>
    fieldErrors[key] ? <p className="text-sm text-red-400">{fieldErrors[key]}</p> : null;

  return (
    <CoachSubpageShell title={L("プロフィール")}>
      {coach ? (
        <>
          <CoachPhotoForm
            application={coach}
            nickname={coach.name || profile?.nickname || null}
            token={token}
            onSaved={setCoach}
          />
          <Link href={coachApplyPath} className="block text-sm text-blue-300 underline">{L("応募内容を確認する")}</Link>
          {coach.status === "approved" ? (
            <form onSubmit={onSubmit} className="space-y-5">
              <label className="block space-y-1 text-sm">
                <span className="font-semibold">{L("名前")}</span>
                <Input value={draft.name} maxLength={80} autoComplete="name" required
                  onChange={(e) => setDraft((d) => ({ ...d, name: e.target.value }))} />
                {fieldError("name")}
              </label>
              <fieldset className="space-y-2">
                <legend className="text-sm font-semibold">{L("担当種目(複数可)")}</legend>
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
                <span className="font-semibold">{L("記録")}</span>
                <Input
                  value={draft.best_record}
                  required
                  maxLength={200}
                  onChange={(e) => setDraft((d) => ({ ...d, best_record: e.target.value }))}
                />
                {fieldError("best_record")}
              </label>

              <label className="block space-y-1 text-sm">
                <span className="font-semibold">{L("成績や指導経験")}</span>
                <Textarea
                  rows={3}
                  value={draft.achievements}
                  required
                  maxLength={500}
                  onChange={(e) => setDraft((d) => ({ ...d, achievements: e.target.value }))}
                />
                {fieldError("achievements")}
              </label>

              <label className="block space-y-1 text-sm">
                <span className="font-semibold">{L("SNS・ホームページ")}</span>
                <Textarea
                  rows={3}
                  required
                  value={draft.sns_urls}
                  onChange={(e) => setDraft((d) => ({ ...d, sns_urls: e.target.value }))}
                />
                <span className="block text-xs text-slate-300">{L("SNSのアカウント名やホームページのURLなどを自由に入力できます。1行に1件ずつ、1〜3件（各300文字以内）入力してください。公開後はコーチの紹介に表示されます。")}</span>
                {fieldError("sns_urls")}
              </label>

              <label className="block space-y-1 text-sm">
                <span className="font-semibold">{L("ひとこと")}</span>
                <Input
                  value={draft.hitokoto}
                  maxLength={100}
                  onChange={(e) => setDraft((d) => ({ ...d, hitokoto: e.target.value }))}
                />
                <span className="block text-xs text-slate-300">{L("選手へのひとことを入力できます（任意・100文字以内）。公開後はコーチの紹介に表示されます。")}</span>
                {fieldError("hitokoto")}
              </label>

              {message ? (
                <div className="flex items-center gap-3 rounded-xl border border-green-400/40 bg-green-500/15 p-4">
                  <p className="text-sm font-semibold text-green-300">{message}</p>
                </div>
              ) : null}
              {error ? <p className="text-sm text-red-400">{error}</p> : null}
              <Button type="submit" className="w-full" disabled={pending}>
                {L("プロフィールを保存する")}
              </Button>
            </form>
          ) : null}
        </>
      ) : (
        <p className="text-sm text-red-400">{L("プロフィールを読み込めませんでした")}</p>
      )}
    </CoachSubpageShell>
  );
}
