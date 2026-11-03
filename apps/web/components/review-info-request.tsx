"use client";

import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { UploadProgress } from "@/components/upload-progress";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { uploadToCloudinary, type CloudinarySignature, type CloudinaryUploadProgress } from "@/lib/cloudinary-upload";
import { formatDateTime } from "@/lib/format";
import { maxReviewSeconds, minInfoRequestChars, reviewInfoReasonLabels, tooLong, videoDuration } from "@/lib/review";
import type { CoachReviewRequest, ReviewInfoReason, ReviewInfoRequest, ReviewRequest } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import { useState, type FormEvent } from "react";

// 返却前の「内容確認のための差し戻し」。コーチが1依頼につき1回だけ、質問を付けて依頼を返し、
// 依頼者は24時間以内に相談内容を直して（動画も差し替えられる）再提出する。

const maxBytes = 100 * 1024 * 1024;

// 空白と記号を除いた文字数（サーバーと同じ数え方）。
function meaningfulLength(s: string) {
  return (s.match(/[\p{L}\p{N}]/gu) ?? []).length;
}

// コーチが差し戻すフォーム。引き受ける前（打診中）でも、引き受けたあと（添削中）でも使える。
export function CoachInfoRequestForm({
  requestID,
  token,
  onSent,
}: {
  requestID: string;
  token: string | null;
  onSent: (request: CoachReviewRequest) => void;
}) {
  const [body, setBody] = useState("");
  const [reason, setReason] = useState<ReviewInfoReason | "">("");
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (meaningfulLength(body) < minInfoRequestChars) {
      setError(`どこが・何が分からないかを具体的に書いてください（${minInfoRequestChars}文字以上）`);
      return;
    }
    if (!acquire()) return;
    setError("");
    try {
      onSent(
        await apiFetch<CoachReviewRequest>(`/v1/coach/requests/${requestID}/info-request`, {
          method: "POST",
          token,
          body: JSON.stringify({ body: body.trim(), reason: reason || null }),
        }),
      );
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.fields?.body || err.fields?.reason || err.message : "送信に失敗しました");
    } finally {
      release();
    }
  }

  return (
    <details className="rounded-xl border border-white/20 glass bg-white/10 p-3 text-sm">
      <summary className="cursor-pointer font-semibold">内容確認のため差し戻す（1回だけ）</summary>
      <form onSubmit={onSubmit} className="mt-3 space-y-3">
        <p className="text-xs text-slate-300">
          依頼の内容が分からないときに、依頼者に質問して直してもらえます。差し戻しは1つの依頼に1回だけです。依頼者は24時間以内に再提出し、再提出から24時間が新しい返却期限になります。再提出がなければ依頼は終了します。
        </p>
        <label className="block space-y-1">
          <span className="text-xs text-slate-300">分からない点の種類（任意）</span>
          <select
            value={reason}
            onChange={(e) => setReason(e.target.value as ReviewInfoReason | "")}
            disabled={pending}
            className="w-full rounded-md border border-white/25 bg-white/10 px-3 py-2 text-base text-slate-100"
          >
            <option value="">選ばない</option>
            {(Object.keys(reviewInfoReasonLabels) as ReviewInfoReason[]).map((r) => (
              <option key={r} value={r}>
                {reviewInfoReasonLabels[r]}
              </option>
            ))}
          </select>
        </label>
        <label className="block space-y-1">
          <span className="text-xs text-slate-300">どこが・何が分からないか（必須）</span>
          <Textarea
            value={body}
            onChange={(e) => setBody(e.target.value)}
            rows={4}
            maxLength={1000}
            disabled={pending}
            placeholder="例：動画のスタートと後半のどちらを見てほしいのかが分かりません。気になっている区間を教えてください。"
          />
        </label>
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
        <Button type="submit" className="w-full" disabled={pending}>
          {pending ? "送っています" : "質問を付けて差し戻す"}
        </Button>
      </form>
    </details>
  );
}

// 差し戻しの履歴。コーチの質問と、再提出したら直す前の相談内容を出す。
export function ReviewInfoHistory({ info, viewer }: { info: ReviewInfoRequest; viewer: "coach" | "requester" }) {
  const waiting = !info.resubmitted_at;
  return (
    <section className="space-y-2 rounded-xl border border-amber-400/40 bg-amber-500/15 p-3 text-sm">
      <p className="font-semibold text-white">
        {waiting ? (viewer === "coach" ? "差し戻し中（依頼者の再提出を待っています）" : "コーチから依頼内容の確認が届いています") : "内容確認の差し戻し（再提出済み）"}
      </p>
      {info.reason ? <p className="text-xs text-slate-200">種類：{reviewInfoReasonLabels[info.reason]}</p> : null}
      <p className="whitespace-pre-wrap break-words">コーチの質問：{info.body}</p>
      {waiting ? (
        <p className="text-xs text-slate-200">再提出の期限：{formatDateTime(info.resubmit_by)}（過ぎると依頼は終了し、お支払いは取り消し・返金されます）</p>
      ) : (
        <>
          {info.original_question ? <p className="whitespace-pre-wrap break-words text-xs text-slate-300">直す前の相談内容：{info.original_question}</p> : null}
          <p className="text-xs text-slate-300">
            再提出：{info.resubmitted_at ? formatDateTime(info.resubmitted_at) : ""}
            {info.video_replaced ? "（動画を差し替えました）" : ""}
          </p>
        </>
      )}
    </section>
  );
}

// 依頼者が相談内容を直して再提出するフォーム。動画の差し替えは任意。
export function ResubmitForm({ request, token, onChange }: { request: ReviewRequest; token: string | null; onChange: (r: ReviewRequest) => void }) {
  const [question, setQuestion] = useState(request.question ?? "");
  const [file, setFile] = useState<File | null>(null);
  const [error, setError] = useState("");
  const [upload, setUpload] = useState<CloudinaryUploadProgress | null>(null);
  const [phase, setPhase] = useState<"upload" | "saving">("upload");
  const { pending, acquire, release } = useActionLock();

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
    if (!question.trim()) {
      setError("相談内容を記入してください");
      return;
    }
    if (file && file.size > maxBytes) {
      setError("100MB以内の動画にしてください");
      return;
    }
    if (!acquire()) return;
    setError("");
    try {
      let video = "";
      if (file) {
        const signed = await apiFetch<CloudinarySignature>("/v1/uploads/coaching-video", { method: "POST", token });
        setPhase("upload");
        setUpload({ loaded: 0, total: file.size, percent: 0 });
        const payload = await uploadToCloudinary("video", file, signed, setUpload);
        if (payload.public_id !== signed.public_id) throw new Error("アップロードに失敗しました");
        video = payload.public_id;
        setPhase("saving");
      }
      onChange(
        await apiFetch<ReviewRequest>(`/v1/coaching/requests/${request.id}/resubmit`, {
          method: "POST",
          token,
          body: JSON.stringify({ question: question.trim(), video_public_id: video }),
        }),
      );
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.fields?.question || err.fields?.video_public_id || err.message : err instanceof Error ? err.message : "送信に失敗しました");
    } finally {
      setUpload(null);
      release();
    }
  }

  return (
    <form onSubmit={onSubmit} className="space-y-3 rounded-xl border border-white/20 glass bg-white/10 p-3 text-sm">
      <h2 className="font-semibold">相談内容を直して再提出する</h2>
      <p className="text-xs text-slate-300">コーチの質問に答えるように相談内容を直してください。再提出すると、返却期限は再提出から24時間後になります。</p>
      <label className="block space-y-1">
        <span className="text-xs text-slate-300">相談内容</span>
        <Textarea value={question} onChange={(e) => setQuestion(e.target.value)} rows={5} maxLength={1000} required disabled={pending} />
        <span className="block text-xs text-slate-300">{question.length}/1000文字</span>
      </label>
      <label className="block space-y-1">
        <span className="text-xs text-slate-300">動画を差し替える（任意・{maxReviewSeconds}秒まで）</span>
        <input
          className="block w-full text-sm text-slate-300 file:mr-3 file:rounded-full file:border-0 file:bg-white/15 file:px-3 file:py-2 file:text-sm file:font-semibold file:text-slate-100"
          type="file"
          accept="video/mp4,video/quicktime,video/webm"
          onChange={(e) => void onPickFile(e.target.files?.[0] ?? null)}
          disabled={pending}
        />
      </label>
      {upload ? <UploadProgress noun="動画" phase={phase} progress={upload} fileName={file?.name} fileSize={file?.size} /> : null}
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
      <Button type="submit" className="w-full" disabled={pending}>
        {pending ? "送っています" : "再提出する"}
      </Button>
    </form>
  );
}
