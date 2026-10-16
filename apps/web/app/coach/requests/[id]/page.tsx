"use client";

import { useAuth } from "@/components/auth-provider";
import { ReviewResponseView } from "@/components/review-response";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { UploadProgress } from "@/components/upload-progress";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { coachEventLabels, coachTierLabels } from "@/lib/coach-recruit";
import { uploadToCloudinary, type CloudinarySignature, type CloudinaryUploadProgress } from "@/lib/cloudinary-upload";
import { formatDateTime } from "@/lib/format";
import {
  clarifyReasonLabels,
  maxResponseChars,
  maxReviewSeconds,
  reviewFocusLabels,
  tooLong,
  videoDuration,
} from "@/lib/review";
import type { CoachReviewRequest } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";

// 引き受けた依頼に返答を書くコーチの画面。「わからなかった」と言われたら、ここから補足を1回返す。

const maxBytes = 100 * 1024 * 1024;

function Field({
  label,
  value,
  onChange,
  error,
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  error?: string;
  placeholder?: string;
}) {
  const over = value.length > maxResponseChars;
  return (
    <label className="block space-y-1 text-sm">
      <span className="font-semibold">{label}</span>
      <Textarea rows={3} value={value} onChange={(e) => onChange(e.target.value)} placeholder={placeholder} aria-invalid={Boolean(error) || over} />
      <span className={`block text-right text-xs ${over ? "text-red-400" : "text-slate-300"}`}>
        {value.length}/{maxResponseChars}
      </span>
      {error ? <span className="block text-sm text-red-400">{error}</span> : null}
    </label>
  );
}

function fieldErrors(err: unknown): Record<string, string> {
  if (err instanceof ApiRequestError) return err.fields ?? { form: err.message };
  return { form: err instanceof Error ? err.message : "送信に失敗しました" };
}

function ResponseForm({ request, token, onSent }: { request: CoachReviewRequest; token: string | null; onSent: (r: CoachReviewRequest) => void }) {
  const [good, setGood] = useState("");
  const [fix1, setFix1] = useState("");
  const [fix2, setFix2] = useState("");
  const [drill, setDrill] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [upload, setUpload] = useState<CloudinaryUploadProgress | null>(null);
  const { pending, acquire, release } = useActionLock();

  async function onPickFile(picked: File | null) {
    setErrors({});
    setFile(picked);
    if (!picked) return;
    if (picked.size > maxBytes) {
      setErrors({ demo_video_public_id: "100MB以内の動画にしてください" });
      setFile(null);
    } else if (tooLong(await videoDuration(picked))) {
      setErrors({ demo_video_public_id: `見本動画は${maxReviewSeconds}秒以内にしてください` });
      setFile(null);
    }
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!acquire()) return;
    setErrors({});
    try {
      let demo = "";
      if (file) {
        const signed = await apiFetch<CloudinarySignature>("/v1/uploads/coaching-video", { method: "POST", token });
        setUpload({ loaded: 0, total: file.size, percent: 0 });
        const payload = await uploadToCloudinary("video", file, signed, setUpload);
        if (payload.public_id !== signed.public_id) throw new Error("アップロードに失敗しました");
        demo = payload.public_id;
      }
      const sent = await apiFetch<CoachReviewRequest>(`/v1/coaching/requests/${request.id}/response`, {
        method: "POST",
        token,
        body: JSON.stringify({ good_point: good, fix_points: [fix1, fix2], drill, demo_video_public_id: demo }),
      });
      onSent(sent);
    } catch (err) {
      setErrors(fieldErrors(err));
    } finally {
      setUpload(null);
      release();
    }
  }

  return (
    <form onSubmit={onSubmit} className="space-y-4">
      <h2 className="font-bold">返答を書く</h2>
      <p className="text-xs text-slate-300">
        URL・メールアドレス・電話番号・SNS の ID など、連絡先らしきものは送れません。直接のやりとりに誘うと利用停止になります。
      </p>
      <p className="text-xs text-slate-300">返答と一緒に、ニックネーム・担当種目・記録・成績・競技年数が利用者に届きます。</p>
      <Field label="良い点（1つ）" value={good} onChange={setGood} error={errors.good_point} />
      <Field label="直す点（1つ目）" value={fix1} onChange={setFix1} error={errors.fix_points} />
      <Field label="直す点（2つ目、なくてもよい）" value={fix2} onChange={setFix2} />
      <Field label="おすすめのドリル（1つ）" value={drill} onChange={setDrill} error={errors.drill} />
      <label className="block space-y-1 text-sm">
        <span className="font-semibold">見本動画（なくてもよい、{maxReviewSeconds}秒まで）</span>
        <input
          className="block w-full text-sm text-slate-300 file:mr-3 file:rounded-full file:border-0 file:bg-white/15 file:px-3 file:py-2 file:text-sm file:font-semibold file:text-slate-100"
          type="file"
          accept="video/mp4,video/quicktime,video/webm"
          onChange={(e) => void onPickFile(e.target.files?.[0] ?? null)}
          disabled={pending}
        />
        {errors.demo_video_public_id ? <span className="block text-sm text-red-400">{errors.demo_video_public_id}</span> : null}
      </label>
      {upload && file ? <UploadProgress noun="動画" phase="upload" progress={upload} fileName={file.name} fileSize={file.size} /> : null}
      {errors.form ? <p className="text-sm text-red-400">{errors.form}</p> : null}
      <Button type="submit" className="w-full" disabled={pending}>
        {pending ? "送っています" : "返答を送る"}
      </Button>
    </form>
  );
}

function ClarificationForm({ request, token, onSent }: { request: CoachReviewRequest; token: string | null; onSent: (r: CoachReviewRequest) => void }) {
  const [text, setText] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const { pending, acquire, release } = useActionLock();

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!acquire()) return;
    setErrors({});
    try {
      onSent(
        await apiFetch<CoachReviewRequest>(`/v1/coaching/requests/${request.id}/clarification`, {
          method: "POST",
          token,
          body: JSON.stringify({ clarification: text }),
        }),
      );
    } catch (err) {
      setErrors(fieldErrors(err));
    } finally {
      release();
    }
  }

  return (
    <form onSubmit={onSubmit} className="space-y-3">
      <Field label="補足（1回だけ送れます）" value={text} onChange={setText} error={errors.clarification} />
      {errors.form ? <p className="text-sm text-red-400">{errors.form}</p> : null}
      <Button type="submit" className="w-full" disabled={pending}>
        {pending ? "送っています" : "補足を送る"}
      </Button>
    </form>
  );
}

export default function CoachReviewRequestPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const { id } = useParams<{ id: string }>();
  const [request, setRequest] = useState<CoachReviewRequest | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!loading && !session) {
      router.replace(`/login?next=${encodeURIComponent(window.location.pathname)}`);
    }
  }, [loading, session, router]);

  useEffect(() => {
    if (!token || !id) return;
    let cancelled = false;
    apiFetch<CoachReviewRequest>(`/v1/coach/requests/${id}`, { token })
      .then((d) => {
        if (!cancelled) setRequest(d);
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof ApiRequestError && err.status === 404 ? "依頼が見つかりません" : "読み込みに失敗しました");
      });
    return () => {
      cancelled = true;
    };
  }, [token, id]);

  if (error) return <p className="text-sm text-red-400">{error}</p>;
  if (loading || !session || !request) return <p className="text-sm text-slate-200">読み込み中...</p>;

  const response = request.response;
  const needsClarification = request.status === "clarifying" && response && !response.hidden;

  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-1">
        <h1 className="text-2xl font-bold">
          {coachEventLabels[request.event]} · {coachTierLabels[request.tier]}
        </h1>
        <p className="text-sm text-slate-200">見てほしい点: {request.focus.map((f) => reviewFocusLabels[f]).join("、")}</p>
        {request.status === "in_review" ? (
          <p className="text-sm text-slate-300">返却の目安: {formatDateTime(request.due_at)}まで</p>
        ) : null}
      </div>

      {request.video_url ? (
        <video
          src={request.video_url}
          poster={request.thumbnail_url || undefined}
          controls
          playsInline
          preload="metadata"
          className="w-full rounded-xl bg-black"
        />
      ) : null}

      {request.status === "in_review" ? <ResponseForm request={request} token={token} onSent={setRequest} /> : null}

      {response ? <ReviewResponseView response={response} /> : null}

      {needsClarification && response.clarify_reason ? (
        <section className="space-y-3 rounded-xl border border-amber-400/40 bg-amber-500/15 p-3">
          <p className="text-sm font-semibold text-white">「{clarifyReasonLabels[response.clarify_reason]}」と言われました</p>
          <ClarificationForm request={request} token={token} onSent={setRequest} />
        </section>
      ) : null}

      <Link href="/coach" className="block text-center text-sm text-blue-300 underline">
        コーチ画面へ
      </Link>
    </div>
  );
}
