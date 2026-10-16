"use client";

import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { UploadProgress } from "@/components/upload-progress";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { coachEventLabels, coachEvents, coachTierLabels } from "@/lib/coach-recruit";
import { uploadToCloudinary, type CloudinarySignature, type CloudinaryUploadProgress } from "@/lib/cloudinary-upload";
import { maxReviewSeconds, reviewFocusLabels, reviewFocusOptions, tooLong, videoDuration } from "@/lib/review";
import type { CoachEvent, CoachingMe, CoachTier, ReviewFocus, ReviewRequest } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";

// フォーム添削の依頼を送る。決済ができるまではデモ版で、招待コードを入れた人だけが使える。

const maxBytes = 100 * 1024 * 1024;

const chip = (on: boolean) =>
  `rounded-full border px-3 py-1.5 text-sm transition active:scale-95 ${
    on ? "border-blue-400 bg-blue-500/30 text-white" : "border-white/25 bg-white/10 text-slate-200"
  }`;

function DemoCodeForm({ token, onJoined }: { token: string | null; onJoined: () => void }) {
  const [code, setCode] = useState("");
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (!acquire()) return;
    setError("");
    try {
      await apiFetch("/v1/coaching/demo", { method: "POST", token, body: JSON.stringify({ code }) });
      onJoined();
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.fields?.code || err.message : "送信に失敗しました");
    } finally {
      release();
    }
  }

  return (
    <form onSubmit={onSubmit} className="space-y-3">
      <p className="text-sm text-slate-200">
        フォーム添削は準備中です。いまはデモ版として、運営から招待コードを受け取った人だけが使えます。
      </p>
      <Input
        value={code}
        onChange={(e) => setCode(e.target.value)}
        placeholder="招待コード(8文字)"
        autoCapitalize="characters"
        autoComplete="off"
        aria-label="招待コード"
      />
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
      <Button type="submit" className="w-full" disabled={pending || code.trim() === ""}>
        デモに参加する
      </Button>
    </form>
  );
}

export default function NewReviewRequestPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [me, setMe] = useState<CoachingMe | null>(null);
  const [loadError, setLoadError] = useState("");
  const [tier, setTier] = useState<CoachTier>("coach");
  const [event, setEvent] = useState<CoachEvent | "">("");
  const [focus, setFocus] = useState<ReviewFocus[]>([]);
  const [file, setFile] = useState<File | null>(null);
  const [error, setError] = useState("");
  const [upload, setUpload] = useState<CloudinaryUploadProgress | null>(null);
  const [uploadPhase, setUploadPhase] = useState<"upload" | "saving">("upload");
  const { pending, acquire, release } = useActionLock();

  useEffect(() => {
    if (!loading && !session) {
      router.replace(`/login?next=${encodeURIComponent(window.location.pathname)}`);
    }
  }, [loading, session, router]);

  async function loadMe() {
    try {
      setMe(await apiFetch<CoachingMe>("/v1/me/coaching", { token }));
    } catch {
      setLoadError("読み込みに失敗しました");
    }
  }

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

  function toggleFocus(value: ReviewFocus) {
    setFocus((current) => (current.includes(value) ? current.filter((f) => f !== value) : [...current, value]));
  }

  async function onPickFile(picked: File | null) {
    setError("");
    setFile(picked);
    if (!picked) return;
    if (tooLong(await videoDuration(picked))) {
      setError(`動画は${maxReviewSeconds}秒以内にしてください`);
      setFile(null);
    }
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!event) {
      setError("種目を選んでください");
      return;
    }
    if (focus.length === 0) {
      setError("見てほしい点を1つ以上選んでください");
      return;
    }
    if (!file) {
      setError("動画を選んでください");
      return;
    }
    if (file.size > maxBytes) {
      setError("100MB以内の動画にしてください");
      return;
    }
    if (!acquire()) return;
    setError("");
    try {
      const signed = await apiFetch<CloudinarySignature>("/v1/uploads/coaching-video", { method: "POST", token });
      setUploadPhase("upload");
      setUpload({ loaded: 0, total: file.size, percent: 0 });
      const payload = await uploadToCloudinary("video", file, signed, setUpload);
      if (payload.public_id !== signed.public_id) throw new Error("アップロードに失敗しました");
      setUploadPhase("saving");
      const created = await apiFetch<ReviewRequest>("/v1/coaching/requests", {
        method: "POST",
        token,
        body: JSON.stringify({ tier, event, focus, video_public_id: payload.public_id }),
      });
      router.push(`/coaching/requests/${created.id}`);
    } catch (err) {
      if (err instanceof ApiRequestError) {
        const field = err.fields ? Object.values(err.fields)[0] : "";
        setError(field || err.message);
      } else {
        setError(err instanceof Error ? err.message : "送信に失敗しました");
      }
      setUpload(null);
      release();
    }
  }

  if (loading || !session || (!me && !loadError)) {
    return <p className="text-sm text-slate-200">読み込み中...</p>;
  }
  if (!me) {
    return <p className="text-sm text-red-400">{loadError}</p>;
  }

  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-2">
        <p className="inline-flex rounded-full border border-blue-400/40 bg-blue-500/15 px-2.5 py-0.5 text-xs font-semibold text-blue-300">
          デモ版
        </p>
        <h1 className="text-2xl font-bold">フォーム添削を頼む</h1>
      </div>

      {!me.demo ? (
        <DemoCodeForm token={token} onJoined={() => void loadMe()} />
      ) : !me.birth_month ? (
        <div className="space-y-3">
          <p className="text-sm text-slate-200">先に生まれた年月を登録してください。18歳未満の人は保護者の同意が必要です。</p>
          <Link
            href="/me/coaching"
            className="flex min-h-11 w-full items-center justify-center rounded-md bg-blue-600 px-4 text-sm font-semibold text-white"
          >
            生まれた年月を登録する
          </Link>
        </div>
      ) : (
        <form onSubmit={onSubmit} className="space-y-5">
          <fieldset className="space-y-2">
            <legend className="text-sm font-semibold">誰に見てもらうか</legend>
            <div className="flex flex-wrap gap-2">
              {(Object.keys(coachTierLabels) as CoachTier[]).map((t) => (
                <button key={t} type="button" aria-pressed={tier === t} onClick={() => setTier(t)} className={chip(tier === t)}>
                  {coachTierLabels[t]}
                </button>
              ))}
            </div>
          </fieldset>

          <label className="block space-y-1 text-sm">
            <span className="font-semibold">種目</span>
            <select
              className="w-full rounded-md border border-white/25 glass bg-white/10 px-3 py-2 text-base text-slate-100 outline-none focus:border-blue-400"
              value={event}
              onChange={(e) => setEvent(e.target.value as CoachEvent | "")}
            >
              <option value="">選んでください</option>
              {coachEvents.map((value) => (
                <option key={value} value={value}>
                  {coachEventLabels[value]}
                </option>
              ))}
            </select>
          </label>

          <fieldset className="space-y-2">
            <legend className="text-sm font-semibold">見てほしい点(複数可)</legend>
            <div className="flex flex-wrap gap-2">
              {reviewFocusOptions.map((value) => (
                <button
                  key={value}
                  type="button"
                  aria-pressed={focus.includes(value)}
                  onClick={() => toggleFocus(value)}
                  className={chip(focus.includes(value))}
                >
                  {reviewFocusLabels[value]}
                </button>
              ))}
            </div>
          </fieldset>

          <label className="block space-y-1 text-sm">
            <span className="font-semibold">動画({maxReviewSeconds}秒まで)</span>
            <input
              className="block w-full text-sm text-slate-300 file:mr-3 file:rounded-full file:border-0 file:bg-white/15 file:px-3 file:py-2 file:text-sm file:font-semibold file:text-slate-100"
              type="file"
              accept="video/mp4,video/quicktime,video/webm"
              onChange={(e) => void onPickFile(e.target.files?.[0] ?? null)}
              disabled={pending}
            />
            <span className="block text-xs text-slate-300">
              ユニフォームの学校名やゼッケンなど、個人がわかるものが写らないようにしてください。動画はコーチだけが見られます。
            </span>
          </label>

          {upload ? (
            <UploadProgress noun="動画" phase={uploadPhase} progress={upload} fileName={file?.name} fileSize={file?.size} />
          ) : null}
          {error ? <p className="text-sm text-red-400">{error}</p> : null}
          <p className="text-xs text-slate-300">デモ版のため料金はかかりません。返ってくる目安は送った翌日の12時です。</p>
          <Button type="submit" className="w-full" disabled={pending || !file}>
            {pending ? "送っています" : "送る"}
          </Button>
        </form>
      )}
    </div>
  );
}
