"use client";

import { useSolicitationGuard } from "@/components/contact-notice";
import { WorkOrderView } from "@/components/work-order-terms";
import { PageLoading } from "@/components/page-loading";

import { useAuth } from "@/components/auth-provider";
import { CoachInfoRequestForm, ReviewInfoHistory, SentBanner } from "@/components/review-info-request";
import { ReviewResponseView } from "@/components/review-response";
import { CoachWithdrawForm } from "@/components/review-withdraw";
import { CoachingConsultations } from "@/components/coaching-consultations";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { UploadProgress } from "@/components/upload-progress";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { coachEventLabels, coachTierLabels } from "@/lib/coach-recruit";
import { eagerVideoQuery, uploadVideoToCloudinary, type CloudinarySignature, type CloudinaryUploadProgress } from "@/lib/cloudinary-upload";
import { coachingMessagingEnabled, messagingClosedNote } from "@/lib/features";
import { formatDateTime } from "@/lib/format";
import { t, type Locale, L } from "@/lib/i18n";
import { useLocale } from "@/components/locale-provider";
import {
  clarifyReasonLabels,
  maxResponseChars,
  maxReviewSeconds,
  reviewQuestionText,
  tooLong,
  videoDuration,
} from "@/lib/review";
import type { CoachReviewRequest } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useRef, useState, type FormEvent } from "react";

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

function fieldErrors(err: unknown, locale: Locale): Record<string, string> {
  if (err instanceof ApiRequestError) return err.fields ?? { form: err.message };
  return { form: err instanceof Error ? err.message : t(locale, "common.sendFailed") };
}

function ResponseForm({ request, token, onSent, locale }: { request: CoachReviewRequest; token: string | null; onSent: (r: CoachReviewRequest) => void; locale: Locale }) {
  const [good, setGood] = useState("");
  const guard = useSolicitationGuard();
  const [fix1, setFix1] = useState("");
  const [fix2, setFix2] = useState("");
  const [drill, setDrill] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const uploaded = useRef<{ file: File; id: string } | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [upload, setUpload] = useState<CloudinaryUploadProgress | null>(null);
  const { pending, acquire, release } = useActionLock();

  async function onPickFile(picked: File | null) {
    setErrors({});
    setFile(picked);
    if (!picked) return;
    if (picked.size > maxBytes) {
      setErrors({ demo_video_public_id: t(locale, "coach.detail.demoTooBig") });
      setFile(null);
    } else if (tooLong(await videoDuration(picked))) {
      setErrors({ demo_video_public_id: t(locale, "coach.detail.demoTooLong", { seconds: maxReviewSeconds }) });
      setFile(null);
    }
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!guard.allow([good, fix1, fix2, drill].join("\n"))) return;
    if (!acquire()) return;
    setErrors({});
    try {
      let demo = "";
      if (file && uploaded.current?.file === file) {
        // 返答がエラーになって直して送り直すときは、上げ済みの動画を使い回す（同じ動画を上げ直して孤児を増やさない）。
        demo = uploaded.current.id;
      } else if (file) {
        const signed = await apiFetch<CloudinarySignature>(`/v1/uploads/coaching-video${eagerVideoQuery}`, { method: "POST", token });
        setUpload({ loaded: 0, total: file.size, percent: 0 });
        const payload = await uploadVideoToCloudinary(file, signed, token, setUpload);
        if (payload.public_id !== signed.public_id) throw new Error(L("アップロードに失敗しました"));
        demo = payload.public_id;
        uploaded.current = { file, id: demo };
      }
      const sent = await apiFetch<CoachReviewRequest>(`/v1/coaching/requests/${request.id}/response`, {
        method: "POST",
        token,
        body: JSON.stringify({ good_point: good, fix_points: [fix1, fix2], drill, demo_video_public_id: demo }),
      });
      onSent(sent);
    } catch (err) {
      setErrors(fieldErrors(err, locale));
    } finally {
      setUpload(null);
      release();
    }
  }

  return (
    <form onSubmit={onSubmit} className="space-y-4">
      <h2 className="font-bold">{t(locale, "coach.detail.responseTitle")}</h2>
      <p className="text-xs text-slate-300">
        {t(locale, "coach.detail.contactNote")}
      </p>
      {guard.warning}
      <Field label={t(locale, "coach.detail.good")} value={good} onChange={setGood} error={errors.good_point} />
      <Field label={t(locale, "coach.detail.fix1")} value={fix1} onChange={setFix1} error={errors.fix_points} />
      <Field label={t(locale, "coach.detail.fix2")} value={fix2} onChange={setFix2} />
      <Field label={t(locale, "coach.detail.drill")} value={drill} onChange={setDrill} error={errors.drill} />
      <label className="block space-y-1 text-sm">
        <span className="font-semibold">{t(locale, "coach.detail.demoVideo", { seconds: maxReviewSeconds })}</span>
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
        {pending ? t(locale, "common.sending") : t(locale, "coach.detail.sendResponse")}
      </Button>
    </form>
  );
}

function ClarificationForm({ request, token, onSent, locale }: { request: CoachReviewRequest; token: string | null; onSent: (r: CoachReviewRequest) => void; locale: Locale }) {
  const [text, setText] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const { pending, acquire, release } = useActionLock();
  const guard = useSolicitationGuard();

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!guard.allow(text)) return;
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
      setErrors(fieldErrors(err, locale));
    } finally {
      release();
    }
  }

  return (
    <form onSubmit={onSubmit} className="space-y-3">
      <p className="text-xs text-slate-300">{t(locale, "coach.detail.contactNote")}</p>
      {guard.warning}
      <Field label={t(locale, "coach.detail.followup")} value={text} onChange={setText} error={errors.clarification} />
      {errors.form ? <p className="text-sm text-red-400">{errors.form}</p> : null}
      <Button type="submit" className="w-full" disabled={pending}>
        {pending ? t(locale, "common.sending") : t(locale, "coach.detail.sendFollowup")}
      </Button>
    </form>
  );
}

export default function CoachReviewRequestPage() {
  const { session, token, loading } = useAuth();
  const locale = useLocale();
  const router = useRouter();
  const { id } = useParams<{ id: string }>();
  const [request, setRequest] = useState<CoachReviewRequest | null>(null);
  const [error, setError] = useState("");
  const [startError, setStartError] = useState("");
  const { pending: starting, acquire: acquireStart, release: releaseStart } = useActionLock();
  // 直前に送った種類。送った内容の先頭に完了バナーを出す（自動では消さない）。
  const [sent, setSent] = useState<"response" | "clarification" | "info" | null>(null);

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
        if (!cancelled) setError(err instanceof ApiRequestError && err.status === 404 ? t(locale, "coach.detail.notFound") : t(locale, "coach.detail.loadError"));
      });
    return () => {
      cancelled = true;
    };
  }, [token, id, locale]);

  async function start() {
    if (!acquireStart()) return;
    setStartError("");
    try {
      setRequest(await apiFetch<CoachReviewRequest>(`/v1/coach/requests/${id}/start`, { method: "POST", token }));
    } catch (err) {
      setStartError(err instanceof ApiRequestError ? err.message : L("添削を開始できませんでした"));
    } finally {
      releaseStart();
    }
  }

  if (error) return <p className="text-sm text-red-400">{error}</p>;
  if (loading || !session || !request) return <PageLoading screen label={t(locale, "common.loading")} />;

  const response = request.response;
  const needsClarification = request.status === "clarifying" && response && !response.hidden;

  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-1">
        <h1 className="text-2xl font-bold">
          {coachEventLabels[request.event]} · {coachTierLabels[request.tier]}
        </h1>
        <p className="whitespace-pre-wrap break-words text-sm text-slate-200">{t(locale, "coach.detail.question", { text: reviewQuestionText(request) })}</p>
        {request.status === "in_review" ? (
          <p className="text-sm text-slate-300">{t(locale, "coach.detail.due", { date: formatDateTime(request.due_at) })}</p>
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

      {request.work_order ? <WorkOrderView order={request.work_order} directReview /> : null}

      {sent === "info" && request.info_request ? <SentBanner text={t(locale, "coach.detail.infoSent")} /> : null}

      {request.info_request ? <ReviewInfoHistory info={request.info_request} viewer="coach" /> : null}

      {request.status === "in_review" && !request.started_at ? (
        <div className="space-y-2">
          <Button type="button" className="w-full" disabled={starting} onClick={() => void start()}>{L("添削をはじめる")}</Button>
          {startError ? <p role="alert" className="text-sm text-red-400">{startError}</p> : null}
        </div>
      ) : null}

      {request.status === "in_review" && request.started_at ? <ResponseForm request={request} token={token} onSent={(r) => { setSent("response"); setRequest(r); }} locale={locale} /> : null}

      {request.status === "in_review" && !request.info_request && coachingMessagingEnabled ? (
        <CoachInfoRequestForm requestID={request.id} token={token} onSent={(r) => { setSent("info"); setRequest(r); }} />
      ) : null}

      {request.status === "in_review" || (request.status === "needs_info" && request.info_request?.from_status === "in_review") ? (
        <CoachWithdrawForm requestID={request.id} token={token} onWithdrawn={() => router.push("/coach/reviewing")} />
      ) : null}

      {sent === "response" && response ? <SentBanner text={t(locale, "coach.detail.responseSent")} /> : null}

      {response ? <ReviewResponseView response={response} /> : null}

      {response && id ? <CoachingConsultations rootID={id} kind="review" token={token} /> : null}

      {needsClarification && response.clarify_reason ? (
        <section className="space-y-3 rounded-xl border border-amber-400/40 bg-amber-500/15 p-3">
          {sent === "clarification" ? <SentBanner text={t(locale, "coach.detail.clarificationSent")} /> : null}
          <p className="text-sm font-semibold text-white">{t(locale, "coach.detail.clarifiedAs", { reason: clarifyReasonLabels[response.clarify_reason] })}</p>
          {response.clarify_body ? <p className="whitespace-pre-wrap text-sm">{response.clarify_body}</p> : null}
          {coachingMessagingEnabled ? <ClarificationForm request={request} token={token} onSent={(r) => { setSent("clarification"); setRequest(r); }} locale={locale} /> : <p className="text-sm text-slate-300">{L(messagingClosedNote)}</p>}
        </section>
      ) : null}

      <Link href="/coach" className="block text-center text-sm text-blue-300 underline">
        {t(locale, "coach.detail.toCoach")}
      </Link>
    </div>
  );
}
