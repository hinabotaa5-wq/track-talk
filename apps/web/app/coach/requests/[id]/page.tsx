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
import { coachingMessagingEnabled } from "@/lib/features";
import { athleteText, originalTextClass } from "@/lib/coaching-display";
import { formatDateTime } from "@/lib/format";
import { t, type Locale, L } from "@/lib/i18n";
import { useLocale } from "@/components/locale-provider";
import {
  maxResponseChars,
  maxReviewSeconds,
  reviewQuestionText,
  tooLong,
  videoDuration,
} from "@/lib/review";
import type { CoachReviewRequest } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useRef, useState, type FormEvent } from "react";

// 引き受けた依頼に返答を書くコーチの画面。

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
  const [drill, setDrill] = useState("");
  const [hitokoto, setHitokoto] = useState("");
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
    if (!guard.allow([good, fix1, drill, hitokoto].join("\n"))) return;
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
        body: JSON.stringify({
          good_point: good,
          fix_points: [fix1],
          drill,
          hitokoto,
          demo_video_public_id: demo,
        }),
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
      {/* 回答の型（本体 #1067）：良い点→改善点→おすすめのドリル→コーチなら明日から何をするか（hitokoto 列を流用）。長く書かせず、型で濃くする。 */}
      <Field label={t(locale, "coach.detail.good")} value={good} onChange={setGood} error={errors.good_point} placeholder={L("残すべき所を1つ")} />
      <Field label={L("改善点")} value={fix1} onChange={setFix1} error={errors.fix_points} placeholder={L("一番の課題を1つ。なぜ遅くなるかまで書く")} />
      <Field label={t(locale, "coach.detail.drill")} value={drill} onChange={setDrill} error={errors.drill} placeholder={L("ドリル名と回数（例：壁押しドリル 10秒×3本）")} />
      <Field label={L("コーチなら明日から何をするか")} value={hitokoto} onChange={setHitokoto} error={errors.hitokoto} placeholder={L("自分なら明日から何をするか、具体的に書く")} />
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

// 申込で選手が任意で書いた選手情報（本体 #1013 補足）。値のある項目だけを、相談内容と並べて出す。
function AthleteInfo({ request }: { request: CoachReviewRequest }) {
  const rows = [
    { label: L("PB（自己ベスト）"), value: request.pb },
    { label: L("目標タイム"), value: request.goal_time },
    { label: L("風速"), value: request.wind_speed },
    { label: L("その時の体のコンディション"), value: request.body_condition },
  ].filter((row): row is { label: string; value: string } => Boolean(row.value));
  if (rows.length === 0) return null;
  return (
    <dl aria-label={L("選手情報")} className="space-y-2">
      {rows.map((row) => (
        <div key={row.label} className="space-y-0.5">
          <dt className="text-xs font-semibold text-slate-300">{row.label}</dt>
          <dd className="whitespace-pre-wrap break-words text-sm">{row.value}</dd>
        </div>
      ))}
    </dl>
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
  // 直前に送った種類。完了バナーを画面上部に重ねて表示し、部品内のタイマーで3秒後に消す。
  const [sent, setSent] = useState<"response" | "info" | null>(null);

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
  // 海外コーチ向けに、選手の相談文の英訳があれば英訳を主に・原文を直下に併記する（本体 #916）。
  const question = request.question ? athleteText(request.question, request.question_en, { mine: false, locale }) : null;

  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-1">
        <h1 className="text-2xl font-bold">
          {coachEventLabels[request.event]} · {coachTierLabels[request.tier]}
        </h1>
        {request.status === "in_review" ? (
          <p className="text-sm text-slate-300">{t(locale, "coach.detail.due", { date: formatDateTime(request.due_at) })}</p>
        ) : null}
      </div>

      <section aria-label={L("依頼内容")} className="space-y-3 rounded-2xl border border-white/20 glass bg-white/5 p-4">
        <h2 className="text-sm font-semibold text-slate-300">{L("依頼内容")}</h2>
        {request.video_url ? (
          <video
            src={request.video_url}
            poster={request.thumbnail_url || undefined}
            controls
            playsInline
            preload="metadata"
            className="mx-auto w-full max-w-[260px] max-h-[300px] rounded-xl bg-black"
          />
        ) : null}
        <div className="space-y-1">
          <p className="whitespace-pre-wrap break-words text-sm">{question ? question.text : reviewQuestionText(request)}</p>
          {question?.original != null ? <p lang="ja" className={originalTextClass}>{question.original}</p> : null}
          <p className="text-xs text-slate-300"><time dateTime={request.created_at}>{formatDateTime(request.created_at)}</time></p>
        </div>
        <AthleteInfo request={request} />
      </section>

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

      {response ? <ReviewResponseView response={response} viewer="coach" /> : null}

      {/* 追加質問は販売を終了した（本体 #984）。販売終了前に購入された分だけ、受諾・回答のために出る。 */}
      {response && id ? <CoachingConsultations rootID={id} kind="review" token={token} /> : null}

      {request.work_order ? <WorkOrderView order={request.work_order} /> : null}
    </div>
  );
}
